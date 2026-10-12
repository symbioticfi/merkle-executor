package redemptions

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func nativeNoonConfig() Config {
	input := uint(3)
	return Config{Family: "noon", Mode: "redeem-usn", ChainID: 1,
		Token:       "0xdA67B4284609d2d48e5d10cfAc411572727dc1eD",
		OutputAsset: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Target:      "0x9F8D0a683871125083170F65430F8A15aFfa0086",
		Amount:      "1234567890123456789", Action: "2:0", AmountInput: &input}
}

func nativeNoonFixture(t *testing.T) (Config, provider.Snapshot, *issuerReader) {
	t.Helper()
	cfg := nativeNoonConfig()
	s, r := issuerFixture(t, issuerConfig("noon"))
	r.results = map[string][]byte{}
	h, token := native.NewNoonRedeemHandlerV2().GetABI(), erc20.NewERC20().GetABI()
	target, asset, usn := common.HexToAddress(cfg.Target), common.HexToAddress(cfg.OutputAsset), common.HexToAddress(cfg.Token)
	amount, _ := new(big.Int).SetString(cfg.Amount, 10)
	for method, value := range map[string]any{"usnToken": usn, "treasury": issuerAddress(90), "redeemLimitPerBlock": new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)), "currentBlockRedeemAmount": big.NewInt(0), "lastRedeemBlock": big.NewInt(41)} {
		r.reply(target, h, method, nil, value)
	}
	r.reply(target, h, "redeemableCollaterals", []any{asset}, true)
	r.reply(target, h, "isWhitelisted", []any{s.Account}, true)
	r.reply(target, h, "collateralDecimals", []any{asset}, uint8(6))
	r.reply(target, h, "calculateCollateralAmount", []any{asset, amount}, big.NewInt(1_234_567))
	r.reply(asset, token, "balanceOf", []any{issuerAddress(90)}, big.NewInt(10_000_000))
	r.reply(asset, token, "allowance", []any{issuerAddress(90), target}, big.NewInt(10_000_000))
	return cfg, s, r
}

func TestNoonNativeRejectsUnsupportedConfiguration(t *testing.T) {
	for name, edit := range map[string]func(*Config){
		"chain": func(c *Config) { c.ChainID = 31337 }, "token": func(c *Config) { c.Token = issuerAddress(1).Hex() },
		"usdt":   func(c *Config) { c.OutputAsset = "0xdAC17F958D2ee523a2206206994597C13D831ec7" },
		"target": func(c *Config) { c.Target = issuerAddress(3).Hex() },
		"zero":   func(c *Config) { c.Amount = "0" }, "negative": func(c *Config) { c.Amount = "-1" },
		"overflow":   func(c *Config) { c.Amount = new(big.Int).Lsh(big.NewInt(1), 256).String() },
		"request-id": func(c *Config) { c.RequestID = "0" }, "epoch": func(c *Config) { c.Epoch = "0" },
		"second-action": func(c *Config) { c.SecondAction = "2:1" }, "intermediate-input": func(c *Config) { input := uint(4); c.IntermediateInput = &input },
		"cdo": func(c *Config) { c.CDO = issuerAddress(4).Hex() }, "strategy": func(c *Config) { c.Strategy = issuerAddress(5).Hex() },
	} {
		t.Run(name, func(t *testing.T) {
			cfg, _, r := nativeNoonFixture(t)
			edit(&cfg)
			if _, err := Factory(issuerNode(t, cfg), providerRegistry.Deps{Chain: r}); err == nil {
				t.Fatal("unsupported native Noon route accepted")
			}
		})
	}
}

func TestNoonNativePreparesFullAmountQuoteAndBatchesReadiness(t *testing.T) {
	cfg, s, r := nativeNoonFixture(t)
	batch := &batchIssuerReader{issuerReader: r, failIndex: -1}
	out, err := issuerProvider(t, cfg, batch).Prepare(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := new(big.Int).SetString(cfg.Amount, 10)
	assertIssuerAction(t, out.Actions[cfg.Action], native.NewNoonRedeemHandlerV2().GetABI(), "redeemOnchain", common.HexToAddress(cfg.OutputAsset), amount)
	if out.Inputs[3] != cfg.Amount || len(out.Actions) != 1 || len(out.SkipSteps) != 0 {
		t.Fatal("native amount changed units or installed extra actions")
	}
	if len(batch.groups) != 2 || batch.groups[0] != 9 || batch.groups[1] != 2 {
		t.Fatalf("readiness did not retain two pinned read stages: %v", batch.groups)
	}
	if got := issuerProvider(t, cfg, r).(*Provider).ExclusiveTargets()[cfg.Action]; got != common.HexToAddress(cfg.Target) {
		t.Fatal("native redeemer receipt target is not exclusive")
	}
}

func TestNoonNativeReadinessFailsClosed(t *testing.T) {
	for _, name := range []string{"wrong-usn", "wrong-decimals", "not-whitelisted", "unsupported-collateral", "no-treasury", "zero-quote", "malformed-quote", "stale-oracle", "rpc-failure", "empty-treasury", "low-treasury-allowance", "block-exhausted", "over-limit", "future-counter"} {
		t.Run(name, func(t *testing.T) {
			cfg, s, r := nativeNoonFixture(t)
			h, tokens := native.NewNoonRedeemHandlerV2().GetABI(), erc20.NewERC20().GetABI()
			target, asset := common.HexToAddress(cfg.Target), common.HexToAddress(cfg.OutputAsset)
			amount, _ := new(big.Int).SetString(cfg.Amount, 10)
			idleExpected := false
			switch name {
			case "wrong-usn":
				r.reply(target, h, "usnToken", nil, issuerAddress(888))
			case "wrong-decimals":
				r.reply(target, h, "collateralDecimals", []any{asset}, uint8(18))
			case "not-whitelisted":
				idleExpected = true
				r.reply(target, h, "isWhitelisted", []any{s.Account}, false)
			case "unsupported-collateral":
				idleExpected = true
				r.reply(target, h, "redeemableCollaterals", []any{asset}, false)
			case "no-treasury":
				idleExpected = true
				r.reply(target, h, "treasury", nil, common.Address{})
			case "zero-quote":
				idleExpected = true
				r.reply(target, h, "calculateCollateralAmount", []any{asset, amount}, big.NewInt(0))
			case "malformed-quote":
				r.results[callKey(target, native.NewNoonRedeemHandlerV2().PackCalculateCollateralAmount(asset, amount))] = []byte{1}
			case "stale-oracle":
				batch := &batchIssuerReader{issuerReader: r, failIndex: 8}
				if _, err := issuerProvider(t, cfg, batch).Prepare(t.Context(), s); err == nil || errors.Is(err, provider.ErrNotReady) {
					t.Fatal("stale oracle quote was accepted or treated as normal idle")
				}
				return
			case "rpc-failure":
				r.err = errors.New("RPC unavailable")
			case "empty-treasury":
				idleExpected = true
				r.reply(asset, tokens, "balanceOf", []any{issuerAddress(90)}, big.NewInt(0))
			case "low-treasury-allowance":
				idleExpected = true
				r.reply(asset, tokens, "allowance", []any{issuerAddress(90), target}, big.NewInt(1_234_566))
			case "block-exhausted":
				idleExpected = true
				r.reply(target, h, "lastRedeemBlock", nil, big.NewInt(42))
				r.reply(target, h, "redeemLimitPerBlock", nil, amount)
				r.reply(target, h, "currentBlockRedeemAmount", nil, big.NewInt(1))
			case "over-limit":
				idleExpected = true
				r.reply(target, h, "redeemLimitPerBlock", nil, new(big.Int).Sub(amount, big.NewInt(1)))
			case "future-counter":
				r.reply(target, h, "lastRedeemBlock", nil, big.NewInt(43))
			}
			out, err := issuerProvider(t, cfg, r).Prepare(t.Context(), s)
			if err == nil || errors.Is(err, provider.ErrNotReady) != idleExpected || len(out.Actions) != 0 {
				t.Fatalf("wrong readiness outcome: output=%+v error=%v", out, err)
			}
		})
	}
}

func TestNoonNativePriorBlockBudgetResetsAndBoundaryFits(t *testing.T) {
	for _, old := range []bool{true, false} {
		cfg, s, r := nativeNoonFixture(t)
		h := native.NewNoonRedeemHandlerV2().GetABI()
		target := common.HexToAddress(cfg.Target)
		amount, _ := new(big.Int).SetString(cfg.Amount, 10)
		r.reply(target, h, "redeemLimitPerBlock", nil, amount)
		if old {
			r.reply(target, h, "currentBlockRedeemAmount", nil, new(big.Int).Lsh(big.NewInt(1), 255))
		} else {
			r.reply(target, h, "lastRedeemBlock", nil, big.NewInt(42))
		}
		if _, err := issuerProvider(t, cfg, r).Prepare(t.Context(), s); err != nil {
			t.Fatalf("eligible counter boundary rejected: %v", err)
		}
	}
}

func noonReceiptLog(s provider.Snapshot, target common.Address, event abi.Event, from, to common.Address, data []byte) types.Log {
	return types.Log{Address: target, Topics: []common.Hash{event.ID, common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())}, Data: data, TxHash: s.Execution.TxHash, BlockHash: s.Execution.BlockHash, BlockNumber: s.Execution.BlockNumber.Uint64()}
}

func nativeNoonReceipt(t *testing.T, cfg Config, s provider.Snapshot, out provider.Output) []types.Log {
	t.Helper()
	h, tokens := native.NewNoonRedeemHandlerV2().GetABI(), erc20.NewERC20().GetABI()
	amount, _ := new(big.Int).SetString(cfg.Amount, 10)
	payment := big.NewInt(1_200_000) // Execution may differ from the old preview; the recipe owns minimum-output enforcement.
	redeemData, _ := h.Events["Redeemed"].Inputs.NonIndexed().Pack(amount, payment)
	burnData, _ := tokens.Events["Transfer"].Inputs.NonIndexed().Pack(amount)
	paymentData, _ := tokens.Events["Transfer"].Inputs.NonIndexed().Pack(payment)
	return []types.Log{
		issuerManageLog(t, s, []protocolio.ManagedCall{{Target: common.HexToAddress(cfg.Target), Data: out.Actions[cfg.Action]}}),
		noonReceiptLog(s, common.HexToAddress(cfg.Target), h.Events["Redeemed"], s.Account, common.HexToAddress(cfg.OutputAsset), redeemData),
		noonReceiptLog(s, common.HexToAddress(cfg.Token), tokens.Events["Transfer"], s.Account, common.Address{}, burnData),
		noonReceiptLog(s, common.HexToAddress(cfg.OutputAsset), tokens.Events["Transfer"], issuerAddress(90), s.Account, paymentData),
	}
}

func TestNoonNativeReconcilesBurnAndAccountPayment(t *testing.T) {
	for _, fault := range []string{"valid", "missing-manage", "wrong-managed-amount", "missing-redeemed", "foreign-recipient", "missing-burn", "missing-payment", "duplicate-redeemed", "changed-output"} {
		t.Run(fault, func(t *testing.T) {
			cfg, s, r := nativeNoonFixture(t)
			p := issuerProvider(t, cfg, r)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			s.Execution.Logs = nativeNoonReceipt(t, cfg, s, out)
			switch fault {
			case "missing-manage":
				s.Execution.Logs = s.Execution.Logs[1:]
			case "wrong-managed-amount":
				data := bytes.Clone(out.Actions[cfg.Action])
				data[len(data)-1] ^= 1
				s.Execution.Logs[0] = issuerManageLog(t, s, []protocolio.ManagedCall{{Target: common.HexToAddress(cfg.Target), Data: data}})
			case "missing-redeemed":
				s.Execution.Logs = append(s.Execution.Logs[:1], s.Execution.Logs[2:]...)
			case "foreign-recipient":
				s.Execution.Logs[1].Topics[1] = common.BytesToHash(issuerAddress(88).Bytes())
			case "missing-burn":
				s.Execution.Logs = append(s.Execution.Logs[:2], s.Execution.Logs[3:]...)
			case "missing-payment":
				s.Execution.Logs = s.Execution.Logs[:3]
			case "duplicate-redeemed":
				s.Execution.Logs = append(s.Execution.Logs, s.Execution.Logs[1])
			case "changed-output":
				out.Actions[cfg.Action] = bytes.Clone(out.Actions[cfg.Action])
				out.Actions[cfg.Action][len(out.Actions[cfg.Action])-1] ^= 1
			}
			for i := range s.Execution.Logs {
				s.Execution.Logs[i].Index = uint(i)
			}
			progress, err := p.Advance(t.Context(), s, out, nil)
			if fault == "valid" {
				if err != nil || !progress.Done || progress.Outcome != "redeemed" {
					t.Fatalf("native burn/payout did not complete: %+v %v", progress, err)
				}
			} else if err == nil || progress.Done {
				t.Fatalf("unproven native redemption completed: %+v %v", progress, err)
			}
		})
	}
}

func TestNoonNativeAcceptsCanonicalUSNToUSDCConfiguration(t *testing.T) {
	cfg := nativeNoonConfig()
	_, reader := issuerFixture(t, issuerConfig("noon"))
	if _, err := Factory(issuerNode(t, cfg), providerRegistry.Deps{Chain: reader}); err != nil {
		t.Fatalf("native USN redemption configuration rejected: %v", err)
	}
}
