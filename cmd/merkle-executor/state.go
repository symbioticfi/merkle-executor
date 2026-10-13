package main

import (
	"encoding/json"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func stateCommands() []*cobra.Command {
	var statusPath string
	status := &cobra.Command{Use: "status", Short: "Inspect durable jobs without RPC access or signing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(statusPath)
		if err != nil {
			return err
		}
		load := func(key string, out any) (bool, error) { return state.Read(cfg.StateDir, key, out) }
		if err := checkStateIdentity(load, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
			return err
		}
		var names []string
		if _, err := load("strategies.v1", &names); err != nil {
			return err
		}
		for _, s := range cfg.Strategies {
			if !slices.Contains(names, s.Name) {
				names = append(names, s.Name)
			}
		}
		type row struct {
			Name, ID, Phase, Transaction string
			Outcomes                     []string
		}
		rows := make([]row, 0, len(names))
		for _, name := range names {
			var job keeper.Job
			found, err := load("strategy."+name+".v1", &job)
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			r := row{Name: name, ID: job.ID, Phase: job.Phase, Transaction: job.Result.Hash.Hex()}
			for _, p := range job.Progress {
				r.Outcomes = append(r.Outcomes, p.Outcome)
			}
			rows = append(rows, r)
		}
		e := json.NewEncoder(cmd.OutOrStdout())
		e.SetIndent("", "  ")
		return e.Encode(rows)
	}}
	status.Flags().StringVar(&statusPath, "config", "", "configuration path")
	_ = status.MarkFlagRequired("config")
	var resetPath, name string
	reset := &cobra.Command{Use: "reset-strategy", Short: "Clear a terminal strategy checkpoint after inspection", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(resetPath)
		if err != nil {
			return err
		}
		store, err := state.Open(cfg.StateDir)
		if err != nil {
			return err
		}
		defer store.Close()
		if err := checkStateIdentity(store.Load, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
			return err
		}
		var tx struct{ ID string }
		found, err := store.Load("txmanager.active.v1", &tx)
		if err != nil {
			return err
		}
		if found {
			return errors.New("transaction journal still occupied; resume and acknowledge before resetting")
		}
		var names []string
		if _, err := store.Load("strategies.v1", &names); err != nil {
			return err
		}
		if !slices.Contains(names, name) {
			return errors.New("unknown strategy")
		}
		var job keeper.Job
		found, err = store.Load("strategy."+name+".v1", &job)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("no saved job")
		}
		if job.Phase != "settled" && job.Phase != "failed" {
			return errors.New("cannot reset unfinished work; reconcile it first")
		}
		return store.Delete("strategy." + name + ".v1")
	}}
	reset.Flags().StringVar(&resetPath, "config", "", "configuration path")
	reset.Flags().StringVar(&name, "name", "", "strategy name")
	_ = reset.MarkFlagRequired("config")
	_ = reset.MarkFlagRequired("name")
	return []*cobra.Command{status, reset}
}

func checkStateIdentity(load func(string, any) (bool, error), chainID uint64, caller string) error {
	var saved struct {
		ChainID uint64         `json:"chainId"`
		Caller  common.Address `json:"caller"`
	}
	found, err := load("deployment.v1", &saved)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no deployment identity in state directory")
	}
	if saved.ChainID != chainID || saved.Caller == (common.Address{}) || caller != "" && saved.Caller != common.HexToAddress(caller) {
		return errors.New("state directory belongs to another chain or caller")
	}
	return nil
}
