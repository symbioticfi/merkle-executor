package transaction

import (
	"context"
	"math/big"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestPendingFeesWaitForNewBlockAndPaceRebroadcasts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m, reg := metricManager(t, chain, 0)
		if err := m.Sender("merkle").Submit(t.Context(), ExecutionRequest{To: common.Address{1}, Data: []byte{2}}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		original := chain.attempts()[0]
		time.Sleep(29 * time.Second)
		if len(chain.attempts()) != 1 {
			t.Fatal("successful broadcast repeated at receipt-poll cadence")
		}
		time.Sleep(2 * time.Second)
		if attempts := chain.attempts(); len(attempts) != 2 || attempts[1].Hash() != original.Hash() {
			t.Fatal("unchanged chain head increased fees")
		}
		chain.mu.Lock()
		chain.advanceLocked("new fee decision block")
		chain.mu.Unlock()
		time.Sleep(30 * time.Second)
		attempts := chain.attempts()
		if len(attempts) != 3 || attempts[2].Hash() == original.Hash() || attempts[2].Nonce() != original.Nonce() || attempts[2].GasFeeCap().Cmp(bump(original.GasFeeCap())) < 0 || attempts[2].GasTipCap().Cmp(bump(original.GasTipCap())) < 0 {
			t.Fatal("new block did not permit bounded fee progress")
		}
		if metricValue(t, reg, "test_transaction_broadcasts_total", map[string]string{"kind": "replacement"}) != 1 || metricValue(t, reg, "test_transaction_broadcasts_total", map[string]string{"kind": "rebroadcast"}) != 1 {
			t.Fatal("replacement and exact-rebroadcast accounting wrong")
		}
		chain.mine(attempts[2], types.ReceiptStatusSuccessful, 0)
	})
}

func TestAmbiguousBroadcastRetriesSoonWithoutNewSignature(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		backend := &initialResponseBackend{practicalChain: chain, response: context.DeadlineExceeded}
		m, _ := metricManager(t, backend, 0)
		if err := m.Sender("merkle").Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		original := chain.attempts()[0]
		time.Sleep(4 * time.Second)
		if len(chain.attempts()) != 1 {
			t.Fatal("ambiguous response retried at receipt-poll cadence")
		}
		time.Sleep(2 * time.Second)
		if attempts := chain.attempts(); len(attempts) != 2 || attempts[1].Hash() != original.Hash() {
			t.Fatal("ambiguous broadcast did not retry exact signed bytes within five seconds")
		}
	})
}

func TestReplacementFeeCeilingStillAllowsExactRebroadcast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m, reg := metricManager(t, chain, 0)
		_, admitted := m.SendAsync(t.Context(), Request{To: common.Address{1}, MaxFeePerGas: big.NewInt(45e9)})
		if !admitted {
			t.Fatal("request was not admitted")
		}
		synctest.Wait()
		original := chain.attempts()[0]
		chain.mu.Lock()
		chain.advanceLocked("replacement exceeds configured fee ceiling")
		chain.mu.Unlock()
		time.Sleep(31 * time.Second)
		synctest.Wait()
		if attempts := chain.attempts(); len(attempts) != 2 || attempts[1].Hash() != original.Hash() {
			t.Fatal("fee ceiling prevented relay progress or changed the signed identity")
		}
		if metricValue(t, reg, "test_transaction_broadcasts_total", map[string]string{"kind": "replacement"}) != 0 {
			t.Fatal("failed replacement pricing counted as a broadcast")
		}
	})
}
