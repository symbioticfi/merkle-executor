package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKIndependentModule(t *testing.T) {
	root := filepath.Join("..", "..", "sdk")
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("independently versioned SDK missing: %v", err)
	}
	if !strings.Contains(string(data), "module github.com/symbioticfi/merkle-executor/sdk\n") {
		t.Fatal("SDK has wrong module identity")
	}
	allowed := map[string]bool{"architecture": true, "config": true, "observability": true, "version": true}
	err = filepath.WalkDir(filepath.Join("..", "..", "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(filepath.Join("..", "..", "internal"), path)
		if err != nil {
			return err
		}
		if !allowed[strings.Split(relative, string(filepath.Separator))[0]] {
			t.Errorf("reusable component belongs in the SDK: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
