package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/symbioticfi/merkle-executor/internal/keeper"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/state"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
)

func testCLIConfig(directory string) config.Config {
	return config.Config{
		Mode:             "simulate",
		Chain:            config.Chain{ChainID: 31337, RPCURL: "http://127.0.0.1:1"},
		Signer:           config.Signer{Address: common.HexToAddress("0xabcd").Hex(), KeyEnv: "MERKLE_CLI_TEST_KEY"},
		StateDir:         filepath.Join(directory, "state"),
		OperationTimeout: 10 * time.Second,
		HTTPTimeout:      time.Second,
		PollInterval:     time.Second,
		TxManager:        config.TxManager{Confirmations: 1, PollInterval: 10 * time.Millisecond, MaxFeeGwei: 30},
		Strategies:       []config.Strategy{{Name: "funding", Executor: common.HexToAddress("0x1111").Hex(), Account: common.HexToAddress("0x2222").Hex(), Tree: filepath.Join(directory, "permissions.json"), Interval: time.Hour}},
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
		"status": {"config"}, "reset-strategy": {"config", "name"}, "version": {},
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
	t.Setenv(cfg.Signer.KeyEnv, "deliberately-invalid-cli-test-key")
	path := writeCLIConfig(t, directory, cfg)
	if output, err := executeCLI(t, "check-config", "--config", path); err != nil || output != "configuration valid\n" {
		t.Fatalf("check-config %q: %v", output, err)
	}
	if calls.Load() != 0 {
		t.Fatal("check-config contacted RPC")
	}
	if _, err := os.Stat(cfg.StateDir); !os.IsNotExist(err) {
		t.Fatalf("check-config created state: %v", err)
	}
}

func TestBindStateIdentity(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	caller := common.HexToAddress("0xabcdef")
	if err = bindState(store, 31337, caller.Hex()); err != nil {
		t.Fatal(err)
	}
	if err = bindState(store, 31337, strings.ToLower(caller.Hex())); err != nil {
		t.Fatalf("equivalent caller encoding rejected: %v", err)
	}
	for _, other := range []struct {
		chain  uint64
		caller string
	}{{1, caller.Hex()}, {31337, common.HexToAddress("0x9999").Hex()}} {
		if err = bindState(store, other.chain, other.caller); err == nil {
			t.Fatal("foreign state identity accepted")
		}
	}
	var saved struct {
		ChainID uint64
		Caller  common.Address
	}
	if found, err := store.Load("deployment.v1", &saved); err != nil || !found || saved.ChainID != 31337 || saved.Caller != caller {
		t.Fatalf("identity overwritten: %+v / %v", saved, err)
	}
}

func seedCLIState(t *testing.T, cfg config.Config, job keeper.Job, occupied bool) *state.Store {
	t.Helper()
	store, err := state.Open(cfg.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err = bindState(store, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
		t.Fatal(err)
	}
	if err = store.Save("strategies.v1", []string{"funding"}); err != nil {
		t.Fatal(err)
	}
	if err = store.Save("strategy.funding.v1", job); err != nil {
		t.Fatal(err)
	}
	if occupied {
		if err = store.Save("txmanager.active.v1", map[string]string{"id": job.ID}); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func TestStatusWhileDaemonOwnsState(t *testing.T) {
	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	cfg.Mode = "send"
	t.Setenv(cfg.Signer.KeyEnv, "invalid-key-that-status-must-not-read")
	path := writeCLIConfig(t, directory, cfg)
	job := keeper.Job{ID: "funding-1", Phase: "authorized", Result: txmanager.Result{Hash: common.HexToHash("0x1234")}, Progress: []provider.Progress{{Outcome: "waiting"}}, Plan: keeper.Plan{Outputs: []provider.Output{{Data: json.RawMessage(`{"privateDetail":"must-not-be-emitted"}`)}}}}
	store := seedCLIState(t, cfg, job, false)
	// Keep the exclusive daemon lock throughout the status command.
	output, err := executeCLI(t, "status", "--config", path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name, ID, Phase, Transaction string
		Outcomes                     []string
	}
	if err = json.Unmarshal([]byte(output), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != job.ID || rows[0].Phase != "authorized" || rows[0].Transaction != job.Result.Hash.Hex() || len(rows[0].Outcomes) != 1 || rows[0].Outcomes[0] != "waiting" {
		t.Fatalf("status lost saved progress: %s", output)
	}
	if strings.Contains(output, "must-not-be-emitted") {
		t.Fatal("status exposed provider runtime data")
	}
	var retained keeper.Job
	if found, err := store.Load("strategy.funding.v1", &retained); err != nil || !found || retained.Phase != "authorized" {
		t.Fatalf("status mutated work: %v", err)
	}
}

func TestStateCommandsRejectOtherDeployment(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"chain":  func(c *config.Config) { c.Chain.ChainID++ },
		"caller": func(c *config.Config) { c.Signer.Address = common.HexToAddress("0x9999").Hex() },
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			cfg := testCLIConfig(directory)
			store := seedCLIState(t, cfg, keeper.Job{ID: "funding-1", Phase: "settled"}, false)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			mutate(&cfg)
			path := writeCLIConfig(t, directory, cfg)
			for _, args := range [][]string{{"status", "--config", path}, {"reset-strategy", "--config", path, "--name", "funding"}} {
				if _, err := executeCLI(t, args...); err == nil || !strings.Contains(err.Error(), "another chain or caller") {
					t.Fatalf("foreign deployment accepted: %v", err)
				}
			}
			var retained keeper.Job
			if found, err := state.Read(cfg.StateDir, "strategy.funding.v1", &retained); err != nil || !found {
				t.Fatalf("foreign reset removed job: %v", err)
			}
		})
	}
}

func TestResetStrategyRequiresTerminalAcknowledgedWork(t *testing.T) {
	for _, tc := range []struct {
		name, phase       string
		occupied, allowed bool
	}{
		{"prepared", "prepared", false, false}, {"authorized", "authorized", false, false},
		{"invalid", "unknown", false, false}, {"unacknowledged", "settled", true, false},
		{"settled", "settled", false, true}, {"failed", "failed", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			cfg := testCLIConfig(directory)
			path := writeCLIConfig(t, directory, cfg)
			store := seedCLIState(t, cfg, keeper.Job{ID: "funding-1", Phase: tc.phase}, tc.occupied)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			_, err := executeCLI(t, "reset-strategy", "--config", path, "--name", "funding")
			if (err == nil) != tc.allowed {
				t.Fatalf("reset allowed=%t, error %v", tc.allowed, err)
			}
			var retained keeper.Job
			found, err := state.Read(cfg.StateDir, "strategy.funding.v1", &retained)
			if err != nil || found == tc.allowed {
				t.Fatalf("wrong checkpoint retention: found=%t / %v", found, err)
			}
		})
	}
}

func TestStatusWithoutIdentityDoesNotCreateState(t *testing.T) {
	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	path := writeCLIConfig(t, directory, cfg)
	if _, err := executeCLI(t, "status", "--config", path); err == nil {
		t.Fatal("unbound state accepted")
	}
	if _, err := os.Stat(cfg.StateDir); !os.IsNotExist(err) {
		t.Fatalf("status created state: %v", err)
	}
}
