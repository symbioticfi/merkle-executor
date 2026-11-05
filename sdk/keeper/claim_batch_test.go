package keeper

import (
	"bytes"
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"testing"
)

func TestRepeatedClaimsPreserveRecipeAndPermissions(t *testing.T) {
	leaf := recipes.Leaf{Target: common.HexToAddress("0x77"), Value: new(big.Int), Data: make([]byte, 36), DataMask: make([]byte, 36)}
	copy(leaf.Data, []byte{1, 2, 3, 4})
	for i := 4; i < 36; i++ {
		leaf.DataMask[i] = 255
	}
	leaf.Hash, _ = recipes.HashLeaf(leaf)
	args := abi.Arguments{}
	for _, n := range []string{"uint256", "bytes32[]", "uint256[][]"} {
		v, _ := abi.NewType(n, "", nil)
		args = append(args, abi.Argument{Type: v})
	}
	raw, _ := args.Pack(big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, raw...)}}
	tree := &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	first := append(hexutil.Bytes{}, leaf.Data...)
	second := append(hexutil.Bytes{}, leaf.Data...)
	second[35] = 9
	one, _, _ := recipes.Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": first}, nil)
	two, _, _ := recipes.Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": second}, nil)
	want, _ := recipes.Multicall([][]byte{one, two})
	out := provider.Output{Actions: map[string]hexutil.Bytes{"0:0": first}, Repeats: []map[string]hexutil.Bytes{{"0:0": second}}}
	got, err := repeatActions(recipe, tree, one, []provider.Output{out})
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("batch: %v", err)
	}
	saved, _ := json.Marshal(out)
	var restored provider.Output
	_ = json.Unmarshal(saved, &restored)
	if !bytes.Equal(restored.Repeats[0]["0:0"], second) {
		t.Fatal("durable data lost")
	}
	for _, mode := range []string{"selector", "unowned", "input", "side effect"} {
		t.Run(mode, func(t *testing.T) {
			var changed provider.Output
			_ = json.Unmarshal(saved, &changed)
			r := recipes.CloneRecipe(recipe)
			switch mode {
			case "selector":
				changed.Repeats[0]["0:0"][0] ^= 1
			case "unowned":
				changed.Repeats[0] = map[string]hexutil.Bytes{"0:1": second}
			case "input":
				changed.Inputs = map[uint]string{0: "1"}
			case "side effect":
				r.Steps = append(r.Steps, r.Steps[0])
			}
			if _, err := repeatActions(r, tree, one, []provider.Output{changed}); err == nil {
				t.Fatal("unsafe repeated recipe accepted")
			}
		})
	}
}
