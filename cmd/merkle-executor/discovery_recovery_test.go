package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	executorABI "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type recoveryNoIOPlanner struct{}

func (recoveryNoIOPlanner) PrepareAll(context.Context, []rt.Strategy) []rt.PreparationResult {
	panic("recovery must not prepare")
}
func (recoveryNoIOPlanner) Validate(context.Context, rt.Plan) error {
	panic("recovery must not simulate")
}
func (recoveryNoIOPlanner) ConfirmedSnapshot(context.Context, chainio.Snapshot) (chainio.Snapshot, error) {
	panic("recovery must not read chain")
}
func (recoveryNoIOPlanner) VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error {
	panic("recovery must not read chain")
}

type recoveryNoIODriver struct{}

func (recoveryNoIODriver) Reserve(context.Context, string) error {
	panic("invalid recovery must not reserve")
}
func (recoveryNoIODriver) Release(context.Context, string) error { panic("recovery must not release") }
func (recoveryNoIODriver) Execute(context.Context, rt.Intent, rt.PersistTransaction) rt.TransactionResult {
	panic("recovery must not send")
}
func (recoveryNoIODriver) Resume(context.Context, rt.Transaction, rt.PersistTransaction) rt.TransactionResult {
	panic("invalid recovery must not observe")
}

func recoveryFixturePlan(t *testing.T, chainID uint64, executor, account, caller common.Address) rt.Plan {
	t.Helper()
	hash, err := recipes.HashRecipe(recipes.Recipe{})
	if err != nil {
		t.Fatal(err)
	}
	return rt.Plan{Snapshot: chainio.Snapshot{ChainID: new(big.Int).SetUint64(chainID), BlockNumber: big.NewInt(1), BlockHash: common.HexToHash("0x01"), Executor: executor, Account: account, Caller: caller, RecipeHash: hash}, Calldata: executorABI.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)}
}

func TestDerivedAccountAndCuratorChangesCannotRewriteActiveWork(t *testing.T) {
	for _, change := range []string{"account", "caller", "chain", "url", "credential reference"} {
		t.Run(change, func(t *testing.T) {
			cfg := testCLIConfig(t.TempDir())
			name := cfg.Strategies[0]
			savedChain := cfg.Chain.ChainID
			account := common.HexToAddress("0x2222")
			caller := common.HexToAddress(cfg.Signer.Address)
			savedCaller := caller
			old := rt.Config{Name: name, Executor: name, Account: account.Hex(), Interval: cfg.Runtime.StrategyInterval, Revision: cfg.StrategyFingerprint(name, account, caller)}
			plan := recoveryFixturePlan(t, savedChain, common.HexToAddress(name), account, caller)
			switch change {
			case "account":
				account = common.HexToAddress("0x3333")
			case "caller":
				caller = common.HexToAddress("0x4444")
			case "chain":
				cfg.Chain.ChainID++
			case "url":
				cfg.Curator.URL = "http://other.invalid/api/v1"
			case "credential reference":
				cfg.Curator.APIKeyEnv = "OTHER_CURATOR_KEY"
			}
			current := rt.Config{Name: name, Executor: name, Account: account.Hex(), Interval: cfg.Runtime.StrategyInterval, Revision: cfg.StrategyFingerprint(name, account, caller)}
			store, err := state.Open(cfg.StateDir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			w := rt.Workflow{Version: 2, IdentityVersion: 2, Name: name, ID: "owned", ConfigHash: old.Fingerprint(), Phase: "prepared", Plan: plan}
			if err = store.ActivateNamespace(rt.Namespace, map[string]any{"deployment": rt.Deployment{Kind: "deployment", Version: 2, ChainID: new(big.Int).SetUint64(savedChain), Caller: savedCaller}, rt.WorkflowKey(name): w}); err != nil {
				t.Fatal(err)
			}
			worker := rt.Runtime{Store: store, Planner: recoveryNoIOPlanner{}, Driver: recoveryNoIODriver{}, Mode: "send", OperationTimeout: time.Second, ChainID: new(big.Int).SetUint64(cfg.Chain.ChainID), Caller: caller, Strategies: []rt.Strategy{{Config: current}}}
			if err = worker.Start(context.Background()); err == nil {
				t.Fatal("changed source identity accepted")
			}
			var retained rt.Workflow
			if found, err := store.LoadNamespace(rt.Namespace, rt.WorkflowKey(name), &retained); err != nil || !found || retained.ConfigHash != w.ConfigHash || retained.Phase != w.Phase {
				t.Fatal("recovery rewrote active identity")
			}
		})
	}
}

func TestRecoveryFencesRemovedExecutors(t *testing.T) {
	cfg := testCLIConfig(t.TempDir())
	caller := common.HexToAddress(cfg.Signer.Address)
	for _, removed := range []string{strings.ToLower(common.HexToAddress("0x2222").Hex()), strings.ToLower(common.HexToAddress("0x3333").Hex())} {
		for _, tc := range []struct {
			phase    string
			reserved bool
		}{{"prepared", false}, {"authorized", true}, {"settled", true}, {"failed", true}} {
			t.Run(removed+"/"+tc.phase, func(t *testing.T) {
				directory := t.TempDir()
				store, err := state.Open(directory)
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				w := rt.Workflow{Version: 2, IdentityVersion: 2, Name: removed, ID: "removed-owned", ConfigHash: "removed-executor-policy", Phase: tc.phase, Reserved: tc.reserved, Plan: recoveryFixturePlan(t, cfg.Chain.ChainID, common.HexToAddress(removed), common.HexToAddress("0x9999"), caller)}
				if tc.reserved {
					w.Transaction = &rt.Transaction{Intent: rt.Intent{ID: w.ID}, Sender: caller, ChainID: new(big.Int).SetUint64(cfg.Chain.ChainID), Attempts: []hexutil.Bytes{[]byte("existing-signed-bytes")}}
				}
				key := rt.WorkflowKey(removed)
				if err = store.ActivateNamespace(rt.Namespace, map[string]any{"deployment": rt.Deployment{Kind: "deployment", Version: 2, ChainID: new(big.Int).SetUint64(cfg.Chain.ChainID), Caller: caller}, key: w}); err != nil {
					t.Fatal(err)
				}
				bytesFor := func() []byte {
					t.Helper()
					hash := sha256.Sum256([]byte(key))
					raw, err := os.ReadFile(filepath.Join(directory, rt.Namespace, hex.EncodeToString(hash[:])+".json"))
					if err != nil {
						t.Fatal(err)
					}
					return raw
				}
				before := bytesFor()
				if err = rt.Preflight(store, new(big.Int).SetUint64(cfg.Chain.ChainID), caller, cfg.Strategies); err == nil || !strings.Contains(err.Error(), "removed from configuration") {
					t.Fatalf("removed owner admitted: %v", err)
				}
				if !bytes.Equal(before, bytesFor()) {
					t.Fatal("recovery rewrote removed ownership")
				}
				rows, err := store.ListNamespace(rt.Namespace)
				if err != nil || len(rows) != 2 {
					t.Fatal("removed inventory identifier removed", err)
				}
			})
		}
	}
}
