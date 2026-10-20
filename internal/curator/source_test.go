package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestSourceFetchesExactActiveRootWithBoundedFailures(t *testing.T) {
	account, adapter := common.HexToAddress("0x1"), common.HexToAddress("0x99")
	leaf := recipes.Leaf{Target: common.HexToAddress("0x2"), Value: new(big.Int), Data: []byte{1, 2, 3, 4}, DataMask: make([]byte, 4)}
	var err error
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	root := leaf.Hash
	metadata, err := json.Marshal(map[string]any{"chainId": 31337, "root": root.Hex(), "account": account.Hex(), "adapter": adapter.Hex(), "calls": []any{map[string]any{"hash": root.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}}}})
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"bundle":{"chainId":31337,"root":%q,"account":%q,"adapter":%q,"leaves":[%q],"metadata":%s},"bundleHash":%q,"deadline":"1800000000","signer":%q,"signature":"0x","createdAt":"2026-10-20T00:00:00Z"}`, root.Hex(), account.Hex(), adapter.Hex(), root.Hex(), metadata, common.Hash{}.Hex(), account.Hex())
	for _, mode := range []string{"valid", "404", "oversize", "malformed", "redirect", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			calls, redirects := 0, 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects++; _, _ = w.Write([]byte(body)) }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v1/merkle/"+strings.ToLower(adapter.Hex())+"/permission-sets/"+root.Hex() || r.URL.RawQuery != "" {
					t.Error("source requested another root/chain")
				}
				switch mode {
				case "404":
					http.Error(w, "private body", http.StatusNotFound)
					return
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
					return
				case "malformed":
					_, _ = w.Write([]byte("private malformed"))
					return
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat("x", 1025)))
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			source, err := New(protocolio.HTTPConfig{URL: server.URL + "/api/v1", MaxResponseBytes: 1024}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				return common.LeftPadBytes(adapter.Bytes(), 32), nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			raw, err := source.PermissionTree(ctx, big.NewInt(31337), recipes.Snapshot{Root: root, Account: account, BlockNumber: 123})
			if mode == "valid" {
				if err != nil || string(raw) != string(metadata) {
					t.Fatalf("wire bytes changed: %s %v", raw, err)
				}
			} else if err == nil {
				t.Fatal("failed source accepted")
			}
			if redirects != 0 || calls > 1 {
				t.Fatal("redirect/fallback sent another request")
			}
			if err != nil && (strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), server.URL)) {
				t.Fatal("source leaked response or URL")
			}
		})
	}
}

type readerFunc func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f readerFunc) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return f(ctx, msg, block)
}
