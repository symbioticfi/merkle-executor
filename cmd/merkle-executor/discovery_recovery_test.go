package main

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func TestDerivedAccountAndCuratorChangesCannotRewriteActiveWork(t *testing.T) {
	for _, change := range []string{"account", "url", "credential reference", "response bound"} {
		t.Run(change, func(t *testing.T) {
			cfg := testCLIConfig(t.TempDir())
			original := cfg.Strategies[0]
			account := common.HexToAddress("0x2222")
			savedRevision := cfg.StrategyFingerprint(original, account)
			switch change {
			case "account":
				account = common.HexToAddress("0x3333")
			case "url":
				cfg.Curator.URL = "http://other.invalid/api/v1"
			case "credential reference":
				cfg.Curator.APIKeyEnv = "OTHER_CURATOR_KEY"
			case "response bound":
				cfg.Curator.MaxResponseBytes = 4096
			}
			current := keeper.Config{Name: original.Name, Executor: original.Executor, Account: account.Hex(), Interval: original.Interval, Revision: cfg.StrategyFingerprint(original, account)}
			old := current
			old.Revision = savedRevision
			store, err := state.Open(cfg.StateDir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			job := keeper.Job{ID: "owned", ConfigHash: old.Fingerprint(), Phase: "prepared"}
			if err = store.Save("strategies.v1", []string{original.Name}); err != nil {
				t.Fatal(err)
			}
			if err = store.Save("strategy."+original.Name+".v1", job); err != nil {
				t.Fatal(err)
			}
			k := keeper.Keeper{Store: store, Strategies: []keeper.Strategy{{Config: current}}}
			if err = k.CheckRecovery(); err == nil {
				t.Fatal("changed source identity accepted")
			}
			var retained keeper.Job
			if found, err := store.Load("strategy."+original.Name+".v1", &retained); err != nil || !found || retained.ConfigHash != job.ConfigHash || retained.Phase != job.Phase {
				t.Fatal("recovery rewrote active identity")
			}
		})
	}
}
