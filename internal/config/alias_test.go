package config

import "testing"

func TestProviderAliasesCannotBypassDurableFingerprint(t *testing.T) {
	for _, inputs := range []string{"{0: &x '1', 1: '2', 2: *x}", "{0: '1', 1: &x '2', 2: *x}"} {
		raw := strategyConfig("  - executor: '0x0000000000000000000000000000000000000002'\n    providers:\n      - name: static\n        config:\n          inputs: " + inputs + "\n")
		if _, err := Load(writeConfig(t, raw)); err == nil {
			t.Fatal("obsolete aliased provider accepted")
		}
	}
}
