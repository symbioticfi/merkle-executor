package config

import (
	"strings"
	"testing"
)

func TestConfigurationRejectsYAMLAnchorsAndAliases(t *testing.T) {
	for _, value := range []string{
		"  rpcUrl: &endpoint http://127.0.0.1:8545\n",
		"  rpcUrl: &endpoint http://127.0.0.1:8545\n  writeRpcUrl: *endpoint\n",
	} {
		raw := strings.Replace(validConfig, "  rpcUrl: http://127.0.0.1:8545\n", value, 1)
		if _, err := Load(writeConfig(t, raw)); err == nil {
			t.Fatal("YAML anchor or alias accepted")
		}
	}
}
