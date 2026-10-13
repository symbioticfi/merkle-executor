// Package state provides an exclusively owned, atomically replaced JSON journal.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/go-errors/errors"
)

// Store serializes local access; the flock excludes other processes sharing dir.
// Close must run only after users of the store have stopped.
type Store struct {
	mu   sync.Mutex
	dir  string
	lock *os.File
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("state directory is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.Errorf("create state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.Errorf("open state lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, errors.Errorf("state directory already owned or cannot lock: %w", err)
	}
	return &Store{dir: dir, lock: lock}, nil
}

func (s *Store) filename(key string) string {
	return filename(s.dir, key)
}

func filename(dir, key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(hash[:])+".json")
}

// Read observes an atomic file snapshot without acquiring the writer's process
// lock. It is intended only for status output, never admission or state changes.
func Read(dir, key string, out any) (bool, error) {
	data, err := os.ReadFile(filename(dir, key))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Errorf("read state snapshot: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, errors.Errorf("decode state snapshot: %w", err)
	}
	return true, nil
}

func (s *Store) Load(key string, out any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return false, errors.New("state store closed")
	}
	data, err := os.ReadFile(s.filename(key))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, errors.Errorf("decode state: %w", err)
	}
	return true, nil
}

// Save syncs the replacement before rename, then syncs the containing directory.
// An error after rename is an uncertain write: callers must fail closed/reload.
func (s *Store) Save(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return errors.New("state store closed")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return errors.Errorf("encode state: %w", err)
	}
	f, err := os.CreateTemp(s.dir, ".pending-*")
	if err != nil {
		return errors.Errorf("create state replacement: %w", err)
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err := f.Write(data); err != nil {
		return errors.Errorf("write state: %w", err)
	}
	if err := f.Sync(); err != nil {
		return errors.Errorf("sync state: %w", err)
	}
	if err := f.Close(); err != nil {
		return errors.Errorf("close state replacement: %w", err)
	}
	if err := os.Rename(f.Name(), s.filename(key)); err != nil {
		return errors.Errorf("replace state: %w", err)
	}
	return s.syncDirectory()
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return errors.New("state store closed")
	}
	if err := os.Remove(s.filename(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("delete state: %w", err)
	}
	return s.syncDirectory()
}

func (s *Store) syncDirectory() error {
	dir, err := os.Open(s.dir)
	if err != nil {
		return errors.Errorf("open state directory for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return errors.Errorf("sync state directory: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	f := s.lock
	s.lock = nil
	unlockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	closeErr := f.Close()
	return errors.Join(unlockErr, closeErr)
}
