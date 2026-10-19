package redemptions

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestHastraIndependentActionsAndRecovery(t *testing.T) {
	for _, mode := range []string{"unwrap", "request"} {
		for _, fault := range []string{"valid", "wrong-asset", "read-failure", "pending", "wrong-receipt"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				cfg := issuerConfig("hastra")
				cfg.Mode = mode
				s, reader := issuerFixture(t, cfg)
				target := common.HexToAddress(cfg.Target)
				switch fault {
				case "wrong-asset":
					reader.reply(target, erc4626.NewERC4626().GetABI(), "asset", nil, issuerAddress(999))
				case "read-failure":
					reader.err = errors.New("read failed")
				case "pending":
					reader.reply(target, figure.NewFigureYieldVault().GetABI(), "pendingRedemptions", []any{s.Account}, big.NewInt(1), big.NewInt(1), big.NewInt(1))
				}
				p := issuerProvider(t, cfg, reader)
				out, err := p.Prepare(t.Context(), s)
				if fault == "wrong-asset" || fault == "read-failure" || (fault == "pending" && mode == "request") {
					if err == nil || (fault == "pending" && !errors.Is(err, provider.ErrNotReady)) {
						t.Fatalf("invalid Hastra state accepted: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				call := protocolio.ManagedCall{Target: target, Data: out.Actions[cfg.Action]}
				if fault == "wrong-receipt" {
					call.Target = issuerAddress(998)
				}
				s.Execution.Logs = append(s.Execution.Logs, issuerManageLog(t, s, []protocolio.ManagedCall{call}))
				before := reader.reads
				p = issuerProvider(t, cfg, reader)
				progress, err := p.Advance(t.Context(), s, out, nil)
				if fault == "wrong-receipt" {
					if err == nil || progress.Done {
						t.Fatal("unrelated receipt completed Hastra action")
					}
					return
				}
				want := "requested"
				if mode == "unwrap" {
					want = "redeemed"
				}
				if err != nil || !progress.Done || progress.Outcome != want || reader.reads != before {
					t.Fatalf("Hastra recovery repeated preparation or changed outcome: %+v %v", progress, err)
				}
			})
		}
	}
}

func TestFalconXBBDoesNotReadAAQueue(t *testing.T) {
	cfg := issuerConfig("pareto")
	cfg.Mode, cfg.Tranche = "direct-request", "bb"
	s, reader := issuerFixture(t, cfg)
	queue := common.HexToAddress(cfg.Target)
	for key := range reader.results {
		if len(key) >= len(queue.Hex()) && key[:len(queue.Hex())] == queue.Hex() {
			delete(reader.results, key)
		}
	}
	p, err := New(cfg, registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Prepare(t.Context(), s); err != nil {
		t.Fatalf("direct BB request depended on AA queue: %v", err)
	}
}
