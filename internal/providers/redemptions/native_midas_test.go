package redemptions

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func inputMidasRecipe(t *testing.T, cfg Config, snapshot provider.Snapshot, reader provider.Reader) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	recipe.Steps = [][]byte{recipe.Steps[0], packedStep(t, recipes.Equal, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 7)}})}
	return recipe, tree
}

func TestAutomaticNativeMidasComposition(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var midas, centrifuge issuerAsset
	for _, asset := range p.assets {
		if asset.Symbol == "mGLOBAL" && midas.Symbol == "" {
			midas = asset
		}
		if asset.Symbol == "JAAA" && centrifuge.Symbol == "" {
			centrifuge = asset
		}
	}
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: issuerAddress(123), BlockNumber: big.NewInt(42)}
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "request", true: "mint-claim-request-with-Midas-fallback"}[mixed], func(t *testing.T) {
			recipe, tree := automaticRecipe(t, automaticConfig(midas, "request"), snapshot, reader)
			amountStep := uint(0)
			if mixed {
				other, otherTree := nativeCentrifugeRecipe(t, snapshot.Account, common.HexToAddress(centrifuge.Target), []string{"mint(uint256,address,address)", "redeem(uint256,address,address)", "requestRedeem(uint256,address,address)"})
				queryIndex := len(other.Queries)
				amountStep = uint(len(other.Steps))
				other.Queries = append(other.Queries, recipe.Queries[0])
				manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
				other.Steps = append(other.Steps, packedStep(t, recipes.Read, readArguments, big.NewInt(int64(queryIndex))), packedStep(t, recipes.Manage, manage, packedValue(recipes.Result, uint64(amountStep)), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Result, uint64(amountStep))}}))
				otherTree.Leaves = append(otherTree.Leaves, tree.Leaves...)
				sealNativeTree(otherTree)
				recipe, tree = other, otherTree
			}
			req, e := p.RecipeRequirements(snapshot, recipe, tree)
			if e != nil {
				t.Fatal(e)
			}
			if e = keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); e != nil {
				t.Fatal(e)
			}
			if len(req.Actions)+len(req.Inputs)+len(req.Values)+len(req.ExclusiveTargets) != 0 {
				t.Fatalf("native Midas request acquired provider ownership: %+v", req)
			}
			if e = recipes.ValidateMutableActions(recipe, tree, nil); e != nil {
				t.Fatal(e)
			}
			for _, result := range []*big.Int{nil, new(big.Int), big.NewInt(100)} {
				out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{amountStep: result})
				if e != nil || len(out.Actions)+len(out.Inputs)+len(out.SkipSteps) != 0 {
					t.Fatalf("native Midas fallback depends on prestate budget: %+v %v", out, e)
				}
			}
			if reader.reads != 0 {
				t.Fatal("native Midas fallback acquired issuer preparation reads")
			}
		})
	}
}

func TestAutomaticNativeMidasStillValidatesIdentityAndBudget(t *testing.T) {
	raw, err := NewAutomatic(registry.Deps{Chain: &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	asset := p.assets[0]
	cfg := automaticConfig(asset, "request")
	snapshot, reader := issuerFixture(t, cfg)
	p.deps.Chain = reader
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	for _, fault := range []string{"masked output", "wrong output", "free input", "bounds input", "literal amount", "result without read"} {
		t.Run(fault, func(t *testing.T) {
			recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
			switch fault {
			case "masked output":
				tree.Leaves[0].DataMask[4] = 255
			case "wrong output":
				tree.Leaves[0].Data[35] ^= 1
			case "free input", "bounds input":
				recipe.Steps[1] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 7)}})
				if fault == "bounds input" {
					recipe.Steps = [][]byte{recipe.Steps[0], packedStep(t, recipes.LessOrEqual, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), recipe.Steps[1]}
				}
			case "literal amount":
				recipe.Steps[1] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Literal, 100)}})
			case "result without read":
				recipe.Steps[0] = packedStep(t, recipes.Add, binary, packedValue(recipes.Literal, 1), packedValue(recipes.Literal, 2))
			}
			sealAutomaticTree(t, &recipe, tree)
			if _, e := p.RecipeRequirements(snapshot, recipe, tree); e == nil || reader.reads != 0 {
				t.Fatalf("native Midas guard bypassed: %v reads=%d", e, reader.reads)
			}
		})
	}
	// Input-backed requests continue to acquire provider readiness and receipt ownership.
	recipe, tree := inputMidasRecipe(t, cfg, snapshot, reader)
	req, e := p.RecipeRequirements(snapshot, recipe, tree)
	if e != nil || len(req.ExclusiveTargets) != 1 || len(req.Inputs) != 1 || req.Inputs[0] != 7 {
		t.Fatalf("input Midas route lost provider ownership: %+v %v", req, e)
	}
	if _, e = p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: new(big.Int)}); !errors.Is(e, provider.ErrNotReady) {
		t.Fatalf("input Midas zero budget lost idle behavior: %v", e)
	}
	out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if e != nil || out.Inputs[7] != "100" || len(out.Actions) != 1 || !strings.Contains(string(out.Data), "midas") {
		t.Fatalf("input Midas request lost exact provider output: %+v %v", out, e)
	}
}
