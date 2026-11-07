package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v3"

	"github.com/symbioticfi/merkle-executor/internal/config"
)

func testCLIConfig(directory string) config.Config {
	path := filepath.Join(directory, "defaults-fixture.yaml")
	raw := []byte("mode: simulate\ncurator:\n  url: http://127.0.0.1:1/api/v1\nchain:\n  chainId: 31337\n  rpcUrl: http://127.0.0.1:1\nstrategies: ['0x0000000000000000000000000000000000001111']\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		panic(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		panic(err)
	}
	cfg.Signer = config.Signer{Address: common.HexToAddress("0xabcd").Hex(), KeyEnv: "MERKLE_CLI_TEST_KEY"}
	cfg.Transaction.MaxFeeGwei = 30
	return *cfg
}

func writeCLIConfig(t *testing.T, directory string, cfg config.Config) string {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "keeper.yaml")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	command := root()
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := command.ExecuteContext(ctx)
	return output.String(), err
}

func TestRootCommandsAndRequiredFlags(t *testing.T) {
	for name, flags := range map[string][]string{
		"run": {"config", "once"}, "plan": {"config"}, "check-config": {"config"},
		"version": {},
	} {
		t.Run(name, func(t *testing.T) {
			command, _, err := root().Find([]string{name})
			if err != nil || command.Name() != name {
				t.Fatalf("command %s missing: %v", name, err)
			}
			for _, flag := range flags {
				if command.Flags().Lookup(flag) == nil {
					t.Fatalf("missing --%s", flag)
				}
			}
			if name != "version" {
				if _, err := executeCLI(t, name); err == nil || !strings.Contains(err.Error(), "required flag") {
					t.Fatalf("missing config accepted: %v", err)
				}
			}
			if _, err := executeCLI(t, name, "unexpected"); err == nil {
				t.Fatal("positional argument accepted")
			}
		})
	}
	if output, err := executeCLI(t, "version"); err != nil || !strings.Contains(output, "(") {
		t.Fatalf("version output %q: %v", output, err)
	}
}

func TestExampleConfigurationCommand(t *testing.T) {
	t.Setenv("MERKLE_EXECUTOR_PRIVATE_KEY", "invalid-key-that-must-not-be-read")
	output, err := executeCLI(t, "check-config", "--config", filepath.Join("..", "..", "config", "example.yaml"))
	if err != nil || output != "configuration valid\n" {
		t.Fatalf("example check %q: %v", output, err)
	}
}

func TestCheckConfigDoesNotReadKeyOrUseRPC(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	cfg.Mode, cfg.Chain.RPCURL = "send", server.URL
	cfg.Curator.URL = server.URL + "/api/v1"
	cfg.Curator.APIKeyEnv = "MISSING_CURATOR_CHECK_CONFIG_KEY"
	t.Setenv(cfg.Signer.KeyEnv, "deliberately-invalid-cli-test-key")
	path := writeCLIConfig(t, directory, cfg)
	if output, err := executeCLI(t, "check-config", "--config", path); err != nil || output != "configuration valid\n" {
		t.Fatalf("check-config %q: %v", output, err)
	}
	if calls.Load() != 0 {
		t.Fatal("check-config contacted RPC")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("check-config created runtime files: %v / %v", entries, err)
	}
}

func TestRemovedStateCommandsAreUnknown(t *testing.T) {
	for _, name := range []string{"status", "reset-strategy"} {
		if _, err := executeCLI(t, name); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("removed command %s accepted: %v", name, err)
		}
	}
}

func TestCheckConfigRejectsRemovedStateDirectory(t *testing.T) {
	directory := t.TempDir()
	path := writeCLIConfig(t, directory, testCLIConfig(directory))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, []byte("stateDir: state\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCLI(t, "check-config", "--config", path); err == nil {
		t.Fatal("removed stateDir accepted")
	}
}
