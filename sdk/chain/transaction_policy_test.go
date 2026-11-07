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
	for _, code := range []string{"nonce too low", "replacement transaction underpriced", "insufficient funds", "already known", "execution reverted"} {
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
	_, err = client.BalanceAt(t.Context(), common.Address{}, nil)
	if err == nil || strings.Contains(err.Error(), "secret-") {
		t.Fatalf("balance read leaked credentials: %v", err)
	}
}
