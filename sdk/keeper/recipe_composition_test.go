package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type exclusiveRecipeProvider struct {
	target common.Address
	calls  int
}

func (p *exclusiveRecipeProvider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	p.calls++
	return provider.Output{}, nil
}
func (p *exclusiveRecipeProvider) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return RecipeRequirements{Actions: []string{"0:0"}, ExclusiveTargets: map[string]common.Address{"0:0": p.target}}, nil
}
func (p *exclusiveRecipeProvider) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) (provider.Output, error) {
	p.calls++
	return provider.Output{}, nil
}
func (*exclusiveRecipeProvider) Advance(context.Context, provider.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}
func (p *exclusiveRecipeProvider) ExclusiveOutputTargets(provider.Output) (map[string]common.Address, error) {
	return map[string]common.Address{"0:0": p.target}, nil
}

type exclusiveRecipeReader struct{ *permissionReader }

func (r *exclusiveRecipeReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	method := account.NewMerkleAccount().GetABI().Methods["hasRole"]
	if msg.To != nil && *msg.To == r.snapshot.Account && len(msg.Data) >= 4 && bytes.Equal(msg.Data[:4], method.ID) {
		return method.Outputs.Pack(true)
	}
	return r.permissionReader.CallContract(ctx, msg, block)
}
func TestPlannerExclusiveRecipePreflightBeforeProviderIO(t *testing.T) {
	target := common.HexToAddress("0x77")
	pack := func(names []string, args ...any) []byte {
		var types abi.Arguments
		for _, name := range names {
			typ, err := abi.NewType(name, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			types = append(types, abi.Argument{Type: typ})
		}
		raw, err := types.Pack(args...)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, mode := range []string{"valid", "unowned target", "direct Call"} {
		t.Run(mode, func(t *testing.T) {
			header := &types.Header{Number: big.NewInt(12)}
			leaf := recipes.Leaf{Target: target, Value: new(big.Int), Data: []byte{1, 2, 3, 4}, DataMask: make([]byte, 4)}
			var err error
			leaf.Hash, err = recipes.HashLeaf(leaf)
			if err != nil {
				t.Fatal(err)
			}
			manage := append([]byte{recipes.Manage}, pack([]string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})...)
			recipe := recipes.Recipe{Steps: [][]byte{manage}}
			if mode == "unowned target" {
				recipe.Steps = append(recipe.Steps, manage)
			}
			if mode == "direct Call" {
				recipe.Steps = append(recipe.Steps, append([]byte{recipes.Call}, pack([]string{"address", "bytes", "uint256[]"}, target, []byte{1, 2, 3, 4}, []*big.Int{})...))
			}
			hash, _ := recipes.HashRecipe(recipe)
			snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: hash, RecipeBlock: 12, Root: leaf.Hash}
			e := executor.NewRecipeExecutor()
			event := e.GetABI().Events["SetRecipe"]
			data, _ := event.Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
			tree := &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
			payload, _, err := recipes.Assemble(recipe, tree, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			reader := &exclusiveRecipeReader{permissionReader: &permissionReader{validationReader: &validationReader{t: t, snapshot: snapshot, header: header, payload: payload}, log: types.Log{Address: snapshot.Executor, BlockNumber: 12, BlockHash: header.Hash(), Topics: []common.Hash{event.ID, hash}, Data: data}}}
			raw, _ := json.Marshal(map[string]any{"chainId": 1, "root": leaf.Hash.Hex(), "leaves": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}}}})
			planner := Planner{Chain: reader, ChainID: big.NewInt(1), Caller: snapshot.Caller, PermissionSource: permissionSourceFunc(func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error) { return raw, nil })}
			pr := &exclusiveRecipeProvider{target: target}
			_, err = planner.Prepare(t.Context(), Strategy{Config: Config{Account: snapshot.Account.Hex(), Executor: snapshot.Executor.Hex()}, Providers: []provider.Provider{pr}})
			if mode == "valid" {
				if err != nil || pr.calls != 1 {
					t.Fatalf("valid claimed action failed: %v calls=%d", err, pr.calls)
				}
			} else if err == nil || pr.calls != 0 {
				t.Fatalf("ambiguous recipe reached provider: %v calls=%d", err, pr.calls)
			}
		})
	}
}
