package config

import (
	"context"
	"os"
	"strings"
	"testing"

	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/data"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"gopkg.in/yaml.v3"
)

func TestProviderAliasesCannotBypassDurableFingerprint(t *testing.T) {
	path := writeConfig(t, validConfig)
	r := registry.NewRegistry()
	if err := data.Register(r); err != nil {
		t.Fatal(err)
	}
	var fingerprints, values []string
	for _, inputs := range []string{"{0: &x '1', 1: '2', 2: *x}", "{0: '1', 1: &x '2', 2: *x}"} {
		raw := strings.Replace(validConfig, "{0: '42'}", inputs, 1)
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			if !strings.Contains(err.Error(), "aliases") {
				t.Fatal(err)
			}
			continue
		}
		pr, err := r.New("static", cfg.Strategies[0].Providers[0].Config, registry.Deps{})
		if err != nil {
			t.Fatal(err)
		}
		out, err := pr.Prepare(context.Background(), provider.Snapshot{})
		if err != nil {
			t.Fatal(err)
		}
		fingerprints = append(fingerprints, cfg.Strategies[0].Fingerprint())
		values = append(values, out.Inputs[2])
	}
	if len(fingerprints) > 0 {
		t.Fatalf("aliases accepted: fingerprints=%v effective input 2=%v", fingerprints, values)
	}
}
func TestCuratorMigrationChangesHistoricalFingerprint(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(validConfig), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Strategies[0].Fingerprint(); got == "0xd18ac7f2923b2b85175af6b900c355eef2cfcf2d3cd745d0a1ca02e41bdb5270" {
		t.Fatalf("legacy fingerprint silently retained: %s", got)
	}
}
