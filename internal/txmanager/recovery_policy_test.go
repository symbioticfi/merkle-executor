package txmanager

import (
	"context"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"math/big"
	"testing"
	"time"

	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func TestRecoveryNeverRebroadcastsStoredRecipe(t *testing.T) {
	b := newChain()
	b.mine = false
	s := testSigner(t)
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	original := newManager(t, b, s, store)
	ctx, cancel := timeout()
	pending, err := original.Send(ctx, request())
	cancel()
	if err == nil || !pending.Pending {
		t.Fatalf("expected retained uncertain work: %+v, %v", pending, err)
	}
	original.Close()
	b.mu.Lock()
	before := len(b.sent)
	b.mu.Unlock()
	recovered := newManager(t, b, s, store)
	retry, stop := context.WithCancel(t.Context())
	stop()
	_, _ = recovered.Send(retry, request())
	ctx, cancel = timeout()
	_, _ = recovered.Send(ctx, request())
	cancel()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.sent) != before {
		t.Fatalf("restart replayed recipe: sends before=%d after=%d", before, len(b.sent))
	}
}

func TestJournalResolvesLateReceiptAfterEngineStopsTracking(t *testing.T) {
	for _, status := range []uint64{types.ReceiptStatusSuccessful, types.ReceiptStatusFailed} {
		name := "confirmed"
		if status == types.ReceiptStatusFailed {
			name = "reverted"
		}
		t.Run(name, func(t *testing.T) {
			store, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			b := newChain()
			b.mine = false
			m, err := New(b, testSigner(t), big.NewInt(1), store, Config{
				MaxFeeGwei: 30, Confirmations: 2, PollInterval: time.Millisecond,
				PendingTimeout: 100 * time.Millisecond, ReplacementInterval: 100 * time.Millisecond,
				ShutdownTimeout: 10 * time.Millisecond,
			}, logr.Discard())
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Start(t.Context()); err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			pending, err := m.Send(ctx, request())
			cancel()
			if !errors.Is(err, context.DeadlineExceeded) || !pending.Pending || !m.engine.LaneReady() {
				t.Fatalf("journal did not retain work after active tracking ended: %+v %v", pending, err)
			}
			if err = m.Acknowledge(t.Context(), request().ID); err == nil {
				t.Fatal("uncertain job acknowledged")
			}
			competing := request()
			competing.ID = "next-recipe"
			if _, err = m.Send(t.Context(), competing); !errors.Is(err, ErrLaneBusy) {
				t.Fatalf("uncertain job lost ownership: %v", err)
			}
			b.mu.Lock()
			sends := len(b.sent)
			original := b.sent[0]
			b.latest = original.Nonce() + 1
			b.receipts[original.Hash()] = &types.Receipt{
				TxHash: original.Hash(), Status: status, BlockNumber: new(big.Int).Set(b.receiptHeader.Number), BlockHash: b.receiptHeader.Hash(),
			}
			b.mu.Unlock()
			got, err := m.Send(t.Context(), request())
			if err != nil || got.Pending || got.Hash != original.Hash() || got.Execution == nil || got.Reverted != (status == types.ReceiptStatusFailed) {
				t.Fatalf("journal failed to resolve late owned receipt: %+v %v", got, err)
			}
			b.mu.Lock()
			after := len(b.sent)
			b.mu.Unlock()
			if after != sends {
				t.Fatalf("receipt recovery broadcast transactions: before=%d after=%d", sends, after)
			}
			var saved record
			found, err := store.Load(journalKey, &saved)
			if err != nil || !found || saved.Terminal != name || saved.Execution == nil || saved.Execution.TxHash != original.Hash() {
				t.Fatalf("late terminal evidence was not durable: %+v %v", saved, err)
			}
		})
	}
}

type blockedEstimate struct {
	*fakeChain
	entered chan struct{}
	release chan struct{}
}

func (b *blockedEstimate) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	close(b.entered)
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-b.release:
		return b.fakeChain.EstimateGas(ctx, msg)
	}
}
func TestCancelledUnsignedAdmissionKeepsIdentityUntilCompletion(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b := &blockedEstimate{fakeChain: newChain(), entered: make(chan struct{}), release: make(chan struct{})}
	m, err := New(b, testSigner(t), big.NewInt(1), store, Config{MaxFeeGwei: 30, PollInterval: time.Millisecond, Confirmations: 2, ShutdownTimeout: 10 * time.Millisecond}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan Result, 1)
	go func() { pending, _ := m.Send(ctx, request()); result <- pending }()
	select {
	case <-b.entered:
	case <-time.After(time.Second):
		t.Fatal("estimate not admitted")
	}
	cancel()
	pending := <-result
	if !pending.Pending || pending.Hash != (common.Hash{}) || m.PendingID() != request().ID {
		t.Fatalf("lost unsigned admitted job: %+v", pending)
	}
	other := request()
	other.ID = "competing"
	if _, err = m.Send(t.Context(), other); !errors.Is(err, ErrLaneBusy) {
		t.Fatalf("competing job entered unsigned lane: %v", err)
	}
	if err = m.Acknowledge(t.Context(), request().ID); err == nil {
		t.Fatal("unsigned admitted job acknowledged")
	}
	close(b.release)
	joined, err := m.Send(t.Context(), request())
	if err != nil || !joined.Confirmed {
		t.Fatalf("admitted job did not finish after caller cancellation: %+v %v", joined, err)
	}
}

type reincludedReceipt struct {
	*fakeChain
	reads int
}

func (b *reincludedReceipt) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads++
	header := b.receiptHeader
	status := uint64(types.ReceiptStatusSuccessful)
	data := byte(1)
	if b.reads > 1 {
		header = &types.Header{Number: big.NewInt(11), GasLimit: 30_000_000, BaseFee: big.NewInt(1), ParentHash: b.receiptHeader.Hash()}
		status = types.ReceiptStatusFailed
		data = 2
	}
	return &types.Receipt{TxHash: hash, BlockNumber: new(big.Int).Set(header.Number), BlockHash: header.Hash(), Status: status, Logs: []*types.Log{{TxHash: hash, BlockNumber: header.Number.Uint64(), BlockHash: header.Hash(), Data: []byte{data}}}}, nil
}
func TestRecoveryPersistsTheReceiptActuallyConfirmedAfterReinclusion(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := testSigner(t)
	req := request()
	tx, err := s.SignTx(t.Context(), types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), To: &req.To, Data: req.Data, Gas: 100000, GasFeeCap: big.NewInt(30), GasTipCap: big.NewInt(1)}), big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(journalKey, &record{Version: 1, ID: req.ID, Sender: s.Address(), ChainID: "1", Raw: raw, Confirmations: 1}); err != nil {
		t.Fatal(err)
	}
	b := &reincludedReceipt{fakeChain: newChain()}
	m, err := New(b, s, big.NewInt(1), store, Config{MaxFeeGwei: 30, PollInterval: time.Millisecond}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Send(t.Context(), req)
	if err != nil || !got.Reverted || got.Confirmed || got.Execution == nil || got.Execution.BlockNumber.Int64() != 11 || got.Execution.Logs[0].Data[0] != 2 {
		t.Fatalf("persisted pre-confirmation evidence: %+v, %v", got, err)
	}
}

type unavailableAttempt struct {
	*fakeChain
	unavailable common.Hash
}

func (b *unavailableAttempt) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	if hash == b.unavailable {
		return nil, errors.New("older hash RPC unavailable")
	}
	return b.fakeChain.TransactionReceipt(ctx, hash)
}
func TestRecoveryChecksNewerReceiptWhenOlderHashReadFails(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := testSigner(t)
	req := request()
	raws := make([]hexutil.Bytes, 2)
	hashes := make([]common.Hash, 2)
	for i := range 2 {
		tx, err := s.SignTx(t.Context(), types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), To: &req.To, Data: req.Data, Gas: 100000, GasFeeCap: big.NewInt(int64(30 + i*10)), GasTipCap: big.NewInt(int64(1 + i))}), big.NewInt(1))
		if err != nil {
			t.Fatal(err)
		}
		raws[i], err = tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		hashes[i] = tx.Hash()
	}
	if err = store.Save(journalKey, &record{Version: 1, ID: req.ID, Sender: s.Address(), ChainID: "1", Raw: raws[0], Attempts: raws[1:], Confirmations: 2}); err != nil {
		t.Fatal(err)
	}
	b := &unavailableAttempt{fakeChain: newChain(), unavailable: hashes[0]}
	b.receipts[hashes[1]] = &types.Receipt{TxHash: hashes[1], BlockNumber: big.NewInt(10), BlockHash: b.receiptHeader.Hash(), Status: 1}
	m, err := New(b, s, big.NewInt(1), store, Config{MaxFeeGwei: 30, PollInterval: time.Millisecond}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Send(t.Context(), req)
	if err != nil || !got.Confirmed || got.Hash != hashes[1] {
		t.Fatalf("older unavailable hash hid canonical replacement: %+v %v", got, err)
	}
}
