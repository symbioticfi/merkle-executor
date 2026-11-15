package transaction

import (
	"context"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

// This fixture models only an account and its canonical chain. It deliberately
// does not reuse the manager's lifecycle implementation or another host's mocks.
type practicalChain struct {
	mu       sync.Mutex
	nonce    uint64
	pending  uint64
	head     uint64
	headers  map[uint64]*types.Header
	receipts map[common.Hash]*types.Receipt
	sent     []*types.Transaction
	funding  func(context.Context) error
}

func newPracticalChain() *practicalChain {
	b := &practicalChain{headers: map[uint64]*types.Header{}, receipts: map[common.Hash]*types.Receipt{}}
	b.headers[0] = &types.Header{Number: new(big.Int), GasLimit: 30_000_000, BaseFee: big.NewInt(20e9)}
	return b
}

func (b *practicalChain) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nonce, nil
}
func (b *practicalChain) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return max(b.nonce, b.pending), nil
}
func (b *practicalChain) TransactionSenderBalanceAt(ctx context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	if b.funding != nil {
		if err := b.funding(ctx); err != nil {
			return nil, err
		}
	}
	return new(big.Int).Lsh(big.NewInt(1), 128), nil
}
func (b *practicalChain) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.head
	if number != nil {
		n = number.Uint64()
	}
	if header := b.headers[n]; header != nil {
		return types.CopyHeader(header), nil
	}
	return nil, ethereum.NotFound
}
func (b *practicalChain) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, header := range b.headers {
		if header.Hash() == hash {
			return types.CopyHeader(header), nil
		}
	}
	return nil, ethereum.NotFound
}
func (b *practicalChain) FeeHistory(_ context.Context, count uint64, newest *big.Int, _ []float64) (*ethereum.FeeHistory, error) {
	end := uint64(0)
	if newest != nil {
		end = newest.Uint64()
	}
	history := &ethereum.FeeHistory{OldestBlock: new(big.Int).SetUint64(end + 1 - min(count, end+1))}
	for range count {
		history.Reward = append(history.Reward, []*big.Int{big.NewInt(1e9)})
		history.BaseFee = append(history.BaseFee, big.NewInt(20e9))
		history.GasUsedRatio = append(history.GasUsedRatio, 0.5)
	}
	history.BaseFee = append(history.BaseFee, big.NewInt(20e9))
	return history, nil
}
func (*practicalChain) EstimateGas(context.Context, ethereum.CallMsg) (uint64, error) {
	return 21_000, nil
}
func (b *practicalChain) SendTransaction(_ context.Context, tx *types.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent = append(b.sent, tx)
	b.pending = max(b.pending, tx.Nonce()+1)
	return nil
}
func (b *practicalChain) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if receipt := b.receipts[hash]; receipt != nil {
		copy := *receipt
		copy.BlockNumber = new(big.Int).Set(receipt.BlockNumber)
		return &copy, nil
	}
	return nil, ethereum.NotFound
}
func (b *practicalChain) advanceLocked(extra string) *types.Header {
	parent := b.headers[b.head]
	b.head++
	header := &types.Header{Number: new(big.Int).SetUint64(b.head), ParentHash: parent.Hash(), GasLimit: 30_000_000, BaseFee: big.NewInt(20e9), Time: b.head * 12, Extra: []byte(extra)}
	b.headers[b.head] = header
	return header
}
func (b *practicalChain) mine(tx *types.Transaction, status, depth uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	header := b.advanceLocked("")
	b.receipts[tx.Hash()] = &types.Receipt{TxHash: tx.Hash(), Status: status, BlockNumber: new(big.Int).Set(header.Number), BlockHash: header.Hash(), GasUsed: 21_000, EffectiveGasPrice: big.NewInt(21e9)}
	b.nonce = max(b.nonce, tx.Nonce()+1)
	for range depth {
		b.advanceLocked("")
	}
}
func (b *practicalChain) orphan(hash common.Hash) {
	b.mu.Lock()
	defer b.mu.Unlock()
	receipt := b.receipts[hash]
	for n := receipt.BlockNumber.Uint64(); n <= b.head; n++ {
		header := types.CopyHeader(b.headers[n])
		header.Extra = []byte("replacement fork")
		if n > 0 {
			header.ParentHash = b.headers[n-1].Hash()
		}
		b.headers[n] = header
	}
}
func (b *practicalChain) attempts() []*types.Transaction {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*types.Transaction(nil), b.sent...)
}

type practicalSigner struct {
	signer.Signer
	calls atomic.Int64
	sign  func(context.Context, *types.Transaction, *big.Int) (*types.Transaction, error)
}

func (s *practicalSigner) SignTx(ctx context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	s.calls.Add(1)
	if s.sign != nil {
		return s.sign(ctx, tx, chainID)
	}
	return s.Signer.SignTx(ctx, tx, chainID)
}
func practicalSigning(t *testing.T, digit string) signer.Signer {
	t.Helper()
	s, err := signer.NewFromHexKey("000000000000000000000000000000000000000000000000000000000000000" + digit)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func practicalManager(t *testing.T, chain *practicalChain, signing signer.Signer, depth uint64) *Manager {
	t.Helper()
	return practicalManagerConfig(t, chain, signing, Config{MaxFeeGwei: 100, Confirmations: depth})
}
func practicalManagerConfig(t *testing.T, chain *practicalChain, signing signer.Signer, cfg Config) *Manager {
	t.Helper()
	m, err := New(chain, signing, big.NewInt(1), cfg, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()
	t.Cleanup(func() { stop(); <-done })
	return m
}
func practicalAttempt(t *testing.T, chain *practicalChain, count int) *types.Transaction {
	t.Helper()
	for range 10_000 {
		if attempts := chain.attempts(); len(attempts) >= count {
			return attempts[count-1]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("transaction never reached the fixture")
	return nil
}
func practicalDifferentAttempt(t *testing.T, chain *practicalChain, previous common.Hash) *types.Transaction {
	t.Helper()
	for range 10_000 {
		for _, tx := range chain.attempts() {
			if tx.Hash() != previous {
				return tx
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("different transaction never reached the fixture")
	return nil
}
func practicalExecution(t *testing.T, sender *Sender) *ExecutionResult {
	t.Helper()
	for range 10_000 {
		result, _ := sender.Poll(t.Context())
		if result != nil {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("canonical execution never completed")
	return nil
}

func TestPracticalOlderWinnerNeverCreditsFreshCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m := practicalManager(t, chain, practicalSigning(t, "1"), 0)
		oldResults, accepted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Data: []byte{1}, Deadline: time.Now().Add(20 * time.Millisecond)})
		if !accepted {
			t.Fatal("old work was not admitted")
		}
		old := practicalAttempt(t, chain, 1)
		oldResult := <-oldResults
		if !oldResult.Uncertain() || oldResult.Receipt != nil {
			t.Fatalf("expired signed call lost its unknown result: %+v", oldResult)
		}
		freshResults, accepted := m.SendAsync(t.Context(), Request{To: common.Address{2}, Data: []byte{2}, Deadline: time.Now().Add(time.Minute)})
		if !accepted {
			t.Fatal("fresh work was not admitted after expiry")
		}
		fresh := practicalAttempt(t, chain, 2)
		if fresh.Nonce() != old.Nonce() || fresh.Hash() == old.Hash() {
			t.Fatal("fresh work skipped unresolved nonce or reused old business bytes")
		}
		chain.mine(old, types.ReceiptStatusSuccessful, 0)
		freshResult := <-freshResults
		if freshResult.Included() || freshResult.Receipt != nil || !freshResult.Uncertain() || !errors.Is(freshResult.Err, ErrNonceConsumed) {
			t.Fatalf("older winner credited a different business call: %+v", freshResult)
		}
	})
}

func TestPracticalInitialFundingExpiryNeverSigns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		chain.funding = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
		signing := &practicalSigner{Signer: practicalSigning(t, "1")}
		m := practicalManager(t, chain, signing, 0)
		sender := m.Sender("claim")
		operation, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		err := sender.Submit(operation, ExecutionRequest{To: common.Address{1}})
		if !errors.Is(err, context.DeadlineExceeded) || signing.calls.Load() != 0 || sender.Busy() || len(chain.attempts()) != 0 {
			t.Fatalf("expired unsigned funding produced owned work: err=%v signs=%d busy=%v", err, signing.calls.Load(), sender.Busy())
		}
	})
}

func TestPracticalInitialFundingCancellationNeverSigns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		funding := make(chan struct{})
		chain.funding = func(ctx context.Context) error {
			close(funding)
			<-ctx.Done()
			return ctx.Err()
		}
		signing := &practicalSigner{Signer: practicalSigning(t, "1")}
		m := practicalManager(t, chain, signing, 0)
		sender := m.Sender("claim")
		operation, cancel := context.WithCancel(t.Context())
		defer cancel()
		result := make(chan error, 1)
		go func() { result <- sender.Submit(operation, ExecutionRequest{To: common.Address{1}}) }()
		<-funding
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) || signing.calls.Load() != 0 || sender.Busy() || len(chain.attempts()) != 0 {
			t.Fatalf("canceled unsigned funding became owned work: err=%v signs=%d busy=%v", err, signing.calls.Load(), sender.Busy())
		}
	})
}

func TestPracticalSigningExpiryRetainsExactIdentity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		local := practicalSigning(t, "1")
		var signed *types.Transaction
		signing := &practicalSigner{Signer: local}
		signing.sign = func(_ context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
			var err error
			signed, err = local.SignTx(context.Background(), tx, id)
			time.Sleep(20 * time.Millisecond)
			return signed, err
		}
		m := practicalManager(t, chain, signing, 0)
		sender := m.Sender("claim")
		operation, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		_ = sender.Submit(operation, ExecutionRequest{To: common.Address{1}, Data: []byte{1}})
		if signed == nil || !sender.Busy() || len(chain.attempts()) != 0 {
			t.Fatal("signing expiry forgot identity or broadcast expired initial work")
		}
		chain.mine(signed, types.ReceiptStatusSuccessful, 0)
		result := practicalExecution(t, sender)
		if result.Hash != signed.Hash() || sender.Busy() {
			t.Fatalf("retained signature did not resolve at its exact hash: %+v", result)
		}
	})
}

func TestPracticalCallerCancellationAfterCaptureKeepsOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		local := practicalSigning(t, "1")
		var signed *types.Transaction
		signing := &practicalSigner{Signer: local}
		signing.sign = func(ctx context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
			var err error
			signed, err = local.SignTx(ctx, tx, id)
			return signed, err
		}
		m := practicalManager(t, chain, signing, 0)
		sender := m.Sender("claim")
		operation, cancel := context.WithCancel(t.Context())
		if err := sender.Submit(operation, ExecutionRequest{To: common.Address{1}}); err != nil {
			t.Fatal(err)
		}
		cancel()
		if signed == nil || !sender.Busy() {
			t.Fatal("caller cancellation cleared signed ownership")
		}
		chain.mine(signed, types.ReceiptStatusSuccessful, 0)
		result := practicalExecution(t, sender)
		if result.Hash != signed.Hash() || sender.Busy() {
			t.Fatalf("caller cancellation prevented canonical completion: %+v", result)
		}
	})
}

func TestPracticalWrongSignerOrPayloadCannotBecomeOwned(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "wrong signer", true: "changed payload"}[changed], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				chain := newPracticalChain()
				local, other := practicalSigning(t, "1"), practicalSigning(t, "2")
				signing := &practicalSigner{Signer: local}
				signing.sign = func(ctx context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
					if changed {
						modified := types.NewTx(&types.DynamicFeeTx{ChainID: id, Nonce: tx.Nonce() + 1, To: tx.To(), Data: tx.Data(), Value: tx.Value(), Gas: tx.Gas(), GasTipCap: tx.GasTipCap(), GasFeeCap: tx.GasFeeCap()})
						return local.SignTx(ctx, modified, id)
					}
					return other.SignTx(ctx, tx, id)
				}
				m := practicalManager(t, chain, signing, 0)
				sender := m.Sender("claim")
				if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err == nil || sender.Busy() || len(chain.attempts()) != 0 {
					t.Fatal("invalid signer output became a signed lifecycle", err)
				}
			})
		})
	}
}

func TestPracticalCanonicalRevertWaitsForDepthAfterReorg(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m := practicalManager(t, chain, practicalSigning(t, "1"), 2)
		sender := m.Sender("claim")
		if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
			t.Fatal(err)
		}
		tx := practicalAttempt(t, chain, 1)
		chain.mine(tx, types.ReceiptStatusFailed, 1)
		time.Sleep(3 * time.Second)
		if result, _ := sender.Poll(t.Context()); result != nil || !sender.Busy() {
			t.Fatal("shallow revert released signed ownership")
		}
		chain.orphan(tx.Hash())
		chain.mu.Lock()
		chain.advanceLocked("canonical fork") // Enough depth, but the stored receipt is on the orphan.
		chain.mu.Unlock()
		time.Sleep(3 * time.Second)
		if result, _ := sender.Poll(t.Context()); result != nil || !sender.Busy() {
			t.Fatal("orphaned revert released signed ownership")
		}
		chain.mu.Lock()
		delete(chain.receipts, tx.Hash())
		chain.mu.Unlock()
		time.Sleep(3 * time.Second)
		if result, _ := sender.Poll(t.Context()); result != nil || !sender.Busy() {
			t.Fatal("missing receipt released signed ownership after nonce advancement")
		}
		chain.mine(tx, types.ReceiptStatusFailed, 2)
		result := practicalExecution(t, sender)
		if result.Hash != tx.Hash() || !result.Reverted || result.Execution == nil || sender.Busy() {
			t.Fatalf("canonical deep revert did not complete: %+v", result)
		}
	})
}

func TestPracticalReplacementDeadlineCapturesWithoutBroadcast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		local := practicalSigning(t, "1")
		signing := &practicalSigner{Signer: local}
		signing.sign = func(ctx context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
			if signing.calls.Load() > 1 {
				// A signer may finish a signature even after cancellation.
				time.Sleep(30 * time.Millisecond)
				ctx = context.Background()
			}
			return local.SignTx(ctx, tx, id)
		}
		m := practicalManagerConfig(t, chain, signing, Config{
			MaxFeeGwei: 100, PollInterval: time.Millisecond,
			ReplacementInterval: 10 * time.Millisecond, ShutdownTimeout: time.Millisecond,
		})
		results, admitted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Deadline: time.Now().Add(25 * time.Millisecond)})
		if !admitted {
			t.Fatal("request was not admitted")
		}
		practicalAttempt(t, chain, 1)
		chain.mu.Lock()
		chain.advanceLocked("replacement block")
		chain.mu.Unlock()
		result := <-results
		lateBroadcast := false
		for _, tx := range chain.attempts() {
			lateBroadcast = lateBroadcast || tx.Hash() == result.Hash
		}
		if signing.calls.Load() != 2 || len(result.Attempts) != 2 || lateBroadcast || !result.Uncertain() || !errors.Is(result.Err, context.DeadlineExceeded) {
			t.Fatalf("late replacement lost capture or broadcast after deadline: signs=%d broadcasts=%d result=%+v", signing.calls.Load(), len(chain.attempts()), result)
		}
	})
}

func TestPracticalNonceAdvanceDoesNotPreventNativeExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m := practicalManagerConfig(t, chain, practicalSigning(t, "1"), Config{
			MaxFeeGwei: 100, Confirmations: 2, PollInterval: time.Millisecond,
			ShutdownTimeout: time.Millisecond,
		})
		results, admitted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Deadline: time.Now().Add(10 * time.Millisecond)})
		if !admitted {
			t.Fatal("request was not admitted")
		}
		tx := practicalAttempt(t, chain, 1)
		chain.mine(tx, types.ReceiptStatusSuccessful, 0)
		time.Sleep(15 * time.Millisecond)
		select {
		case result := <-results:
			if !result.Uncertain() || result.Included() || !errors.Is(result.Err, context.DeadlineExceeded) || !m.LaneReady() {
				t.Fatalf("unconfirmed nonce advance credited or retained expired native work: %+v", result)
			}
		default:
			t.Fatal("unconfirmed nonce advance kept expired native work active")
		}
		// Its exact hash must remain owned despite releasing the business work.
		chain.mine(tx, types.ReceiptStatusSuccessful, 2)
		time.Sleep(2 * time.Millisecond)
		fresh, admitted := m.SendAsync(t.Context(), Request{To: common.Address{2}, Deadline: time.Now().Add(time.Minute)})
		if !admitted {
			t.Fatal("fresh native work was not admitted after old receipt confirmed")
		}
		next := practicalDifferentAttempt(t, chain, tx.Hash())
		if next.Nonce() != tx.Nonce()+1 {
			t.Fatal("fresh work did not advance after exact old receipt confirmation")
		}
		chain.mine(next, types.ReceiptStatusSuccessful, 2)
		if result := <-fresh; !result.Included() || result.Hash != next.Hash() {
			t.Fatalf("fresh operation did not own its confirmed hash: %+v", result)
		}
	})
}
