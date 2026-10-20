package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

type rpcFixture struct {
	mu       sync.Mutex
	chainID  string
	fail     bool
	requests []rpcRequest
	server   *httptest.Server
}
type rpcRequest struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func newRPCFixture(t *testing.T, chainID string) *rpcFixture {
	t.Helper()
	f := &rpcFixture{chainID: chainID}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
			return
		}
		f.mu.Lock()
		failEarly := f.fail
		f.mu.Unlock()
		if failEarly {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream-error-body-test-secret"))
			return
		}
		var request rpcRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Error("invalid JSON-RPC request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, request)
		fail := f.fail
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream-error-body-test-secret"))
			return
		}
		var result any
		switch request.Method {
		case "eth_chainId":
			result = f.chainID
		case "eth_call":
			result = "0x"
		case "eth_estimateGas":
			result = "0x5208"
		case "eth_getTransactionCount":
			result = "0x7"
		case "eth_sendRawTransaction":
			result = common.HexToHash("0x123")
		case "eth_getTransactionReceipt":
			result = nil
		case "eth_getLogs":
			result = []types.Log{}
		case "eth_getBlockByNumber", "eth_getBlockByHash":
			result = &types.Header{Number: big.NewInt(12), Difficulty: big.NewInt(0), GasLimit: 30_000_000, GasUsed: 21000, Time: 123, Extra: []byte{}, BaseFee: big.NewInt(1)}
		default:
			t.Errorf("unexpected JSON-RPC method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func testTransaction() *types.Transaction {
	to := common.HexToAddress("0x1001")
	return types.NewTx(&types.LegacyTx{To: &to, Nonce: 7, Gas: 21000, GasPrice: big.NewInt(1)})
}

func TestRPCRoutesReadsAndWritesToTheirVerifiedEndpoints(t *testing.T) {
	read, write := newRPCFixture(t, "0x1"), newRPCFixture(t, "0x1")
	client, err := Dial(context.Background(), read.server.URL, write.server.URL, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	a := common.HexToAddress("0x1001")
	if _, err := client.CallContract(ctx, ethereum.CallMsg{To: &a}, big.NewInt(12)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.HeaderByNumber(ctx, big.NewInt(12)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.HeaderByHash(ctx, common.HexToHash("0x12")); err != nil {
		t.Fatal(err)
	}
	if _, err := client.FilterLogs(ctx, ethereum.FilterQuery{}); err != nil {
		t.Fatal(err)
	}
	if gas, err := client.EstimateGas(ctx, ethereum.CallMsg{To: &a}); err != nil || gas != 21000 {
		t.Fatalf("estimate: %d %v", gas, err)
	}
	if _, err := client.TransactionReceipt(ctx, common.HexToHash("0x123")); !errors.Is(err, ethereum.NotFound) {
		t.Fatalf("missing receipt identity lost: %v", err)
	}
	if n, err := client.NonceAt(ctx, a, nil); err != nil || n != 7 {
		t.Fatalf("mined nonce: %d %v", n, err)
	}
	if n, err := client.PendingNonceAt(ctx, a); err != nil || n != 7 {
		t.Fatalf("pending nonce: %d %v", n, err)
	}
	if err := client.SendTransaction(ctx, testTransaction()); err != nil {
		t.Fatal(err)
	}
	read.mu.Lock()
	defer read.mu.Unlock()
	write.mu.Lock()
	defer write.mu.Unlock()
	if len(read.requests) != 7 || len(write.requests) != 4 {
		t.Fatalf("unexpected RPC routing: read=%d write=%d", len(read.requests), len(write.requests))
	}
	for _, r := range read.requests {
		if r.Method == "eth_getTransactionCount" || r.Method == "eth_sendRawTransaction" {
			t.Fatal("nonce or submission sent to read endpoint")
		}
	}
	if string(write.requests[1].Params[1]) != `"latest"` || string(write.requests[2].Params[1]) != `"pending"` {
		t.Fatal("nonce tags were not preserved")
	}
}

func TestRPCRejectsWrongChainOnEitherEndpoint(t *testing.T) {
	for _, which := range []string{"read", "write"} {
		t.Run(which, func(t *testing.T) {
			readID, writeID := "0x1", "0x1"
			if which == "read" {
				readID = "0x2"
			} else {
				writeID = "0x2"
			}
			read, write := newRPCFixture(t, readID), newRPCFixture(t, writeID)
			client, err := Dial(context.Background(), read.server.URL, write.server.URL, big.NewInt(1))
			if err == nil {
				client.Close()
				t.Fatal("mismatched network accepted")
			}
		})
	}
}

func TestRPCFailuresRedactCredentialsAndBodiesButPreserveErrorIdentity(t *testing.T) {
	fixture := newRPCFixture(t, "0x1")
	endpoint := fixture.server.URL + "/endpoint-path-test-secret?key=endpoint-query-test-secret"
	client, err := Dial(context.Background(), endpoint, "", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	fixture.mu.Lock()
	fixture.fail = true
	fixture.mu.Unlock()
	a := common.HexToAddress("0x1001")
	operations := map[string]func(context.Context) error{
		"call": func(ctx context.Context) error {
			_, e := client.CallContract(ctx, ethereum.CallMsg{To: &a}, nil)
			return e
		},
		"header-number":     func(ctx context.Context) error { _, e := client.HeaderByNumber(ctx, nil); return e },
		"header-hash":       func(ctx context.Context) error { _, e := client.HeaderByHash(ctx, common.Hash{}); return e },
		"logs":              func(ctx context.Context) error { _, e := client.FilterLogs(ctx, ethereum.FilterQuery{}); return e },
		"estimate":          func(ctx context.Context) error { _, e := client.EstimateGas(ctx, ethereum.CallMsg{To: &a}); return e },
		"receipt":           func(ctx context.Context) error { _, e := client.TransactionReceipt(ctx, common.Hash{}); return e },
		"nonce":             func(ctx context.Context) error { _, e := client.NonceAt(ctx, a, nil); return e },
		"pending-nonce":     func(ctx context.Context) error { _, e := client.PendingNonceAt(ctx, a); return e },
		"send":              func(ctx context.Context) error { return client.SendTransaction(ctx, testTransaction()) },
		"balance":           func(ctx context.Context) error { _, e := client.BalanceAt(ctx, a, nil); return e },
		"telemetry-balance": func(ctx context.Context) error { _, e := client.ReadBalanceAt(ctx, a); return e },
		"telemetry-nonces":  func(ctx context.Context) error { _, _, e := client.ReadNonces(ctx, a); return e },
		"fee-history":       func(ctx context.Context) error { _, e := client.FeeHistory(ctx, 3, nil, []float64{50}); return e },
		"next-block-gas": func(ctx context.Context) error {
			_, e := client.EstimateGasNextBlock(ctx, ethereum.CallMsg{To: &a}, &types.Header{Number: big.NewInt(1)}, time.Second)
			return e
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			err := operation(context.Background())
			if err == nil {
				t.Fatal("expected RPC failure")
			}
			for _, rendered := range []string{err.Error(), fmt.Sprintf("%+v", err), errors.Errorf("keeper operation: %w", err).Error()} {
				if strings.Contains(rendered, "test-secret") || strings.Contains(rendered, fixture.server.URL) {
					t.Fatal("RPC error exposed endpoint or response body")
				}
			}
			if errors.Unwrap(err) == nil {
				t.Fatal("sanitized error lost underlying cause")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := operation(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation identity lost: %v", err)
			}
		})
	}
	// A transport failure includes the request URL, unlike an HTTP status error.
	fixture.server.Close()
	_, err = client.CallContract(context.Background(), ethereum.CallMsg{To: &a}, nil)
	if err == nil || strings.Contains(err.Error(), "test-secret") || strings.Contains(err.Error(), fixture.server.URL) {
		t.Fatal("transport error exposed credential-bearing endpoint")
	}
	if cause := errors.Unwrap(err); cause == nil || !strings.Contains(cause.Error(), "endpoint-query-test-secret") {
		t.Fatal("transport fixture did not exercise a credential-bearing cause")
	}
}

func TestRPCStartupCancellationRetainsIdentityWithoutEndpointDetails(t *testing.T) {
	fixture := newRPCFixture(t, "0x1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Dial(ctx, fixture.server.URL+"/startup-test-secret", "", big.NewInt(1))
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("startup cancellation was not safely preserved: %v", err)
	}
}

func TestRPCNeverFollowsReadOrSubmissionRedirects(t *testing.T) {
	for _, mode := range []string{"read identity", "write identity", "read call", "submission"} {
		t.Run(mode, func(t *testing.T) {
			target := newRPCFixture(t, "0x1")
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request rpcRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				redirect := mode == "read identity" || mode == "write identity" || mode == "read call" && request.Method == "eth_call" || mode == "submission" && request.Method == "eth_sendRawTransaction"
				if redirect {
					http.Redirect(w, r, target.server.URL+"/private-redirect-path", http.StatusTemporaryRedirect)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"})
			}))
			defer source.Close()
			readURL, writeURL := source.URL, ""
			if mode == "write identity" || mode == "submission" {
				readURL = newRPCFixture(t, "0x1").server.URL
				writeURL = source.URL
			}
			client, err := Dial(context.Background(), readURL, writeURL, big.NewInt(1))
			if mode == "read identity" || mode == "write identity" {
				if err == nil {
					client.Close()
					t.Error("redirected chain identity accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				if mode == "submission" {
					err = client.SendTransaction(context.Background(), testTransaction())
				} else {
					a := common.HexToAddress("0x123")
					_, err = client.CallContract(context.Background(), ethereum.CallMsg{To: &a}, nil)
				}
				if err == nil {
					t.Error("RPC followed redirect")
				}
			}
			if err != nil && strings.Contains(err.Error(), "private-redirect-path") {
				t.Fatal("redirect URL leaked")
			}
			target.mu.Lock()
			defer target.mu.Unlock()
			if len(target.requests) != 0 {
				t.Fatalf("redirect target received %d requests", len(target.requests))
			}
		})
	}
}
