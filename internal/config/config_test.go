package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const validConfig = `curator:
  url: http://127.0.0.1:1/api/v1
chain:
  chainId: 1
  rpcUrl: http://127.0.0.1:8545
signer:
  address: '0x0000000000000000000000000000000000000001'
strategies:
  - name: example
    executor: '0x0000000000000000000000000000000000000002'
    providers:
      - name: static
        config:
          inputs: {0: '42'}
`

func writeConfig(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestConfigurationDefaultsAndPaths(t *testing.T) {
	t.Parallel()
	p := writeConfig(t, validConfig)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "simulate" || c.Signer.KeyEnv != "" || c.StateDir != filepath.Join(filepath.Dir(p), "state") {
		t.Fatal(c)
	}
}
func TestRejectMalformedConfiguration(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"unknown":           validConfig + "typo: true\n",
		"multiple":          validConfig + "---\nmode: send\n",
		"zero caller":       strings.Replace(validConfig, "0000000000000000000000000000000000000001", "0000000000000000000000000000000000000000", 1),
		"send without fee":  validConfig + "mode: send\n",
		"negative interval": validConfig + "pollInterval: -1s\n",
		"missing rpc":       strings.Replace(validConfig, "http://127.0.0.1:8545", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, text)); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}
func TestFingerprintIgnoresCommentsAndPosition(t *testing.T) {
	t.Parallel()
	var a, b Config
	if err := yaml.Unmarshal([]byte(validConfig), &a); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte("# documentation\n\n"+validConfig), &b); err != nil {
		t.Fatal(err)
	}
	if a.Strategies[0].Fingerprint() != b.Strategies[0].Fingerprint() {
		t.Fatal("comments changed identity")
	}
	b.Strategies[0].Executor = "different"
	if a.Strategies[0].Fingerprint() == b.Strategies[0].Fingerprint() {
		t.Fatal("account did not change identity")
	}
}

func TestTransactionHorizonDefaultsAndInvalidLimits(t *testing.T) {
	t.Parallel()
	defaults, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.TxManager.Horizon.MaxBlocks != 6 || defaults.TxManager.Horizon.BlockTimeMs != 12000 || defaults.TxManager.PendingTimeoutMs != 300000 {
		t.Fatal("unexpected transaction defaults")
	}
	for name, block := range map[string]string{
		"legacy static fee":         "maxFeePerGas: '30000000000'",
		"removed passive timeout":   "lateReceiptTimeoutMs: 600000",
		"removed passive hash cap":  "lateReceiptMaxHashes: 1024",
		"negative fee":              "maxFeeGwei: -1",
		"nan fee":                   "maxFeeGwei: .nan",
		"infinite fee":              "maxFeeGwei: .inf",
		"negative broadcast":        "broadcastTimeoutMs: -1",
		"overflow broadcast":        "broadcastTimeoutMs: 9223372036854775807",
		"pending shorter than bump": "pendingTimeoutMs: 1000",
		"horizon too long":          "horizon: {maxBlocks: 13}",
		"nan percentile":            "horizon: {congestedRewardPercentile: .nan}",
		"tip ordering":              "horizon: {tipFloorGwei: 16}",
		"gas headroom":              "horizon: {gasHeadroomBps: 2000, fallbackGasHeadroomBps: 1000}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, validConfig+"txManager:\n  "+block+"\n")); err == nil {
				t.Fatal("accepted invalid transaction policy")
			}
		})
	}
}

func TestOptionalGlobalMulticallAddress(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"", true}, {"0x0000000000000000000000000000000000000005", true}, {"0x0000000000000000000000000000000000000000", false}, {"not-address", false}} {
		t.Run(tc.value, func(t *testing.T) {
			text := validConfig
			if tc.value != "" {
				text = strings.Replace(text, "  chainId: 1", "  chainId: 1\n  multicall3: '"+tc.value+"'", 1)
			}
			_, err := Load(writeConfig(t, text))
			if (err == nil) != tc.valid {
				t.Fatalf("multicall address validation: %v", err)
			}
		})
	}
}

func TestRecipeCacheBudgetConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		valid      bool
	}{
		{"default", "", true}, {"disabled", "recipeCacheBytes: 0\n", true}, {"bounded", "recipeCacheBytes: 4096\n", true}, {"negative", "recipeCacheBytes: -1\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, validConfig+tc.yaml))
			if (err == nil) != tc.valid {
				t.Fatalf("cache budget: %v", err)
			}
			if err == nil {
				want := int64(16 << 20)
				if tc.name == "disabled" {
					want = 0
				}
				if tc.name == "bounded" {
					want = 4096
				}
				if cfg.RecipeCacheBytes != want {
					t.Fatalf("budget=%d want%d", cfg.RecipeCacheBytes, want)
				}
			}
		})
	}
}

func TestExecutorOnlyStrategyUsesRequiredCurator(t *testing.T) {
	text := strings.ReplaceAll(validConfig, "    account: '0x0000000000000000000000000000000000000003'\n", "")
	text = strings.ReplaceAll(text, "    tree: permissions.json\n", "")
	text = strings.Replace(text, "  url: http://127.0.0.1:1/api/v1", "  url: http://127.0.0.1:1/api/v1\n  apiKeyEnv: UNSET_CURATOR_CONFIG_TEST_KEY", 1)
	if _, err := Load(writeConfig(t, text)); err != nil {
		t.Fatalf("executor-only curator configuration rejected: %v", err)
	}
}

func TestCuratorValidationRejectsObsoleteAndUnsafeConfiguration(t *testing.T) {
	for name, raw := range map[string]string{
		"old account":      strings.Replace(validConfig, "    executor:", "    account: '0x0000000000000000000000000000000000000003'\n    executor:", 1),
		"old file":         strings.Replace(validConfig, "    executor:", "    tree: permissions.json\n    executor:", 1),
		"missing curator":  strings.Replace(validConfig, "curator:\n  url: http://127.0.0.1:1/api/v1\n", "", 1),
		"credential URL":   strings.Replace(validConfig, "http://127.0.0.1:1/api/v1", "https://user:secret@example.invalid/api/v1", 1),
		"query URL":        strings.Replace(validConfig, "http://127.0.0.1:1/api/v1", "http://example.invalid/api/v1?token=private", 1),
		"negative timeout": strings.Replace(validConfig, "curator:", "curator:\n  timeoutSeconds: -1", 1),
		"large timeout":    strings.Replace(validConfig, "curator:", "curator:\n  timeoutSeconds: 121", 1),
		"negative bound":   strings.Replace(validConfig, "curator:", "curator:\n  maxResponseBytes: -1", 1),
		"large bound":      strings.Replace(validConfig, "curator:", "curator:\n  maxResponseBytes: 16777217", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, raw)); err == nil {
				t.Fatal("invalid curator configuration accepted")
			}
		})
	}
}
