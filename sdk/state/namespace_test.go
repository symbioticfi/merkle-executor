package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNamespaceActivationIsAtomicAndPreservesLegacy(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Save("legacy", map[string]int{"version": 1}); err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateNamespace("v2", map[string]any{"first": map[string]int{"version": 2}, "invalid": make(chan int)}); err == nil {
		t.Fatal("partial import accepted")
	}
	if _, err = os.Stat(filepath.Join(s.dir, "v2")); !os.IsNotExist(err) {
		t.Fatal("failed import activated", err)
	}
	if err = s.ActivateNamespace("v2", map[string]any{"first": map[string]int{"version": 2}}); err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateNamespace("v2", map[string]any{}); err == nil {
		t.Fatal("active namespace replaced")
	}
	var got map[string]int
	if found, err := s.LoadNamespace("v2", "first", &got); err != nil || !found || got["version"] != 2 {
		t.Fatal(found, got, err)
	}
	if found, err := s.Load("legacy", &got); err != nil || !found || got["version"] != 1 {
		t.Fatal("legacy changed", found, got, err)
	}
	rows, err := ListNamespace(s.dir, "v2")
	if err != nil || len(rows) != 1 || !json.Valid(rows[0]) {
		t.Fatal(rows, err)
	}
	if err = s.SaveNamespace("../escape", "bad", 1); err == nil {
		t.Fatal("escaped namespace")
	}
	if err = s.DeleteNamespace("v2", "first"); err != nil {
		t.Fatal(err)
	}
	if rows, err = s.ListNamespace("v2"); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}

func TestNamespaceOperationsRetainRootLockAndRejectCorruption(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ActivateNamespace("v2", map[string]any{"one": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(s.dir); err == nil {
		t.Fatal("namespace released root lock")
	}
	if err = os.WriteFile(filename(filepath.Join(s.dir, "v2"), "one"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListNamespace("v2"); err == nil {
		t.Fatal("corrupt checkpoint ignored")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveNamespace("v2", "one", 1); err == nil {
		t.Fatal("closed namespace written")
	}
}
