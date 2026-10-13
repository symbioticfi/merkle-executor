package config

import (
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestProviderAliasesCannotBypassDurableFingerprint(t *testing.T) {
	for _, inputs := range []string{"{0: &x '1', 1: '2', 2: *x}", "{0: '1', 1: &x '2', 2: *x}"} {
		raw := strategyConfig("  - executor: '0x0000000000000000000000000000000000000002'\n    providers:\n      - name: static\n        config:\n          inputs: " + inputs + "\n")
		if _, err := Load(writeConfig(t, raw)); err == nil {
			t.Fatal("obsolete aliased provider accepted")
		}
	}
}
func TestAddressOnlyPolicyChangesHistoricalFingerprint(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.StrategyFingerprint(cfg.Strategies[0], common.HexToAddress("0x03"), common.HexToAddress("0x01")); got == "0xe13fbcba55240945069ca4f178b0ab14e0de1c143f2d3fd56bc2e4970476089e" {
		t.Fatal("legacy label policy retained")
	}
}
