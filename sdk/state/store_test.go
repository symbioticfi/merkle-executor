package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStorePersistsAtomicReplacementAndDeleteAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("../intent", map[string]int{"nonce": 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("../intent", map[string]int{"nonce": 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("../intent", make(chan int)); err == nil {
		t.Fatal("invalid JSON replaced record")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var got map[string]int
	found, err := s.Load("../intent", &got)
	if err != nil || !found || got["nonce"] != 4 {
		t.Fatalf("persisted value: %v %v %v", found, got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, _ := e.Info()
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("record permissions: %s", e.Name())
		}
	}
	if err := s.Delete("../intent"); err != nil {
		t.Fatal(err)
	}
	if found, err := s.Load("../intent", &got); err != nil || found {
		t.Fatalf("deleted record: %v %v", found, err)
	}
	if err := s.Delete("missing"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreLockAndClosedUse(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir); err == nil {
		other.Close()
		t.Fatal("second process owner accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("x", 1); err == nil {
		t.Fatal("closed store accepted write")
	}
	if _, err := s.Load("x", new(int)); err == nil {
		t.Fatal("closed store accepted read")
	}
	if err := s.Delete("x"); err == nil {
		t.Fatal("closed store accepted delete")
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
}

func TestStoreSurvivesUncleanProcessExit(t *testing.T) {
	const childDirectory = "MERKLE_STATE_CHILD_DIRECTORY"
	if dir := os.Getenv(childDirectory); dir != "" {
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save("signed", map[string]string{"hash": "durable"}); err != nil {
			t.Fatal(err)
		}
		// Skip Close and all deferred cleanup, as a killed keeper would.
		os.Exit(0)
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestStoreSurvivesUncleanProcessExit$")
	cmd.Env = []string{childDirectory + "=" + dir}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("writer process: %v %s", err, output)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var saved map[string]string
	if found, err := s.Load("signed", &saved); err != nil || !found || saved["hash"] != "durable" {
		t.Fatalf("process restart lost journal: %v %v %v", found, saved, err)
	}
}

func TestCorruptRecordIsNotTreatedAsMissing(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := os.WriteFile(s.filename("signed"), []byte(`{"hash":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("signed", new(map[string]string)); err == nil {
		t.Fatal("corrupt journal was ignored")
	}
}
