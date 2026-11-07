package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRuntimeReadAndTransactionPolicyComesFromYAML(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig+"runtime:\n  maxPreparations: 7\nreads:\n  maxConcurrent: 3\ntransaction:\n  pollInterval: 3s\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.MaxPreparations != 7 || cfg.Reads.MaxConcurrent != 3 || cfg.Transaction.PollInterval != 3*time.Second || cfg.Transaction.Horizon.MaxBlocks != 6 {
		t.Fatal("effective YAML policy lost")
	}
}
func TestReadBoundsAndExplicitZeroFailClosed(t *testing.T) {
	for _, block := range []string{"reads:\n  maxConcurrent: 0\n", "reads:\n  maxMembers: 257\n", "runtime:\n  operationTimeout: 0s\n", "transaction:\n  pollInterval: 0s\n"} {
		if _, err := Load(writeConfig(t, validConfig+block)); err == nil {
			t.Fatalf("accepted %q", block)
		}
	}
}
func TestConfigure1024ExecutorRegistry(t *testing.T) {
	text := validConfig[:strings.Index(validConfig, "strategies:")] + "strategies:\n"
	for i := range 1024 {
		text += fmt.Sprintf("  - '0x%040x'\n", i+1)
	}
	if _, err := Load(writeConfig(t, text)); err != nil {
		t.Fatal(err)
	}
}
