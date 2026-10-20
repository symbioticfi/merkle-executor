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
	"github.com/symbioticfi/merkle-executor/internal/curator"
	"github.com/symbioticfi/merkle-executor/internal/observability"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlanerfq"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/aave"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func runBot(ctx context.Context, path string, once, planOnly bool, out io.Writer) error {
	return runBotWithHTTP(ctx, path, once, planOnly, out, nil)
}

// runBotWithHTTP permits a bounded transport to serve official API fixtures
// while preserving the production endpoints and all recipe validation.
func runBotWithHTTP(ctx context.Context, path string, once, planOnly bool, out io.Writer, httpClient *http.Client) error {
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
	startup, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
	client, err := chain.Dial(startup, cfg.Chain.RPCURL, cfg.Chain.WriteRPCURL, chainID, common.HexToAddress(config.StandardMulticall3))
	cancel()
	if err != nil {
		return err
	}
	defer client.Close()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: config.DefaultHTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	source, err := curator.New(cfg.Curator.HTTPConfig(), httpClient, client)
	if err != nil {
		return err
	}
	targets := make([]common.Address, len(cfg.Strategies))
	for i, sc := range cfg.Strategies {
		targets[i] = common.HexToAddress(sc)
	}
	discovery, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
	accounts, caller, err := discoverIdentity(discovery, client, targets)
	cancel()
	if err != nil {
		return err
	}
	if cfg.Signer.Address != "" && common.HexToAddress(cfg.Signer.Address) != caller {
		return errors.New("signer.address assertion does not match executor caller")
	}
	var txSigner signer.Signer
	if cfg.Mode == "send" {
		txSigner, err = signer.FromEnv(cfg.Signer.KeyEnv)
		if err != nil {
			return err
		}
		if txSigner.Address() != caller {
			return errors.New("signer key does not match executor caller")
		}
	}
	strategies := make([]keeper.Strategy, 0, len(cfg.Strategies))
	for _, sc := range cfg.Strategies {
		s := keeper.Strategy{Config: keeper.Config{Name: sc, Executor: sc, Account: accounts[common.HexToAddress(sc)].Hex(), Interval: config.DefaultStrategyInterval, Revision: cfg.StrategyFingerprint(sc, accounts[common.HexToAddress(sc)], caller)}}
		for _, registration := range []struct {
			name string
			new  func(providerRegistry.Deps) (provider.Provider, error)
		}{{"cowswap", cowswap.NewAutomatic}, {"merkl", merkl.NewAutomatic}, {"liquidlane-rfq", liquidlanerfq.NewAutomatic}} {
			p, err := registration.new(providerRegistry.Deps{Chain: client, HTTP: httpClient})
			if err != nil {
				return errors.Errorf("configure provider %s: %w", registration.name, err)
			}
			s.Providers = append(s.Providers, p)
		}
		capacity, err := aave.New(client)
		if err != nil {
			return errors.Errorf("configure Aave capacity: %w", err)
		}
		s.Providers = append(s.Providers, capacity, keeper.NewRecipeInputs())
		strategies = append(strategies, s)
	}
	planner := &keeper.Planner{PermissionSource: source, Chain: client, ChainID: chainID, Caller: caller, Confirmations: config.DefaultConfirmations, RecipeCache: recipes.NewRecipeCache(len(cfg.Strategies), config.DefaultRecipeCacheBytes)}
	if planOnly {
		plans := make(map[string]keeper.Plan)
		for _, s := range strategies {
			operation, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
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
	if err := bindState(store, cfg.Chain.ChainID, caller.Hex()); err != nil {
		return err
	}
	metrics := observability.NewMetrics()
	k := &keeper.Keeper{Store: store, Planner: planner, Strategies: strategies, Mode: cfg.Mode, OperationTimeout: config.DefaultOperationTimeout, Log: log}
	if cfg.Mode == "send" {
		txm, err := txmanager.NewWithMetrics(client, txSigner, chainID, store, cfg.TxManager.EngineConfig(), log, metrics.Registerer())
		if err != nil {
			return err
		}
		if err := txm.Start(ctx); err != nil {
			return err
		}
		defer txm.Close()
		k.Sender = txm
	}
	if err := k.CheckRecovery(); err != nil {
		return err
	}
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
		timer := time.NewTimer(config.DefaultPollInterval)
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
