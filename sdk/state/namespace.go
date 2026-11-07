package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
)

func namespacePath(dir, namespace string) (string, error) {
	if namespace == "" || namespace == "." || strings.HasPrefix(namespace, ".") || filepath.Base(namespace) != namespace || strings.ContainsAny(namespace, `/\\`) {
		return "", errors.New("invalid state namespace")
	}
	path := filepath.Join(dir, namespace)
	if info, err := os.Lstat(path); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return "", errors.New("state namespace is not a directory")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

// ReadNamespace observes one checkpoint without owning the process lock. It is
// suitable for status output, never recovery or admission decisions.
func ReadNamespace(dir, namespace, key string, out any) (bool, error) {
	path, err := namespacePath(dir, namespace)
	if err != nil {
		return false, err
	}
	return Read(path, key, out)
}

func (s *Store) LoadNamespace(namespace, key string, out any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return false, errors.New("state store closed")
	}
	return ReadNamespace(s.dir, namespace, key, out)
}

func (s *Store) SaveNamespace(namespace, key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return errors.New("state store closed")
	}
	path, err := namespacePath(s.dir, namespace)
	if err != nil {
		return err
	}
	return save(path, key, value)
}

func (s *Store) DeleteNamespace(namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return errors.New("state store closed")
	}
	path, err := namespacePath(s.dir, namespace)
	if err != nil {
		return err
	}
	if err = os.Remove(filename(path, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("delete checkpoint: %w", err)
	}
	return syncDirectory(path)
}

// ListNamespace decodes each active JSON file once, rejecting unreadable or
// malformed checkpoints instead of treating them as absent ownership.
func ListNamespace(dir, namespace string) ([]json.RawMessage, error) {
	path, err := namespacePath(dir, namespace)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Errorf("list checkpoints: %w", err)
	}
	rows := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pending-") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, errors.New("unexpected checkpoint entry")
		}
		raw, err := os.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, errors.Errorf("read checkpoint: %w", err)
		}
		if !json.Valid(raw) {
			return nil, errors.New("invalid checkpoint JSON")
		}
		rows = append(rows, json.RawMessage(raw))
	}
	return rows, nil
}

func (s *Store) ListNamespace(namespace string) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil, errors.New("state store closed")
	}
	return ListNamespace(s.dir, namespace)
}

// ActivateNamespace leaves original records untouched. All staged files and the
// staging directory are synced before a single rename makes the import visible.
// Any error, including a parent fsync after rename, requires callers to reload.
func (s *Store) ActivateNamespace(namespace string, values map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return errors.New("state store closed")
	}
	path, err := namespacePath(s.dir, namespace)
	if err != nil {
		return err
	}
	if _, err = os.Stat(path); err == nil {
		return errors.New("state namespace already active")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	staging, err := os.MkdirTemp(s.dir, ".import-")
	if err != nil {
		return errors.Errorf("stage namespace: %w", err)
	}
	defer os.RemoveAll(staging)
	for key, value := range values {
		if err = save(staging, key, value); err != nil {
			return err
		}
	}
	if err = syncDirectory(staging); err != nil {
		return err
	}
	if err = os.Rename(staging, path); err != nil {
		return errors.Errorf("activate namespace: %w", err)
	}
	return syncDirectory(s.dir)
}
