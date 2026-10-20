package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuerArtifactsMatchSourcePins(t *testing.T) {
	dir := filepath.Join("testdata", "issuers")
	source, err := os.ReadFile(filepath.Join(dir, "Fixture.sol"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(source)
	for _, name := range []string{"FixtureShare", "FixtureAsyncIssuer"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var artifact struct {
				Source, SourceSha256, Compiler string
			}
			if err := json.Unmarshal(raw, &artifact); err != nil {
				t.Fatal(err)
			}
			if artifact.Source != "Fixture.sol" || artifact.SourceSha256 != hex.EncodeToString(digest[:]) || !strings.HasPrefix(artifact.Compiler, "0.8.37+") {
				t.Fatal("issuer artifact differs from its source/compiler pin")
			}
			if artifact := loadIssuerArtifact(t, name); len(artifact.Bytecode) == 0 || len(artifact.ABI.Methods) == 0 {
				t.Fatal("issuer artifact lacks executable ABI or bytecode")
			}
		})
	}
}

func TestNoonArtifactsMatchSourcePins(t *testing.T) {
	var provenance struct {
		Compiler      string
		Sources       map[string]string
		Artifacts     map[string]string
		FixtureSha256 string
	}
	read := func(name string, target any) []byte {
		t.Helper()
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if target != nil {
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatal(err)
			}
		}
		return data
	}
	dir := filepath.Join("testdata", "issuers", "noon")
	read(filepath.Join(dir, "provenance.json"), &provenance)
	if provenance.Compiler != "v0.8.28+commit.7893614a" || len(provenance.Sources) != 30 || len(provenance.Artifacts) != 3 {
		t.Fatal("verified source closure/compiler pin missing")
	}
	var sources map[string]struct{ Content string }
	read(filepath.Join(dir, "sources.json"), &sources)
	check := func(data []byte, expected string) {
		t.Helper()
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != expected {
			t.Fatal("source or artifact differs from verified provenance")
		}
	}
	if len(sources) != len(provenance.Sources) {
		t.Fatal("source bundle contains an unpinned source")
	}
	for name, expected := range provenance.Sources {
		check([]byte(sources[name].Content), expected)
	}
	for name, expected := range provenance.Artifacts {
		check(read(filepath.Join(dir, name), nil), expected)
	}
	check(read(filepath.Join(dir, "Fixture.sol"), nil), provenance.FixtureSha256)
}
