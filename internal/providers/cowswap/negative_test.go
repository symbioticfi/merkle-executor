package cowswap

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	sb "github.com/symbioticfi/merkle-executor/api/bindings/cowsettlement"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	ab "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type negativeChain struct{ reorg atomic.Bool }

func (c *negativeChain) HeaderByNumber(_ context.Context, b *big.Int) (*types.Header, error) {
	h := testHeader(b)
	if c.reorg.Load() {
		h.Extra = []byte("reorg")
	}
	return h, nil
}
func (*negativeChain) CallContract(_ context.Context, m ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if *m.To == addr(4) {
		a := sb.NewCowSettlement().GetABI()
		method, err := a.MethodById(m.Data[:4])
		if err != nil {
			return nil, err
		}
		if method.Name == "domainSeparator" {
			return method.Outputs.Pack(protocolio.Domain("Gnosis Protocol", "v2", big.NewInt(1), addr(4)))
		}
		return method.Outputs.Pack(new(big.Int))
	}
	return ab.NewMerkleAccount().GetABI().Methods["signatures"].Outputs.Pack(true)
}
func negativeConfig(url string) Config {
	return Config{HTTP: protocolio.HTTPConfig{URL: url}, Settlement: addr(4).Hex(), SellToken: addr(1).Hex(), BuyToken: addr(2).Hex(), SellAmount: "1000", MinBuyAmount: "1800", ValiditySeconds: 600, Action: "1:0"}
}
func negativeSnapshot() provider.Snapshot {
	return provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000, BlockHash: testHeader(big.NewInt(12)).Hash()}
}

func TestPrepareRejectsIncompatibleQuotesWithoutOrderPost(t *testing.T) {
	cases := []struct {
		name   string
		change func(*wire.CowOrder)
	}{
		{"kind", func(o *wire.CowOrder) { o.Kind = "buy" }},
		{"partial fill", func(o *wire.CowOrder) { o.PartiallyFillable = true }},
		{"expiry", func(o *wire.CowOrder) { o.ValidTo-- }},
		{"sell budget", func(o *wire.CowOrder) { o.SellAmount = "1001" }},
		{"sell plus fee budget", func(o *wire.CowOrder) { o.FeeAmount = "11" }},
		{"sell token", func(o *wire.CowOrder) { o.SellToken = addr(9).Hex() }},
		{"buy token", func(o *wire.CowOrder) { o.BuyToken = addr(9).Hex() }},
		{"receiver", func(o *wire.CowOrder) { o.Receiver = addr(9).Hex() }},
		{"app data", func(o *wire.CowOrder) { o.AppData = common.HexToHash("0x1").Hex() }},
		{"sell balance", func(o *wire.CowOrder) { o.SellTokenBalance = "internal" }},
		{"buy balance", func(o *wire.CowOrder) { o.BuyTokenBalance = "internal" }},
		{"slippage takes below minimum", func(o *wire.CowOrder) { o.BuyAmount = "1817" }},
		{"minimum", func(o *wire.CowOrder) { o.BuyAmount = "1700" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/quote" {
					posts++
					w.WriteHeader(500)
					return
				}
				q := sampleQuote()
				q.FeeAmount = "10"
				q.BuyAmount = "2000"
				tc.change(&q)
				_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 1, Verified: true})
			}))
			defer srv.Close()
			p, err := fixtureProvider(negativeConfig(srv.URL), providerRegistry.Deps{Chain: &negativeChain{}, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.Prepare(context.Background(), negativeSnapshot())
			if err == nil || len(out.Actions) != 0 || posts != 0 {
				t.Fatalf("incompatible quote authorized: %v posts=%d", err, posts)
			}
		})
	}
}

func TestAdvanceRejectsIdentityAndUncertainOrderbookResponses(t *testing.T) {
	for _, mode := range []string{"owner", "uid", "sell", "buy", "expiry", "unknown", "cancelled", "post rejected", "post ambiguous", "post uid", "lookup error", "reorg during lookup", "reorg during quote"} {
		t.Run(mode, func(t *testing.T) {
			chain := &negativeChain{}
			posts, gets := 0, 0
			var identity prepared
			var sent []wire.CowOrder
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/quote":
					q := sampleQuote()
					q.FeeAmount = "10"
					q.BuyAmount = "2000"
					if mode == "reorg during quote" {
						chain.reorg.Store(true)
					}
					_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 1, Verified: true})
				case "/api/v1/orders":
					posts++
					var order wire.CowOrder
					if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
						t.Error(err)
					}
					sent = append(sent, order)
					switch mode {
					case "post rejected":
						w.WriteHeader(400)
					case "post ambiguous":
						w.WriteHeader(502)
					default:
						_ = json.NewEncoder(w).Encode("different-uid")
					}
				default:
					gets++
					if r.URL.Path != "/api/v1/orders/"+identity.UID {
						t.Error("lookup identity changed")
					}
					if strings.HasPrefix(mode, "post ") {
						http.NotFound(w, r)
						return
					}
					if mode == "lookup error" {
						w.WriteHeader(502)
						return
					}
					if mode == "reorg during lookup" {
						chain.reorg.Store(true)
						http.NotFound(w, r)
						return
					}
					status := wire.CowOrderStatus{UID: identity.UID, Owner: addr(3).Hex(), Status: "open", SellAmount: "1000", BuyAmount: "1980", ValidTo: 1600}
					switch mode {
					case "owner":
						status.Owner = addr(9).Hex()
					case "uid":
						status.UID = "wrong"
					case "sell":
						status.SellAmount = "989"
					case "buy":
						status.BuyAmount = "1981"
					case "expiry":
						status.ValidTo--
					case "unknown":
						status.Status = "mystery"
					case "cancelled":
						status.Status = "cancelled"
					}
					_ = json.NewEncoder(w).Encode(status)
				}
			}))
			defer srv.Close()
			p, err := fixtureProvider(negativeConfig(srv.URL), providerRegistry.Deps{Chain: chain, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			snap := negativeSnapshot()
			out, err := p.Prepare(context.Background(), snap)
			if mode == "reorg during quote" {
				if err == nil || posts != 0 {
					t.Fatal("reorganized quote accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(out.Data, &identity); err != nil {
				t.Fatal(err)
			}
			saved := bytes.Clone(out.Data)
			snap.BlockNumber = big.NewInt(13)
			snap.BlockHash = testHeader(snap.BlockNumber).Hash()
			for range 2 { // recreate provider: retries retain the same authorized UID/order.
				p, err = fixtureProvider(negativeConfig(srv.URL), providerRegistry.Deps{Chain: chain, HTTP: srv.Client()})
				if err != nil {
					t.Fatal(err)
				}
				progress, err := p.Advance(context.Background(), snap, out, nil)
				if progress.Done || (err == nil) != (mode == "post ambiguous") {
					t.Fatalf("unsafe advancement: %+v %v", progress, err)
				}
				if mode != "reorg during lookup" && progress.Reference != identity.UID {
					t.Fatal("durable identity lost")
				}
			}
			if !bytes.Equal(saved, out.Data) {
				t.Fatal("persisted output mutated")
			}
			if strings.HasPrefix(mode, "post ") {
				if posts != 2 || gets != 2 {
					t.Fatalf("must lookup before each identical retry: gets=%d posts=%d", gets, posts)
				}
				for _, o := range sent {
					o.AppData = o.AppDataHash
					raw, _ := EncodeOrder(o)
					expected, _ := EncodeOrder(identity.Order)
					if !bytes.Equal(raw, expected) {
						t.Fatal("retry changed order")
					}
				}
			} else if posts != 0 {
				t.Fatal("rejected lookup posted an order")
			}
		})
	}
}
