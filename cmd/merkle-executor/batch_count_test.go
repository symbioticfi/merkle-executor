package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	distributor "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/indexer"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/merkl"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestMerklPlannerNetworkRequestCounts(t *testing.T) {
	account, token, dist, aggregate, target, caller := common.HexToAddress("0x03"), common.HexToAddress("0x01"), common.HexToAddress("0x04"), common.HexToAddress("0x05"), common.HexToAddress("0x06"), common.HexToAddress("0x07")
	root := crypto.Keccak256Hash(protocolio.AddressWord(account), protocolio.AddressWord(token), protocolio.Word(big.NewInt(100)))
	claim := distributor.NewMerklDistributor().PackClaim([]common.Address{account}, []common.Address{token}, []*big.Int{big.NewInt(100)}, [][][32]byte{{}})
	leaf := recipes.Leaf{Target: dist, Value: new(big.Int), Data: claim, DataMask: make([]byte, len(claim))}
	var err error
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})}}
	recipeHash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[uint64]*types.Header{11: {Number: big.NewInt(11), Difficulty: new(big.Int), Time: 1000}, 12: {Number: big.NewInt(12), Difficulty: new(big.Int), Time: 1001}, 13: {Number: big.NewInt(13), Difficulty: new(big.Int), Time: 1002}}
	eABI := executor.NewRecipeExecutor().GetABI()
	var contract func(common.Address, []byte) []byte
	contract = func(to common.Address, data []byte) []byte {
		if to == aggregate {
			method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
			args, err := method.Inputs.Unpack(data[4:])
			if err != nil {
				t.Fatal(err)
			}
			calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
			results := make([]multicall.Multicall3Result, len(calls))
			for i, c := range calls {
				results[i] = multicall.Multicall3Result{Success: true, ReturnData: contract(c.Target, c.CallData)}
			}
			raw, err := method.Outputs.Pack(results)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
		var a abi.ABI
		switch to {
		case target:
			a = eABI
		case account:
			a = accountbinding.NewMerkleAccount().GetABI()
		case dist:
			a = distributor.NewMerklDistributor().GetABI()
		default:
			t.Fatalf("unexpected target %s", to)
		}
		method, err := a.MethodById(data[:4])
		if err != nil {
			t.Fatal(err)
		}
		var values []any
		switch method.Name {
		case "account":
			values = []any{account}
		case "caller":
			values = []any{caller}
		case "recipeHash":
			values = []any{recipeHash}
		case "recipeBlock":
			values = []any{big.NewInt(11)}
		case "root":
			values = []any{leaf.Hash}
		case "hasRole":
			values = []any{true}
		case "getMerkleRoot":
			values = []any{root}
		case "claimRecipient":
			values = []any{common.Address{}}
		case "claimed":
			values = []any{big.NewInt(80), big.NewInt(900), root}
		case "execute":
			values = []any{[]*big.Int{new(big.Int)}}
		default:
			t.Fatalf("unexpected getter %s", method.Name)
		}
		raw, err := method.Outputs.Pack(values...)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	requests, arrays, methods := 0, 0, 0
	currentHead := uint64(12)
	advanceOnValidation := false
	simulationBlock := `"0xc"`
	latestReads := 0
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&raw)
		type request struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		var reqs []request
		batch := raw[0] == '['
		if batch {
			arrays++
			_ = json.Unmarshal(raw, &reqs)
		} else {
			reqs = make([]request, 1)
			_ = json.Unmarshal(raw, &reqs[0])
		}
		requests++
		methods += len(reqs)
		responses := make([]any, len(reqs))
		for i, q := range reqs {
			var value any
			switch q.Method {
			case "eth_chainId":
				value = "0x1"
			case "eth_getBlockByNumber":
				if string(q.Params[0]) == `"latest"` {
					latestReads++
					if advanceOnValidation && latestReads == 2 {
						currentHead = 13
					}
				}
				n := currentHead
				if string(q.Params[0]) != `"latest"` {
					var encoded string
					_ = json.Unmarshal(q.Params[0], &encoded)
					parsed, err := hexutil.DecodeUint64(encoded)
					if err != nil {
						t.Error(err)
					}
					n = parsed
				}
				value = headers[n]
			case "eth_getLogs":
				t.Fatal("request-time log scan")

			case "eth_call":
				var call struct {
					To    common.Address
					From  common.Address
					Input hexutil.Bytes
				}
				_ = json.Unmarshal(q.Params[0], &call)
				if call.To == target && bytes.Equal(call.Input[:4], eABI.Methods["execute"].ID) {
					if call.From != caller || string(q.Params[1]) != simulationBlock {
						t.Errorf("simulation context changed caller=%s block=%s", call.From, q.Params[1])
					}
				}
				value = hexutil.Bytes(contract(call.To, call.Input))
			default:
				t.Fatalf("unexpected RPC %s", q.Method)
			}
			responses[i] = map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": value}
		}
		if batch {
			_ = json.NewEncoder(w).Encode(responses)
		} else {
			_ = json.NewEncoder(w).Encode(responses[0])
		}
	}))
	defer rpc.Close()
	rewards := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: []wire.MerklReward{{Token: wire.MerklToken{Address: token.Hex()}, Amount: "100", Claimed: "80", Pending: "0", Proofs: []string{}}}}})
	}))
	defer rewards.Close()
	backend, err := chain.Dial(context.Background(), rpc.URL, "", big.NewInt(1), aggregate)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	indexed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recipes/"+target.Hex() || r.URL.Query().Get("hash") != recipeHash.Hex() || r.URL.Query().Get("block") != "11" {
			t.Error("unbound recipe query")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"executor": target, "recipeHash": recipeHash, "blockNumber": "11", "blockHash": headers[11].Hash(), "transactionHash": common.HexToHash("0x1234"), "logIndex": 0, "queries": []hexutil.Bytes{}, "steps": []hexutil.Bytes{recipe.Steps[0]}})
	}))
	defer indexed.Close()
	backend.RecipePublications, err = indexer.New(indexed.URL, indexed.Client())
	if err != nil {
		t.Fatal(err)
	}

	cfg := merkl.Config{HTTP: protocolio.HTTPConfig{URL: rewards.URL}, Distributor: dist.Hex(), Multicall3: aggregate.Hex(), Tokens: []string{token.Hex()}, ProofLengths: []int{0}, Action: "0:0", CalldataBytes: len(claim), MinClaimable: "1"}
	p, err := merkl.New(cfg, provider.Deps{Chain: backend, HTTP: rewards.Client()})
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := json.Marshal(map[string]any{"root": leaf.Hash.Hex(), "leaves": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": dist.Hex(), "value": "zero", "data": hexutil.Encode(claim), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}}}})
	path := filepath.Join(t.TempDir(), "tree.json")
	if err := os.WriteFile(path, tree, 0600); err != nil {
		t.Fatal(err)
	}
	planner := keeper.Planner{Chain: backend, ChainID: big.NewInt(1), Caller: caller}

	var previous []byte
	for _, advanced := range []bool{false, true} {
		planner.RecipeCache = recipes.NewRecipeCache(1, 1<<20)
		t.Run(map[bool]string{false: "same", true: "advanced"}[advanced], func(t *testing.T) {
			for _, warm := range []bool{false, true} {
				t.Run(map[bool]string{false: "cold", true: "warm"}[warm], func(t *testing.T) {
					latestReads = 0
					currentHead = 12
					advanceOnValidation = advanced
					simulationBlock = `"0xc"`
					requests, arrays, methods = 0, 0, 0
					plan, err := planner.Prepare(t.Context(), keeper.Strategy{Config: keeper.Config{Account: account.Hex(), Executor: target.Hex(), Tree: path}, Providers: []provider.Provider{p}})
					wantMethods := 9
					if advanced {
						wantMethods = 11
					}
					wantRequests, wantArrays := 6, 5
					if warm {
						wantRequests, wantArrays = 5, 4
					}
					if err != nil || requests != wantRequests || arrays != wantArrays || methods != wantMethods || len(plan.Outputs) != 1 || !bytes.Equal(plan.Outputs[0].Actions["0:0"], claim) {
						t.Fatalf("Merkl Prepare requests=%d arrays=%d methods=%d error=%v", requests, arrays, methods, err)
					}
					if previous != nil && !bytes.Equal(previous, plan.Calldata) {
						t.Fatal("head advance changed approved execution")
					}
					previous = bytes.Clone(plan.Calldata)
					simulationBlock = `"latest"`
					requests, arrays, methods = 0, 0, 0
					wantMethods = 5
					if advanced {
						wantMethods = 6
					}
					if err := planner.Validate(t.Context(), plan); err != nil || requests != 3 || arrays != 3 || methods != wantMethods {
						t.Fatalf("Validate requests=%d arrays=%d methods=%d error=%v", requests, arrays, methods, err)
					}
				})
			}
		})
	}
}
