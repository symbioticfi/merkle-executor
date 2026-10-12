package cowswap

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	hostconfig "github.com/symbioticfi/merkle-executor/internal/config"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
)

func TestInternalHTTPAliasCannotMoveDurableCoWOrder(t *testing.T) {
	hits := [2]int{}
	var identity prepared
	servers := make([]*httptest.Server, 2)
	for i := range servers {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits[i]++
			switch r.URL.Path {
			case "/api/v1/quote":
				q := sampleOrder()
				q.BuyAmount = "2000"
				q.FeeAmount = "10"
				_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 1})
			case "/api/v1/orders":
				_ = json.NewEncoder(w).Encode(identity.UID)
			default:
				http.NotFound(w, r)
			}
		}))
		defer servers[i].Close()
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	var oldFingerprint string
	var durable []byte
	for i := range servers {
		anchor0, anchor1 := "&route ", ""
		if i == 1 {
			anchor0, anchor1 = "", "&route "
		}
		raw := "chain: {chainId: 1, rpcUrl: 'http://localhost'}\nsigner: {address: '" + addr(3).Hex() + "'}\nstrategies:\n  - name: cow\n    account: '" + addr(3).Hex() + "'\n    executor: '" + addr(5).Hex() + "'\n    tree: tree.json\n    providers:\n      - name: cowswap\n        config:\n          http:\n            <<: [{url: " + anchor0 + "'" + servers[0].URL + "'}, {url: " + anchor1 + "'" + servers[1].URL + "'}]\n            url: *route\n          settlement: '" + addr(4).Hex() + "'\n          sellToken: '" + addr(1).Hex() + "'\n          buyToken: '" + addr(2).Hex() + "'\n          sellAmount: '1000'\n          minBuyAmount: '1800'\n          slippageBps: 100\n          validitySeconds: 600\n          appData: '" + sampleOrder().AppData + "'\n          action: '1:0'\n"
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := hostconfig.Load(path)
		if err != nil {
			if !strings.Contains(err.Error(), "aliases") {
				t.Fatal(err)
			}
			continue
		}
		p, err := Factory(cfg.Strategies[0].Providers[0].Config, registry.Deps{Chain: &negativeChain{}, HTTP: servers[i].Client()})
		if err != nil {
			t.Fatal(err)
		}
		snap := negativeSnapshot()
		out, err := p.Prepare(context.Background(), snap)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			oldFingerprint = cfg.Strategies[0].Fingerprint()
			durable = bytes.Clone(out.Data)
		} else if oldFingerprint != cfg.Strategies[0].Fingerprint() || !bytes.Equal(durable, out.Data) {
			t.Fatal("fixture did not preserve config fingerprint and durable order identity")
		}
		if err = json.Unmarshal(out.Data, &identity); err != nil {
			t.Fatal(err)
		}
		snap.BlockNumber = big.NewInt(13)
		snap.BlockHash = testHeader(snap.BlockNumber).Hash()
		result, err := p.Advance(context.Background(), snap, out, nil)
		if err != nil || result.Reference != identity.UID {
			t.Fatalf("advance: %+v %v", result, err)
		}
	}
	if hits[0] != 0 || hits[1] != 0 {
		t.Fatalf("anchor relocation moved an identical durable order between endpoints: hits=%v fingerprint=%s uid=%s", hits, oldFingerprint, identity.UID)
	}
}
