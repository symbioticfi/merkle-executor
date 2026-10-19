package providers_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	"github.com/symbioticfi/merkle-executor/internal/providers/securitize"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func compositionProvider[C any, P provider.Provider](t *testing.T, constructor func(C, registry.Deps) (P, error), cfg C) P {
	t.Helper()
	p, err := constructor(cfg, registry.Deps{Chain: unusedReader{}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func testExecute(t *testing.T, targets ...common.Address) []byte {
	t.Helper()
	word, _ := abi.NewType("uint256", "", nil)
	hashes, _ := abi.NewType("bytes32[]", "", nil)
	bindings, _ := abi.NewType("uint256[][]", "", nil)
	args := abi.Arguments{{Type: word}, {Type: hashes}, {Type: bindings}}
	steps, runtime := make([][]byte, len(targets)), make([][]byte, len(targets))
	for i, target := range targets {
		raw, err := args.Pack(big.NewInt(1), [][32]byte{{1}}, [][]*big.Int{{}})
		if err != nil {
			t.Fatal(err)
		}
		steps[i] = append([]byte{9}, raw...)
		runtime[i] = account.NewMerkleAccount().PackManage([]account.IMerkleAccountAction{{Target: target, Value: new(big.Int), Data: []byte{1, 2, 3, 4}}})[4:]
	}
	return executor.NewRecipeExecutor().PackExecute(nil, steps, nil, runtime)
}
func testAddress(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }
func TestIssuerCompositionUsesRealRouteDeclarations(t *testing.T) {
	token, target, asset, cdo := testAddress(1), testAddress(2), testAddress(3), testAddress(4)
	for _, tc := range []struct {
		family, mode string
		targets      []common.Address
	}{
		{"midas", "request", []common.Address{target}}, {"openeden", "request", []common.Address{target}},
		{"centrifuge", "request", []common.Address{target}}, {"centrifuge", "claim", []common.Address{target}},
		{"figure", "request", []common.Address{token, target}},
		{"noon", "request", []common.Address{token}}, {"noon", "claim", []common.Address{target}},
		{"pareto", "queue-request", []common.Address{target}}, {"pareto", "queue-claim", []common.Address{target}},
		{"pareto", "direct-request", []common.Address{cdo}}, {"pareto", "direct-claim", []common.Address{cdo}}, {"pareto", "instant-claim", []common.Address{cdo}},
	} {
		t.Run(tc.family+"/"+tc.mode, func(t *testing.T) {
			cfg := redemptions.Config{Family: tc.family, Mode: tc.mode, ChainID: 1, Token: token.Hex(), OutputAsset: asset.Hex(), Target: target.Hex(), Amount: "1", Action: "0:0"}
			if tc.family == "figure" {
				cfg.SecondAction = "1:0"
			}
			if tc.family == "noon" && tc.mode == "claim" {
				cfg.Amount = ""
				cfg.RequestID = "0"
			}
			if tc.family == "pareto" {
				cfg.CDO = cdo.Hex()
				cfg.Strategy = testAddress(5).Hex()
				if tc.mode == "queue-claim" {
					cfg.Epoch = "0"
				}
				if tc.mode == "queue-claim" || tc.mode == "direct-claim" || tc.mode == "instant-claim" {
					cfg.Amount = ""
				}
			}
			pr := compositionProvider(t, redemptions.New, cfg)
			if err := keeper.ValidateComposition(testExecute(t, tc.targets...), []provider.Provider{pr}); err != nil {
				t.Fatal(err)
			}
			repeated := append(append([]common.Address{}, tc.targets...), tc.targets[0])
			if err := keeper.ValidateComposition(testExecute(t, repeated...), []provider.Provider{pr}); err == nil {
				t.Fatal("static issuer duplicate accepted")
			}
		})
	}
}
func TestSecuritizeRejectsRepeatedStaticAndMixedRoutes(t *testing.T) {
	token, ramp, payment := testAddress(1), testAddress(2), testAddress(3)
	for _, mode := range []string{"wallet", "modern", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			cfg := securitize.Config{Mode: mode, ChainID: 1, Token: token.Hex(), Target: ramp.Hex(), Amount: "100", Action: "0:0"}
			target := ramp
			if mode == "wallet" {
				target = token
			} else {
				cfg.PaymentToken = payment.Hex()
				cfg.MinOutput = "90"
			}
			first := compositionProvider(t, securitize.New, cfg)
			if err := keeper.ValidateComposition(testExecute(t, target, payment), []provider.Provider{first}); err != nil {
				t.Fatalf("unrelated later payment movement: %v", err)
			}
			if err := keeper.ValidateComposition(testExecute(t, target, target), []provider.Provider{first}); err == nil {
				t.Fatal("static repeated route accepted")
			}
			cfg.Action = "1:0"
			second := compositionProvider(t, securitize.New, cfg)
			if err := keeper.ValidateComposition(testExecute(t, target, target), []provider.Provider{first, second}); err == nil {
				t.Fatal("repeated configured route accepted")
			}
			mixed := compositionProvider(t, redemptions.New, redemptions.Config{Family: "midas", Mode: "request", ChainID: 1, Token: token.Hex(), OutputAsset: payment.Hex(), Target: target.Hex(), Amount: "100", Action: "1:0"})
			if err := keeper.ValidateComposition(testExecute(t, target, target), []provider.Provider{first, mixed}); err == nil {
				t.Fatal("mixed provider route accepted")
			}
		})
	}
}
