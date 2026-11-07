package redemptions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	midas "github.com/symbioticfi/merkle-executor/api/bindings/midasredemption"
	noon "github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	openeden "github.com/symbioticfi/merkle-executor/api/bindings/openedenexpress"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretoqueue"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretostrategy"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type batchIssuerReader struct {
	*issuerReader
	groups    []int
	failIndex int
}

func (r *batchIssuerReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.groups = append(r.groups, len(requests))
	out := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		if request.Kind != protocolio.ReadCall || request.Call.From != (common.Address{}) {
			r.t.Fatal("issuer call context changed")
		}
		if i == r.failIndex {
			out[i].Err = errors.New("speculative unavailable")
			continue
		}
		out[i].Data, out[i].Err = r.CallContract(ctx, request.Call, request.Block)
	}
	return out, nil
}
func TestIssuerIndependentReadStagesBatchWithoutChangingPayload(t *testing.T) {
	for _, tc := range []struct {
		family, mode string
		count        int
	}{{"midas", "request", 4}, {"centrifuge", "request", 2}, {"centrifuge", "claim", 3}, {"openeden", "request", 4}, {"figure", "request", 4}, {"noon", "request", 3}, {"noon", "claim", 4}, {"pareto", "queue-request", 12}, {"pareto", "direct-request", 8}, {"pareto", "queue-claim", 12}, {"pareto", "direct-claim", 10}, {"pareto", "instant-claim", 7}} {
		t.Run(tc.family+"/"+tc.mode, func(t *testing.T) {
			cfg := issuerConfig(tc.family)
			cfg.Mode = tc.mode
			if tc.family == "pareto" && strings.HasSuffix(tc.mode, "claim") {
				cfg.Amount = ""
				if tc.mode == "queue-claim" {
					cfg.Epoch = "7"
				}
			}
			if tc.family == "noon" && tc.mode == "claim" {
				cfg.Amount = ""
				cfg.RequestID = "7"
			}
			snap, single := issuerFixture(t, cfg)
			if tc.family == "noon" && tc.mode == "claim" {
				a := noon.NewNoonWithdrawalHandler().GetABI()
				single.reply(common.HexToAddress(cfg.Target), a, "getWithdrawalRequest", []any{snap.Account, big.NewInt(7)}, struct {
					Amount    *big.Int
					Timestamp *big.Int
					Claimed   bool
				}{big.NewInt(50), big.NewInt(100), false})
				single.reply(common.HexToAddress(cfg.Target), a, "withdrawPeriod", nil, big.NewInt(200))
			}
			expected, err := issuerProvider(t, cfg, single).Prepare(context.Background(), snap)
			if err != nil {
				t.Fatal(err)
			}
			r := &batchIssuerReader{issuerReader: single, failIndex: -1}
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), snap)
			if err != nil {
				t.Fatal(err)
			}
			left, _ := json.Marshal(expected)
			right, _ := json.Marshal(out)
			if !bytes.Equal(left, right) {
				t.Fatal("batch changed authorized payload/inputs")
			}
			if len(r.groups) != 1 || r.groups[0] != tc.count {
				t.Fatalf("groups=%v want one group of%d", r.groups, tc.count)
			}
		})
	}
}
func TestIssuerSpeculativeErrorsDoNotMaskEarlierIdleOrTopology(t *testing.T) {
	for _, tc := range []struct {
		family, mode string
		fail         int
	}{{"midas", "request", 3}, {"openeden", "request", 3}, {"figure", "request", 3}, {"centrifuge", "claim", 2}, {"noon", "claim", 3}, {"pareto", "queue-request", 11}, {"pareto", "queue-claim", 10}, {"pareto", "direct-claim", 13}, {"pareto", "instant-claim", 10}} {
		t.Run(tc.family+"/"+tc.mode, func(t *testing.T) {
			cfg := issuerConfig(tc.family)
			cfg.Mode = tc.mode
			if tc.family == "pareto" && strings.HasSuffix(tc.mode, "claim") {
				cfg.Amount = ""
				if tc.mode == "queue-claim" {
					cfg.Epoch = "7"
				}
			}
			if tc.family == "noon" {
				cfg.Amount = ""
				cfg.RequestID = "7"
			}
			snap, r := issuerFixture(t, cfg)
			target := common.HexToAddress(cfg.Target)
			switch tc.family {
			case "midas":
				r.reply(target, midas.NewMidasRedemption().GetABI(), "paused", nil, true)
			case "openeden":
				r.reply(target, openeden.NewOpenEdenExpress().GetABI(), "pausedRedeem", nil, true)
			case "figure":
				r.reply(target, figure.NewFigureYieldVault().GetABI(), "pendingRedemptions", []any{snap.Account}, big.NewInt(1), big.NewInt(0), big.NewInt(0))
			case "centrifuge":
				r.reply(target, async.NewAsyncRedeemVault().GetABI(), "asset", nil, issuerAddress(999))
			case "noon":
				r.reply(target, noon.NewNoonWithdrawalHandler().GetABI(), "getWithdrawalRequest", []any{snap.Account, big.NewInt(7)}, struct {
					Amount    *big.Int
					Timestamp *big.Int
					Claimed   bool
				}{big.NewInt(0), big.NewInt(100), false})
			case "pareto":
				cdo, strategy := common.HexToAddress(cfg.CDO), common.HexToAddress(cfg.Strategy)
				switch tc.mode {
				case "queue-request":
					r.reply(cdo, paretocdo.NewParetoCDO().GetABI(), "defaulted", nil, true)
				case "queue-claim":
					r.reply(target, paretoqueue.NewParetoQueue().GetABI(), "userWithdrawalsEpochs", []any{snap.Account, big.NewInt(7)}, big.NewInt(0))
				case "direct-claim":
					r.reply(strategy, paretostrategy.NewParetoStrategy().GetABI(), "lastWithdrawRequest", []any{snap.Account}, big.NewInt(8))
				case "instant-claim":
					r.reply(strategy, paretostrategy.NewParetoStrategy().GetABI(), "instantWithdrawsRequests", []any{snap.Account}, big.NewInt(0))
				}
			}
			batch := &batchIssuerReader{issuerReader: r, failIndex: tc.fail}
			_, err := issuerProvider(t, cfg, batch).Prepare(context.Background(), snap)
			if len(batch.groups) != 1 {
				t.Fatal("provider did not exercise batch path")
			}
			if tc.family == "centrifuge" {
				if err == nil || !strings.Contains(err.Error(), "topology") {
					t.Fatalf("topology error masked:%v", err)
				}
			} else if !errors.Is(err, provider.ErrNotReady) {
				t.Fatalf("idle branch masked:%v", err)
			}
		})
	}
}
