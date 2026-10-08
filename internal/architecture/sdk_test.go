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
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, forbidden := range []string{"github.com/symbioticfi/merkle-executor/internal/", "github.com/symbioticfi/merkle-executor/api/", "github.com/symbioticfi/vault-solver/"} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("SDK imports host code: %s: %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
