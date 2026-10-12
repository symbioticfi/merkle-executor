package parse

import (
	"gopkg.in/yaml.v3"
	"math/big"
	"strings"
	"testing"
)

func TestUint256(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"0", "42", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()} {
		if _, err := Uint256(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"", "-1", "+1", "0x1", "1.0", "1e18", " 1", new(big.Int).Lsh(big.NewInt(1), 256).String()} {
		if _, err := Uint256(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestStrictProviderConfigurationRejectsAliases(t *testing.T) {
	for _, raw := range []string{"inputs: {0: &x '1', 1: *x}", "inputs: {<<: &x {0: '1'}, 1: *x}"} {
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
			t.Fatal(err)
		}
		var out struct {
			Inputs map[uint]string `yaml:"inputs"`
		}
		if err := DecodeStrict(node, &out); err == nil || !strings.Contains(err.Error(), "aliases") {
			t.Fatalf("alias config accepted: %v", err)
		}
	}
}
