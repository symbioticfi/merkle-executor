package txengine

import (
	"context"
	"math/big"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-logr/logr"
)

type delayedWitnessBackend struct {
	*witnessBackend

	delay time.Duration
}

func (b *delayedWitnessBackend) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(b.delay):
		return b.witnessBackend.HeaderByNumber(ctx, number)
	}
}

func (b *delayedWitnessBackend) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(b.delay):
		return b.witnessBackend.TransactionReceipt(ctx, hash)
	}
}

func TestCanonicalPreflightAllowsThreeReadBudgets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tx := types.NewTx(&types.DynamicFeeTx{Nonce: 7, To: ptr(common.Address{1})})
		b := &delayedWitnessBackend{witnessBackend: newWitnessBackend(tx, 0), delay: 700 * time.Millisecond}
		m := New(b, mustSigner(t), big.NewInt(1), Config{}, logr.Discard())
		result, done := m.confirmPendingReceipt(t.Context(), &pendingTransaction{
			req: Request{Label: "slow-inclusion"}, nonce: tx.Nonce(),
		}, txAttempt{hash: tx.Hash()}, b.receipts[tx.Hash()])
		if !done || result.Outcome != OutcomeConfirmed || m.confirmedNonceFloor != tx.Nonce()+1 {
			t.Fatalf("three healthy 700ms reads did not confirm inclusion: %+v, done=%t", result, done)
		}
	})
}

func TestConfirmationAcceptsOmittedLogMetadata(t *testing.T) {
	tx := types.NewTx(&types.DynamicFeeTx{Nonce: 7, To: ptr(common.Address{1})})
	b := newWitnessBackend(tx, 2)
	b.receipts[tx.Hash()].Logs = []*types.Log{{Data: []byte{1}}}
	m := New(b, mustSigner(t), big.NewInt(1), Config{}, logr.Discard())
	result, done := m.confirmPendingReceipt(t.Context(), &pendingTransaction{
		nonce: tx.Nonce(),
	}, txAttempt{hash: tx.Hash()}, b.receipts[tx.Hash()])
	if !done || result.Outcome != OutcomeConfirmed {
		t.Fatalf("optional log metadata prevented generic confirmation: %+v, done=%t", result, done)
	}
}
