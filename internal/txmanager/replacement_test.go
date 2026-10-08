package txmanager

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/internal/state"
	engine "github.com/symbioticfi/merkle-executor/internal/txengine"
)

func TestReplacementPersistsBeforeBroadcastAndOriginalReceiptWins(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b := newChain()
	b.mine = false
	sent := make(chan *types.Transaction, 16)
	b.beforeBroadcast = func(tx *types.Transaction) {
		var saved record
		found, err := store.Load(journalKey, &saved)
		if err != nil || !found {
			t.Errorf("missing signed journal before broadcast: %v", err)
		}
		present := false
		for _, raw := range append([]hexutil.Bytes{saved.Raw}, saved.Attempts...) {
			var owned types.Transaction
			if owned.UnmarshalBinary(raw) == nil && owned.Hash() == tx.Hash() {
				present = true
			}
		}
		if !present {
			t.Error("replacement broadcast before its bytes were durable")
		}
		b.feeErr = context.DeadlineExceeded // Force the upstream cached-bump fallback.
		sent <- tx
	}
	m, err := New(b, testSigner(t), big.NewInt(1), store, Config{MaxFeeGwei: 30, Confirmations: 2, PollInterval: time.Millisecond, ReplacementInterval: 5 * time.Millisecond, PendingTimeout: time.Second, ShutdownTimeout: 10 * time.Millisecond, Horizon: engine.HorizonConfig{BlockTime: 2 * time.Millisecond}}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	completed := make(chan struct {
		result Result
		err    error
	}, 1)
	go func() {
		r, e := m.Send(ctx, request())
		completed <- struct {
			result Result
			err    error
		}{r, e}
	}()
	first := waitSigned(t, ctx, sent)
	second := waitSigned(t, ctx, sent)
	if second.Hash() == first.Hash() || second.Nonce() != first.Nonce() || second.GasFeeCap().Cmp(first.GasFeeCap()) <= 0 || second.GasTipCap().Cmp(first.GasTipCap()) <= 0 {
		t.Fatal("not a higher-fee same-nonce replacement")
	}
	b.mu.Lock()
	b.receipts[first.Hash()] = &types.Receipt{TxHash: first.Hash(), BlockNumber: big.NewInt(10), BlockHash: b.receiptHeader.Hash(), Status: 1}
	b.latest = first.Nonce() + 1
	b.mu.Unlock()
	select {
	case got := <-completed:
		if got.err != nil || !got.result.Confirmed || got.result.Hash != first.Hash() || len(got.result.Attempts) < 2 {
			t.Fatalf("lost original execution: %+v %v", got.result, got.err)
		}
	case <-ctx.Done():
		t.Fatal("original receipt not resolved")
	}
	if err = m.Acknowledge(request().ID); err != nil {
		t.Fatal(err)
	}
}
func waitSigned(t *testing.T, ctx context.Context, sent <-chan *types.Transaction) *types.Transaction {
	t.Helper()
	select {
	case tx := <-sent:
		return tx
	case <-ctx.Done():
		t.Fatal("replacement never broadcast")
		return nil
	}
}
