package chain

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
)

func TestTransactionErrorsKeepClassificationWithoutSecrets(t *testing.T) {
	for _, code := range []string{"nonce too low", "replacement transaction underpriced", "insufficient funds", "already known", "already imported", "execution reverted"} {
		t.Run(code, func(t *testing.T) {
			cause := errors.New(code + " at https://rpc.example/private-key?secret=hidden")
			err := rpcFailure("submit", cause)
			if !strings.Contains(err.Error(), code) || strings.Contains(err.Error(), "private-key") || strings.Contains(err.Error(), "hidden") {
				t.Fatalf("unsafe classification: %s", err)
			}
			if !errors.Is(err, cause) {
				t.Fatal("lost error cause")
			}
		})
	}
}

// Preserve typed RPC error causes through sanitization.
func TestInvalidParamsCauseSurvivesRedaction(t *testing.T) {
	cause := &invalidParams{}
	var rpcErr rpc.Error
	if !errors.As(rpcFailure("estimate", cause), &rpcErr) || rpcErr.ErrorCode() != -32602 {
		t.Fatal("lost RPC code")
	}
}

type invalidParams struct{}

func (*invalidParams) Error() string  { return "invalid params" }
func (*invalidParams) ErrorCode() int { return -32602 }

func TestBalanceReadRedactsTransportCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage
			Method string
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Method == "eth_chainId" {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"})
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("secret-response-body"))
	}))
	defer server.Close()
	client, err := Dial(t.Context(), server.URL+"/secret-path?key=secret-query", "", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, read := range []func() (*big.Int, error){
		func() (*big.Int, error) { return client.BalanceAt(t.Context(), common.Address{}, nil) },
		func() (*big.Int, error) { return client.TransactionSenderBalanceAt(t.Context(), common.Address{}, nil) },
	} {
		_, err = read()
		if err == nil || strings.Contains(err.Error(), "secret-") {
			t.Fatalf("balance read leaked credentials: %v", err)
		}
	}
}

func TestTransactionSenderBalanceUsesWriteRPCWithoutFallback(t *testing.T) {
	read, write := newRPCFixture(t, "0x1"), newRPCFixture(t, "0x1")
	client, err := Dial(t.Context(), read.server.URL, write.server.URL, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	account := common.HexToAddress("0x1234")
	if balance, err := client.BalanceAt(t.Context(), account, big.NewInt(12)); err != nil || balance.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("ordinary balance read: %v %v", balance, err)
	}
	if balance, err := client.TransactionSenderBalanceAt(t.Context(), account, nil); err != nil || balance.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("sender balance read: %v %v", balance, err)
	}
	read.mu.Lock()
	if len(read.requests) != 2 || read.requests[1].Method != "eth_getBalance" || string(read.requests[1].Params[1]) != `"0xc"` {
		t.Errorf("ordinary balance read left read RPC: %+v", read.requests)
	}
	read.mu.Unlock()
	write.mu.Lock()
	if len(write.requests) != 2 || write.requests[1].Method != "eth_getBalance" || string(write.requests[1].Params[0]) != `"`+account.Hex()+`"` || string(write.requests[1].Params[1]) != `"latest"` {
		t.Errorf("sender balance read left write RPC: %+v", write.requests)
	}
	write.fail = true
	write.mu.Unlock()
	if _, err := client.TransactionSenderBalanceAt(t.Context(), account, nil); err == nil {
		t.Fatal("failed write-RPC balance read fell back to read RPC")
	}
	read.mu.Lock()
	defer read.mu.Unlock()
	if len(read.requests) != 2 {
		t.Fatal("write-RPC failure attempted a read-RPC fallback")
	}
}
