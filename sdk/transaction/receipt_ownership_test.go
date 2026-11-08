package transaction

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

func TestReceiptPollingKeepsFirstOwnerOfRepeatedSignedHash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		backend := &initialResponseBackend{practicalChain: chain, response: errors.New("transaction underpriced")}
		manager, reg := metricManager(t, backend, 0)
		request := Request{To: common.Address{1}, Solver: "first", Label: "old"}
		first := manager.Send(t.Context(), request)
		if !first.Uncertain() {
			t.Fatal("signed rejection lost its ownership")
		}
		// A rejection does not raise the accepted fee floor. Identical intent can
		// therefore produce the same signed hash for a later request.
		backend.response = nil
		request.Solver, request.Label = "second", "new"
		results, admitted := manager.SendAsync(t.Context(), request)
		if !admitted {
			t.Fatal("second request was not admitted")
		}
		synctest.Wait()
		attempts := chain.attempts()
		if len(attempts) != 2 || attempts[0].Hash() != attempts[1].Hash() {
			t.Fatal("fixture did not reproduce shared signed identity")
		}
		// Complete the ordinary rotation before the new signature's priority turn.
		time.Sleep(time.Second)
		synctest.Wait()
		chain.mine(attempts[0], types.ReceiptStatusSuccessful, 0)
		time.Sleep(time.Second)
		synctest.Wait()
		result := <-results
		if result.Included() || !result.Uncertain() || !errors.Is(result.Err, ErrNonceConsumed) {
			t.Fatalf("older signed identity settled a newer request: %+v", result)
		}
		if metricValue(t, reg, "test_transaction_gas_used_total", map[string]string{"solver": "first", "label": "old"}) != 21_000 || metricValue(t, reg, "test_transaction_gas_used_total", map[string]string{"solver": "second", "label": "new"}) != 0 {
			t.Fatal("canonical cost was charged to the newer owner")
		}
		if metricValue(t, reg, "test_transaction_requests_total", map[string]string{"solver": "first", "outcome": "uncertain"}) != 1 || metricValue(t, reg, "test_transaction_requests_total", map[string]string{"solver": "second", "outcome": "uncertain"}) != 1 || metricValue(t, reg, "test_transaction_unresolved_nonce", nil) != 0 {
			t.Fatal("late resolution changed request outcomes or retained nonce ownership")
		}
	})
}
