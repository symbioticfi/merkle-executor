// Package service composes the runtime, integrations and bounded I/O once per process.
package service

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
	"github.com/symbioticfi/merkle-executor/internal/observability"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/transaction"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type Options struct {
	Once, PlanOnly bool
	Output         io.Writer
	HTTP           *http.Client
}

func Run(ctx context.Context, cfg *config.Config, opts Options) (err error) {
	mode := cfg.Mode
	if opts.PlanOnly {
		mode = "simulate"
	}
	log, flush, err := observability.NewLogger(cfg.Observability.Debug)
	if err != nil {
		return err
	}
	defer flush()
	drained := true
	// Process ownership precedes network activity. Plan is a read-only command.
	var store *state.Store
	if !opts.PlanOnly {
		store, err = state.Open(cfg.StateDir)
		if err != nil {
			return err
		}
		defer func() {
			if drained {
				err = errors.Join(err, store.Close())
			}
		}()
	}
	chainID := new(big.Int).SetUint64(cfg.Chain.ChainID)
	if store != nil {
		if err := rt.Preflight(store, chainID, common.HexToAddress(cfg.Signer.Address), cfg.Strategies); err != nil {
			return err
		}
	}
	startup, cancel := context.WithTimeout(ctx, cfg.Runtime.OperationTimeout)
	client, err := chain.Dial(startup, cfg.Chain.RPCURL, cfg.Chain.WriteRPCURL, chainID, common.HexToAddress(cfg.Chain.Multicall3))
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		if drained {
			client.Close()
		}
	}()
	reads, err := chainio.NewScheduler(client, cfg.Reads)
	if err != nil {
		return err
	}
	readCtx, stopReads := context.WithCancel(context.WithoutCancel(ctx))
	readDone := make(chan error, 1)
	go func() { readDone <- reads.Run(readCtx) }()
	defer func() {
		if drained {
			_ = reads.Close()
			stopReads()
			<-readDone
		}
	}()
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: cfg.Runtime.HTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		defer func() {
			if drained {
				opts.HTTP.CloseIdleConnections()
			}
		}()
	}
	source, err := permissions.New(cfg.Curator.HTTPConfig(), opts.HTTP)
	if err != nil {
		return err
	}
	targets := make([]common.Address, len(cfg.Strategies))
	for i, name := range cfg.Strategies {
		targets[i] = common.HexToAddress(name)
	}
	discovery, cancel := context.WithTimeout(ctx, cfg.Runtime.OperationTimeout)
	accounts, caller, err := DiscoverIdentity(discovery, reads, targets)
	cancel()
	if err != nil {
		return err
	}
	if cfg.Signer.Address != "" && common.HexToAddress(cfg.Signer.Address) != caller {
		return errors.New("signer.address assertion does not match executor caller")
	}
	registered, converters, err := integrations(registry.Deps{Chain: reads, HTTP: opts.HTTP})
	if err != nil {
		return err
	}
	strategies := make([]rt.Strategy, len(cfg.Strategies))
	legacy := make(map[string][]rt.LegacyConverter, len(strategies))
	for i, name := range cfg.Strategies {
		account := accounts[common.HexToAddress(name)]
		current := rt.Config{Name: name, Executor: name, Account: account.Hex(), Interval: cfg.Runtime.StrategyInterval, Revision: cfg.StrategyFingerprint(name, account, caller)}
		old := current
		old.Interval = time.Minute // exact v1 serializer, independent of v2 scheduling policy
		old.Revision = cfg.LegacyStrategyFingerprint(name, account, caller)
		strategies[i] = rt.Strategy{Config: current, Integrations: registered, LegacyConfigHash: old.Fingerprint()}
		legacy[name] = converters
	}
	planner := &rt.Planner{PermissionSource: source, Chain: reads, ChainID: chainID, Caller: caller, Confirmations: cfg.Transaction.Confirmations, RecipeCache: recipes.NewRecipeCache(len(strategies), cfg.Runtime.RecipeCacheBytes), MaxConcurrent: cfg.Runtime.MaxPreparations}
	if opts.PlanOnly {
		operation, cancel := context.WithTimeout(ctx, cfg.Runtime.OperationTimeout)
		defer cancel()
		results := planner.PrepareAll(operation, strategies)
		plans := make(map[string]rt.Plan, len(results))
		for i, result := range results {
			if result.Err != nil {
				return errors.Errorf("prepare %s: %w", strategies[i].Config.Name, result.Err)
			}
			plans[strategies[i].Config.Name] = result.Plan
		}
		encoder := json.NewEncoder(opts.Output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plans)
	}
	var driver rt.Driver
	if mode == "send" {
		signing, err := signer.FromEnv(cfg.Signer.KeyEnv)
		if err != nil {
			return err
		}
		if signing.Address() != caller {
			return errors.New("signer key does not match executor caller")
		}
		driver, err = transaction.New(client, client, signing, chainID, cfg.Transaction)
		if err != nil {
			return err
		}
	}
	metrics := observability.NewMetrics()
	worker := &rt.Runtime{Store: store, Planner: planner, Driver: driver, Strategies: strategies, Mode: mode, OperationTimeout: cfg.Runtime.OperationTimeout, ShutdownTimeout: cfg.Runtime.ShutdownTimeout, MaxConcurrent: cfg.Runtime.MaxPreparations, Log: log, Observe: metrics.Observe, ChainID: chainID, Caller: caller, LegacyConverters: legacy}
	if err := worker.Start(ctx); err != nil {
		return err
	}
	defer func() {
		closeErr := worker.Close()
		if closeErr != nil {
			drained = false
		}
		err = errors.Join(err, closeErr)
	}()
	metrics.Ready(true)
	defer metrics.Ready(false)
	serverErrors := make(chan error, 1)
	if cfg.Observability.Addr != "" {
		server := metrics.Server(cfg.Observability.Addr)
		go func() {
			if e := server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
				serverErrors <- errors.New("observability server failed")
			}
		}()
		defer func() {
			shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Runtime.ShutdownTimeout)
			defer cancel()
			err = errors.Join(err, server.Shutdown(shutdown))
		}()
	}
	log.Info("merkle executor started", "mode", mode, "strategies", len(strategies), "chainId", cfg.Chain.ChainID)
	for {
		err := worker.Tick(ctx)
		metrics.Ready(err == nil)
		if opts.Once {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		timer := time.NewTimer(cfg.Runtime.PollInterval)
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

// DiscoverIdentity accepts one caller for all executors in this process.
func DiscoverIdentity(ctx context.Context, reader recipes.Reader, executors []common.Address) (map[common.Address]common.Address, common.Address, error) {
	identities, err := recipes.DiscoverExecutors(ctx, reader, executors)
	if err != nil {
		return nil, common.Address{}, err
	}
	accounts := make(map[common.Address]common.Address, len(identities))
	var caller common.Address
	for target, identity := range identities {
		if caller != (common.Address{}) && caller != identity.Caller {
			return nil, common.Address{}, errors.New("executors require different callers; use separate processes")
		}
		caller, accounts[target] = identity.Caller, identity.Account
	}
	if caller == (common.Address{}) {
		return nil, common.Address{}, errors.New("no executor caller discovered")
	}
	return accounts, caller, nil
}
