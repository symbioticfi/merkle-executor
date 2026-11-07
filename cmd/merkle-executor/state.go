package main

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func stateCommands() []*cobra.Command {
	var statusPath string
	status := &cobra.Command{Use: "status", Short: "Inspect durable workflows without RPC access or signing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(statusPath)
		if err != nil {
			return err
		}
		load := func(key string, out any) (bool, error) {
			return state.ReadNamespace(cfg.StateDir, rt.Namespace, key, out)
		}
		if err = checkStateIdentity(load, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
			return err
		}
		workflows, err := readStateWorkflows(func() ([]json.RawMessage, error) { return state.ListNamespace(cfg.StateDir, rt.Namespace) }, load)
		if err != nil {
			return err
		}
		type row struct {
			Identifier, ID, Phase, Transaction string
			Outcomes                           []string
		}
		rows := make([]row, 0, len(workflows))
		for _, w := range workflows {
			item := row{Identifier: w.Name, ID: w.ID, Phase: w.Phase}
			if w.Transaction != nil && w.Transaction.Execution != nil {
				item.Transaction = w.Transaction.Execution.TxHash.Hex()
			}
			for _, record := range w.Plan.Operations {
				item.Outcomes = append(item.Outcomes, record.Progress.Outcome)
			}
			rows = append(rows, item)
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(rows)
	}}
	status.Flags().StringVar(&statusPath, "config", "", "configuration path")
	_ = status.MarkFlagRequired("config")
	var resetPath, executor string
	reset := &cobra.Command{Use: "reset-strategy", Short: "Clear a terminal workflow checkpoint after inspection", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		address, err := parse.Address(executor)
		if err != nil {
			return errors.New("--executor requires a full nonzero address")
		}
		identifier := strings.ToLower(address.Hex())
		cfg, err := config.Load(resetPath)
		if err != nil {
			return err
		}
		store, err := state.Open(cfg.StateDir)
		if err != nil {
			return err
		}
		defer store.Close()
		load := func(key string, out any) (bool, error) { return store.LoadNamespace(rt.Namespace, key, out) }
		if err = checkStateIdentity(load, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
			return err
		}
		workflows, err := readStateWorkflows(func() ([]json.RawMessage, error) { return store.ListNamespace(rt.Namespace) }, load)
		if err != nil {
			return err
		}
		var selected *rt.Workflow
		for i := range workflows {
			w := &workflows[i]
			if w.Reserved || w.Phase == "pending" {
				return errors.New("transaction reservation still occupied; resume before resetting")
			}
			if w.Name == identifier {
				selected = w
			}
		}
		if selected == nil {
			return errors.New("unknown strategy")
		}
		if selected.Phase != "settled" && selected.Phase != "failed" {
			return errors.New("cannot reset unfinished work; reconcile it first")
		}
		return store.DeleteNamespace(rt.Namespace, rt.WorkflowKey(identifier))
	}}
	reset.Flags().StringVar(&resetPath, "config", "", "configuration path")
	reset.Flags().StringVar(&executor, "executor", "", "executor address")
	_ = reset.MarkFlagRequired("config")
	_ = reset.MarkFlagRequired("executor")
	return []*cobra.Command{status, reset}
}

func checkStateIdentity(load func(string, any) (bool, error), chainID uint64, caller string) error {
	var saved rt.Deployment
	found, err := load("deployment", &saved)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no deployment identity in state directory; migrate state by starting the runtime")
	}
	if saved.Kind != "deployment" || saved.Version != 2 || saved.ChainID == nil || saved.ChainID.Cmp(new(big.Int).SetUint64(chainID)) != 0 || saved.Caller == (common.Address{}) || caller != "" && saved.Caller != common.HexToAddress(caller) {
		return errors.New("state directory belongs to another chain or caller")
	}
	return nil
}

func readStateWorkflows(list func() ([]json.RawMessage, error), load func(string, any) (bool, error)) ([]rt.Workflow, error) {
	rows, err := list()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]rt.Workflow, 0, len(rows))
	identities := 0
	for _, raw := range rows {
		var header struct {
			Kind string `json:"kind"`
		}
		if err = json.Unmarshal(raw, &header); err != nil {
			return nil, err
		}
		if header.Kind != "" {
			if header.Kind != "deployment" {
				return nil, errors.New("invalid saved checkpoint kind")
			}
			var saved json.RawMessage
			found, err := load("deployment", &saved)
			if err != nil || !found || string(saved) != string(raw) {
				return nil, errors.New("invalid saved deployment checkpoint")
			}
			identities++
			if identities > 1 {
				return nil, errors.New("duplicate deployment checkpoint")
			}
			continue
		}
		var w rt.Workflow
		if err = json.Unmarshal(raw, &w); err != nil {
			return nil, err
		}
		address, err := parse.Address(w.Name)
		if err != nil || strings.ToLower(address.Hex()) != w.Name {
			return nil, errors.New("invalid saved executor address")
		}
		if w.Version != 2 || w.ID == "" || seen[w.Name] {
			return nil, errors.New("invalid or duplicate saved workflow")
		}
		var actual json.RawMessage
		found, err := load(rt.WorkflowKey(w.Name), &actual)
		if err != nil || !found || string(actual) != string(raw) {
			return nil, errors.New("saved workflow filename differs from identity")
		}
		seen[w.Name] = true
		out = append(out, w)
	}
	if identities != 1 {
		return nil, errors.New("missing deployment checkpoint")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
