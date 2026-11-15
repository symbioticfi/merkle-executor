package merkl

import (
	"context"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestAutomaticBatchesDistinctRewardsUsingSingleClaimRecipe(t *testing.T) {
	raw, err := os.ReadFile("testdata/frontend-single-claim.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Account common.Address
		Recipe  hexutil.Bytes
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
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: fixture.Account}
	tokens := []common.Address{addr(201), addr(202)}
	hashes := make([]common.Hash, 2)
	for i, token := range tokens {
		hashes[i] = crypto.Keccak256Hash(common.LeftPadBytes(snapshot.Account.Bytes(), 32), common.LeftPadBytes(token.Bytes(), 32), common.LeftPadBytes(big.NewInt(int64(100+i)).Bytes(), 32))
	}
	first, second := hashes[0], hashes[1]
	if first.Cmp(second) > 0 {
		first, second = second, first
	}
	root := crypto.Keccak256Hash(first[:], second[:])
	rewards := make([]wire.MerklReward, 2)
	for i, token := range tokens {
		rewards[i] = wire.MerklReward{Token: wire.MerklToken{Address: token.Hex(), Decimals: 6}, Amount: big.NewInt(int64(100 + i)).String(), Claimed: "0", Pending: "0", Proofs: []string{hashes[1-i].Hex()}}
	}
	for _, test := range []string{"both ready", "first claimed", "all claimed", "bad proof"} {
		t.Run(test, func(t *testing.T) {
			rows := append([]wire.MerklReward(nil), rewards...)
			if test == "bad proof" {
				rows[1].Amount = "999"
			}
			fetches, aggregates := 0, 0
			getterCounts := map[string]int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fetches++
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
			}))
			defer srv.Close()
			endpoint, _ := url.Parse(srv.URL)
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r = r.Clone(r.Context())
				r.URL.Host = endpoint.Host
				r.URL.Scheme = endpoint.Scheme
				return http.DefaultTransport.RoundTrip(r)
			})}
			underlying := reader(func(_ context.Context, m ethereum.CallMsg, _ *big.Int) ([]byte, error) {
				a := binding.NewMerklDistributor().GetABI()
				method, e := a.MethodById(m.Data[:4])
				if e != nil {
					return nil, e
				}
				getterCounts[method.Name]++
				switch method.Name {
				case "getMerkleRoot":
					return method.Outputs.Pack(root)
				case "claimRecipient":
					return method.Outputs.Pack(common.Address{})
				case "claimed":
					v, _ := method.Inputs.Unpack(m.Data[4:])
					n := int64(0)
					if test == "all claimed" || test == "first claimed" && v[1].(common.Address) == tokens[0] {
						n = 100
						if v[1].(common.Address) == tokens[1] {
							n = 101
						}
					}
					return method.Outputs.Pack(big.NewInt(n), new(big.Int), root)
				}
				t.Fatal("unexpected read")
				return nil, nil
			})
			chain := reader(func(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
				aggregates++
				method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				args, err := method.Inputs.Unpack(msg.Data[4:])
				if err != nil {
					return nil, err
				}
				calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
				if len(calls) != 6 {
					t.Fatalf("shared reward reads = %d, want 6", len(calls))
				}
				result := make([]multicall.Multicall3Result, len(calls))
				for i, call := range calls {
					data, err := underlying(ctx, ethereum.CallMsg{To: &call.Target, Data: call.CallData}, block)
					if err != nil {
						return nil, err
					}
					result[i] = multicall.Multicall3Result{Success: true, ReturnData: data}
				}
				return method.Outputs.Pack(result)
			})
			p := &Automatic{deps: provider.Deps{HTTP: client, Chain: chain}}
			out, err := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, nil)
			if aggregates != 1 || getterCounts["getMerkleRoot"] != 1 || getterCounts["claimRecipient"] != 3 || getterCounts["claimed"] != 2 {
				t.Fatalf("reward getters were repeated: aggregates=%d getters=%v", aggregates, getterCounts)
			}

			if test == "all claimed" || test == "bad proof" {
				if err == nil {
					t.Fatal("invalid or idle batch accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if fetches != 1 || len(out) == 0 || len(out[0].Inputs) != 0 || len(out[0].Actions) != 1 {
				t.Fatalf("wrong preparation: %+v HTTP %d", out, fetches)
			}
			want := 2
			if test == "first claimed" {
				want = 1
			}
			if len(out) != want {
				t.Fatal("wrong claim count")
			}
			variants := out
			method := binding.NewMerklDistributor().GetABI().Methods["claim"]
			for i, v := range variants {
				args, err := method.Inputs.Unpack(v.Actions["0:0"][4:])
				if err != nil {
					t.Fatal(err)
				}
				index := i
				if test == "first claimed" {
					index = 1
				}
				if args[1].([]common.Address)[0] != tokens[index] || len(args[3].([][][32]byte)[0]) != 1 {
					t.Fatal("incorrect reward or dynamic proof length")
				}
			}
			for _, execution := range out {
				if execution.Task != nil {
					t.Fatal("synchronous claim retained settlement work")
				}
			}
		})
	}
}
