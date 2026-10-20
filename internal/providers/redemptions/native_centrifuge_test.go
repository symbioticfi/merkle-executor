package redemptions

import (
	"bytes"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func nativeCentrifugeRecipe(t *testing.T, account, target common.Address, selectors []string) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	var recipe recipes.Recipe
	tree := &recipes.Tree{ChainID: big.NewInt(1)}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	fields := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("address")}, {Type: argumentType("address")}}
	for i, signature := range selectors {
		args, err := fields.Pack(new(big.Int), account, account)
		if err != nil {
			t.Fatal(err)
		}
		data := append(bytes.Clone(crypto.Keccak256([]byte(signature))[:4]), args...)
		mask := make([]byte, len(data))
		for at := 4; at < 36; at++ {
			mask[at] = 255
		}
		leaf := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		leaf.Hash, err = recipes.HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		tree.Leaves = append(tree.Leaves, leaf)
		query, err := queryArguments.Pack(target, append(bytes.Clone(crypto.Keccak256([]byte("maxRedeem(address)"))[:4]), common.LeftPadBytes(account[:], 32)...), []*big.Int{}, new(big.Int))
		if err != nil {
			t.Fatal(err)
		}
		recipe.Queries = append(recipe.Queries, query)
		step := uint64(len(recipe.Steps))
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Read, readArguments, big.NewInt(int64(i))), packedStep(t, recipes.Manage, manage, packedValue(recipes.Result, step), [][32]byte{leaf.Hash}, [][]*big.Int{{packedBinding(4, recipes.Result, step)}}))
	}
	sealNativeTree(tree)
	return recipe, tree
}

func sealNativeTree(tree *recipes.Tree) {
	// The complete tree uses the same sorted, reverse-leaf representation as the backend.
	sorted := append([]recipes.Leaf(nil), tree.Leaves...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if bytes.Compare(sorted[i].Hash[:], sorted[j].Hash[:]) > 0 {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	hashes := make([]common.Hash, 2*len(sorted)-1)
	for i, l := range sorted {
		hashes[len(hashes)-1-i] = l.Hash
	}
	for i := len(sorted) - 2; i >= 0; i-- {
		a, b := hashes[2*i+1], hashes[2*i+2]
		if bytes.Compare(a[:], b[:]) > 0 {
			a, b = b, a
		}
		hashes[i] = crypto.Keccak256Hash(a[:], b[:])
	}
	tree.Root = hashes[0]
	for i, l := range sorted {
		position := len(hashes) - 1 - i
		var proof []common.Hash
		for position > 0 {
			if position%2 == 0 {
				proof = append(proof, hashes[position-1])
			} else {
				proof = append(proof, hashes[position+1])
			}
			position = (position - 1) / 2
		}
		for j := range tree.Leaves {
			if tree.Leaves[j].Hash == l.Hash {
				tree.Leaves[j].Proof = proof
			}
		}
	}
}

func TestAutomaticNativeCentrifugeComposition(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var asset issuerAsset
	for _, a := range p.assets {
		if a.Family == "centrifuge" {
			asset = a
			break
		}
	}
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: issuerAddress(123), BlockNumber: big.NewInt(42)}
	for _, selectors := range [][]string{{"mint(uint256,address,address)", "redeem(uint256,address,address)"}, {"mint(uint256,address,address)", "redeem(uint256,address,address)", "requestRedeem(uint256,address,address)"}} {
		t.Run(strings.Join(selectors, "/"), func(t *testing.T) {
			recipe, tree := nativeCentrifugeRecipe(t, snapshot.Account, common.HexToAddress(asset.Target), selectors)
			req, e := p.RecipeRequirements(snapshot, recipe, tree)
			if e != nil {
				t.Fatal(e)
			}
			if e = keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); e != nil {
				t.Fatalf("deterministic native issuer calls rejected: %v", e)
			}
			if len(req.Actions)+len(req.Inputs)+len(req.Values)+len(req.ExclusiveTargets) != 0 {
				t.Fatalf("native calls acquired provider ownership: %+v", req)
			}
			if e = recipes.ValidateMutableActions(recipe, tree, nil); e != nil {
				t.Fatal(e)
			}
			for _, results := range []map[uint]*big.Int{nil, {0: new(big.Int), 2: new(big.Int), 4: new(big.Int)}} {
				out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, results)
				if e != nil || len(out.Actions)+len(out.Inputs)+len(out.SkipSteps) != 0 {
					t.Fatalf("native recipe requires provider data: %+v %v", out, e)
				}
				targets, e := p.ExclusiveOutputTargets(out)
				if e != nil || len(targets) != 0 {
					t.Fatalf("native recovery acquired target: %+v %v", targets, e)
				}
				progress, e := p.Advance(t.Context(), snapshot, out, nil)
				if e != nil || !progress.Done {
					t.Fatalf("native empty recovery: %+v %v", progress, e)
				}
			}
			if reader.reads != 0 {
				t.Fatal("native route made issuer preparation reads")
			}
		})
	}
}

func TestAutomaticNativeCentrifugeRejectsUnconstrainedFields(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var asset issuerAsset
	for _, a := range p.assets {
		if a.Family == "centrifuge" {
			asset = a
			break
		}
	}
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: issuerAddress(123), BlockNumber: big.NewInt(42)}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	for _, fault := range []string{"free input", "bounds input", "literal amount", "result without read", "masked receiver", "wrong receiver"} {
		t.Run(fault, func(t *testing.T) {
			recipe, tree := nativeCentrifugeRecipe(t, snapshot.Account, common.HexToAddress(asset.Target), []string{"redeem(uint256,address,address)"})
			switch fault {
			case "free input", "bounds input":
				recipe.Steps[1] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Input, 7)}})
				if fault == "bounds input" {
					recipe.Steps = [][]byte{recipe.Steps[0], packedStep(t, recipes.LessOrEqual, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), recipe.Steps[1]}
				}
			case "literal amount":
				recipe.Steps[1] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Literal, 100)}})
			case "result without read":
				recipe.Steps[0] = packedStep(t, recipes.Add, binary, packedValue(recipes.Literal, 1), packedValue(recipes.Literal, 2))
			case "masked receiver":
				tree.Leaves[0].DataMask[36] = 255
			case "wrong receiver":
				tree.Leaves[0].Data[67] ^= 1
			}
			sealAutomaticTree(t, &recipe, tree)
			_, e := p.RecipeRequirements(snapshot, recipe, tree)
			if e == nil || reader.reads != 0 {
				t.Fatalf("unconstrained native route accepted: %v reads=%d", e, reader.reads)
			}
		})
	}
}

func TestAutomaticCentrifugeInputKeepsExclusiveReceiptOwnership(t *testing.T) {
	raw, err := NewAutomatic(registry.Deps{Chain: &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var asset issuerAsset
	for _, a := range p.assets {
		if a.Family == "centrifuge" {
			asset = a
			break
		}
	}
	cfg := automaticConfig(asset, "claim")
	snapshot, reader := issuerFixture(t, cfg)
	p.deps.Chain = reader
	recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	recipe.Steps = [][]byte{recipe.Steps[0], packedStep(t, recipes.Equal, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Input, 7)}})}
	req, e := p.RecipeRequirements(snapshot, recipe, tree)
	if e != nil || len(req.ExclusiveTargets) != 1 || len(req.Inputs) != 1 || req.Inputs[0] != 7 {
		t.Fatalf("input route lost provider ownership: %+v %v", req, e)
	}
	out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if e != nil || out.Inputs[7] != "100" || len(out.Actions) != 1 {
		t.Fatalf("input route lost exact amount: %+v %v", out, e)
	}
	// A second transaction-wide issuer effect still cannot share this receipt route.
	recipe.Steps = append(recipe.Steps, bytes.Clone(recipe.Steps[2]))
	if e = keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); e == nil || !strings.Contains(e.Error(), "unowned managed call to an exclusive target") {
		t.Fatalf("provider-owned target sharing accepted: %v", e)
	}
	if _, e = p.PrepareRecipe(t.Context(), snapshot, recipes.Recipe{Queries: recipe.Queries, Steps: recipe.Steps[:3]}, tree, map[uint]*big.Int{0: new(big.Int)}); !errors.Is(e, provider.ErrNotReady) {
		t.Fatalf("empty input budget lost idle policy: %v", e)
	}
}
