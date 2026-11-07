package transaction

import (
	"context"
	"math"
	"math/big"
	"sync"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/signer"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

type fixture struct {
	mu        sync.Mutex
	nonce     uint64
	headers   map[uint64]*types.Header
	head      uint64
	receipts  map[common.Hash]*types.Receipt
	sends     []*types.Transaction
	onSend    func(*types.Transaction) error
	onNonce   func(uint64)
	estimate  func(context.Context) (uint64, error)
	nonceErr  error
	headerErr error
}

func newFixture() *fixture {
	f := &fixture{headers: make(map[uint64]*types.Header), receipts: make(map[common.Hash]*types.Receipt)}
	f.mine(3)
	return f
}
func (f *fixture) mine(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for range count {
		n := f.head + 1
		var parent common.Hash
		if h := f.headers[f.head]; h != nil {
			parent = h.Hash()
		}
		f.headers[n] = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: parent, GasLimit: 30_000_000, BaseFee: big.NewInt(1_000_000_000), Time: n * 12}
		f.head = n
	}
}
func (f *fixture) NonceAt(_ context.Context, _ common.Address, block *big.Int) (uint64, error) {
	if block != nil {
		return 0, errors.New("nonce must be read from mined latest state")
	}
	f.mu.Lock()
	nonce, hook, err := f.nonce, f.onNonce, f.nonceErr
	f.mu.Unlock()
	if hook != nil {
		hook(nonce)
	}
	return nonce, err
}
func (f *fixture) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.headerErr != nil {
		return nil, f.headerErr
	}
	x := f.head
	if n != nil {
		x = n.Uint64()
	}
	if h := f.headers[x]; h != nil {
		return types.CopyHeader(h), nil
	}
	return nil, ethereum.NotFound
}
func (f *fixture) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, h := range f.headers {
		if h.Hash() == hash {
			return types.CopyHeader(h), nil
		}
	}
	return nil, ethereum.NotFound
}
func (f *fixture) EstimateGas(ctx context.Context, _ ethereum.CallMsg) (uint64, error) {
	if f.estimate != nil {
		return f.estimate(ctx)
	}
	return 21_000, nil
}
func (f *fixture) SendTransaction(_ context.Context, tx *types.Transaction) error {
	f.mu.Lock()
	f.sends = append(f.sends, tx)
	fn := f.onSend
	f.mu.Unlock()
	if fn != nil {
		return fn(tx)
	}
	return nil
}
func (f *fixture) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.receipts[hash]; r != nil {
		copy := *r
		return &copy, nil
	}
	return nil, ethereum.NotFound
}
func (f *fixture) include(tx *types.Transaction, number uint64, status uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h := f.headers[number]
	f.receipts[tx.Hash()] = &types.Receipt{TxHash: tx.Hash(), BlockNumber: new(big.Int).SetUint64(number), BlockHash: h.Hash(), Status: status}
	f.nonce = tx.Nonce() + 1
}
func (f *fixture) sent() []*types.Transaction {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*types.Transaction(nil), f.sends...)
}

func config() Config {
	return Config{MaxFeeGwei: 100, PriorityFeeGwei: .1, GasHeadroomBps: 500, Confirmations: 2, PollInterval: time.Millisecond, ReadTimeout: 5 * time.Millisecond, EstimateTimeout: 5 * time.Millisecond, BroadcastTimeout: 5 * time.Millisecond}
}
func testSigner(t *testing.T) signer.Signer {
	t.Helper()
	s, err := signer.NewFromHexKey("59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func driver(t *testing.T, f *fixture) *Driver {
	t.Helper()
	d, err := New(f, f, testSigner(t), big.NewInt(31337), config())
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func intent() rt.Intent {
	return rt.Intent{ID: "job", To: common.HexToAddress("0x1234"), Data: hexutil.Bytes{1, 2, 3}}
}

func TestPrebroadcastFailuresReleaseSender(t *testing.T) {
	for _, kind := range []string{"intent", "canceled", "nonce", "estimate", "estimate timeout", "zero gas", "overflow gas", "fee limit", "header", "nonce changed during estimate", "nonce changed during signing", "signer"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture()
			signing := testSigner(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			i := intent()
			switch kind {
			case "intent":
				i.Data = nil
			case "canceled":
				cancel()
			case "nonce":
				f.nonceErr = errors.New("nonce unavailable")
			case "estimate":
				f.estimate = func(context.Context) (uint64, error) { return 0, errors.New("execution reverted") }
			case "estimate timeout":
				f.estimate = func(ctx context.Context) (uint64, error) { <-ctx.Done(); return 0, ctx.Err() }
			case "zero gas":
				f.estimate = func(context.Context) (uint64, error) { return 0, nil }
			case "overflow gas":
				f.estimate = func(context.Context) (uint64, error) { return math.MaxUint64, nil }
			case "fee limit":
				f.headers[f.head].BaseFee = gweiToWei(200)
			case "header":
				f.headerErr = errors.New("header unavailable")
			case "nonce changed during estimate":
				f.estimate = func(context.Context) (uint64, error) { f.nonce = 1; return 21_000, nil }
			case "nonce changed during signing":
				signing = hookSigner{Signer: signing, hook: func() { f.nonce = 1 }}
			case "signer":
				signing = hookSigner{Signer: signing, err: errors.New("signing unavailable")}
			}
			d, err := New(f, f, signing, big.NewInt(31337), config())
			if err != nil {
				t.Fatal(err)
			}
			r := d.Execute(ctx, i)
			if !r.NotSubmitted || r.Err == nil || r.Execution != nil || len(f.sent()) != 0 {
				t.Fatalf("prebroadcast result=%+v sends=%d", r, len(f.sent()))
			}
			d.mu.Lock()
			owned := d.owned
			d.mu.Unlock()
			if owned {
				t.Fatal("prebroadcast failure kept sender owned")
			}
		})
	}
}

type hookSigner struct {
	signer.Signer
	hook func()
	err  error
}

func (s hookSigner) SignTx(ctx context.Context, tx *types.Transaction, chain *big.Int) (*types.Transaction, error) {
	if s.hook != nil {
		s.hook()
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.Signer.SignTx(ctx, tx, chain)
}

func TestCanonicalSuccessAndRevertReleaseSender(t *testing.T) {
	for _, status := range []uint64{types.ReceiptStatusSuccessful, types.ReceiptStatusFailed} {
		t.Run(map[uint64]string{0: "revert", 1: "success"}[status], func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			f.onSend = func(tx *types.Transaction) error { f.include(tx, 1, status); return nil }
			for range 2 {
				r := d.Execute(t.Context(), intent())
				if r.Err != nil || r.NotSubmitted || r.Execution == nil || r.Reverted != (status == types.ReceiptStatusFailed) {
					t.Fatalf("canonical result=%+v", r)
				}
			}
			sent := f.sent()
			if len(sent) != 2 || sent[0].Nonce() != 0 || sent[1].Nonce() != 1 || sent[0].Gas() != 22_050 || sent[0].GasFeeCap().Cmp(gweiToWei(config().MaxFeeGwei)) != 0 || sent[0].GasTipCap().Cmp(gweiToWei(config().PriorityFeeGwei)) != 0 {
				t.Fatalf("unexpected nonce/gas/fee: %v", sent)
			}
		})
	}
}

func TestAmbiguousBroadcastObservesOneAttempt(t *testing.T) {
	for _, sendErr := range []error{context.DeadlineExceeded, errors.New("nonce too low"), errors.New("insufficient funds")} {
		t.Run(sendErr.Error(), func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			f.onSend = func(tx *types.Transaction) error { f.include(tx, 1, 1); return sendErr }
			r := d.Execute(t.Context(), intent())
			if r.Err != nil || r.NotSubmitted || r.Execution == nil || len(f.sent()) != 1 {
				t.Fatalf("ambiguous result=%+v sends=%d", r, len(f.sent()))
			}
		})
	}
}

func TestCancellationRetainsSenderAfterBroadcast(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.onSend = func(*types.Transaction) error { cancel(); return context.DeadlineExceeded }
	r := d.Execute(ctx, intent())
	if r.NotSubmitted || r.Execution != nil || !errors.Is(r.Err, ErrPending) || !errors.Is(r.Err, context.Canceled) || len(f.sent()) != 1 {
		t.Fatalf("cancel result=%+v sends=%d", r, len(f.sent()))
	}
	if next := d.Execute(t.Context(), intent()); !next.NotSubmitted || !errors.Is(next.Err, ErrReserved) || len(f.sent()) != 1 {
		t.Fatalf("cancellation released sender: %+v", next)
	}
}

func TestForeignMinedNonceKeepsObservingUntilCancellation(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sent := make(chan struct{})
	f.onSend = func(*types.Transaction) error {
		f.mu.Lock()
		f.nonce = 1
		f.mu.Unlock()
		close(sent)
		return errors.New("nonce too low")
	}
	done := make(chan rt.TransactionResult, 1)
	go func() { done <- d.Execute(ctx, intent()) }()
	<-sent
	select {
	case r := <-done:
		t.Fatalf("foreign nonce ended observation: %+v", r)
	case <-time.After(15 * time.Millisecond):
	}
	if next := d.Execute(t.Context(), intent()); !errors.Is(next.Err, ErrReserved) {
		t.Fatalf("foreign nonce released sender: %+v", next)
	}
	cancel()
	r := <-done
	if r.NotSubmitted || r.Execution != nil || !errors.Is(r.Err, ErrNonceConsumed) || len(f.sent()) != 1 {
		t.Fatalf("foreign nonce result=%+v", r)
	}
}

func TestCanonicalDepthAndReorgDoNotReleaseSender(t *testing.T) {
	for _, kind := range []string{"insufficient depth", "orphan receipt", "removed log", "wrong transaction"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			f.onSend = func(tx *types.Transaction) error {
				n := uint64(1)
				if kind == "insufficient depth" {
					n = 3
				}
				f.include(tx, n, 1)
				f.mu.Lock()
				defer f.mu.Unlock()
				r := f.receipts[tx.Hash()]
				switch kind {
				case "orphan receipt":
					r.BlockHash = common.HexToHash("0xdead")
				case "removed log":
					r.Logs = []*types.Log{{Removed: true, TxHash: tx.Hash(), BlockHash: r.BlockHash, BlockNumber: n}}
				case "wrong transaction":
					r.TxHash = common.HexToHash("0xbad")
				}
				return nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
			defer cancel()
			r := d.Execute(ctx, intent())
			if r.NotSubmitted || r.Execution != nil || !errors.Is(r.Err, ErrPending) || len(f.sent()) != 1 {
				t.Fatalf("invalid evidence result=%+v", r)
			}
		})
	}
}

func TestMinedNonceReceiptReadRaceStillFindsOwnedReceipt(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	var sent *types.Transaction
	f.onSend = func(tx *types.Transaction) error { sent = tx; f.mu.Lock(); f.nonce = 1; f.mu.Unlock(); return nil }
	f.onNonce = func(nonce uint64) {
		if nonce > 0 {
			f.include(sent, 1, 1)
		}
	}
	r := d.Execute(t.Context(), intent())
	if r.Err != nil || r.Execution == nil || r.Execution.TxHash != sent.Hash() || len(f.sent()) != 1 {
		t.Fatalf("late owned receipt result=%+v", r)
	}
}

func TestSenderFencesConcurrentPreparation(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	entered, unblock := make(chan struct{}), make(chan struct{})
	f.estimate = func(context.Context) (uint64, error) { close(entered); <-unblock; return 0, errors.New("stop") }
	done := make(chan rt.TransactionResult, 1)
	go func() { done <- d.Execute(t.Context(), intent()) }()
	<-entered
	r := d.Execute(t.Context(), intent())
	if !r.NotSubmitted || !errors.Is(r.Err, ErrReserved) || len(f.sent()) != 0 {
		t.Fatalf("concurrent preparation entered: %+v", r)
	}
	close(unblock)
	if r := <-done; !r.NotSubmitted {
		t.Fatalf("preparation result=%+v", r)
	}
}

func TestConfirmedExecutionRejectsLaggingWriteNonce(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	f.onSend = func(tx *types.Transaction) error { f.include(tx, 1, 1); return nil }
	if r := d.Execute(t.Context(), intent()); r.Err != nil {
		t.Fatal(r.Err)
	}
	f.mu.Lock()
	f.nonce = 0
	f.mu.Unlock()
	if r := d.Execute(t.Context(), intent()); !r.NotSubmitted || r.Err == nil || len(f.sent()) != 1 {
		t.Fatalf("lagging nonce reused: %+v", r)
	}
}
