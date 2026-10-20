package config

import (
	"github.com/ethereum/go-ethereum/common"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInternalHTTPAliasCannotMoveDurableCoWOrder(t *testing.T) {
	hits := [2]int{}
	servers := make([]*httptest.Server, 2)
	for i := range servers {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits[i]++
			w.WriteHeader(http.StatusNoContent)
		}))
		defer servers[i].Close()
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	for i := range servers {
		anchor0, anchor1 := "&route ", ""
		if i == 1 {
			anchor0, anchor1 = "", "&route "
		}
		raw := "curator: {url: 'http://localhost/api/v1'}\nchain: {chainId: 1, rpcUrl: 'http://localhost'}\nsigner: {address: '" + common.HexToAddress("0x3").Hex() + "'}\nstrategies:\n  - name: cow\n    executor: '" + common.HexToAddress("0x5").Hex() + "'\n    providers:\n      - name: cowswap\n        config:\n          http:\n            <<: [{url: " + anchor0 + "'" + servers[0].URL + "'}, {url: " + anchor1 + "'" + servers[1].URL + "'}]\n            url: *route\n          settlement: '" + common.HexToAddress("0x4").Hex() + "'\n          sellToken: '" + common.HexToAddress("0x1").Hex() + "'\n          buyToken: '" + common.HexToAddress("0x2").Hex() + "'\n          sellAmount: '1000'\n          minBuyAmount: '1800'\n          slippageBps: 100\n          validitySeconds: 600\n          appData: '" + common.Hash{}.Hex() + "'\n          action: '1:0'\n"
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("obsolete aliased provider reached the address-only daemon")
		}
	}
	if hits[0] != 0 || hits[1] != 0 {
		t.Fatalf("obsolete aliased provider reached HTTP endpoints: hits=%v", hits)
	}
}
