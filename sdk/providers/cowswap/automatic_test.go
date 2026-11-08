package cowswap

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	settlement "github.com/symbioticfi/merkle-executor/sdk/bindings/cowsettlement"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type boundedCowFixture struct {
	Account, Token, BuyToken, Reads, Settlement, Spender common.Address
	Recipe                                               hexutil.Bytes
}

func boundedCow(t *testing.T) (boundedCowFixture, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	raw, err := os.ReadFile("testdata/frontend-bounded-cow.json")
	if err != nil {
		t.Fatal(err)
	}
	var f boundedCowFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	r, err := recipes.DecodeRecipe(f.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	return f, r, tree
}

type cowFixtureTransport func(*http.Request) (*http.Response, error)

func (f cowFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func boundedHTTP(t *testing.T, posts *int, saved *prepared, pinned *common.Hash) *http.Client {
	t.Helper()
	return &http.Client{Transport: cowFixtureTransport(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		switch r.URL.Path {
		case "/mainnet/api/v1/quote":
			var request wire.CowQuoteRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			expected, hash := appData(defaultSlippageBPS)
			expectedHash := hash.Hex()
			if pinned != nil {
				expected, expectedHash = pinned.Hex(), ""
			}
			if request.AppData != expected || request.AppDataHash != expectedHash {
				t.Errorf("pinned appData changed: %+v", request)
			}
			quotedAppData := request.AppDataHash
			if pinned != nil {
				quotedAppData = pinned.Hex()
			}
			q := wire.CowOrder{SellToken: request.SellToken, BuyToken: request.BuyToken, Receiver: request.Receiver, SellAmount: "990", BuyAmount: "2000", FeeAmount: "10", ValidTo: request.ValidTo, AppData: quotedAppData, Kind: "sell", SellTokenBalance: "erc20", BuyTokenBalance: "erc20"}
			_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 1, Verified: true})
		case "/mainnet/api/v1/orders":
			*posts++
			var order wire.CowOrder
			if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
				t.Fatal(err)
			}
			expected, expectedHash := saved.FullAppData, saved.Order.AppData
			if pinned != nil {
				expected, expectedHash = pinned.Hex(), ""
			}
			if order.AppData != expected || order.AppDataHash != expectedHash || order.ValidTo != saved.Order.ValidTo || order.QuoteID != saved.Order.QuoteID {
				t.Errorf("POST changed saved identity: %+v", order)
			}
			_ = json.NewEncoder(w).Encode(saved.UID)
		default:
			if r.URL.Host == "bff.cow.fi" {
				_, _ = w.Write([]byte(`{"slippageBps":0}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
		return w.Result(), nil
	})}
}

type boundedCowReader struct {
	negativeChain
	settlement common.Address
}

func (r *boundedCowReader) CallContract(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	if *m.To == r.settlement {
		a := settlement.NewCowSettlement().GetABI()
		method, err := a.MethodById(m.Data[:4])
		if err != nil {
			return nil, err
		}
		if method.Name == "domainSeparator" {
			return method.Outputs.Pack(protocolDomain(big.NewInt(1), r.settlement))
		}
		return method.Outputs.Pack(new(big.Int))
	}
	return r.negativeChain.CallContract(ctx, m, b)
}
func TestAutomaticPreparesOriginalBoundedCowRecipe(t *testing.T) {
	zero, nonzero := common.Hash{}, common.HexToHash("0x1234")
	for _, tc := range []struct {
		name   string
		pinned *common.Hash
	}{{"pinned zero", &zero}, {"pinned nonzero", &nonzero}, {"mutable AUTO metadata", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			f, r, tree := boundedCow(t)
			old := tree.Leaves[0].Hash
			if tc.pinned != nil {
				copy(tree.Leaves[0].Data[356:388], tc.pinned[:])
			} else {
				for i := 356; i < 388; i++ {
					tree.Leaves[0].DataMask[i] = 255
				}
			}
			var err error
			tree.Leaves[0].Hash, err = recipes.HashLeaf(tree.Leaves[0])
			if err != nil {
				t.Fatal(err)
			}
			a, b := tree.Leaves[0].Hash, tree.Leaves[1].Hash
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			tree.Root = crypto.Keccak256Hash(a[:], b[:])
			tree.Leaves[0].Proof = []common.Hash{tree.Leaves[1].Hash}
			tree.Leaves[1].Proof = []common.Hash{tree.Leaves[0].Hash}
			for i, step := range r.Steps {
				r.Steps[i] = bytes.ReplaceAll(step, old[:], tree.Leaves[0].Hash[:])
			}
			chain := &boundedCowReader{settlement: f.Settlement}
			s := negativeSnapshot()
			s.Account = f.Account
			posts := 0
			var saved prepared
			deps := provider.Deps{Chain: chain, HTTP: boundedHTTP(t, &posts, &saved, tc.pinned)}
			p := &Automatic{deps: deps}
			req, err := p.RecipeRequirements(s, r, tree)
			if err != nil {
				t.Fatal(err)
			}
			if err = recipes.ValidatePrefix(r, req.Values, recipes.PrefixRuntime{Tree: tree}); err != nil {
				t.Fatal("bounded prefix requires unavailable provider inputs", err)
			}
			values := map[uint]*big.Int{0: big.NewInt(1000), 1: big.NewInt(1000), 4: big.NewInt(2), 5: big.NewInt(2000), 6: big.NewInt(20), 7: big.NewInt(1980), 9: big.NewInt(1000)}
			original, _ := json.Marshal(tree)
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, values)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(tree)
			if !bytes.Equal(original, after) {
				t.Fatal("automatic provider mutated committed permissions")
			}
			if err = json.Unmarshal(out[0].Data, &saved); err != nil {
				t.Fatal(err)
			}
			if tc.pinned != nil {
				if saved.Order.AppData != tc.pinned.Hex() || saved.FullAppData != "" {
					t.Fatal("pinned appData was replaced", saved)
				}
			} else if saved.FullAppData == "" || crypto.Keccak256Hash([]byte(saved.FullAppData)).Hex() != saved.Order.AppData {
				t.Fatal("mutable appData lost AUTO document/hash", saved)
			}
			if saved.Order.ValidTo != 1300 || out[0].Inputs[0] != "1000" || out[0].Inputs[1] != "1980" || out[0].Inputs[2] != "1300" {
				t.Fatalf("bounded identity/economics changed: %+v %+v", saved, out[0].Inputs)
			}
			if posts != 0 {
				t.Fatal("preparation published an order")
			}
			if _, err = out[0].Task.Advance(t.Context(), s); err == nil || posts != 0 {
				t.Fatal("order posted before confirmed Sign")
			}
			cfg, err := p.derive(s, r, tree)
			if err != nil {
				t.Fatal(err)
			}
			cfg.cfg.SellAmount, cfg.cfg.MinBuyAmount = "1000", "1980"
			runtime, err := New(cfg.cfg, deps)
			if err != nil {
				t.Fatal(err)
			}
			s.BlockNumber = big.NewInt(13)
			s.BlockHash = testHeader(s.BlockNumber).Hash()
			if _, err = runtime.advance(t.Context(), s, saved); err != nil || posts != 1 {
				t.Fatal("hash-only order was not posted with retained identity", err, posts)
			}
		})
	}
}

func TestAutomaticExpiryUsesCommittedClock(t *testing.T) {
	for _, tc := range []struct {
		name        string
		clock, want int64
	}{
		{"snapshot clock", 1000, 1300},
		{"earlier clock tightens expiry", 900, 1200},
		{"elapsed lifetime stays idle", 700, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, r, tree := boundedCow(t)
			s := negativeSnapshot()
			s.Account = f.Account
			posts := 0
			var saved prepared
			p := &Automatic{deps: provider.Deps{Chain: &boundedCowReader{settlement: f.Settlement}, HTTP: boundedHTTP(t, &posts, &saved, new(common.Hash))}}
			values := map[uint]*big.Int{0: big.NewInt(1000), 1: big.NewInt(1000), 4: big.NewInt(2), 5: big.NewInt(2000), 6: big.NewInt(20), 7: big.NewInt(1980), 9: big.NewInt(tc.clock)}
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, values)
			if tc.want == 0 {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatal("expired committed lifetime was not idle", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(out[0].Data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Order.ValidTo != tc.want {
				t.Fatalf("expiry=%d want=%d", saved.Order.ValidTo, tc.want)
			}
		})
	}
}
