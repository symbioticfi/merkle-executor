package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestProviderAliasesCannotBypassDurableFingerprint(t *testing.T) {
	for _, inputs := range []string{"{0: &x '1', 1: '2', 2: *x}", "{0: '1', 1: &x '2', 2: *x}"} {
		raw := validConfig + "    providers:\n      - name: static\n        config:\n          inputs: " + inputs + "\n"
		if _, err := Load(writeConfig(t, raw)); err == nil {
			t.Fatal("obsolete aliased provider accepted")
		}
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
