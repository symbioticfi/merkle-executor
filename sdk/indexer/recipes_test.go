package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestIndexedRecipeRequestIsBoundToOriginalBodyAndProvenance(t *testing.T) {
	executor, hash := common.HexToAddress("0x1234"), common.HexToHash("0x5678")
	for _, mode := range []string{"valid", "missing", "malformed", "height"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/recipes/"+executor.Hex() || r.URL.Query().Get("hash") != hash.Hex() || r.URL.Query().Get("block") != "12" {
					t.Error("unbound indexed recipe query")
				}
				if mode == "missing" {
					w.WriteHeader(404)
					return
				}
				if mode == "malformed" {
					_, _ = w.Write([]byte(`{"executor":"0x1"}`))
					return
				}
				n := "12"
				if mode == "height" {
					n = "012"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"executor": executor, "recipeHash": hash, "blockNumber": n, "blockHash": common.HexToHash("0xabcd"), "transactionHash": common.HexToHash("0xefab"), "logIndex": 3, "queries": []hexutil.Bytes{{1, 2}}, "steps": []hexutil.Bytes{{3, 4}}})
			}))
			defer srv.Close()
			c, err := New(srv.URL, srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			p, err := c.RecipePublication(context.Background(), executor, hash, 12)
			if mode != "valid" {
				if err == nil {
					t.Fatal("invalid indexed publication accepted")
				}
				return
			}
			if err != nil || p.Executor != executor || p.RecipeHash != hash || p.BlockNumber != 12 || p.LogIndex != 3 || hexutil.Encode(p.Recipe.Queries[0]) != "0x0102" || hexutil.Encode(p.Recipe.Steps[0]) != "0x0304" {
				t.Fatalf("publication %+v: %v", p, err)
			}
		})
	}
}

func TestIndexerURLRequiresCredentialFreeHTTP(t *testing.T) {
	for _, url := range []string{"", "ftp://host", "http://user:password@host", "http://host?token=1", "http://host#token"} {
		if _, err := New(url, nil); err == nil {
			t.Fatal("unsafe indexed URL accepted")
		}
	}
}
