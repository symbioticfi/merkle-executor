package redemptions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	share "github.com/symbioticfi/merkle-executor/api/bindings/centrifugeshare"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	midas "github.com/symbioticfi/merkle-executor/api/bindings/midasredemption"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	openeden "github.com/symbioticfi/merkle-executor/api/bindings/openedenexpress"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretoqueue"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretostrategy"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type issuerReader struct {
	t       *testing.T
	block   *big.Int
	results map[string][]byte
	headers map[uint64]*types.Header
	err     error
	reads   int
}

func callKey(target common.Address, data []byte) string { return target.Hex() + string(data) }

func (r *issuerReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.t.Helper()
	r.reads++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if block == nil || block.Cmp(r.block) != 0 {
		r.t.Fatalf("issuer read did not use pinned block: %v", block)
	}
	if r.err != nil {
		return nil, r.err
	}
	if msg.To == nil {
		r.t.Fatal("issuer read has no target")
	}
	result, ok := r.results[callKey(*msg.To, msg.Data)]
	if !ok {
		r.t.Fatalf("unexpected issuer target or parameters: target=%s data=%x", msg.To, msg.Data)
	}
	return bytes.Clone(result), nil
}

func (r *issuerReader) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if h := r.headers[number.Uint64()]; h != nil {
		return types.CopyHeader(h), nil
	}
	return nil, ethereum.NotFound
}

func (r *issuerReader) reply(target common.Address, contract abi.ABI, method string, inputs []any, outputs ...any) {
	r.t.Helper()
	data, err := contract.Pack(method, inputs...)
	if err != nil {
		r.t.Fatal(err)
	}
	encoded, err := contract.Methods[method].Outputs.Pack(outputs...)
	if err != nil {
		r.t.Fatal(err)
	}
	r.results[callKey(target, data)] = encoded
}

func issuerAddress(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }

func issuerNode(t *testing.T, value any) yaml.Node {
	t.Helper()
	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var raw yaml.Node
	if err = yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func issuerConfig(family string) Config {
	input := uint(3)
	cfg := Config{Family: family, Mode: "request", ChainID: 1, Token: issuerAddress(1).Hex(), OutputAsset: issuerAddress(2).Hex(), Target: issuerAddress(3).Hex(), Amount: "100", Action: "2:0", AmountInput: &input}
	if family == "figure" {
		intermediate := uint(4)
		cfg.SecondAction, cfg.IntermediateInput = "2:1", &intermediate
	}
	if family == "pareto" {
		cfg.Mode, cfg.CDO, cfg.Strategy = "queue-request", issuerAddress(4).Hex(), issuerAddress(5).Hex()
	}
	return cfg
}

func issuerFixture(t *testing.T, cfg Config) (provider.Snapshot, *issuerReader) {
	t.Helper()
	receipt := &types.Header{Number: big.NewInt(41), Time: 900}
	pinned := &types.Header{Number: big.NewInt(42), Time: 1000, ParentHash: receipt.Hash()}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(42), BlockHash: pinned.Hash(), Timestamp: 1000, Account: issuerAddress(99), Execution: &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(41), BlockHash: receipt.Hash()}}
	r := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}, headers: map[uint64]*types.Header{41: receipt, 42: pinned}}
	token, asset, target := common.HexToAddress(cfg.Token), common.HexToAddress(cfg.OutputAsset), common.HexToAddress(cfg.Target)
	amount, ok := new(big.Int).SetString(cfg.Amount, 10)
	if !ok {
		amount = big.NewInt(100)
	}
	switch cfg.Family {
	case "midas":
		a := midas.NewMidasRedemption().GetABI()
		r.reply(target, a, "mToken", nil, token)
		r.reply(target, a, "tokensConfig", []any{asset}, issuerAddress(90), big.NewInt(0), big.NewInt(10000), true)
		r.reply(target, a, "paused", nil, false)
		r.reply(target, a, "minAmount", nil, big.NewInt(10))
	case "centrifuge":
		r.reply(token, share.NewCentrifugeShare().GetABI(), "vault", []any{asset}, target)
		a := async.NewAsyncRedeemVault().GetABI()
		r.reply(target, a, "asset", nil, asset)
		r.reply(target, a, "maxRedeem", []any{s.Account}, big.NewInt(80))
	case "openeden":
		a := openeden.NewOpenEdenExpress().GetABI()
		r.reply(target, a, "token", nil, token)
		r.reply(target, a, "redeemAsset", nil, asset)
		r.reply(target, a, "pausedRedeem", nil, false)
		r.reply(target, a, "redeemMinimum", nil, big.NewInt(10))
	case "figure":
		a, v := figure.NewFigureYieldVault().GetABI(), erc4626.NewERC4626().GetABI()
		r.reply(token, v, "asset", nil, target)
		r.reply(target, a, "asset", nil, asset)
		r.reply(target, a, "pendingRedemptions", []any{s.Account}, big.NewInt(0), big.NewInt(0), big.NewInt(0))
		r.reply(token, v, "previewRedeem", []any{amount}, big.NewInt(125))
	case "noon":
		r.reply(token, erc4626.NewERC4626().GetABI(), "asset", nil, asset)
		h := noonwithdrawalhandler.NewNoonWithdrawalHandler().GetABI()
		r.reply(target, h, "usn", nil, asset)
		r.reply(target, h, "STAKING_VAULT_ROLE", nil, [32]byte{1})
		r.reply(target, h, "hasRole", []any{[32]byte{1}, token}, true)
	case "pareto":
		cdo, strategy := common.HexToAddress(cfg.CDO), common.HexToAddress(cfg.Strategy)
		q, c, st := paretoqueue.NewParetoQueue().GetABI(), paretocdo.NewParetoCDO().GetABI(), paretostrategy.NewParetoStrategy().GetABI()
		r.reply(target, q, "tranche", nil, token)
		r.reply(target, q, "underlying", nil, asset)
		r.reply(target, q, "idleCDOEpoch", nil, cdo)
		r.reply(target, q, "strategy", nil, strategy)
		r.reply(cdo, c, "AATranche", nil, token)
		r.reply(cdo, c, "token", nil, asset)
		r.reply(cdo, c, "strategy", nil, strategy)
		r.reply(strategy, st, "idleCDO", nil, cdo)
		r.reply(strategy, st, "token", nil, asset)
		r.reply(cdo, c, "defaulted", nil, false)
		r.reply(cdo, c, "isWalletAllowed", []any{s.Account}, true)
		r.reply(cdo, c, "isEpochRunning", nil, cfg.Mode != "direct-request")
		r.reply(strategy, st, "epochNumber", nil, big.NewInt(8))
		epoch, valid := new(big.Int).SetString(cfg.Epoch, 10)
		if !valid {
			epoch = big.NewInt(7)
		}
		r.reply(target, q, "userWithdrawalsEpochs", []any{s.Account, epoch}, big.NewInt(30))
		r.reply(target, q, "epochWithdrawPrice", []any{epoch}, big.NewInt(1e18))
		r.reply(target, q, "epochPendingClaims", []any{epoch}, big.NewInt(0))
		r.reply(strategy, st, "withdrawsRequests", []any{s.Account}, big.NewInt(40))
		r.reply(strategy, st, "apr0Users", []any{s.Account}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0))
		r.reply(strategy, st, "instantWithdrawsRequests", []any{s.Account}, big.NewInt(50))
		r.reply(strategy, st, "lastWithdrawRequest", []any{s.Account}, big.NewInt(7))
		r.reply(cdo, c, "epochEndDate", nil, big.NewInt(2000))
		r.reply(cdo, c, "allowInstantWithdraw", nil, true)
	}
	return s, r
}

func issuerProvider(t *testing.T, cfg Config, r provider.Reader) provider.Provider {
	t.Helper()
	p, err := Factory(issuerNode(t, cfg), providerRegistry.Deps{Chain: r})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func assertIssuerAction(t *testing.T, data []byte, contract abi.ABI, method string, expected ...any) {
	t.Helper()
	if len(data) < 4 || !bytes.Equal(data[:4], contract.Methods[method].ID) {
		t.Fatalf("wrong issuer selector: %x, expected %s", data, method)
	}
	args, err := contract.Methods[method].Inputs.Unpack(data[4:])
	if err != nil || len(args) != len(expected) {
		t.Fatalf("wrong %s parameters: got %v, expected %v: %v", method, args, expected, err)
	}
	for i := range args {
		if actual, ok := args[i].(*big.Int); ok {
			if want, valid := expected[i].(*big.Int); valid && actual.Cmp(want) == 0 {
				continue
			}
		}
		if !reflect.DeepEqual(args[i], expected[i]) {
			t.Fatalf("wrong %s parameter %d: got %v, expected %v", method, i, args[i], expected[i])
		}
	}
}

func TestEveryLiquidLaneCatalogueAssetPreparesDirectIssuerRequest(t *testing.T) {
	data, err := os.ReadFile("../../../config/assets/liquid-lane.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var catalogue struct {
		Assets []struct {
			Config `yaml:",inline"`
			Symbol string `yaml:"symbol"`
		} `yaml:"assets"`
	}
	if err = yaml.Unmarshal(data, &catalogue); err != nil {
		t.Fatal(err)
	}
	wanted := strings.Fields("mF-ONE mROX mHYPER CarryTradeUSDTRYLeverage StockMarketTRBasisTrade mM1-USD JAAA JTRSY HYB deJAAA deJTRSY HYBOND PRIME mGLOBAL AA_FalconXUSDC sUSN")
	if len(catalogue.Assets) != len(wanted) {
		t.Fatalf("catalogue asset coverage: got %d, expected %d", len(catalogue.Assets), len(wanted))
	}
	seen := map[string]bool{}
	for _, entry := range catalogue.Assets {
		t.Run(entry.Symbol, func(t *testing.T) {
			if seen[entry.Symbol] {
				t.Fatal("duplicate catalogue asset")
			}
			seen[entry.Symbol] = true
			cfg := issuerConfig(entry.Family)
			cfg.ChainID, cfg.Token, cfg.OutputAsset, cfg.Target = entry.ChainID, entry.Token, entry.OutputAsset, entry.Target
			cfg.CDO, cfg.Strategy = entry.CDO, entry.Strategy
			s, r := issuerFixture(t, cfg)
			before, _ := json.Marshal(s)
			p := issuerProvider(t, cfg, r)
			out, prepareErr := p.Prepare(context.Background(), s)
			if prepareErr != nil {
				t.Fatal(prepareErr)
			}
			if out.Inputs[3] != "100" || len(out.SkipSteps) != 0 {
				t.Fatalf("unexpected amount or skipped request: %+v", out)
			}
			a := out.Actions[cfg.Action]
			switch cfg.Family {
			case "midas":
				assertIssuerAction(t, a, midas.NewMidasRedemption().GetABI(), "redeemRequest", common.HexToAddress(cfg.OutputAsset), big.NewInt(100))
			case "centrifuge":
				assertIssuerAction(t, a, async.NewAsyncRedeemVault().GetABI(), "requestRedeem", big.NewInt(100), s.Account, s.Account)
			case "openeden":
				assertIssuerAction(t, a, openeden.NewOpenEdenExpress().GetABI(), "requestRedeem", s.Account, big.NewInt(100))
			case "figure":
				assertIssuerAction(t, a, erc4626.NewERC4626().GetABI(), "redeem", big.NewInt(100), s.Account, s.Account)
				assertIssuerAction(t, out.Actions[cfg.SecondAction], figure.NewFigureYieldVault().GetABI(), "requestRedeem", big.NewInt(125))
				if out.Inputs[4] != "125" || len(out.Actions) != 2 {
					t.Fatalf("wrong Figure second action: %+v", out)
				}
			case "noon":
				assertIssuerAction(t, a, erc4626.NewERC4626().GetABI(), "redeem", big.NewInt(100), common.HexToAddress(cfg.Target), s.Account)
			case "pareto":
				assertIssuerAction(t, a, paretoqueue.NewParetoQueue().GetABI(), "requestWithdraw", big.NewInt(100))
			}
			after, _ := json.Marshal(s)
			if !bytes.Equal(before, after) || r.reads == 0 {
				t.Fatal("snapshot mutated or issuer was not validated")
			}
		})
	}
	for _, symbol := range wanted {
		if !seen[symbol] {
			t.Errorf("missing catalogue asset %s", symbol)
		}
	}
}

func TestCentrifugeClaimCapsSharesAndRetainsConfiguredLimit(t *testing.T) {
	for _, available := range []int64{0, 20, 100, 150} {
		t.Run(big.NewInt(available).String(), func(t *testing.T) {
			cfg := issuerConfig("centrifuge")
			cfg.Mode = "claim"
			s, r := issuerFixture(t, cfg)
			a := async.NewAsyncRedeemVault()
			r.reply(common.HexToAddress(cfg.Target), a.GetABI(), "maxRedeem", []any{s.Account}, big.NewInt(available))
			p := issuerProvider(t, cfg, r)
			out, err := p.Prepare(context.Background(), s)
			if available == 0 {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("empty claim must idle: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := min(available, 100)
			assertIssuerAction(t, out.Actions[cfg.Action], a.GetABI(), "redeem", big.NewInt(want), s.Account, s.Account)
			if out.Inputs[3] != big.NewInt(want).String() {
				t.Fatal("claim input does not match executable shares")
			}
			// A low first claim must not lower the provider's limit for later claims.
			out.Actions[cfg.Action][0] ^= 0xff
			r.reply(common.HexToAddress(cfg.Target), a.GetABI(), "maxRedeem", []any{s.Account}, big.NewInt(150))
			later, err := p.Prepare(context.Background(), s)
			if err != nil || later.Inputs[3] != "100" {
				t.Fatalf("claim preparation retained mutated output or amount: %+v %v", later, err)
			}
			assertIssuerAction(t, later.Actions[cfg.Action], a.GetABI(), "redeem", big.NewInt(100), s.Account, s.Account)
		})
	}
}

func TestIssuerPreparationRejectsWrongTopologyAndMalformedReplies(t *testing.T) {
	for _, family := range []string{"midas", "centrifuge", "openeden", "figure", "pareto", "noon"} {
		for _, fault := range []string{"topology", "malformed", "transport", "chain", "cancelled"} {
			t.Run(family+"/"+fault, func(t *testing.T) {
				cfg := issuerConfig(family)
				s, r := issuerFixture(t, cfg)
				p := issuerProvider(t, cfg, r)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				switch fault {
				case "chain":
					s.ChainID = big.NewInt(2)
				case "cancelled":
					cancel()
				case "transport":
					r.err = errors.New("fixture network failure")
				default:
					for key := range r.results {
						if fault == "malformed" {
							r.results[key] = []byte{1}
						} else {
							r.results[key] = protocolio.AddressWord(issuerAddress(999))
						}
					}
				}
				if _, err := p.Prepare(ctx, s); err == nil || errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("invalid chain/issuer response must be an error: %v", err)
				}
				if fault == "chain" && r.reads != 0 {
					t.Fatal("wrong chain queried an issuer")
				}
			})
		}
	}
}

func TestIssuerIdleConditions(t *testing.T) {
	for _, fault := range []string{"midas-paused", "midas-minimum", "openeden-paused", "openeden-minimum", "figure-pending", "figure-zero-preview"} {
		t.Run(fault, func(t *testing.T) {
			family := strings.Split(fault, "-")[0]
			cfg := issuerConfig(family)
			s, r := issuerFixture(t, cfg)
			target := common.HexToAddress(cfg.Target)
			switch fault {
			case "midas-paused":
				r.reply(target, midas.NewMidasRedemption().GetABI(), "paused", nil, true)
			case "midas-minimum":
				r.reply(target, midas.NewMidasRedemption().GetABI(), "minAmount", nil, big.NewInt(101))
			case "openeden-paused":
				r.reply(target, openeden.NewOpenEdenExpress().GetABI(), "pausedRedeem", nil, true)
			case "openeden-minimum":
				r.reply(target, openeden.NewOpenEdenExpress().GetABI(), "redeemMinimum", nil, big.NewInt(101))
			case "figure-pending":
				r.reply(target, figure.NewFigureYieldVault().GetABI(), "pendingRedemptions", []any{s.Account}, big.NewInt(1), big.NewInt(1), big.NewInt(999))
			case "figure-zero-preview":
				r.reply(common.HexToAddress(cfg.Token), erc4626.NewERC4626().GetABI(), "previewRedeem", []any{big.NewInt(100)}, big.NewInt(0))
			}
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if !errors.Is(err, provider.ErrNotReady) || len(out.Actions) != 0 {
				t.Fatalf("issuer not ready should yield no request: %+v %v", out, err)
			}
		})
	}
}

func TestMidasRejectsDisabledOutputInsteadOfChoosingAnotherAsset(t *testing.T) {
	cfg := issuerConfig("midas")
	s, r := issuerFixture(t, cfg)
	r.reply(common.HexToAddress(cfg.Target), midas.NewMidasRedemption().GetABI(), "tokensConfig", []any{common.HexToAddress(cfg.OutputAsset)}, common.Address{}, big.NewInt(0), big.NewInt(0), false)
	if _, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s); err == nil || errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("disabled output asset must fail validation: %v", err)
	}
}

func TestParetoRequestsAndClaimsUseCorrectOwnerEpochAndTarget(t *testing.T) {
	for _, mode := range []string{"queue-request", "direct-request", "queue-claim", "direct-claim", "instant-claim"} {
		t.Run(mode, func(t *testing.T) {
			cfg := issuerConfig("pareto")
			cfg.Mode = mode
			if strings.HasSuffix(mode, "claim") {
				cfg.Amount = ""
			}
			if mode == "queue-claim" {
				cfg.Epoch = "7"
			}
			s, r := issuerFixture(t, cfg)
			p := issuerProvider(t, cfg, r)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			q, c := paretoqueue.NewParetoQueue().GetABI(), paretocdo.NewParetoCDO().GetABI()
			wantAmount := "100"
			switch mode {
			case "queue-request":
				assertIssuerAction(t, out.Actions[cfg.Action], q, "requestWithdraw", big.NewInt(100))
			case "direct-request":
				assertIssuerAction(t, out.Actions[cfg.Action], c, "requestWithdraw", big.NewInt(100), common.HexToAddress(cfg.Token))
			case "queue-claim":
				wantAmount = "30"
				assertIssuerAction(t, out.Actions[cfg.Action], q, "claimWithdrawRequest", big.NewInt(7))
			case "direct-claim":
				wantAmount = "40"
				assertIssuerAction(t, out.Actions[cfg.Action], c, "claimWithdrawRequest")
			case "instant-claim":
				wantAmount = "50"
				assertIssuerAction(t, out.Actions[cfg.Action], c, "claimInstantWithdrawRequest")
			}
			if out.Inputs[3] != wantAmount {
				t.Fatalf("incorrect owned request amount: %s", out.Inputs[3])
			}
			target := common.HexToAddress(cfg.Target)
			if !strings.HasPrefix(mode, "queue-") {
				target = common.HexToAddress(cfg.CDO)
			}
			s.Execution.Logs = []types.Log{issuerManageLog(t, s, []protocolio.ManagedCall{{Target: target, Data: out.Actions[cfg.Action]}})}
			progress, err := p.Advance(context.Background(), s, out, nil)
			wantOutcome := "requested"
			if strings.HasSuffix(mode, "claim") {
				wantOutcome = "claimed"
			}
			if err != nil || !progress.Done || progress.Outcome != wantOutcome {
				t.Fatalf("wrong Pareto outcome or target correlation: %+v %v", progress, err)
			}
		})
	}
}

func TestParetoReadinessAndClosedVaultClaim(t *testing.T) {
	cases := []struct {
		name, mode, method string
		value              any
		ready              bool
	}{
		{"default", "queue-request", "defaulted", true, false},
		{"ineligible", "queue-request", "isWalletAllowed", false, false},
		{"queue-wrong-phase", "queue-request", "isEpochRunning", false, false},
		{"direct-wrong-phase", "direct-request", "isEpochRunning", true, false},
		{"queue-empty", "queue-claim", "userWithdrawalsEpochs", big.NewInt(0), false},
		{"queue-unpriced", "queue-claim", "epochWithdrawPrice", big.NewInt(0), false},
		{"queue-unfunded", "queue-claim", "epochPendingClaims", big.NewInt(1), false},
		{"direct-empty", "direct-claim", "withdrawsRequests", big.NewInt(0), false},
		{"direct-same-epoch", "direct-claim", "lastWithdrawRequest", big.NewInt(8), false},
		{"direct-future-epoch", "direct-claim", "lastWithdrawRequest", big.NewInt(9), false},
		{"closed-vault", "direct-claim", "epochEndDate", big.NewInt(0), true},
		{"instant-empty", "instant-claim", "instantWithdrawsRequests", big.NewInt(0), false},
		{"instant-disabled", "instant-claim", "allowInstantWithdraw", false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := issuerConfig("pareto")
			cfg.Mode = tt.mode
			if strings.HasSuffix(tt.mode, "claim") {
				cfg.Amount = ""
			}
			if tt.mode == "queue-claim" {
				cfg.Epoch = "7"
			}
			s, r := issuerFixture(t, cfg)
			q, c, st := paretoqueue.NewParetoQueue().GetABI(), paretocdo.NewParetoCDO().GetABI(), paretostrategy.NewParetoStrategy().GetABI()
			target, contract := common.HexToAddress(cfg.CDO), c
			var args []any
			switch tt.method {
			case "userWithdrawalsEpochs":
				target, contract, args = common.HexToAddress(cfg.Target), q, []any{s.Account, big.NewInt(7)}
			case "epochWithdrawPrice", "epochPendingClaims":
				target, contract, args = common.HexToAddress(cfg.Target), q, []any{big.NewInt(7)}
			case "withdrawsRequests", "lastWithdrawRequest", "instantWithdrawsRequests":
				target, contract, args = common.HexToAddress(cfg.Strategy), st, []any{s.Account}
			case "isWalletAllowed":
				args = []any{s.Account}
			}
			if tt.name == "closed-vault" {
				r.reply(common.HexToAddress(cfg.Strategy), st, "lastWithdrawRequest", []any{s.Account}, big.NewInt(8))
			}
			r.reply(target, contract, tt.method, args, tt.value)
			out, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s)
			if tt.ready {
				if err != nil || len(out.Actions) != 1 {
					t.Fatalf("closed-vault receipt should be claimable: %+v %v", out, err)
				}
			} else if !errors.Is(err, provider.ErrNotReady) || len(out.Actions) != 0 {
				t.Fatalf("unready request/claim should idle without actions: %+v %v", out, err)
			}
		})
	}
}

func TestParetoValidatesEveryConfiguredTopologyEdge(t *testing.T) {
	for _, edge := range []string{"queue-tranche", "queue-underlying", "queue-cdo", "queue-strategy", "cdo-tranche", "cdo-token", "cdo-strategy", "strategy-cdo", "strategy-token"} {
		t.Run(edge, func(t *testing.T) {
			cfg := issuerConfig("pareto")
			s, r := issuerFixture(t, cfg)
			q, c, st := paretoqueue.NewParetoQueue().GetABI(), paretocdo.NewParetoCDO().GetABI(), paretostrategy.NewParetoStrategy().GetABI()
			checks := map[string]struct {
				target   common.Address
				contract abi.ABI
				method   string
			}{
				"queue-tranche":    {common.HexToAddress(cfg.Target), q, "tranche"},
				"queue-underlying": {common.HexToAddress(cfg.Target), q, "underlying"},
				"queue-cdo":        {common.HexToAddress(cfg.Target), q, "idleCDOEpoch"},
				"queue-strategy":   {common.HexToAddress(cfg.Target), q, "strategy"},
				"cdo-tranche":      {common.HexToAddress(cfg.CDO), c, "AATranche"},
				"cdo-token":        {common.HexToAddress(cfg.CDO), c, "token"},
				"cdo-strategy":     {common.HexToAddress(cfg.CDO), c, "strategy"},
				"strategy-cdo":     {common.HexToAddress(cfg.Strategy), st, "idleCDO"},
				"strategy-token":   {common.HexToAddress(cfg.Strategy), st, "token"},
			}
			check := checks[edge]
			r.reply(check.target, check.contract, check.method, nil, issuerAddress(999))
			if _, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s); err == nil || errors.Is(err, provider.ErrNotReady) {
				t.Fatalf("wrong topology edge accepted or treated as idle: %v", err)
			}
		})
	}
}

func TestRedemptionFactoryRejectsInvalidConfiguration(t *testing.T) {
	cases := map[string]func(*Config){
		"family":                func(c *Config) { c.Family = "unknown" },
		"mode":                  func(c *Config) { c.Mode = "claim" },
		"chain":                 func(c *Config) { c.ChainID = 0 },
		"token":                 func(c *Config) { c.Token = "0x123" },
		"asset":                 func(c *Config) { c.OutputAsset = common.Address{}.Hex() },
		"target":                func(c *Config) { c.Target = "" },
		"same-asset":            func(c *Config) { c.OutputAsset = c.Token },
		"zero-amount":           func(c *Config) { c.Amount = "0" },
		"negative-amount":       func(c *Config) { c.Amount = "-1" },
		"overflow-amount":       func(c *Config) { c.Amount = new(big.Int).Lsh(big.NewInt(1), 256).String() },
		"action":                func(c *Config) { c.Action = "02:0" },
		"extra-cdo":             func(c *Config) { c.CDO = issuerAddress(4).Hex() },
		"extra-strategy":        func(c *Config) { c.Strategy = issuerAddress(5).Hex() },
		"extra-second-action":   func(c *Config) { c.SecondAction = "2:1" },
		"extra-epoch":           func(c *Config) { c.Epoch = "1" },
		"extra-intermediate":    func(c *Config) { n := uint(1); c.IntermediateInput = &n },
		"large-input":           func(c *Config) { n := uint(4096); c.AmountInput = &n },
		"figure-missing-second": func(c *Config) { *c = issuerConfig("figure"); c.SecondAction = "" },
		"figure-same-actions":   func(c *Config) { *c = issuerConfig("figure"); c.SecondAction = c.Action },
		"figure-same-inputs":    func(c *Config) { *c = issuerConfig("figure"); c.IntermediateInput = c.AmountInput },
		"pareto-missing-cdo":    func(c *Config) { *c = issuerConfig("pareto"); c.CDO = "" },
		"pareto-missing-epoch":  func(c *Config) { *c = issuerConfig("pareto"); c.Mode = "queue-claim"; c.Amount = "" },
		"pareto-claim-amount":   func(c *Config) { *c = issuerConfig("pareto"); c.Mode = "direct-claim" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := issuerConfig("midas")
			change(&cfg)
			_, r := issuerFixture(t, issuerConfig("midas"))
			if _, err := Factory(issuerNode(t, cfg), providerRegistry.Deps{Chain: r}); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	_, r := issuerFixture(t, issuerConfig("midas"))
	if _, err := Factory(issuerNode(t, issuerConfig("midas")), providerRegistry.Deps{}); err == nil {
		t.Fatal("missing chain reader accepted")
	}
	if _, err := Factory(issuerNode(t, map[string]any{"unexpected": true}), providerRegistry.Deps{Chain: r}); err == nil {
		t.Fatal("unknown configuration field accepted")
	}
}

func TestRegistryAndFullWidthRequestAmount(t *testing.T) {
	cfg := issuerConfig("midas")
	amount := new(big.Int).Lsh(big.NewInt(1), 255)
	cfg.Amount = amount.String()
	s, r := issuerFixture(t, cfg)
	registry := providerRegistry.NewRegistry()
	if err := Register(registry); err != nil {
		t.Fatal(err)
	}
	if err := Register(registry); err == nil {
		t.Fatal("duplicate provider registration accepted")
	}
	p, err := registry.New("redemptions", issuerNode(t, cfg), providerRegistry.Deps{Chain: r})
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Prepare(context.Background(), s)
	if err != nil || out.Inputs[3] != amount.String() {
		t.Fatalf("runtime amount narrowed below uint256: %+v %v", out, err)
	}
	assertIssuerAction(t, out.Actions[cfg.Action], midas.NewMidasRedemption().GetABI(), "redeemRequest", common.HexToAddress(cfg.OutputAsset), amount)
}

func TestPrepareRequiresPinnedAccountSnapshot(t *testing.T) {
	for _, field := range []string{"chain", "block", "account"} {
		t.Run(field, func(t *testing.T) {
			cfg := issuerConfig("midas")
			s, r := issuerFixture(t, cfg)
			switch field {
			case "chain":
				s.ChainID = nil
			case "block":
				s.BlockNumber = nil
			case "account":
				s.Account = common.Address{}
			}
			if _, err := issuerProvider(t, cfg, r).Prepare(context.Background(), s); err == nil || r.reads != 0 {
				t.Fatalf("unpinned account snapshot accepted or queried chain: %v", err)
			}
		})
	}
}

func issuerManageLog(t *testing.T, s provider.Snapshot, calls []protocolio.ManagedCall) types.Log {
	t.Helper()
	event := account.NewMerkleAccount().GetABI().Events[account.MerkleAccountManageEventName]
	actions := make([]account.IMerkleAccountAction, len(calls))
	for i, call := range calls {
		actions[i] = account.IMerkleAccountAction{Target: call.Target, Value: new(big.Int), Data: call.Data}
	}
	data, err := event.Inputs.NonIndexed().Pack(actions)
	if err != nil {
		t.Fatal(err)
	}
	return types.Log{Address: s.Account, Topics: []common.Hash{event.ID}, Data: data, TxHash: s.Execution.TxHash, BlockHash: s.Execution.BlockHash, BlockNumber: s.Execution.BlockNumber.Uint64()}
}

func TestAdvanceRequiresCanonicalExactExecutedActions(t *testing.T) {
	for _, mode := range []string{"request", "claim", "figure"} {
		for _, fault := range []string{"valid", "skipped", "foreign-account", "foreign-target", "different-data", "missing-receipt", "orphaned-receipt", "orphaned-snapshot", "changed-chain", "changed-account", "figure-missing-second"} {
			if fault == "figure-missing-second" && mode != "figure" {
				continue
			}
			t.Run(mode+"/"+fault, func(t *testing.T) {
				cfg := issuerConfig("centrifuge")
				cfg.Mode = mode
				if mode == "figure" {
					cfg = issuerConfig("figure")
				}
				s, r := issuerFixture(t, cfg)
				p := issuerProvider(t, cfg, r)
				out, err := p.Prepare(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				target := common.HexToAddress(cfg.Target)
				if mode == "figure" {
					target = common.HexToAddress(cfg.Token)
				}
				calls := []protocolio.ManagedCall{{Target: target, Data: bytes.Clone(out.Actions[cfg.Action])}}
				if mode == "figure" {
					calls = append(calls, protocolio.ManagedCall{Target: common.HexToAddress(cfg.Target), Data: bytes.Clone(out.Actions[cfg.SecondAction])})
				}
				if fault == "foreign-target" {
					calls[0].Target = issuerAddress(999)
				}
				if fault == "different-data" {
					calls[0].Data[len(calls[0].Data)-1] ^= 1
				}
				if fault == "figure-missing-second" {
					calls = calls[:1]
				}
				s.Execution.Logs = []types.Log{issuerManageLog(t, s, calls)}
				switch fault {
				case "skipped":
					s.Execution.Logs = nil
				case "foreign-account":
					s.Execution.Logs[0].Address = issuerAddress(999)
				case "missing-receipt":
					s.Execution = nil
				case "orphaned-receipt":
					r.headers[41].Extra = []byte("fork")
				case "orphaned-snapshot":
					r.headers[42].Extra = []byte("fork")
				case "changed-chain":
					s.ChainID = big.NewInt(2)
				case "changed-account":
					s.Account = issuerAddress(998)
				}
				before, _ := json.Marshal(out)
				progress, err := p.Advance(context.Background(), s, out, nil)
				if fault != "valid" {
					if err == nil || progress.Done {
						t.Fatalf("invalid execution accepted: %+v %v", progress, err)
					}
					return
				}
				want := "requested"
				if mode == "claim" {
					want = "claimed"
				}
				if err != nil || !progress.Done || progress.Outcome != want || progress.Reference != s.Execution.TxHash.Hex() {
					t.Fatalf("wrong execution outcome: %+v %v", progress, err)
				}
				after, _ := json.Marshal(out)
				if !bytes.Equal(before, after) {
					t.Fatal("advance mutated persisted output")
				}
				again, err := p.Advance(context.Background(), s, out, nil)
				if err != nil || !reflect.DeepEqual(again, progress) {
					t.Fatalf("receipt reconciliation is not idempotent: %+v %v", again, err)
				}
			})
		}
	}
}
