package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/state"
	sdktx "github.com/symbioticfi/merkle-executor/sdk/submission"
)

func TestDerivedAccountAndCuratorChangesCannotRewriteActiveWork(t *testing.T) {
	for _, change := range []string{"account", "caller", "chain", "url", "credential reference"} {
		t.Run(change, func(t *testing.T) {
			cfg := testCLIConfig(t.TempDir())
			original := cfg.Strategies[0]
			account := common.HexToAddress("0x2222")
			caller := common.HexToAddress(cfg.Signer.Address)
			savedRevision := cfg.StrategyFingerprint(original, account, caller)
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
			current := keeper.Config{Name: original, Executor: original, Account: account.Hex(), Interval: config.DefaultStrategyInterval, Revision: cfg.StrategyFingerprint(original, account, caller)}
			old := current
			old.Revision = savedRevision
			store, err := state.Open(cfg.StateDir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			job := keeper.Job{ID: "owned", ConfigHash: old.Fingerprint(), Phase: "prepared"}
			if err = store.Save("strategies.v1", []string{original}); err != nil {
				t.Fatal(err)
			}
			if err = store.Save("strategy."+original+".v1", job); err != nil {
				t.Fatal(err)
			}
			k := keeper.Keeper{Store: store, Strategies: []keeper.Strategy{{Config: current}}}
			if err = k.CheckRecovery(); err == nil {
				t.Fatal("changed source identity accepted")
			}
			var retained keeper.Job
			if found, err := store.Load("strategy."+original+".v1", &retained); err != nil || !found || retained.ConfigHash != job.ConfigHash || retained.Phase != job.Phase {
				t.Fatal("recovery rewrote active identity")
			}
		})
	}
}

type removedIdentitySender struct{ pending string }

func (s *removedIdentitySender) PendingID() string { return s.pending }
func (*removedIdentitySender) Send(context.Context, sdktx.Request) (sdktx.Result, error) {
	panic("recovery must not send")
}
func (*removedIdentitySender) Acknowledge(context.Context, string) error {
	panic("recovery must not acknowledge")
}

func TestRecoveryFencesRemovedExecutors(t *testing.T) {
	cfg, err := config.Load(writeCLIConfig(t, t.TempDir(), testCLIConfig(t.TempDir())))
	if err != nil {
		t.Fatal(err)
	}
	executor := cfg.Strategies[0]
	strategies := []keeper.Strategy{{Config: keeper.Config{Name: executor, Executor: executor, Interval: config.DefaultStrategyInterval}}}
	for _, removed := range []string{strings.ToLower(common.HexToAddress("0x2222").Hex()), strings.ToLower(common.HexToAddress("0x3333").Hex())} {
		for _, tc := range []struct {
			phase   string
			pending bool
		}{{"prepared", false}, {"authorized", true}, {"settled", true}, {"failed", true}} {
			t.Run(removed+"/"+tc.phase, func(t *testing.T) {
				directory := t.TempDir()
				store, err := state.Open(directory)
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				job := keeper.Job{ID: "removed-owned", Phase: tc.phase, ConfigHash: "removed-executor-policy", Plan: keeper.Plan{Outputs: []provider.Output{{Data: json.RawMessage(`{"uid":"durable-order"}`)}}}}
				key := "strategy." + removed + ".v1"
				if err := store.Save("strategies.v1", []string{removed}); err != nil {
					t.Fatal(err)
				}
				if err := store.Save(key, job); err != nil {
					t.Fatal(err)
				}
				sender := &removedIdentitySender{}
				if tc.pending {
					sender.pending = job.ID
					if err := store.Save("txmanager.active.v1", map[string]string{"id": job.ID, "signed": "existing-signed-bytes"}); err != nil {
						t.Fatal(err)
					}
				}
				bytesFor := func(key string) []byte {
					t.Helper()
					hash := sha256.Sum256([]byte(key))
					raw, err := os.ReadFile(filepath.Join(directory, hex.EncodeToString(hash[:])+".json"))
					if err != nil {
						t.Fatal(err)
					}
					return raw
				}
				before := bytesFor(key)
				var signed []byte
				if tc.pending {
					signed = bytesFor("txmanager.active.v1")
				}
				worker := keeper.Keeper{Store: store, Sender: sender, Strategies: strategies}
				if err := worker.CheckRecovery(); err == nil || !strings.Contains(err.Error(), "removed from configuration") {
					t.Fatalf("removed owner admitted: %v", err)
				}
				if !bytes.Equal(before, bytesFor(key)) || tc.pending && (!bytes.Equal(signed, bytesFor("txmanager.active.v1")) || sender.pending != job.ID) {
					t.Fatal("recovery rewrote removed ownership")
				}
				var inventory []string
				if _, err := store.Load("strategies.v1", &inventory); err != nil || !slices.Contains(inventory, removed) {
					t.Fatal("removed inventory identifier removed", err)
				}
			})
		}
	}
}
