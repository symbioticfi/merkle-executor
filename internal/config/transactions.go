package config

import (
	"math"
	"time"

	"github.com/go-errors/errors"
	engine "github.com/symbioticfi/merkle-executor/internal/txengine"
)

// TxManager exposes the deployment's absolute transaction spend limit.
type TxManager struct {
	MaxFeeGwei float64 `yaml:"maxFeeGwei"`
}

func (c TxManager) validate(required bool) error {
	if c.MaxFeeGwei < 0 || required && c.MaxFeeGwei == 0 || math.IsNaN(c.MaxFeeGwei) || math.IsInf(c.MaxFeeGwei, 0) {
		return errors.New("txManager.maxFeeGwei must be finite and positive")
	}
	return nil
}

// EngineConfig is the fixed runtime policy; only its fee ceiling is configurable.
func (c TxManager) EngineConfig() engine.Config {
	return engine.Config{
		Confirmations: DefaultConfirmations, MaxFeeGwei: c.MaxFeeGwei, PollInterval: DefaultTransactionPollInterval,
		BroadcastTimeout: 5 * time.Second, AccountPollInterval: 30 * time.Second,
		ReplacementInterval: 30 * time.Second, PendingTimeout: 5 * time.Minute, ShutdownTimeout: time.Minute,
		Horizon: engine.HorizonConfig{
			MaxBlocks: 6, BlockTime: 12 * time.Second, TipFloorGwei: 0.02, FullBlockTipGwei: 0.1,
			CongestedTipFloorGwei: 0.2, CongestedTipCapGwei: 15, CongestedRewardBlocks: 3, CongestedRewardPercentile: 50,
			EscalateAfterFullBlocks: 2, StallAfterBlocks: 3, GasHeadroomBps: 500, FallbackGasHeadroomBps: 1000,
		},
	}
}
