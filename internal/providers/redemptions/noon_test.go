package redemptions

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	"github.com/symbioticfi/merkle-executor/api/protocolio"
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

func TestNoonRequestUsesHandlerReceiverAndTokenTarget(t *testing.T) {
	cfg := issuerConfig("noon")
	s, r := issuerFixture(t, cfg)
	p := issuerProvider(t, cfg, r)
	out, err := p.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	token, handler := common.HexToAddress(cfg.Token), common.HexToAddress(cfg.Target)
	assertIssuerAction(t, out.Actions[cfg.Action], erc4626.NewERC4626().GetABI(), "redeem", big.NewInt(100), handler, s.Account)
	// Independently encoded with cast calldata.
	if hexutil.Encode(out.Actions[cfg.Action]) != "0xba087652000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000063" {
		t.Fatal("request calldata differs from independent vector")
	}
	if out.Inputs[3] != "100" {
		t.Fatal("request input must remain denominated in sUSN shares")
	}
	s.Execution.Logs = []types.Log{issuerManageLog(t, s, []protocolio.ManagedCall{{Target: token, Data: out.Actions[cfg.Action]}})}
	progress, err := p.Advance(context.Background(), s, out, nil)
	if err != nil || !progress.Done || progress.Outcome != "requested" {
		t.Fatalf("request submission not reconciled: %+v %v", progress, err)
	}
	s.Execution.Logs = []types.Log{issuerManageLog(t, s, []protocolio.ManagedCall{{Target: handler, Data: out.Actions[cfg.Action]}})}
	if _, err = p.Advance(context.Background(), s, out, nil); err == nil {
		t.Fatal("request must execute on token, not handler")
	}

}

func TestNoonClaimUsesOwnedRequestAndLiveWithdrawalPeriod(t *testing.T) {
	for _, tt := range []struct {
		name      string
		amount    int64
		timestamp int64
		period    *big.Int
		claimed   bool
		ready     bool
		hardError bool
	}{
		{name: "matured", amount: 125, timestamp: 900, period: big.NewInt(99), ready: true},
		{name: "maturity-boundary", amount: 125, timestamp: 900, period: big.NewInt(100), ready: true},
		{name: "period-increased", amount: 125, timestamp: 900, period: big.NewInt(101)},
		{name: "future-request", amount: 125, timestamp: 1001, period: big.NewInt(0)},
		{name: "zero-period", amount: 125, timestamp: 1000, period: big.NewInt(0), ready: true},
		{name: "empty", amount: 0, timestamp: 900, period: big.NewInt(0)},
		{name: "claimed", amount: 125, timestamp: 900, period: big.NewInt(0), claimed: true},
		{name: "overflow", amount: 125, timestamp: 900, period: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)), hardError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := issuerConfig("noon")
			cfg.Mode, cfg.Amount, cfg.RequestID = "claim", "", "0"
			s, r := issuerFixture(t, cfg)
			a := noonwithdrawalhandler.NewNoonWithdrawalHandler()
			target := common.HexToAddress(cfg.Target)
			request := struct {
				Amount    *big.Int
				Timestamp *big.Int
				Claimed   bool
			}{big.NewInt(tt.amount), big.NewInt(tt.timestamp), tt.claimed}
			r.reply(target, a.GetABI(), "getWithdrawalRequest", []any{s.Account, big.NewInt(0)}, request)
			r.reply(target, a.GetABI(), "withdrawPeriod", nil, tt.period)
			// Revoking new-request authorization must not strand existing claims.
			delete(r.results, callKey(target, a.PackSTAKINGVAULTROLE()))
			delete(r.results, callKey(target, a.PackHasRole([32]byte{1}, common.HexToAddress(cfg.Token))))
			p := issuerProvider(t, cfg, r)
			out, err := p.Prepare(context.Background(), s)
			if !tt.ready {
				if err == nil || errors.Is(err, provider.ErrNotReady) == tt.hardError || len(out.Actions) != 0 {
					t.Fatalf("wrong claim readiness: %+v %v", out, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertIssuerAction(t, out.Actions[cfg.Action], a.GetABI(), "claimWithdrawal", big.NewInt(0))
			if hexutil.Encode(out.Actions[cfg.Action]) != "0xf84444360000000000000000000000000000000000000000000000000000000000000000" {
				t.Fatal("claim calldata differs from independent cast vector")
			}
			if out.Inputs[3] != "125" {
				t.Fatalf("claim input must use USN entitlement: %s", out.Inputs[3])
			}
			s.Execution.Logs = []types.Log{issuerManageLog(t, s, []protocolio.ManagedCall{{Target: target, Data: out.Actions[cfg.Action]}})}
			progress, err := p.Advance(context.Background(), s, out, nil)
			if err != nil || !progress.Done || progress.Outcome != "claimed" {
				t.Fatalf("claim not reconciled: %+v %v", progress, err)
			}
			s.Execution.Logs = []types.Log{issuerManageLog(t, s, []protocolio.ManagedCall{{Target: target, Data: a.PackClaimWithdrawal(big.NewInt(1))}})}
			if _, err = p.Advance(context.Background(), s, out, nil); err == nil {
				t.Fatal("different request claim was accepted")
			}
		})
	}
}

func TestNoonRejectsMismatchedAssetsAndUnauthorizedVault(t *testing.T) {
	for _, fault := range []string{"token-asset", "handler-asset", "vault-role", "malformed-role", "malformed-authorization"} {
		t.Run(fault, func(t *testing.T) {
			cfg := issuerConfig("noon")
			s, r := issuerFixture(t, cfg)
			token, target := common.HexToAddress(cfg.Token), common.HexToAddress(cfg.Target)
			h := noonwithdrawalhandler.NewNoonWithdrawalHandler()
			switch fault {
			case "token-asset":
				r.reply(token, erc4626.NewERC4626().GetABI(), "asset", nil, issuerAddress(888))
			case "handler-asset":
				r.reply(target, h.GetABI(), "usn", nil, issuerAddress(888))
			case "vault-role":
				r.reply(target, h.GetABI(), "hasRole", []any{[32]byte{1}, token}, false)
			case "malformed-role":
				r.results[callKey(target, h.PackSTAKINGVAULTROLE())] = []byte{1}
			case "malformed-authorization":
				r.results[callKey(target, h.PackHasRole([32]byte{1}, token))] = []byte{1}
			}
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if err == nil || errors.Is(err, provider.ErrNotReady) || len(out.Actions) != 0 {
				t.Fatalf("invalid Noon topology accepted or treated as idle: %+v %v", out, err)
			}
		})
	}
}

func TestNoonClaimRejectsUnreadableRequestOrPeriod(t *testing.T) {
	for _, fault := range []string{"request", "period"} {
		t.Run(fault, func(t *testing.T) {
			cfg := issuerConfig("noon")
			cfg.Mode, cfg.Amount, cfg.RequestID = "claim", "", "0"
			s, r := issuerFixture(t, cfg)
			h := noonwithdrawalhandler.NewNoonWithdrawalHandler()
			target := common.HexToAddress(cfg.Target)
			request := struct {
				Amount    *big.Int
				Timestamp *big.Int
				Claimed   bool
			}{big.NewInt(125), big.NewInt(900), false}
			r.reply(target, h.GetABI(), "getWithdrawalRequest", []any{s.Account, big.NewInt(0)}, request)
			r.reply(target, h.GetABI(), "withdrawPeriod", nil, big.NewInt(100))
			data := h.PackGetWithdrawalRequest(s.Account, big.NewInt(0))
			if fault == "period" {
				data = h.PackWithdrawPeriod()
			}
			r.results[callKey(target, data)] = []byte{1}
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if err == nil || errors.Is(err, provider.ErrNotReady) || len(out.Actions) != 0 {
				t.Fatalf("unreadable Noon claim treated as claimable or idle: %+v %v", out, err)
			}
		})
	}
}

func TestNoonClaimConfigurationRequiresExplicitRequestID(t *testing.T) {
	for _, tt := range []struct {
		name string
		edit func(*Config)
	}{
		{"missing-id", func(c *Config) { c.RequestID = "" }},
		{"negative-id", func(c *Config) { c.RequestID = "-1" }},
		{"overflow-id", func(c *Config) { c.RequestID = new(big.Int).Lsh(big.NewInt(1), 256).String() }},
		{"partial-claim", func(c *Config) { c.Amount = "10" }},
		{"request-id", func(c *Config) { c.Mode, c.Amount = "request", "100" }},
		{"other-family", func(c *Config) { c.Family, c.Amount = "centrifuge", "100" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := issuerConfig("noon")
			cfg.Mode, cfg.Amount, cfg.RequestID = "claim", "", "0"
			tt.edit(&cfg)
			_, r := issuerFixture(t, cfg)
			if _, err := Factory(issuerNode(t, cfg), provider.Deps{Chain: r}); err == nil {
				t.Fatal("invalid Noon configuration accepted")
			}
		})
	}
}
