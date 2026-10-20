package chain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
func TestEstimateGasNextBlockUsesReadEndpointAndOverrides(t *testing.T) {
	var seen []json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		value := "0x1"
		if request.Method == "eth_estimateGas" {
			seen = request.Params
			value = "0x5208"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": value})
	}))
	defer server.Close()
	client, err := Dial(t.Context(), server.URL, "", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	estimator, ok := any(client).(interface {
		EstimateGasNextBlock(context.Context, ethereum.CallMsg, *types.Header, time.Duration) (uint64, error)
	})
	if !ok {
		t.Fatal("next-block estimation missing")
	}
	to := common.HexToAddress("0x1234")
	gas, err := estimator.EstimateGasNextBlock(t.Context(), ethereum.CallMsg{To: &to, Data: []byte{1}}, &types.Header{Number: big.NewInt(100), Time: 1000}, 12*time.Second)
	if err != nil || gas != 21000 {
		t.Fatalf("estimate %d: %v", gas, err)
	}
	if len(seen) != 4 || string(seen[1]) != `"latest"` || string(seen[2]) != "null" {
		t.Fatalf("wrong estimate parameters: %s", seen)
	}
	var overrides map[string]string
	if err := json.Unmarshal(seen[3], &overrides); err != nil {
		t.Fatal(err)
	}
	if overrides["number"] != "0x65" || overrides["time"] != "0x3f4" {
		t.Fatalf("wrong next block: %v", overrides)
	}
}

// Keep invalid-params errors discoverable through sanitization for fallback.
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

func TestBalanceTelemetryRedactsTransportCredentials(t *testing.T) {
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
		t.Fatalf("balance telemetry leaked credentials: %v", err)
	}
}
