package main

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/chain"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/keeper"
	"github.com/symbioticfi/merkle-executor/internal/observability"
	"github.com/symbioticfi/merkle-executor/internal/parse"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/data"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlane"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/state"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
)

func runBot(ctx context.Context, path string, once, planOnly bool, out io.Writer) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if planOnly {
		cfg.Mode = "simulate"
	}
	log, flush, err := observability.NewLogger(cfg.Observability.Debug)
	if err != nil {
		return err
	}
	defer flush()
	chainID := new(big.Int).SetUint64(cfg.Chain.ChainID)
	startup, cancel := context.WithTimeout(ctx, cfg.OperationTimeout)
	client, err := chain.Dial(startup, cfg.Chain.RPCURL, cfg.Chain.WriteRPCURL, chainID)
	cancel()
	if err != nil {
		return err
	}
	defer client.Close()
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	registry := provider.NewRegistry()
	for _, register := range []func(*provider.Registry) error{data.Register, liquidlane.Register, merkl.Register, cowswap.Register} {
		if err := register(registry); err != nil {
			return err
		}
	}
	strategies := make([]keeper.Strategy, 0, len(cfg.Strategies))
	for _, sc := range cfg.Strategies {
		s := keeper.Strategy{Config: sc}
		for _, pc := range sc.Providers {
			p, err := registry.New(pc.Name, pc.Config, provider.Deps{Chain: client, HTTP: httpClient, Log: log.WithValues("strategy", sc.Name, "provider", pc.Name)})
			if err != nil {
				return errors.Errorf("configure provider %s: %w", pc.Name, err)
			}
			s.Providers = append(s.Providers, p)
		}
		strategies = append(strategies, s)
	}
	planner := &keeper.Planner{Chain: client, ChainID: chainID, Caller: common.HexToAddress(cfg.Signer.Address), Confirmations: cfg.TxManager.Confirmations}
	if planOnly {
		plans := make(map[string]keeper.Plan)
		for _, s := range strategies {
			operation, cancel := context.WithTimeout(ctx, cfg.OperationTimeout)
			p, err := planner.Prepare(operation, s)
			cancel()
			if err != nil {
				return errors.Errorf("prepare %s: %w", s.Config.Name, err)
			}
			plans[s.Config.Name] = p
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plans)
	}
	store, err := state.Open(cfg.StateDir)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := bindState(store, cfg.Chain.ChainID, cfg.Signer.Address); err != nil {
		return err
	}
	k := &keeper.Keeper{Store: store, Planner: planner, Strategies: strategies, Mode: cfg.Mode, OperationTimeout: cfg.OperationTimeout, Log: log}
	if cfg.Mode == "send" {
		s, err := signer.FromEnv(cfg.Signer.KeyEnv)
		if err != nil {
			return err
		}
		if s.Address() != planner.Caller {
			return errors.New("signer key does not match configured address")
		}
		fee, _ := parse.Uint256(cfg.TxManager.MaxFeePerGas)
		tip, _ := parse.Uint256(cfg.TxManager.MaxPriorityFeePerGas)
		txm, err := txmanager.New(client, s, chainID, store, txmanager.Config{Confirmations: cfg.TxManager.Confirmations, PollInterval: cfg.TxManager.PollInterval, MaxFeePerGas: fee, MaxPriorityFeePerGas: tip, GasLimit: cfg.TxManager.GasLimit}, log)
		if err != nil {
			return err
		}
		k.Sender = txm
	}
	if err := k.CheckRecovery(); err != nil {
		return err
	}
	metrics := observability.NewMetrics()
	k.Observe = metrics.Observe
	metrics.Ready(true)
	defer metrics.Ready(false)
	serverErrors := make(chan error, 1)
	if cfg.Observability.Addr != "" {
		server := metrics.Server(cfg.Observability.Addr)
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrors <- errors.New("observability server failed")
			}
		}()
		defer func() {
			shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		}()
	}
	log.Info("merkle executor started", "mode", cfg.Mode, "strategies", len(strategies), "chainId", cfg.Chain.ChainID)
	for {
		err := k.Tick(ctx)
		metrics.Ready(err == nil)
		if once {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		timer := time.NewTimer(cfg.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case err := <-serverErrors:
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

func bindState(store *state.Store, chainID uint64, caller string) error {
	type identity struct {
		ChainID uint64         `json:"chainId"`
		Caller  common.Address `json:"caller"`
	}
	want := identity{ChainID: chainID, Caller: common.HexToAddress(caller)}
	var old identity
	found, err := store.Load("deployment.v1", &old)
	if err != nil {
		return err
	}
	if found {
		if old != want {
			return errors.New("state directory belongs to another chain or caller")
		}
		return nil
	}
	return store.Save("deployment.v1", want)
}
