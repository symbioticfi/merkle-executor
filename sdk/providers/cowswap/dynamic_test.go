package cowswap

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func decimal(t *testing.T, raw string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		t.Fatal("invalid test integer")
	}
	return v
}

func TestOfficialSDKAutoSlippageGolden(t *testing.T) {
	for _, tc := range []struct {
		name, sell, fee string
		volume, want    uint64
	}{
		{"upstream fallback", "98115217044683860", "1884782955316140", 0, 146},
		{"binary64 56.5 below tie", "1000000", "10300", 5, 56},
		{"binary64 1.5 below tie", "1000000", "100", 1, 1},
		{"zero suggestion defaults", "1", "0", 1, 50},
		{"cap at 100 percent", "100", "100000", 0, 10000},
		{"fallback volume", "1000", "0", 0, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := autoSlippageBPS(decimal(t, tc.sell), decimal(t, tc.fee), tc.volume); got != tc.want {
				t.Fatalf("AUTO = %d, want upstream %d", got, tc.want)
			}
		})
	}
	if got := applyPercentage(big.NewInt(100000000), float64(201)/100); got.String() != "2009999" {
		t.Fatalf("JavaScript coefficient rounding changed: %s", got)
	}
	if got := buyAfterSlippage(big.NewInt(101), 50); got.Int64() != 101 {
		t.Fatalf("SDK subtract-floor rounding changed: %s", got)
	}
}

func TestOfficialChainDefaults(t *testing.T) {
	for _, tc := range []struct {
		id   int64
		slug string
	}{{1, "mainnet"}, {56, "bnb"}, {100, "xdai"}, {137, "polygon"}, {8453, "base"}, {9745, "plasma"}, {42161, "arbitrum_one"}, {43114, "avalanche"}, {57073, "ink"}, {59144, "linea"}, {11155111, "sepolia"}} {
		httpConfig, settlement, relayer, err := ChainDefaults(big.NewInt(tc.id))
		if err != nil || httpConfig.URL != "https://api.cow.fi/"+tc.slug || settlement.Hex() != common.HexToAddress(settlementAddress).Hex() || relayer.Hex() != common.HexToAddress(relayerAddress).Hex() {
			t.Fatalf("chain %d: %v %v %v %v", tc.id, httpConfig, settlement, relayer, err)
		}
	}
	for _, id := range []*big.Int{nil, big.NewInt(-1), big.NewInt(0), big.NewInt(31337), new(big.Int).Lsh(big.NewInt(1), 65)} {
		if _, _, _, err := ChainDefaults(id); err == nil {
			t.Fatal("unknown chain fell back to mainnet")
		}
	}
}

func TestBFFConsumerSchema(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       uint64
		err        bool
	}{
		{"suggestion", `{"slippageBps":201}`, 200, 201, false},
		{"zero", `{"slippageBps":0}`, 200, 0, false},
		{"missing", `{}`, 200, 0, false},
		{"negative", `{"slippageBps":-1}`, 200, 0, true},
		{"too large", `{"slippageBps":10001}`, 200, 0, true},
		{"fraction", `{"slippageBps":0.1}`, 200, 0, true},
		{"malformed", `invalid`, 200, 0, true},
		{"unavailable", `{}`, 503, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/1/markets/"+addr(1).Hex()+"-"+addr(2).Hex()+"/slippageTolerance" || r.URL.RawQuery != "" {
					t.Error("BFF request differs from official route")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			client, err := protocolio.NewHTTP(protocolio.HTTPConfig{URL: srv.URL, TimeoutSeconds: 2}, srv.Client(), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := volumeSuggestion(client)(context.Background(), big.NewInt(1), addr(1), addr(2))
			if got != tc.want || (err != nil) != tc.err {
				t.Fatalf("suggestion %d %v", got, err)
			}
		})
	}
}

func TestAutoPrepareVerifiedAndCancellation(t *testing.T) {
	for _, mode := range []string{"verified", "unverified", "fallback", "timeout", "parent cancelled", "gross mismatch"} {
		t.Run(mode, func(t *testing.T) {
			quotes, suggestions := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				quotes++
				if r.URL.Path != "/api/v1/quote" {
					t.Error("Prepare performed external order mutation")
				}
				var request wire.CowQuoteRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				full, hash := appData(defaultSlippageBPS)
				if request.PriceQuality != "verified" || request.AppData != full || request.AppDataHash != hash.Hex() || request.SellTokenBalance != "erc20" || request.BuyTokenBalance != "erc20" {
					t.Error("quote omitted official verified appData policy")
				}
				q := sampleQuote()
				q.BuyAmount, q.FeeAmount = "2000", "10"
				if mode == "gross mismatch" {
					q.FeeAmount = "9"
				}
				_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 42, Verified: mode != "unverified"})
			}))
			defer srv.Close()
			cfg := negativeConfig(srv.URL)
			p, err := New(cfg, provider.Deps{Chain: &negativeChain{}, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p.volumeBPS = func(c context.Context, _ *big.Int, _, _ common.Address) (uint64, error) {
				suggestions++
				deadline, ok := c.Deadline()
				if !ok || time.Until(deadline) > 2*time.Second {
					t.Error("volatility request lacks independent 2s limit")
				}
				switch mode {
				case "fallback":
					return 0, errors.New("fixture service failure")
				case "timeout":
					<-c.Done()
					return 0, c.Err()
				case "parent cancelled":
					cancel()
					return 0, c.Err()
				default:
					return 5, nil
				}
			}
			out, err := p.Prepare(ctx, negativeSnapshot())
			if quotes != 1 {
				t.Fatalf("quote count %d", quotes)
			}
			if mode == "unverified" || mode == "gross mismatch" || mode == "parent cancelled" {
				if err == nil || len(out.Data) != 0 || mode != "parent cancelled" && suggestions != 0 {
					t.Fatalf("invalid quote/cancellation accepted: %v %d", err, suggestions)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var d prepared
			if err := json.Unmarshal(out.Data, &d); err != nil {
				t.Fatal(err)
			}
			wantBPS := uint64(51)
			if mode == "fallback" || mode == "timeout" {
				wantBPS = 101
			}
			full, hash := appData(wantBPS)
			if d.Version != 1 || d.Order.SellAmount != "1000" || d.Order.FeeAmount != "0" || d.SlippageBPS != wantBPS || d.Order.QuoteID != 42 || d.FullAppData != full || d.Order.AppData != hash.Hex() || crypto.Keccak256Hash([]byte(d.FullAppData)) != hash {
				t.Fatalf("persisted AUTO order invalid: %+v", d)
			}
		})
	}
}
