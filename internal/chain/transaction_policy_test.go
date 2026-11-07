package chain

import (
	"strings"
	"testing"

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

// Typed causes remain available after redaction.
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
