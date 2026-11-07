package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type permissionSourceFunc func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error)

func (f permissionSourceFunc) PermissionTree(c context.Context, id *big.Int, s recipes.Snapshot) ([]byte, error) {
	return f(c, id, s)
}

type permissionReader struct {
	*validationReader
	log types.Log
}

func (r *permissionReader) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return []types.Log{r.log}, nil
}
func (r *permissionReader) CallContract(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	if bytes.Equal(m.Data, r.payload) {
		if m.To == nil || *m.To != r.snapshot.Executor || m.From != r.snapshot.Caller || b == nil || b.Uint64() != 12 {
			r.t.Fatal("wrong prepared simulation")
		}
		r.simulations++
		return r.returned, r.simulationErr
	}
	return r.validationReader.CallContract(ctx, m, b)
}

type permissionProvider struct{ calls int }

func (p *permissionProvider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	p.calls++
	return provider.Output{}, nil
}
func (*permissionProvider) Advance(context.Context, provider.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}
func TestPlannerPermissionSourceVerifiesAllBytesBeforeProviders(t *testing.T) {
	for _, mode := range []string{"valid", "local tree", "source error", "empty", "missing chain", "null chain", "wrong chain", "root", "account", "invalid unused leaf", "invalid unused proof", "mutate chain", "root during HTTP", "reorg during HTTP"} {
		t.Run(mode, func(t *testing.T) {
			header := &types.Header{Number: big.NewInt(12)}
			recipe := recipes.Recipe{}
			hash, _ := recipes.HashRecipe(recipe)
			snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: hash, RecipeBlock: 12, Root: common.HexToHash("0x04")}
			abi := executor.NewRecipeExecutor()
			returned, err := abi.GetABI().Methods["execute"].Outputs.Pack([]*big.Int{})
			if err != nil {
				t.Fatal(err)
			}
			event := abi.GetABI().Events["SetRecipe"]
			data, _ := event.Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
			r := &permissionReader{validationReader: &validationReader{t: t, snapshot: snapshot, header: header, payload: abi.PackExecute(nil, nil, nil, nil), returned: returned}, log: types.Log{Address: snapshot.Executor, BlockNumber: 12, BlockHash: header.Hash(), Topics: []common.Hash{event.ID, hash}, Data: data}}
			doc := map[string]any{"chainId": 1, "root": snapshot.Root.Hex(), "adapter": common.HexToAddress("0x999").Hex(), "leaves": []any{}}
			switch mode {
			case "missing chain":
				delete(doc, "chainId")
			case "null chain":
				doc["chainId"] = nil
			case "wrong chain":
				doc["chainId"] = 2
			case "root":
				doc["root"] = common.HexToHash("0x08").Hex()
			case "account":
				doc["account"] = common.HexToAddress("0x08").Hex()
			case "invalid unused leaf", "invalid unused proof":
				doc["leaves"] = []any{map[string]any{"hash": snapshot.Root.Hex(), "target": snapshot.Account.Hex(), "value": "zero", "data": "0x01020304", "dataMask": "0x00000000", "proof": []any{}}}
				if mode == "invalid unused proof" {
					leaf := recipes.Leaf{Target: snapshot.Account, Value: new(big.Int), Data: []byte{1, 2, 3, 4}, DataMask: make([]byte, 4)}
					hash, err := recipes.HashLeaf(leaf)
					if err != nil {
						t.Fatal(err)
					}
					doc["leaves"].([]any)[0].(map[string]any)["hash"] = hash.Hex()
				}
			}
			raw, _ := json.Marshal(doc)
			calls := 0
			planner := &Planner{Chain: r, ChainID: big.NewInt(1), Caller: snapshot.Caller, PermissionSource: permissionSourceFunc(func(ctx context.Context, id *big.Int, s recipes.Snapshot) ([]byte, error) {
				calls++
				if ctx != t.Context() || id.Cmp(big.NewInt(1)) != 0 || s != snapshot {
					t.Fatal("source did not receive authenticated pin")
				}
				switch mode {
				case "source error":
					return nil, context.Canceled
				case "empty":
					return nil, nil
				case "mutate chain":
					id.SetInt64(99)
				case "root during HTTP":
					r.snapshot.Root = common.HexToHash("0x99")
				case "reorg during HTTP":
					r.header = types.CopyHeader(r.header)
					r.header.Extra = []byte("reorg")
				}
				return raw, nil
			})}
			strategy := Strategy{Config: Config{Account: snapshot.Account.Hex(), Executor: snapshot.Executor.Hex()}}
			if mode == "local tree" {
				planner.PermissionSource = nil
				delete(doc, "chainId")
				raw, _ = json.Marshal(doc)
				strategy.Config.Tree = filepath.Join(t.TempDir(), "tree.json")
				if err := os.WriteFile(strategy.Config.Tree, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			pr := &permissionProvider{}
			strategy.Providers = []provider.Provider{pr}
			_, err = planner.Prepare(t.Context(), strategy)
			ok := mode == "valid" || mode == "local tree" || mode == "mutate chain"
			if ok && !errors.Is(err, provider.ErrNotReady) || !ok && (err == nil || errors.Is(err, provider.ErrNotReady)) {
				t.Fatalf("mode %s: %v", mode, err)
			}
			if planner.ChainID.Int64() != 1 {
				t.Fatal("source mutated planner identity")
			}
			if mode == "source error" && !errors.Is(err, context.Canceled) {
				t.Fatal("source cancellation cause lost")
			}
			if !ok && mode != "root during HTTP" && mode != "reorg during HTTP" && pr.calls != 0 {
				t.Fatal("invalid tree reached provider")
			}
			if mode != "local tree" && calls != 1 {
				t.Fatal("source unexpectedly retried")
			}
		})
	}
}

func TestPlannerPermissionSourceRequiresValidChainBeforeReads(t *testing.T) {
	for name, id := range map[string]*big.Int{"nil": nil, "zero": new(big.Int), "negative": big.NewInt(-1), "overflow": new(big.Int).Lsh(big.NewInt(1), 256)} {
		t.Run(name, func(t *testing.T) {
			r := &validationReader{t: t}
			planner := Planner{Chain: r, ChainID: id, PermissionSource: permissionSourceFunc(func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error) {
				t.Fatal("invalid identity reached source")
				return nil, nil
			})}
			if _, err := planner.Prepare(t.Context(), Strategy{}); err == nil || r.reads != 0 {
				t.Fatalf("invalid chain reached reads: %v reads%d", err, r.reads)
			}
		})
	}
}

func TestPlannerValidationDoesNotFetchPermissions(t *testing.T) {
	header := &types.Header{Number: big.NewInt(12)}
	snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: common.HexToHash("0x04"), Root: common.HexToHash("0x05"), RecipeBlock: 11}
	payload, returned := validationWork(t, true)
	r := &validationReader{t: t, snapshot: snapshot, header: header, payload: payload, returned: returned}
	planner := Planner{Chain: r, Caller: snapshot.Caller, PermissionSource: permissionSourceFunc(func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error) {
		t.Fatal("validation contacted unavailable permission source")
		return nil, errors.New("source unavailable")
	})}
	plan := Plan{Recipe: snapshot, Snapshot: provider.Snapshot{Executor: snapshot.Executor}, Calldata: r.payload}
	if err := planner.Validate(t.Context(), plan); err != nil || r.simulations != 1 {
		t.Fatalf("existing plan validation failed: %v simulations%d", err, r.simulations)
	}
}
