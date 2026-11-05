package main

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/observability"
	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/aave"
	"github.com/symbioticfi/merkle-executor/sdk/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/sdk/providers/liquidlanerfq"
	"github.com/symbioticfi/merkle-executor/sdk/providers/merkl"
	"github.com/symbioticfi/merkle-executor/sdk/providers/threef"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
	"github.com/symbioticfi/merkle-executor/sdk/state"
	"github.com/symbioticfi/merkle-executor/sdk/txmanager"
)

type environment struct {
	cfg        *config.Config
	client     *chain.Client
	planner    *keeper.Planner
	strategies []keeper.Strategy
	log        logr.Logger
	close      func()
}

func planBot(ctx context.Context, path string, out io.Writer) error {
	return planBotWithHTTP(ctx, path, out, nil)
}

func planBotWithHTTP(ctx context.Context, path string, out io.Writer, httpClient *http.Client) error {
	env, err := openEnvironment(ctx, path, httpClient)
	if err != nil {
		return err
	}
	defer env.close()
	operation, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
	defer cancel()
	plan, err := env.planner.PrepareExecution(operation, env.strategies)
	if err != nil {
		return err
	}
	to, data, err := plan.Transaction()
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(struct {
		To   common.Address       `json:"to"`
		Data hexutil.Bytes        `json:"data"`
		Plan keeper.ExecutionPlan `json:"plan"`
	}{to, data, plan})
}

// openEnvironment discovers execution configuration without keys or signing state.
func openEnvironment(ctx context.Context, path string, httpClient *http.Client) (_ *environment, err error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	log, flush, err := observability.NewLogger(cfg.Observability.Debug)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			flush()
		}
	}()
	chainID := new(big.Int).SetUint64(cfg.Chain.ChainID)
	startup, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
	client, err := chain.Dial(startup, cfg.Chain.RPCURL, cfg.Chain.WriteRPCURL, chainID, common.HexToAddress(config.StandardMulticall3))
	cancel()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: config.DefaultHTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	source, err := permissions.New(cfg.Curator.URL, httpClient, client)
	if err != nil {
		return nil, err
	}
	targets := make([]common.Address, len(cfg.Strategies))
	for i, sc := range cfg.Strategies {
		targets[i] = common.HexToAddress(sc)
	}
	discovery, cancel := context.WithTimeout(ctx, config.DefaultOperationTimeout)
	accounts, caller, err := discoverIdentity(discovery, client, targets)
	cancel()
	if err != nil {
		return nil, err
	}
	if cfg.Signer.Address != "" && common.HexToAddress(cfg.Signer.Address) != caller {
		return nil, errors.New("signer.address assertion does not match executor caller")
	}
	strategies := make([]keeper.Strategy, 0, len(cfg.Strategies))
	for _, sc := range cfg.Strategies {
		s := keeper.Strategy{Config: keeper.Config{Name: sc, Executor: sc, Account: accounts[common.HexToAddress(sc)].Hex(), Interval: config.DefaultStrategyInterval, Revision: cfg.StrategyFingerprint(sc, accounts[common.HexToAddress(sc)], caller)}}
		for _, registration := range []struct {
			name string
			new  func(provider.Deps) (keeper.Provider, error)
		}{{"cowswap", cowswap.NewAutomatic}, {"merkl", merkl.NewAutomatic}, {"liquidlane-rfq", liquidlanerfq.NewAutomatic}} {
			p, err := registration.new(provider.Deps{Chain: client, HTTP: httpClient, LookupSecret: os.Getenv})
			if err != nil {
				return nil, errors.Errorf("configure provider %s: %w", registration.name, err)
			}
			s.Providers = append(s.Providers, p)
		}
		capacity, err := aave.New(client)
		if err != nil {
			return nil, errors.Errorf("configure Aave capacity: %w", err)
		}
		claims, err := threef.NewAutomatic(provider.Deps{Chain: client})
		if err != nil {
			return nil, errors.Errorf("configure 3F claims: %w", err)
		}
		s.Providers = append(s.Providers, capacity, claims)
		strategies = append(strategies, s)
	}
	planner := &keeper.Planner{PermissionSource: source, Chain: client, ChainID: chainID, Caller: caller, Confirmations: config.DefaultConfirmations, RecipeCache: recipes.NewRecipeCache(len(cfg.Strategies), config.DefaultRecipeCacheBytes)}
	return &environment{cfg: cfg, client: client, planner: planner, strategies: strategies, log: log,
		close: func() { client.Close(); flush() }}, nil
}

func runBot(ctx context.Context, path string, once bool) error {
	return runBotWithHTTP(ctx, path, once, nil)
}

// runBotWithHTTP permits bounded official API fixture transports.
func runBotWithHTTP(ctx context.Context, path string, once bool, httpClient *http.Client) error {
	env, err := openEnvironment(ctx, path, httpClient)
	if err != nil {
		return err
	}
	defer env.close()
	cfg, client, planner, strategies, log := env.cfg, env.client, env.planner, env.strategies, env.log
	caller, chainID := planner.Caller, planner.ChainID
	var txSigner signer.Signer
	if cfg.Mode == "send" {
		txSigner, err = signerFromEnv(cfg.Signer.KeyEnv)
		if err != nil {
			return err
		}
		if txSigner.Address() != caller {
			return errors.New("signer key does not match executor caller")
		}
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
