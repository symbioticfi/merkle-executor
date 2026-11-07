package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/aave"
	"github.com/symbioticfi/merkle-executor/sdk/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/sdk/providers/liquidlanerfq"
	"github.com/symbioticfi/merkle-executor/sdk/providers/merkl"
	"github.com/symbioticfi/merkle-executor/sdk/providers/threef"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilReadBatchAggregatesMetadataAndRolesButIsolatesCalls(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for local batch integration")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) < 3 {
		t.Fatalf("fixture owners: %v", err)
	}
	owner := owners[0]
	aa, ta, ea := loadServiceArtifact(t, "FixtureAccount"), loadServiceArtifact(t, "FixtureToken"), loadServiceArtifact(t, "RecipeExecutor")
	account := serviceSeedTransaction(t, ctx, client, owner, nil, aa.Bytecode).ContractAddress
	token := serviceSeedTransaction(t, ctx, client, owner, nil, ta.Bytecode).ContractAddress
	target := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, account)...)).ContractAddress
	aggregate := serviceSeedTransaction(t, ctx, client, owner, nil, loadMerklArtifact(t, "Multicall3").Bytecode).ContractAddress
	tokenABI, executorABI := erc20.NewERC20(), executor.NewRecipeExecutor()
	leaves := make([]recipes.Leaf, 2)
	for i := range leaves {
		payload := tokenABI.PackTransfer(owners[i+1], big.NewInt(int64(i+1)))
		leaves[i] = recipes.Leaf{Target: token, Value: new(big.Int), Data: payload, DataMask: make([]byte, len(payload))}
		var err error
		leaves[i].Hash, err = recipes.HashLeaf(leaves[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	first, second := leaves[0].Hash, leaves[1].Hash
	if bytes.Compare(first[:], second[:]) > 0 {
		first, second = second, first
	}
	root := crypto.Keccak256Hash(first[:], second[:])
	hashes := [][32]byte{leaves[0].Hash, leaves[1].Hash}
	serviceSeedTransaction(t, ctx, client, owner, &account, servicePack(t, aa, "configure", root, hashes, target))
	serviceSeedTransaction(t, ctx, client, owner, &token, servicePack(t, ta, "mint", account, big.NewInt(6)))
	serviceSeedTransaction(t, ctx, client, owner, &target, executorABI.PackSetCaller(owner))
	recipe := recipes.Recipe{Steps: [][]byte{issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), hashes, [][]*big.Int{{}, {}})}}
	serviceSeedTransaction(t, ctx, client, owner, &target, executorABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	if err := client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	entries := make([]any, 2)
	for i, leaf := range leaves {
		entries[i] = map[string]any{"hash": leaf.Hash.Hex(), "target": token.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{leaves[1-i].Hash.Hex()}}
	}
	tree, _ := json.Marshal(map[string]any{"root": root.Hex(), "account": account.Hex(), "chainId": 31337, "leaves": entries})
	treePath := filepath.Join(t.TempDir(), "tree.json")
	if err := os.WriteFile(treePath, tree, 0600); err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(endpoint)
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	var mu sync.Mutex
	requests, arrays, methods := 0, 0, 0
	var aggregateSizes []int
	advanceOnValidation := false
	latestReads := 0
	var simulationBlock string
	var simulationGas uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		var batch []struct {
			Method string
			Params []json.RawMessage
		}
		isArray := raw[0] == '['
		if isArray {
			_ = json.Unmarshal(raw, &batch)
		} else {
			batch = make([]struct {
				Method string
				Params []json.RawMessage
			}, 1)
			_ = json.Unmarshal(raw, &batch[0])
		}
		mu.Lock()
		requests++
		methods += len(batch)
		if isArray {
			arrays++
		}
		for _, q := range batch {
			if q.Method != "eth_call" {
				continue
			}
			var call struct {
				To    common.Address
				From  common.Address
				Input hexutil.Bytes
				Gas   hexutil.Uint64
			}
			_ = json.Unmarshal(q.Params[0], &call)
			if simulationBlock != "" && call.To == target && bytes.Equal(call.Input[:4], executorABI.GetABI().Methods["execute"].ID) {
				if call.From != owner || string(q.Params[1]) != simulationBlock || uint64(call.Gas) != simulationGas {
					t.Errorf("simulation context changed caller=%s block=%s gas=%d", call.From, q.Params[1], call.Gas)
				}
			}
			if call.To == aggregate {
				method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				args, err := method.Inputs.Unpack(call.Input[4:])
				if err != nil {
					t.Error(err)
				} else {
					calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
					aggregateSizes = append(aggregateSizes, len(calls))
				}
			}
		}
		shouldAdvance := false
		for _, q := range batch {
			if q.Method == "eth_getBlockByNumber" && string(q.Params[0]) == `"latest"` {
				latestReads++
				if advanceOnValidation && latestReads == 2 {
					shouldAdvance = true
				}
			}
		}
		mu.Unlock()
		if shouldAdvance {
			if err := client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
				t.Error(err)
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	defer server.Close()
	batched, err := chain.Dial(ctx, server.URL, "", big.NewInt(31337), aggregate)
	if err != nil {
		t.Fatal(err)
	}
	defer batched.Close()
	reset := func() {
		mu.Lock()
		defer mu.Unlock()
		requests, arrays, methods, latestReads = 0, 0, 0, 0
		aggregateSizes = nil
	}
	planner := keeper.Planner{Chain: batched, ChainID: big.NewInt(31337), Caller: owner}
	var plan keeper.Plan
	for _, advanced := range []bool{false, true} {
		planner.RecipeCache = recipes.NewRecipeCache(1, 1<<20)
		for _, warm := range []bool{false, true} {
			reset()
			mu.Lock()
			advanceOnValidation = advanced
			mu.Unlock()
			current, err := planner.Prepare(ctx, keeper.Strategy{Config: keeper.Config{Account: account.Hex(), Executor: target.Hex(), Tree: treePath}})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Calldata == nil {
				plan = current
			}
			if !bytes.Equal(current.Calldata, plan.Calldata) || !reflect.DeepEqual(current.Leaves, plan.Leaves) {
				t.Fatal("warm cache/head advance changed authorized bytes or leaves")
			}
			wantRequests, wantArrays, wantMethods := 8, 2, 11
			wantAggregates := []int{5, 7}
			if advanced {
				wantMethods = 13
				wantArrays++
			}
			if warm {
				wantMethods--
				wantArrays--
			}
			mu.Lock()
			if requests != wantRequests || arrays != wantArrays || methods != wantMethods || !reflect.DeepEqual(aggregateSizes, wantAggregates) {
				t.Errorf("Prepare warm=%t advanced=%t requests=%d arrays=%d methods=%d aggregates=%v", warm, advanced, requests, arrays, methods, aggregateSizes)
			}
			mu.Unlock()
			reset()
			// Revalidation uses the same grouped admission as the planner, including
			// current roles, a fresh simulation pin, the caller and the gas budget.
			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			simulationBlock, simulationGas = `"`+hexutil.EncodeBig(header.Number)+`"`, header.GasLimit
			mu.Unlock()
			if err := recipes.ValidatePreparedBatch(ctx, planner.Chain,
				[]recipes.PreparedRecipe{{Snapshot: current.Recipe, Recipe: recipe}},
				[]recipes.ExecutorCall{{Executor: target, Data: current.Calldata}},
				[][]common.Hash{current.Leaves}, []map[uint]string{current.Results}, planner.Caller); err != nil {
				t.Fatal(err)
			}
			wantMethods = 5
			wantArrays = 1
			if advanced {
				wantMethods = 7
				wantArrays = 2
			}
			mu.Lock()
			simulationBlock = ""
			if requests != 3 || arrays != wantArrays || methods != wantMethods || !reflect.DeepEqual(aggregateSizes, []int{7}) {
				t.Errorf("Validate warm=%t advanced=%t requests=%d arrays=%d methods=%d aggregates=%v", warm, advanced, requests, arrays, methods, aggregateSizes)
			}
			mu.Unlock()
		}
	}
	reset()
	var calls []protocolio.ReadRequest
	for _, data := range [][]byte{
		tokenABI.PackTransfer(owners[1], big.NewInt(1)),
		tokenABI.PackApprove(owners[1], big.NewInt(1)),
		tokenABI.PackApprove(owners[1], big.NewInt(2)),
	} {
		calls = append(calls, protocolio.ReadRequest{Kind: protocolio.ReadCall, Call: ethereum.CallMsg{From: account, To: &token, Data: data}, Block: plan.Snapshot.BlockNumber})
	}
	reads, err := protocolio.ReadGroup(ctx, batched, calls...)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		raw, err := reads.At(i).CallData()
		if err != nil || new(big.Int).SetBytes(raw).Cmp(big.NewInt(1)) != 0 {
			t.Fatalf("caller/isolation changed: %x %v", raw, err)
		}
	}
	mu.Lock()
	if requests != 3 || arrays != 0 || methods != 3 || len(aggregateSizes) != 0 {
		t.Errorf("isolated calls requests=%d arrays=%d methods=%d aggregates=%v", requests, arrays, methods, aggregateSizes)
	}
	mu.Unlock()
	for _, check := range []struct {
		data []byte
		want int64
	}{{tokenABI.PackBalanceOf(account), 6}, {tokenABI.PackAllowance(account, owners[1]), 0}} {
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: check.data}, nil)
		if err != nil || new(big.Int).SetBytes(raw).Int64() != check.want {
			t.Fatalf("read mutated chain: %x %v", raw, err)
		}
	}

	// The daemon's installed providers must not prevent a native recipe from
	// batching when the installed providers have no matching runtime work.
	query := issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, token, tokenABI.PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
	result := new(big.Int).Lsh(big.NewInt(int64(recipes.Result)), 216)
	dynamic := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, new(big.Int)),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result, hashes, [][]*big.Int{{}, {}}),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &target, executorABI.PackSetRecipe(dynamic.Queries, dynamic.Steps))
	strategy := keeper.Strategy{Config: keeper.Config{Account: account.Hex(), Executor: target.Hex(), Tree: treePath}}
	for _, newProvider := range []func(provider.Deps) (keeper.Provider, error){cowswap.NewAutomatic, merkl.NewAutomatic, liquidlanerfq.NewAutomatic, threef.NewAutomatic} {
		p, err := newProvider(provider.Deps{Chain: batched})
		if err != nil {
			t.Fatal(err)
		}
		strategy.Providers = append(strategy.Providers, p)
	}
	capacity, err := aave.New(batched)
	if err != nil {
		t.Fatal(err)
	}
	strategy.Providers = append(strategy.Providers, capacity)
	current, err := planner.Prepare(ctx, strategy)
	if err != nil {
		t.Fatal(err)
	}
	method := executorABI.GetABI().Methods["multicall"]
	if !bytes.Equal(current.Calldata[:4], method.ID) {
		t.Fatal("native recipe was not automatically batched")
	}
	args, err := method.Inputs.Unpack(current.Calldata[4:])
	if err != nil || len(args[0].([][]byte)) != 2 {
		t.Fatalf("expected two complete executions: %v", err)
	}
	if len(current.Outputs) != len(strategy.Providers) {
		t.Fatal("provider output groups were lost")
	}
	for _, group := range current.Outputs {
		for _, out := range group {
			if len(out.Actions)+len(out.Inputs)+len(out.SkipSteps) != 0 || out.Task != nil {
				t.Fatal("unmatched provider acquired runtime ownership")
			}
		}
	}
	serviceSeedTransaction(t, ctx, client, owner, &target, current.Calldata)
	if raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: tokenABI.PackBalanceOf(account)}, nil); err != nil || new(big.Int).SetBytes(raw).Sign() != 0 {
		t.Fatalf("batch did not consume both cycles: %x %v", raw, err)
	}
}
