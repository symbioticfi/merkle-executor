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

func testCLIConfig(_ string) config.Config {
	return config.Config{
		Mode:       "simulate",
		Curator:    config.Curator{URL: "http://127.0.0.1:1/api/v1"},
		Chain:      config.Chain{ChainID: 31337, RPCURL: "http://127.0.0.1:1"},
		Signer:     config.Signer{Address: common.HexToAddress("0xabcd").Hex(), KeyEnv: "MERKLE_CLI_TEST_KEY"},
		TxManager:  config.TxManager{MaxFeeGwei: 30},
		Strategies: []string{common.HexToAddress("0x1111").Hex()},
	}
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
	cfg.Curator = config.Curator{URL: server.URL + "/api/v1"}
	t.Setenv(cfg.Signer.KeyEnv, "deliberately-invalid-cli-test-key")
	path := writeCLIConfig(t, directory, cfg)
	if output, err := executeCLI(t, "check-config", "--config", path); err != nil || output != "configuration valid\n" {
		t.Fatalf("check-config %q: %v", output, err)
	}
	if calls.Load() != 0 {
		t.Fatal("check-config contacted RPC")
	}
	if _, err := os.Stat(filepath.Join(directory, "state")); !os.IsNotExist(err) {
		t.Fatalf("check-config created state: %v", err)
	}
}
