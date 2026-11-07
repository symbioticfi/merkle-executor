package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const validConfig = `curator:
  url: http://127.0.0.1:1/api/v1
chain:
  chainId: 1
  rpcUrl: http://127.0.0.1:8545
signer:
  address: '0x0000000000000000000000000000000000000001'
strategies:
  - '0x0000000000000000000000000000000000000002'
`

func writeConfig(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestConfigurationDefaults(t *testing.T) {
	t.Parallel()
	p := writeConfig(t, validConfig)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "simulate" || c.Signer.KeyEnv != "MERKLE_EXECUTOR_PRIVATE_KEY" {
		t.Fatal(c)
	}
}
func TestRejectMalformedConfiguration(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"unknown":                 validConfig + "typo: true\n",
		"removed state directory": validConfig + "stateDir: state\n",
		"multiple":                validConfig + "---\nmode: send\n",
		"zero caller":             strings.Replace(validConfig, "0000000000000000000000000000000000000001", "0000000000000000000000000000000000000000", 1),
		"send without fee":        validConfig + "mode: send\n",
		"negative interval":       validConfig + "pollInterval: -1s\n",
		"missing rpc":             strings.Replace(validConfig, "http://127.0.0.1:8545", "", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, text)); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}
func TestTransactionDefaultsAndInvalidLimits(t *testing.T) {
	t.Parallel()
	defaults, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Transaction.PriorityFeeGwei != 0.1 || defaults.Transaction.GasHeadroomBps != 500 || defaults.Transaction.Confirmations != 2 || defaults.Transaction.PollInterval != 2*time.Second || defaults.Transaction.BroadcastTimeout != 5*time.Second || defaults.Transaction.ReadTimeout != 2*time.Second || defaults.Transaction.EstimateTimeout != 5*time.Second {
		t.Fatal("unexpected transaction defaults")
	}
	for name, block := range map[string]string{
		"removed horizon":        "horizon: {maxBlocks: 6}",
		"removed replacement":    "replacementInterval: 30s",
		"removed pending":        "pendingTimeout: 5m",
		"removed shutdown":       "shutdownTimeout: 1m",
		"fixed polling":          "pollIntervalMs: 2000",
		"fixed broadcast":        "broadcastTimeoutMs: 5000",
		"unsupported static fee": "maxFeePerGas: '30000000000'",
		"negative fee":           "maxFeeGwei: -1",
		"nan fee":                "maxFeeGwei: .nan",
		"infinite fee":           "maxFeeGwei: .inf",
		"negative priority":      "priorityFeeGwei: -1",
		"nan priority":           "priorityFeeGwei: .nan",
		"infinite priority":      "priorityFeeGwei: .inf",
		"negative broadcast":     "broadcastTimeout: -1s",
		"zero confirmations":     "confirmations: 0",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, validConfig+"transaction:\n  "+block+"\n")); err == nil {
				t.Fatal("accepted invalid transaction policy")
			}
		})
	}
}

func TestMulticallAddressIsConfiguredAndValidated(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"", true}, {"0xcA11bde05977b3631167028862bE2a173976CA11", true}, {"0x0000000000000000000000000000000000000005", true}, {"0x0000000000000000000000000000000000000000", false}, {"not-address", false}} {
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

func TestRuntimePolicyFieldsAreNotConfiguration(t *testing.T) {
	for _, field := range []string{
		"recipeCacheBytes: 0", "recipeCacheBytes: 16777216",
		"pollInterval: 15s", "operationTimeout: 30s", "httpTimeout: 15s",
	} {
		t.Run(field, func(t *testing.T) {
			if _, err := Load(writeConfig(t, validConfig+field+"\n")); err == nil {
				t.Fatal("fixed runtime field accepted")
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
		"account mapping":  strategyConfig("  - executor: '0x0000000000000000000000000000000000000002'\n    account: '0x0000000000000000000000000000000000000003'\n"),
		"file mapping":     strategyConfig("  - executor: '0x0000000000000000000000000000000000000002'\n    tree: permissions.json\n"),
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

func TestMinimalConfigurationDerivesSignerAndUsesStandardConstants(t *testing.T) {
	for _, mode := range []string{"simulate", "send"} {
		t.Run(mode, func(t *testing.T) {
			raw := strings.Replace(validConfig, "signer:\n  address: '0x0000000000000000000000000000000000000001'\n", "", 1)
			raw += "mode: " + mode + "\ntransaction:\n  maxFeeGwei: 30\n"
			cfg, err := Load(writeConfig(t, raw))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Signer.Address != "" || cfg.Signer.KeyEnv != "MERKLE_EXECUTOR_PRIVATE_KEY" {
				t.Fatalf("unexpected defaults: %+v", cfg)
			}
		})
	}
}

func TestRuntimePolicyRejectsObsoleteFieldLayout(t *testing.T) {
	for _, override := range []string{"pollInterval: 1s\n", "operationTimeout: 1s\n", "httpTimeout: 1s\n", "transaction: {broadcastTimeoutMs: 1000}\n", "txManager: {maxFeeGwei: 30}\n"} {
		if _, err := Load(writeConfig(t, validConfig+override)); err == nil {
			t.Fatal("obsolete field layout accepted")
		}
	}
}

func TestRuntimeRejectsManualProviders(t *testing.T) {
	for _, name := range []string{"static", "rpc", "http", "cowswap", "merkl", "liquidlane-rfq", "redemptions", "securitize"} {
		t.Run(name, func(t *testing.T) {
			raw := strategyConfig("  - executor: '0x0000000000000000000000000000000000000002'\n    providers:\n      - name: " + name + "\n        config: {}\n")
			if _, err := Load(writeConfig(t, raw)); err == nil {
				t.Fatalf("manual provider accepted: %v", err)
			}
		})
	}
}

func TestCuratorEffectivePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		timeout      int
		bound        int64
	}{
		{"omitted", "", 10, 2097152},
		{"explicit defaults", "  timeoutSeconds: 10\n  maxResponseBytes: 2097152\n", 10, 2097152},
		{"changed timeout", "  timeoutSeconds: 11\n", 11, 2097152},
		{"changed bound", "  maxResponseBytes: 1048576\n", 10, 1048576},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, strings.Replace(validConfig, "curator:\n", "curator:\n"+tc.fields, 1)))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Curator.TimeoutSeconds != tc.timeout || cfg.Curator.MaxResponseBytes != tc.bound {
				t.Fatal("effective curator policy differs")
			}
		})
	}
	if _, err := Load(writeConfig(t, strings.Replace(validConfig, "curator:\n", "curator:\n  timeoutSeconds: 0\n", 1))); err == nil {
		t.Fatal("zero timeout accepted")
	}
}

func strategyConfig(body string) string {
	prefix, _, _ := strings.Cut(validConfig, "strategies:\n")
	return prefix + "strategies:\n" + body
}

func TestScalarStrategyAddressSchema(t *testing.T) {
	const mixed = "0xaBcDeF0000000000000000000000000000000001"
	const canonical = "0xabcdef0000000000000000000000000000000001"
	const other = "0x0000000000000000000000000000000000000002"
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"one", "  - '" + mixed + "'\n", []string{canonical}},
		{"lowercase", "  - '" + canonical + "'\n", []string{canonical}},
		{"uppercase", "  - '0x" + strings.ToUpper(mixed[2:]) + "'\n", []string{canonical}},
		{"ordered", "  - '" + mixed + "'\n  - '" + other + "'\n", []string{canonical, other}},
		{"reordered", "  - '" + other + "'\n  - '" + mixed + "'\n", []string{other, canonical}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, strategyConfig(tc.body)))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.Strategies, tc.want) {
				t.Fatalf("strategies=%v want%v", cfg.Strategies, tc.want)
			}
		})
	}
	many := strings.Builder{}
	for i := 1; i <= 1025; i++ {
		fmt.Fprintf(&many, "  - '0x%040x'\n", i)
	}
	for name, body := range map[string]string{
		"empty list": "  []\n", "too many": many.String(), "empty": "  - ''\n", "zero": "  - '0x0000000000000000000000000000000000000000'\n", "invalid": "  - invalid\n", "short": "  - '0x1'\n", "number": "  - 1\n", "null": "  - null\n", "duplicate": "  - '" + mixed + "'\n  - '" + canonical + "'\n",
		"named object": "  - name: custom\n    executor: '" + mixed + "'\n", "unnamed object": "  - executor: '" + mixed + "'\n", "label": "  - executor-" + canonical + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, strategyConfig(body))); err == nil {
				t.Fatal("invalid strategy accepted")
			}
		})
	}
}

func TestValidateStoresCanonicalAddresses(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Strategies = []string{"0xaBcDeF0000000000000000000000000000000001"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Strategies[0] != "0xabcdef0000000000000000000000000000000001" {
		t.Fatal("direct validation did not canonicalize")
	}
	cfg.Strategies = append(cfg.Strategies, "0xABCDEF0000000000000000000000000000000001")
	if err := cfg.Validate(); err == nil {
		t.Fatal("direct validation accepted duplicate")
	}
	cfg.Strategies = []string{"invalid"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid executor accepted")
	}
}
