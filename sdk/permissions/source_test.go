package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
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
	for _, tc := range []struct{ mode, want string }{
		{"valid", ""},
		{"404", "protocol HTTP status 404"},
		{"429", "protocol HTTP status 429"},
		{"500", "protocol HTTP status 500"},
		{"oversize", "protocol response exceeds configured bound"},
		{"malformed", "invalid protocol JSON response"},
		{"transport", "protocol HTTP transport failed"},
		{"redirect", "protocol HTTP status 307"},
		{"canceled", "context canceled"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			calls, redirects := 0, 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects++; _, _ = w.Write([]byte(body)) }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v1/merkle/"+strings.ToLower(adapter.Hex())+"/"+root.Hex() || r.URL.RawQuery != "" {
					t.Error("source requested another root/chain")
				}
				switch tc.mode {
				case "404":
					http.Error(w, "private body", http.StatusNotFound)
					return
				case "429":
					http.Error(w, "private body", http.StatusTooManyRequests)
					return
				case "500":
					http.Error(w, "private body", http.StatusInternalServerError)
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
			client := server.Client()
			if tc.mode == "transport" {
				client.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.Errorf("private transport details for %s", server.URL)
				})
			}
			source, err := New(protocolio.HTTPConfig{URL: server.URL + "/api/v1", MaxResponseBytes: 1024}, client, readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				return common.LeftPadBytes(adapter.Bytes(), 32), nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.mode == "canceled" {
				cancel()
			}
			raw, err := source.PermissionTree(ctx, big.NewInt(31337), recipes.Snapshot{Root: root, Account: account, BlockNumber: 123})
			if tc.mode == "valid" {
				if err != nil || string(raw) != string(metadata) {
					t.Fatalf("wire bytes changed: %s %v", raw, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) || raw != nil {
				t.Fatalf("want %q and no metadata, got %v", tc.want, err)
			}
			if tc.mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("context cancellation sentinel lost: %v", err)
			}
			wantCalls := 1
			if tc.mode == "canceled" || tc.mode == "transport" {
				wantCalls = 0
			}
			if redirects != 0 || calls != wantCalls {
				t.Fatalf("unexpected request/fallback count: %d requests, %d redirects", calls, redirects)
			}
			if err != nil && (strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), server.URL)) {
				t.Fatal("source leaked response or URL")
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestSourceCancelsWhileReadingHTTPBody(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	adapter := common.HexToAddress("0x99")
	source, err := New(protocolio.HTTPConfig{URL: server.URL}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
		return common.LeftPadBytes(adapter.Bytes(), 32), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		raw, err := source.PermissionTree(ctx, big.NewInt(1), recipes.Snapshot{Account: common.HexToAddress("0x1"), BlockNumber: 123})
		if raw != nil {
			t.Error("canceled body returned permission metadata")
		}
		result <- err
	}()
	select {
	case <-ready:
		cancel()
	case <-ctx.Done():
		t.Fatal("permission request did not begin")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("body read did not preserve cancellation: %v", err)
	}
}

func TestSourceRejectsInvalidChainBeforeAnyIO(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   *big.Int
	}{
		{"nil", nil},
		{"zero", new(big.Int)},
		{"negative", big.NewInt(-1)},
		{"overflow", new(big.Int).Lsh(big.NewInt(1), 256)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := New(protocolio.HTTPConfig{URL: "http://permissions.invalid"}, nil, readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				t.Fatal("invalid chain reached reader")
				return nil, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := source.PermissionTree(t.Context(), tc.id, recipes.Snapshot{Account: common.HexToAddress("0x1")})
			if err == nil || err.Error() != "permission chain ID required" || raw != nil {
				t.Fatalf("invalid chain accepted: %v", err)
			}
		})
	}
}
