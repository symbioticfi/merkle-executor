package vaultsolver_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEngineMatchesPinnedVaultSolverSource(t *testing.T) {
	raw, err := os.ReadFile("source.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Commit string
		Files  map[string]struct {
			SHA256 string `json:"sha256"`
			Local  string `json:"local"`
		}
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 33 || len(manifest.Commit) != 40 {
		t.Fatal("incomplete engine provenance")
	}
	for upstream, file := range manifest.Files {
		t.Run(upstream, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../..", file.Local))
			if err != nil {
				t.Fatal(err)
			}
			restored := strings.NewReplacer(
				"github.com/symbioticfi/merkle-executor/internal/vaultsolver/telemetry", "github.com/symbioticfi/vault-solver/internal/observability",
				"github.com/symbioticfi/merkle-executor/internal/signer", "github.com/symbioticfi/vault-solver/internal/signer",
				"github.com/symbioticfi/merkle-executor/internal/vaultsolver/txmanager", "github.com/symbioticfi/vault-solver/internal/txmanager",
			).Replace(string(data))
			canonical, err := format.Source([]byte(restored))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(canonical)
			if hex.EncodeToString(sum[:]) != file.SHA256 {
				t.Fatal("engine differs from its pinned upstream source; refresh provenance with any upstream update")
			}
		})
	}
}
