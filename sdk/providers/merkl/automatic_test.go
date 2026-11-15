package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func frontendClaims(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	raw, err := os.ReadFile("testdata/frontend-guarded.json")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: common.HexToAddress("0x104"), Timestamp: 1000}
	var fixture struct{ Recipe hexutil.Bytes }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	r, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}

	return s, r, tree
}

func TestAutomaticOwnsAllFrontendGuardedClaims(t *testing.T) {
	s, r, tree := frontendClaims(t)
	p := &Automatic{deps: provider.Deps{Chain: reader(nil)}}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Actions) != 2 || len(req.Inputs) != 4 {
		t.Fatalf("guarded claims not fully owned: %+v", req)
	}
}

func TestAutomaticPreservesCuratorMask(t *testing.T) {
	s, r, tree := frontendClaims(t)
	// A broader permission mask does not change the bytes supplied by the provider.
	leaf := tree.Leaves[0]
	for _, candidate := range tree.Leaves {
		if candidate.Target == s.Account {
			leaf = candidate
			break
		}
	}
	old := leaf.Hash
	leaf.DataMask[4] = 0xff
	var err error
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	leaf.Proof = nil
	tree = &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	var managed []byte
	for _, step := range r.Steps {
		if step[0] == recipes.Manage && bytes.Contains(step, old[:]) {
			managed = step
			break
		}
	}
	r.Steps = [][]byte{bytes.Replace(managed, old[:], leaf.Hash[:], 1)}
	p := &Automatic{deps: provider.Deps{Chain: reader(nil)}}
	_, err = p.RecipeRequirements(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticPreparesAllFrontendClaimsWithOneRewardsFetch(t *testing.T) {
	for _, tc := range []struct {
		name               string
		claimedA, claimedB int64
		corrupt            bool
		literalGate        bool
		want               string
	}{
		{name: "both positive"},
		{name: "idle A and positive B", claimedA: 100},
		{name: "positive A and idle B", claimedB: 200},
		{name: "all idle optional gates", claimedA: 100, claimedB: 200},
		{name: "all idle nonoptional claims", claimedA: 100, claimedB: 200, literalGate: true, want: "rewards below minClaimable"},
		{name: "idle first cannot hide invalid second proof", claimedB: 200, corrupt: true, want: "proof does not match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tree := frontendClaims(t)
			if tc.literalGate {
				for _, step := range r.Steps {
					if step[0] != recipes.Manage {
						continue
					}
					clear(step[1:33])
					step[32] = 1
				}
			}
			raw, err := os.ReadFile("testdata/frontend-guarded.json")
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				RewardRoot common.Hash
				Rewards    []struct {
					Token      common.Address
					Cumulative string
					Proof      []string
				}
			}
			if err = json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			root := fixture.RewardRoot
			rows := make([]wire.MerklReward, len(fixture.Rewards))
			for i, reward := range fixture.Rewards {
				rows[i] = wire.MerklReward{Token: wire.MerklToken{Address: reward.Token.Hex(), Decimals: 18}, Amount: reward.Cumulative, Claimed: "0", Pending: "0", Proofs: reward.Proof}
			}
			if tc.corrupt {
				rows[0].Amount = "101"
			}
			fetches, reads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fetches++
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
			}))
			defer server.Close()
			endpoint, _ := url.Parse(server.URL)
			httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				copy := request.Clone(request.Context())
				copy.URL.Host = endpoint.Host
				copy.URL.Scheme = endpoint.Scheme
				return http.DefaultTransport.RoundTrip(copy)
			})}
			chain := reader(func(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
				reads++
				method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				args, err := method.Inputs.Unpack(msg.Data[4:])
				if err != nil {
					return nil, err
				}
				calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
				if len(calls) != 6 {
					t.Fatalf("shared claim reads = %d, want 6", len(calls))
				}
				for _, call := range calls {
					if !call.AllowFailure {
						t.Fatal("speculative getter can mask an earlier claim branch")
					}
				}

				results := make([]multicall.Multicall3Result, len(calls))
				dist := binding.NewMerklDistributor()
				contractABI := dist.GetABI()
				for i, call := range calls {
					getter, err := contractABI.MethodById(call.CallData[:4])
					if err != nil {
						return nil, err
					}
					var reply []byte
					switch getter.Name {
					case "getMerkleRoot":
						reply, err = getter.Outputs.Pack(root)
					case "claimRecipient":
						reply, err = getter.Outputs.Pack(common.Address{})
					case "claimed":
						values, _ := getter.Inputs.Unpack(call.CallData[4:])
						amount := tc.claimedA
						if values[1].(common.Address) == common.HexToAddress("0x202") {
							amount = tc.claimedB
						}
						reply, err = getter.Outputs.Pack(big.NewInt(amount), big.NewInt(0), root)
					}
					if err != nil {
						return nil, err
					}
					results[i] = multicall.Multicall3Result{Success: true, ReturnData: reply}
				}
				return method.Outputs.Pack(results)
			})
			p := &Automatic{deps: provider.Deps{Chain: chain, HTTP: httpClient}}
			outputs, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			var out provider.Output
			if len(outputs) == 1 {
				out = outputs[0]
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %q, got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Actions) != 2 || fetches != 1 || reads != 1 {
				t.Fatalf("partial or repeated preparation: actions=%d HTTP=%d reads=%d", len(out.Actions), fetches, reads)
			}
			for i := 0; i < 2; i++ {
				wantCum, wantDelta := "200", big.NewInt(200-tc.claimedB).String()
				if i == 0 {
					wantCum, wantDelta = "100", big.NewInt(100-tc.claimedA).String()
				}
				if out.Inputs[uint(2*i)] != wantCum || out.Inputs[uint(2*i+1)] != wantDelta {
					t.Fatalf("cumulative/delta differs: %v", out.Inputs)
				}
			}
			if out.Task != nil {
				t.Fatal("synchronous claim retained settlement work")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAutomaticUsesCuratorClaimGuard(t *testing.T) {
	s, r, tree := frontendClaims(t)
	// The curator can choose another comparison without changing claim fields.
	raw, _ := layoutArgs("uint256", "uint256").Pack(new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(18)), new(big.Int).Lsh(big.NewInt(2), 216))
	needle := append([]byte{recipes.Equal}, raw...)
	actions, err := recipes.ManagedActions(r, tree)
	if err != nil {
		t.Fatal(err)
	}
	action := actions[0]
	at := bytes.Index(action.Leaf.Data, needle)
	if at < 0 {
		t.Fatal("frontend root guard absent")
	}
	action.Leaf.Data[at] = recipes.LessOrEqual
	_, err = claimLayoutOf(action, s.Account)
	if err != nil {
		t.Fatal(err)
	}
}

type batchRewardReader struct{ reader }

func (r batchRewardReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	results := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		if request.Kind != protocolio.ReadCall {
			return nil, errors.New("unexpected non-contract reward read")
		}
		results[i].Data, results[i].Err = r.CallContract(ctx, request.Call, request.Block)
	}
	return results, nil
}

func TestComposedRuntimeClaimReadsOnlyVisitedCandidates(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		for _, tc := range []struct {
			name         string
			firstClaimed bool
			failedToken  int64
			wantToken    int64
			wantGetters  int
			wantRequests int
			wantErr      bool
		}{
			{name: "first positive ignores later failed getter", failedToken: 227, wantToken: 100, wantGetters: 4, wantRequests: 1},
			{name: "second positive reuses shared getters", firstClaimed: true, wantToken: 101, wantGetters: 6, wantRequests: 2},
			{name: "visited failed getter is rejected", firstClaimed: true, failedToken: 101, wantGetters: 6, wantRequests: 2, wantErr: true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				raw, err := os.ReadFile("testdata/frontend-current.json")
				if err != nil {
					t.Fatal(err)
				}
				var fixture struct {
					Recipe  hexutil.Bytes
					Account common.Address
				}
				if err = json.Unmarshal(raw, &fixture); err != nil {
					t.Fatal(err)
				}
				tree, err := recipes.DecodeTree(raw)
				if err != nil {
					t.Fatal(err)
				}
				recipe, err := recipes.DecodeRecipe(fixture.Recipe)
				if err != nil {
					t.Fatal(err)
				}
				rows, root, _ := rewardInventory(t, fixture.Account, 128)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
				}))
				defer server.Close()
				endpoint, _ := url.Parse(server.URL)
				client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					request = request.Clone(request.Context())
					request.URL.Host, request.URL.Scheme = endpoint.Host, endpoint.Scheme
					return http.DefaultTransport.RoundTrip(request)
				})}
				requests, getters := 0, 0
				seen := map[string]bool{}
				read := reader(func(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
					if block == nil || block.Int64() != 12 || msg.To == nil || *msg.To != common.HexToAddress(protocolio.StandardMulticall3) || msg.From != (common.Address{}) {
						t.Fatal("reward reads changed the pinned block, target or caller")
					}
					requests++
					aggregate := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
					args, err := aggregate.Inputs.Unpack(msg.Data[4:])
					if err != nil {
						return nil, err
					}
					calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
					results := make([]multicall.Multicall3Result, len(calls))
					contract := binding.NewMerklDistributor().GetABI()
					for i, call := range calls {
						getters++
						key := call.Target.Hex() + string(call.CallData)
						if seen[key] {
							t.Fatal("reward getter was repeated")
						}
						seen[key] = true
						method, err := contract.MethodById(call.CallData[:4])
						if err != nil {
							return nil, err
						}
						var data []byte
						switch method.Name {
						case "getMerkleRoot":
							data, err = method.Outputs.Pack(root)
						case "claimRecipient":
							data, err = method.Outputs.Pack(common.Address{})
						case "claimed":
							values, unpackErr := method.Inputs.Unpack(call.CallData[4:])
							if unpackErr != nil {
								return nil, unpackErr
							}
							token := values[1].(common.Address)
							if tc.failedToken != 0 && token == addr(tc.failedToken) {
								continue
							}
							claimed := new(big.Int)
							if tc.firstClaimed && token == addr(100) {
								claimed.SetInt64(100)
							}
							data, err = method.Outputs.Pack(claimed, new(big.Int), root)
						default:
							t.Fatalf("unexpected reward getter: %s", method.Name)
						}
						if err != nil {
							return nil, err
						}
						results[i] = multicall.Multicall3Result{Success: true, ReturnData: data}
					}
					return aggregate.Outputs.Pack(results)
				})
				var chain provider.Reader = read
				if mode == "batch" {
					chain = batchRewardReader{read}
				}
				p := &Automatic{deps: provider.Deps{Chain: chain, HTTP: client}}
				snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: fixture.Account, Timestamp: 1000}
				outputs, err := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, nil)
				if tc.wantErr {
					if err == nil || !strings.Contains(err.Error(), "getter failed") {
						t.Fatalf("visited getter failure accepted: %v", err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if len(outputs) != 1 || outputs[0].Inputs[0] != big.NewInt(tc.wantToken).String() || outputs[0].Inputs[1] != "100" || len(outputs[0].Actions) != 1 {
						t.Fatalf("runtime claim selection changed: %+v", outputs)
					}
					for _, payload := range outputs[0].Actions {
						values, err := binding.NewMerklDistributor().GetABI().Methods["claim"].Inputs.Unpack(payload[4:])
						if err != nil {
							t.Fatal(err)
						}
						if values[1].([]common.Address)[0] != addr(tc.wantToken) || values[2].([]*big.Int)[0].Int64() != 100 || len(values[3].([][][32]byte)[0]) != 7 {
							t.Fatalf("selected claim payload changed: %v", values)
						}
					}
				}
				if getters != tc.wantGetters || requests != tc.wantRequests {
					t.Fatalf("unvisited reward reads: getters=%d requests=%d, want %d/%d", getters, requests, tc.wantGetters, tc.wantRequests)
				}
			})
		}
	}
}
