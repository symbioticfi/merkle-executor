package keeper

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type batchProvider struct{ out provider.Output }

func (p batchProvider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return p.out, nil
}
func (p batchProvider) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) (provider.Output, error) {
	return p.out, nil
}
func (batchProvider) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return RecipeRequirements{Actions: []string{"0:0"}}, nil
}
func (batchProvider) Advance(context.Context, provider.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}
func TestPlannerBatchesAndPersistsRepeatedClaimCalldata(t *testing.T) {
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
	hash, _ := recipes.HashRecipe(recipe)
	tree := &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	header := &types.Header{Number: big.NewInt(12)}
	snap := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: hash, RecipeBlock: 12, Root: leaf.Hash}
	first := append(hexutil.Bytes{}, leaf.Data...)
	second := append(hexutil.Bytes{}, leaf.Data...)
	second[35] = 9
	one, _, _ := recipes.Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": first}, nil)
	two, _, _ := recipes.Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": second}, nil)
	payload, err := recipes.Multicall([][]byte{one, two})
	if err != nil {
		t.Fatal(err)
	}
	e := executor.NewRecipeExecutor()
	event := e.GetABI().Events["SetRecipe"]
	eventData, _ := event.Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
	reader := &exclusiveRecipeReader{permissionReader: &permissionReader{validationReader: &validationReader{t: t, snapshot: snap, header: header, payload: payload}, log: types.Log{Address: snap.Executor, BlockNumber: 12, BlockHash: header.Hash(), Topics: []common.Hash{event.ID, hash}, Data: eventData}}}
	document, _ := json.Marshal(map[string]any{"chainId": 1, "root": leaf.Hash, "leaves": []any{map[string]any{"hash": leaf.Hash, "target": leaf.Target, "value": "0", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}}}})
	planner := Planner{Chain: reader, ChainID: big.NewInt(1), Caller: snap.Caller, PermissionSource: permissionSourceFunc(func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error) { return document, nil })}
	out := provider.Output{Actions: map[string]hexutil.Bytes{"0:0": first}, Repeats: []map[string]hexutil.Bytes{{"0:0": second}}}
	plan, err := planner.Prepare(t.Context(), Strategy{Config: Config{Account: snap.Account.Hex(), Executor: snap.Executor.Hex()}, Providers: []provider.Provider{batchProvider{out}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(plan.Calldata) != string(payload) || reader.simulations != 1 {
		t.Fatal("batch was not fully simulated")
	}
	saved, _ := json.Marshal(plan)
	var restored Plan
	if err = json.Unmarshal(saved, &restored); err != nil {
		t.Fatal(err)
	}
	if string(restored.Outputs[0].Repeats[0]["0:0"]) != string(second) {
		t.Fatal("durable plan lost repeated request")
	}
}
