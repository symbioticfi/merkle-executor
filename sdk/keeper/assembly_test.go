package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func mutableInputFixture(t *testing.T) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	leaf := recipes.Leaf{Target: common.HexToAddress("0x77"), Value: new(big.Int), Data: make([]byte, 36), DataMask: make([]byte, 36)}
	copy(leaf.Data, []byte{1, 2, 3, 4})
	for i := 4; i < 36; i++ {
		leaf.DataMask[i] = 255
	}
	leaf.Hash, _ = recipes.HashLeaf(leaf)
	args := abi.Arguments{}
	for _, name := range []string{"uint256", "bytes32[]", "uint256[][]"} {
		value, _ := abi.NewType(name, "", nil)
		args = append(args, abi.Argument{Type: value})
	}
	gate := new(big.Int).Lsh(big.NewInt(1), 216)
	raw, _ := args.Pack(gate, [][32]byte{leaf.Hash}, [][]*big.Int{{}})
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, raw...)}}
	tree := &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	return recipe, tree
}

func TestProviderExecutionsUseNormalAssembly(t *testing.T) {
	recipe, tree := mutableInputFixture(t)
	leaf := tree.Leaves[0]
	first, second := bytes.Clone(leaf.Data), bytes.Clone(leaf.Data)
	second[35] = 9
	outputs := [][]provider.Output{{
		{Inputs: map[uint]string{0: "7"}, Actions: map[string]hexutil.Bytes{"0:0": first}},
		{Inputs: map[uint]string{0: "9"}, Actions: map[string]hexutil.Bytes{"0:0": second}},
	}, {{}}}
	one, _, err := recipes.Assemble(recipe, tree, outputs[0][0].Inputs, outputs[0][0].Actions, nil)
	if err != nil {
		t.Fatal(err)
	}
	two, _, err := recipes.Assemble(recipe, tree, outputs[0][1].Inputs, outputs[0][1].Actions, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := recipes.Multicall([][]byte{one, two})
	got, leaves, err := assembleOutputs(recipe, tree, nil, outputs)
	if err != nil || !bytes.Equal(got, want) || len(leaves) != 1 || leaves[0] != leaf.Hash {
		t.Fatalf("batch differs: %v", err)
	}
	saved, _ := json.Marshal(outputs)
	for _, mode := range []string{"selector", "unowned", "missing input", "implicit repetition"} {
		t.Run(mode, func(t *testing.T) {
			var changed [][]provider.Output
			if err := json.Unmarshal(saved, &changed); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "selector":
				changed[0][1].Actions["0:0"][0] ^= 1
			case "unowned":
				changed[0][1].Actions = map[string]hexutil.Bytes{"0:1": second}
			case "missing input":
				changed[0][1].Inputs = nil
			case "implicit repetition":
				changed[1][0].Inputs = map[uint]string{0: "1"}
			}
			if _, _, err := assembleOutputs(recipe, tree, nil, changed); err == nil {
				t.Fatal("invalid runtime accepted")
			}
		})
	}
}

type declaredProvider struct {
	requirements RecipeRequirements
	outputs      []provider.Output
}

func (p declaredProvider) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return p.requirements, nil
}

func (p declaredProvider) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) ([]provider.Output, error) {
	return p.outputs, nil
}

func TestBuildPlanChecksEachProvidersDeclaredOutputKeys(t *testing.T) {
	for _, mode := range []string{"declared", "wrong input owner", "wrong action owner", "undeclared later action"} {
		t.Run(mode, func(t *testing.T) {
			r, tree := mutableInputFixture(t)
			s := recipes.Snapshot{Executor: common.Address{1}, Account: common.Address{2}, Caller: common.Address{3}, BlockNumber: 12, Root: tree.Root}
			input := map[uint]string{0: "1"}
			data := bytes.Clone(tree.Leaves[0].Data)
			data[35] = 7
			action := map[string]hexutil.Bytes{"0:0": data}
			first := declaredProvider{requirements: RecipeRequirements{Inputs: []uint{0}, Actions: []string{"0:0"}}, outputs: []provider.Output{{Inputs: input, Actions: action}}}
			other := declaredProvider{outputs: []provider.Output{{}}}
			var want string
			switch mode {
			case "wrong input owner":
				first.outputs[0].Inputs = nil
				other.outputs[0].Inputs = input
				want = "undeclared input"
			case "wrong action owner":
				first.outputs[0].Actions = nil
				other.outputs[0].Actions = action
				want = "undeclared action"
			case "undeclared later action":
				first.requirements.Actions = nil
				first.outputs[0].Actions = nil
				first.outputs = append(first.outputs, provider.Output{Inputs: input, Actions: action})
				want = "undeclared action"
			}
			p := &Planner{ChainID: big.NewInt(1)}
			_, _, err := p.buildPlan(t.Context(), Strategy{Providers: []Provider{first, other}}, s, r, tree)
			if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
				t.Fatalf("unexpected output ownership result: %v", err)
			}
		})
	}
}
