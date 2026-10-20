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
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
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
	p := &Automatic{deps: registry.Deps{Chain: reader(nil)}}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Actions) != 2 || len(req.Inputs) != 4 {
		t.Fatalf("guarded claims not fully owned: %+v", req)
	}
}

func TestAutomaticRejectsMutableFrontendGuard(t *testing.T) {
	s, r, tree := frontendClaims(t)
	// Recommit the weakened mask so authentication succeeds and the provider's
	// own identity/framing check is what rejects this permission.
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
	p := &Automatic{deps: registry.Deps{Chain: reader(nil)}}
	_, err = p.RecipeRequirements(s, r, tree)
	if err == nil || !strings.Contains(err.Error(), "identity, guards and proof layout must be pinned") {
		t.Fatalf("mutable guard accepted: %v", err)
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
			p := &Automatic{deps: registry.Deps{Chain: chain, HTTP: httpClient}}
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %q, got %v", tc.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Actions) != 2 || fetches != 1 || reads != 2 {
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
			progress, err := p.Advance(t.Context(), s, out, nil)
			if err != nil || !progress.Done {
				t.Fatalf("persisted batch invalid: %+v %v", progress, err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAutomaticRejectsChangedCanonicalClaimGuard(t *testing.T) {
	s, r, tree := frontendClaims(t)
	// Even fully pinned alternative instructions cannot impersonate the supported
	// root/recipient/ledger/cash guard wire format.
	raw, _ := pairArguments.Pack(new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(18)), new(big.Int).Lsh(big.NewInt(1), 216))
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
	if err == nil || !strings.Contains(err.Error(), "atomic claim guard instructions") {
		t.Fatalf("changed guard accepted: %v", err)
	}
}

func TestClaimRecipientOverridePinsCustody(t *testing.T) {
	for _, tc := range []struct {
		name      string
		recipient common.Address
		data      []byte
		want      string
	}{
		{name: "account", recipient: addr(3)},
		{name: "other recipient", recipient: addr(9), want: "override must pin account"},
		{name: "callback", recipient: addr(3), data: []byte{1}, want: "empty callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := binding.NewMerklDistributor().PackClaimWithRecipient([]common.Address{addr(3)}, []common.Address{addr(1)}, []*big.Int{new(big.Int)}, [][][32]byte{{}}, []common.Address{tc.recipient}, [][]byte{tc.data})
			action := recipes.ManagedAction{Leaf: recipes.Leaf{Target: addr(4), Data: raw, DataMask: make([]byte, len(raw)), Value: new(big.Int)}}
			layout, err := claimLayoutOf(action, addr(3))
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %s,got %v", tc.want, err)
				}
				return
			}
			if err != nil || layout == nil || !layout.direct {
				t.Fatalf("self recipient rejected: %+v %v", layout, err)
			}
		})
	}
}
