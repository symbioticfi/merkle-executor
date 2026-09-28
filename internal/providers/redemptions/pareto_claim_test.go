package redemptions

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretostrategy"
)

func TestParetoDirectClaimIncludesAPR0Receipts(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		normal, principal, settled int64
		interest, rate, epoch      int64
		closed                     bool
		want                       string
	}{
		{name: "APR0-only", principal: 100, rate: 5e16, epoch: 7, want: "105"},
		{name: "mixed", normal: 40, principal: 100, settled: 20, interest: 3, rate: 5e16, epoch: 7, want: "168"},
		{name: "settled-only", settled: 20, interest: 3, want: "23"},
		{name: "round-down", principal: 7, rate: 1e17, epoch: 7, want: "7"},
		{name: "closed-current-epoch", principal: 100, settled: 20, interest: 3, epoch: 8, closed: true, want: "123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := issuerConfig("pareto")
			cfg.Mode, cfg.Amount = "direct-claim", ""
			s, r := issuerFixture(t, cfg)
			st, cdo := paretostrategy.NewParetoStrategy().GetABI(), paretocdo.NewParetoCDO().GetABI()
			target := common.HexToAddress(cfg.Strategy)
			r.reply(target, st, "withdrawsRequests", []any{s.Account}, big.NewInt(tc.normal))
			r.reply(target, st, "apr0Users", []any{s.Account}, big.NewInt(tc.principal), big.NewInt(tc.epoch), big.NewInt(tc.settled), big.NewInt(tc.interest))
			if tc.principal > 0 && tc.epoch < 8 {
				r.reply(target, st, "apr0RateByEpoch", []any{big.NewInt(tc.epoch)}, big.NewInt(tc.rate))
			}
			if tc.closed {
				r.reply(common.HexToAddress(cfg.CDO), cdo, "epochEndDate", nil, big.NewInt(0))
				r.reply(target, st, "lastWithdrawRequest", []any{s.Account}, big.NewInt(8))
			}
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if err != nil || out.Inputs[3] != tc.want {
				t.Fatalf("claimable APR0 receipt missed: amount=%s want=%s err=%v", out.Inputs[3], tc.want, err)
			}
			assertIssuerAction(t, out.Actions[cfg.Action], cdo, "claimWithdrawRequest")
		})
	}
}

func TestParetoAPR0RejectsIssuerArithmeticOverflow(t *testing.T) {
	for _, field := range []string{"interest", "total"} {
		t.Run(field, func(t *testing.T) {
			cfg := issuerConfig("pareto")
			cfg.Mode, cfg.Amount = "direct-claim", ""
			s, r := issuerFixture(t, cfg)
			st := paretostrategy.NewParetoStrategy().GetABI()
			target := common.HexToAddress(cfg.Strategy)
			maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
			principal, settled := big.NewInt(0), maximum
			if field == "interest" {
				principal, settled = maximum, big.NewInt(0)
				r.reply(target, st, "apr0RateByEpoch", []any{big.NewInt(7)}, big.NewInt(2))
			}
			r.reply(target, st, "apr0Users", []any{s.Account}, principal, big.NewInt(7), settled, big.NewInt(0))
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if err == nil || len(out.Actions) != 0 {
				t.Fatal("overflowing issuer arithmetic produced a claim")
			}
		})
	}
}
