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
	"strconv"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestCurrentFrontendVariableProof(t *testing.T) {
	for _, length := range []int{0, 1, 20, 64, 65, -1, -2} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			raw, err := os.ReadFile("testdata/frontend-current.json")
			if err != nil {
				t.Fatal(err)
			}
			tree, err := recipes.DecodeTree(raw)
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
			recipe, err := recipes.DecodeRecipe(fixture.Recipe)
			if err != nil {
				t.Fatal(err)
			}
			locked := length < 0
			lockToken := length == -2
			if locked {
				length = 20
				actions, err := recipes.ManagedActions(recipe, tree)
				if err != nil {
					t.Fatal(err)
				}
				action := actions[0]
				layout, err := claimLayoutOf(action, fixture.Account)
				if err != nil {
					t.Fatal(err)
				}
				at := layout.lengths[0]
				if lockToken {
					at = layout.tokenWords[0]
				}
				clear(action.Leaf.DataMask[at : at+32])
				old := action.Leaf.Hash
				action.Leaf.Hash, err = recipes.HashLeaf(action.Leaf)
				if err != nil {
					t.Fatal(err)
				}
				action.Leaf.Proof = nil
				tree = &recipes.Tree{Root: action.Leaf.Hash, Leaves: []recipes.Leaf{action.Leaf}}
				for i, step := range recipe.Steps {
					recipe.Steps[i] = bytes.ReplaceAll(step, old[:], action.Leaf.Hash[:])
				}
			}
			token := common.HexToAddress("0x201")
			root := crypto.Keccak256Hash(protocolio.AddressWord(fixture.Account), protocolio.AddressWord(token), protocolio.Word(big.NewInt(100)))
			proof := make([]string, length)
			for i := range proof {
				sibling := crypto.Keccak256Hash([]byte{byte(i)})
				proof[i] = sibling.Hex()
				a, b := root, sibling
				if bytes.Compare(a[:], b[:]) > 0 {
					a, b = b, a
				}
				root = crypto.Keccak256Hash(a[:], b[:])
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: []wire.MerklReward{{Token: wire.MerklToken{Address: token.Hex(), Decimals: 18}, Amount: "100", Claimed: "0", Pending: "0", Proofs: proof}}}})
			}))
			defer server.Close()
			endpoint, _ := url.Parse(server.URL)
			h := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r = r.Clone(r.Context())
				r.URL.Host = endpoint.Host
				r.URL.Scheme = endpoint.Scheme
				return http.DefaultTransport.RoundTrip(r)
			})}
			reads := 0
			chain := reader(func(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
				reads++
				if block.Int64() != 12 {
					t.Fatal("unpinned")
				}
				aggregate := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				args, e := aggregate.Inputs.Unpack(msg.Data[4:])
				if e != nil {
					return nil, e
				}
				calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
				out := make([]multicall.Multicall3Result, len(calls))
				for i, c := range calls {
					contractABI := binding.NewMerklDistributor().GetABI()
					method, e := contractABI.MethodById(c.CallData[:4])
					if e != nil {
						return nil, e
					}
					var data []byte
					switch method.Name {
					case "getMerkleRoot":
						data, e = method.Outputs.Pack(root)
					case "claimed":
						data, e = method.Outputs.Pack(big.NewInt(30), big.NewInt(0), root)
					case "claimRecipient":
						data, e = method.Outputs.Pack(common.Address{})
					}
					if e != nil {
						return nil, e
					}
					out[i] = multicall.Multicall3Result{Success: true, ReturnData: data}
				}
				return aggregate.Outputs.Pack(out)
			})
			p := &Automatic{deps: provider.Deps{Chain: chain, HTTP: h}}
			snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: fixture.Account, Timestamp: 1000}
			outputs, err := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, nil)
			var out provider.Output
			if len(outputs) == 1 {
				out = outputs[0]
			}
			if length > 64 || locked {
				if err == nil {
					t.Fatal("oversized proof or pinned word mutation accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actions, err := recipes.ManagedActions(recipe, tree)
			if err != nil {
				t.Fatal(err)
			}
			if len(actions) != 1 || len(out.Actions) != 1 || reads != 1 {
				t.Fatalf("unexpected actions/reads: %d/%d/%d", len(actions), len(out.Actions), reads)
			}
			data := out.Actions[actions[0].Key]
			if len(data) != len(actions[0].Leaf.Data) {
				t.Fatal("permission calldata size changed")
			}
			args, err := binding.NewMerklDistributor().GetABI().Methods["claim"].Inputs.Unpack(data[4:])
			if err != nil {
				t.Fatal(err)
			}
			if args[1].([]common.Address)[0] != token || args[2].([]*big.Int)[0].Int64() != 100 || len(args[3].([][][32]byte)[0]) != length {
				t.Fatalf("wrong claim inputs: %v", args)
			}
			if out.Inputs[0] != token.Big().String() || out.Inputs[1] != "100" {
				t.Fatalf("recipe inputs not filled: %v", out.Inputs)
			}
			if out.Task != nil {
				t.Fatal("synchronous claim retained settlement work")
			}
		})
	}
}
