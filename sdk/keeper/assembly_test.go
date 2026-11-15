package keeper

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestProviderExecutionsUseNormalAssembly(t *testing.T) {
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
