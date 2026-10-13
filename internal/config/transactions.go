package config

import (
	"github.com/go-errors/errors"
	engine "github.com/symbioticfi/merkle-executor/internal/txengine"
	"math"
	"slices"
	"time"
)

// TxManager tunes the shared transaction sender.
type TxManager struct {
	// Confirmations to wait for before treating a transaction as final.
	Confirmations uint64        `yaml:"confirmations"`
	PollInterval  time.Duration `yaml:"pollInterval"`
	// MaxFeeGwei is the required absolute EIP-1559 max fee per gas.
	MaxFeeGwei float64 `yaml:"maxFeeGwei"`
	// BroadcastTimeoutMs bounds one transaction submission RPC call independently of replacement cadence.
	BroadcastTimeoutMs int `yaml:"broadcastTimeoutMs"`
	// AccountPollIntervalMs controls signer balance and nonce telemetry refresh cadence.
	AccountPollIntervalMs int `yaml:"accountPollIntervalMs"`
	// ReplacementIntervalMs paces the fallback fee bump of a pending transaction while fee history is
	// unreadable; with readable fee history, pending transactions are repriced on block evidence.
	ReplacementIntervalMs int `yaml:"replacementIntervalMs"`
	// PendingTimeoutMs bounds active tracking; unresolved recipes remain owned by the durable journal.
	PendingTimeoutMs int `yaml:"pendingTimeoutMs"`
	// ShutdownTimeoutMs bounds how long shutdown drains an accepted transaction lifecycle.
	ShutdownTimeoutMs int `yaml:"shutdownTimeoutMs"`
	// Horizon tunes fee pricing and block-evidence repricing.
	Horizon HorizonFeeConfig `yaml:"horizon"`
}

// HorizonFeeConfig tunes txManager fee pricing: the base-fee horizon of the fee cap, the tip rule,
// the repricing evidence, and gas-estimate headroom. Zero values select the defaults.
type HorizonFeeConfig struct {
	// MaxBlocks is how many blocks the initial fee cap keeps the full tip valid at the maximum
	// EIP-1559 base-fee increase.
	MaxBlocks int `yaml:"maxBlocks"`
	// BlockTimeMs is the chain slot time; it sets the evaluation cadence and the next-block estimate time.
	BlockTimeMs int `yaml:"blockTimeMs"`
	// TipFloorGwei is the tip while the last two blocks had room for the transaction.
	TipFloorGwei float64 `yaml:"tipFloorGwei"`
	// FullBlockTipGwei is the tip when one of the last two blocks had no room for it.
	FullBlockTipGwei float64 `yaml:"fullBlockTipGwei"`
	// CongestedTipFloorGwei and CongestedTipCapGwei clamp the reward-percentile tip used when both
	// of the last two blocks had no room.
	CongestedTipFloorGwei float64 `yaml:"congestedTipFloorGwei"`
	CongestedTipCapGwei   float64 `yaml:"congestedTipCapGwei"`
	// CongestedRewardBlocks and CongestedRewardPercentile pick that tip: the highest percentile
	// reward over this many recent blocks.
	CongestedRewardBlocks     int     `yaml:"congestedRewardBlocks"`
	CongestedRewardPercentile float64 `yaml:"congestedRewardPercentile"`
	// EscalateAfterFullBlocks is how many consecutive full blocks a valid pending transaction must
	// lose before its tip is repriced.
	EscalateAfterFullBlocks int `yaml:"escalateAfterFullBlocks"`
	// StallAfterBlocks is how many blocks with room a valid pending transaction may lose before it is
	// re-estimated and rebroadcast.
	StallAfterBlocks int `yaml:"stallAfterBlocks"`
	// GasHeadroomBps is the headroom over the next-block gas estimate; FallbackGasHeadroomBps applies
	// when the read RPC cannot estimate against the next block.
	GasHeadroomBps         int `yaml:"gasHeadroomBps"`
	FallbackGasHeadroomBps int `yaml:"fallbackGasHeadroomBps"`
}

// Fee pricing defaults, applied to unset txManager.horizon fields.
const (
	DefaultHorizonMaxBlocks                 = 6
	DefaultHorizonBlockTimeMs               = 12_000
	DefaultHorizonTipFloorGwei              = 0.02
	DefaultHorizonFullBlockTipGwei          = 0.1
	DefaultHorizonCongestedTipFloorGwei     = 0.2
	DefaultHorizonCongestedTipCapGwei       = 15
	DefaultHorizonCongestedRewardBlocks     = 3
	DefaultHorizonCongestedRewardPercentile = 50
	DefaultHorizonEscalateAfterFullBlocks   = 2
	DefaultHorizonStallAfterBlocks          = 3
	DefaultHorizonGasHeadroomBps            = 500
	DefaultHorizonFallbackGasHeadroomBps    = 1000

	maxHorizonBlocks       = 12
	maxHorizonWindowBlocks = 64
	maxGasHeadroomBps      = 5000
)

const (
	// Keep in sync with chain.defaultRPCAttemptTimeout (internal/chain/fallback.go).
	DefaultRPCAttemptTimeoutMs   = 20_000
	DefaultBroadcastTimeoutMs    = 5_000
	DefaultAccountPollIntervalMs = 30_000
	DefaultReplacementIntervalMs = 30_000
	DefaultPendingTimeoutMs      = 300_000
	DefaultShutdownTimeoutMs     = 60_000
)

func (c *TxManager) applyDefaults() {
	if c.BroadcastTimeoutMs == 0 {
		c.BroadcastTimeoutMs = DefaultBroadcastTimeoutMs
	}
	if c.AccountPollIntervalMs == 0 {
		c.AccountPollIntervalMs = DefaultAccountPollIntervalMs
	}
	if c.ReplacementIntervalMs == 0 {
		c.ReplacementIntervalMs = DefaultReplacementIntervalMs
	}
	if c.PendingTimeoutMs == 0 {
		c.PendingTimeoutMs = DefaultPendingTimeoutMs
	}
	if c.ShutdownTimeoutMs == 0 {
		c.ShutdownTimeoutMs = DefaultShutdownTimeoutMs
	}
	c.Horizon.applyDefaults()
}
func (c TxManager) validate(required bool) error {
	if c.MaxFeeGwei < 0 || required && c.MaxFeeGwei == 0 ||
		math.IsNaN(c.MaxFeeGwei) || math.IsInf(c.MaxFeeGwei, 0) {
		return errors.New("txManager.maxFeeGwei must be finite and positive")
	}
	if c.BroadcastTimeoutMs <= 0 {
		return errors.New("txManager.broadcastTimeoutMs must be positive")
	}
	if c.AccountPollIntervalMs <= 0 {
		return errors.New("txManager.accountPollIntervalMs must be positive")
	}
	if c.ReplacementIntervalMs <= 0 {
		return errors.New("txManager.replacementIntervalMs must be positive")
	}
	if c.PendingTimeoutMs < c.ReplacementIntervalMs {
		return errors.New("txManager.pendingTimeoutMs must be at least replacementIntervalMs")
	}
	if c.ShutdownTimeoutMs <= 0 {
		return errors.New("txManager.shutdownTimeoutMs must be positive")
	}
	for _, duration := range []int{c.BroadcastTimeoutMs, c.AccountPollIntervalMs, c.ReplacementIntervalMs, c.PendingTimeoutMs, c.ShutdownTimeoutMs} {
		if int64(duration) > math.MaxInt64/int64(time.Millisecond) {
			return errors.New("txManager duration exceeds time.Duration")
		}
	}
	return c.Horizon.validate()
}

func (h *HorizonFeeConfig) applyDefaults() {
	if h.MaxBlocks == 0 {
		h.MaxBlocks = DefaultHorizonMaxBlocks
	}
	if h.BlockTimeMs == 0 {
		h.BlockTimeMs = DefaultHorizonBlockTimeMs
	}
	if h.TipFloorGwei == 0 {
		h.TipFloorGwei = DefaultHorizonTipFloorGwei
	}
	if h.FullBlockTipGwei == 0 {
		h.FullBlockTipGwei = DefaultHorizonFullBlockTipGwei
	}
	if h.CongestedTipFloorGwei == 0 {
		h.CongestedTipFloorGwei = DefaultHorizonCongestedTipFloorGwei
	}
	if h.CongestedTipCapGwei == 0 {
		h.CongestedTipCapGwei = DefaultHorizonCongestedTipCapGwei
	}
	if h.CongestedRewardBlocks == 0 {
		h.CongestedRewardBlocks = DefaultHorizonCongestedRewardBlocks
	}
	if h.CongestedRewardPercentile == 0 {
		h.CongestedRewardPercentile = DefaultHorizonCongestedRewardPercentile
	}
	if h.EscalateAfterFullBlocks == 0 {
		h.EscalateAfterFullBlocks = DefaultHorizonEscalateAfterFullBlocks
	}
	if h.StallAfterBlocks == 0 {
		h.StallAfterBlocks = DefaultHorizonStallAfterBlocks
	}
	if h.GasHeadroomBps == 0 {
		h.GasHeadroomBps = DefaultHorizonGasHeadroomBps
	}
	if h.FallbackGasHeadroomBps == 0 {
		h.FallbackGasHeadroomBps = DefaultHorizonFallbackGasHeadroomBps
	}
}

func (h HorizonFeeConfig) validate() error {
	if h.MaxBlocks < 3 || h.MaxBlocks > maxHorizonBlocks {
		return errors.Errorf("txManager.horizon.maxBlocks must be between 3 and %d", maxHorizonBlocks)
	}
	if h.BlockTimeMs <= 0 || int64(h.BlockTimeMs) > math.MaxInt64/int64(time.Millisecond) {
		return errors.New("txManager.horizon.blockTimeMs must be positive and fit in a time.Duration")
	}
	tips := []float64{h.TipFloorGwei, h.FullBlockTipGwei, h.CongestedTipFloorGwei, h.CongestedTipCapGwei}
	for _, tip := range tips {
		if tip <= 0 || math.IsNaN(tip) || math.IsInf(tip, 0) {
			return errors.New("txManager.horizon tips must be finite and positive")
		}
	}
	if !slices.IsSorted(tips) {
		return errors.New("txManager.horizon tips must satisfy " +
			"tipFloorGwei <= fullBlockTipGwei <= congestedTipFloorGwei <= congestedTipCapGwei")
	}
	if math.IsNaN(h.CongestedRewardPercentile) || math.IsInf(h.CongestedRewardPercentile, 0) || h.CongestedRewardPercentile <= 0 || h.CongestedRewardPercentile > 100 {
		return errors.New("txManager.horizon.congestedRewardPercentile must be in (0, 100]")
	}
	for _, window := range []struct {
		name   string
		blocks int
	}{
		{"congestedRewardBlocks", h.CongestedRewardBlocks},
		{"escalateAfterFullBlocks", h.EscalateAfterFullBlocks},
		{"stallAfterBlocks", h.StallAfterBlocks},
	} {
		if window.blocks < 1 || window.blocks > maxHorizonWindowBlocks {
			return errors.Errorf("txManager.horizon.%s must be between 1 and %d", window.name, maxHorizonWindowBlocks)
		}
	}
	if h.GasHeadroomBps < 0 || h.FallbackGasHeadroomBps < h.GasHeadroomBps || h.FallbackGasHeadroomBps > maxGasHeadroomBps {
		return errors.Errorf(
			"txManager.horizon gas headroom must satisfy 0 <= gasHeadroomBps <= fallbackGasHeadroomBps <= %d",
			maxGasHeadroomBps,
		)
	}
	return nil
}

func (c TxManager) EngineConfig() engine.Config {
	h := c.Horizon
	return engine.Config{
		Confirmations: c.Confirmations, MaxFeeGwei: c.MaxFeeGwei, PollInterval: c.PollInterval,
		BroadcastTimeout:    time.Duration(c.BroadcastTimeoutMs) * time.Millisecond,
		AccountPollInterval: time.Duration(c.AccountPollIntervalMs) * time.Millisecond,
		ReplacementInterval: time.Duration(c.ReplacementIntervalMs) * time.Millisecond,
		PendingTimeout:      time.Duration(c.PendingTimeoutMs) * time.Millisecond,
		ShutdownTimeout:     time.Duration(c.ShutdownTimeoutMs) * time.Millisecond,
		Horizon: engine.HorizonConfig{
			MaxBlocks: h.MaxBlocks, BlockTime: time.Duration(h.BlockTimeMs) * time.Millisecond,
			TipFloorGwei: h.TipFloorGwei, FullBlockTipGwei: h.FullBlockTipGwei,
			CongestedTipFloorGwei: h.CongestedTipFloorGwei, CongestedTipCapGwei: h.CongestedTipCapGwei,
			CongestedRewardBlocks: h.CongestedRewardBlocks, CongestedRewardPercentile: h.CongestedRewardPercentile,
			EscalateAfterFullBlocks: h.EscalateAfterFullBlocks, StallAfterBlocks: h.StallAfterBlocks,
			GasHeadroomBps: h.GasHeadroomBps, FallbackGasHeadroomBps: h.FallbackGasHeadroomBps,
		}}
}

// validateRuntimePolicy permits old files only when their values match the fixed
// profile. The absolute fee ceiling remains explicit because it is a spend limit.
func (c TxManager) validateRuntimePolicy() error {
	want := TxManager{Confirmations: DefaultConfirmations, PollInterval: DefaultTransactionPollInterval, MaxFeeGwei: c.MaxFeeGwei}
	want.applyDefaults()
	if c != want {
		return errors.New("transaction timing, confirmations and horizon settings are fixed by the runtime policy; remove legacy overrides")
	}
	return nil
}
