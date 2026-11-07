package transaction

import (
	"bytes"
	"context"
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
	mu       sync.Mutex
	nonce    uint64
	headers  map[uint64]*types.Header
	head     uint64
	receipts map[common.Hash]*types.Receipt
	sends    []*types.Transaction
	onSend   func(*types.Transaction, int) error
	onNonce  func(uint64)
	estimate func(context.Context) (uint64, error)
	feeErr   bool
	base     int64
}

func newFixture() *fixture {
	f := &fixture{headers: make(map[uint64]*types.Header), receipts: make(map[common.Hash]*types.Receipt), base: 1_000_000_000}
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
		f.headers[n] = &types.Header{Number: new(big.Int).SetUint64(n), ParentHash: parent, GasLimit: 30_000_000, BaseFee: big.NewInt(f.base), Time: n * 12}
		f.head = n
	}
}
func (f *fixture) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	f.mu.Lock()
	nonce, hook := f.nonce, f.onNonce
	f.mu.Unlock()
	if hook != nil {
		hook(nonce)
	}
	return nonce, nil
}
func (f *fixture) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
func (f *fixture) FeeHistory(context.Context, uint64, *big.Int, []float64) (*ethereum.FeeHistory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.feeErr {
		return nil, errors.New("fee read unavailable")
	}
	return &ethereum.FeeHistory{OldestBlock: big.NewInt(int64(f.head) - 2), BaseFee: []*big.Int{big.NewInt(f.base), big.NewInt(f.base), big.NewInt(f.base), big.NewInt(f.base)}, GasUsedRatio: []float64{.5, .5, .5}, Reward: [][]*big.Int{{big.NewInt(1)}, {big.NewInt(1)}, {big.NewInt(1)}}}, nil
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
	n := len(f.sends)
	fn := f.onSend
	f.mu.Unlock()
	if fn != nil {
		return fn(tx, n)
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
	return Config{MaxFeeGwei: 100, Confirmations: 2, Horizon: policyConfig(), PollInterval: time.Millisecond, ReplacementInterval: 5 * time.Millisecond, PendingTimeout: 40 * time.Millisecond, ShutdownTimeout: 5 * time.Millisecond, ReadTimeout: 5 * time.Millisecond, EstimateTimeout: 5 * time.Millisecond, BroadcastTimeout: 5 * time.Millisecond}
}
func testSigner(t *testing.T) signer.Signer {
	t.Helper()
	s, e := signer.NewFromHexKey("59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func driver(t *testing.T, f *fixture) *Driver {
	t.Helper()
	d, e := New(f, f, testSigner(t), big.NewInt(31337), config())
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func intent() rt.Intent {
	return rt.Intent{ID: "job", To: common.HexToAddress("0x1234"), Data: hexutil.Bytes{1, 2, 3}}
}
func reserve(t *testing.T, d *Driver) {
	t.Helper()
	if err := d.Reserve(t.Context(), "job"); err != nil {
		t.Fatal(err)
	}
}
func persist(out *rt.Transaction) rt.PersistTransaction {
	return func(tx rt.Transaction) error { *out = tx; return nil }
}

func TestPersistenceFailurePreventsBroadcastAndRelease(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(map[int]string{1: "unsigned admission", 2: "signed original"}[failAt], func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			reserve(t, d)
			calls := 0
			result := d.Execute(t.Context(), intent(), func(rt.Transaction) error {
				calls++
				if calls == failAt {
					return errors.New("disk unavailable")
				}
				return nil
			})
			if result.Err == nil || len(f.sent()) != 0 {
				t.Fatalf("result=%+v sends=%d", result, len(f.sent()))
			}
			if err := d.Release(t.Context(), "job"); err == nil {
				t.Fatal("storage failure released signer")
			}
			if err := d.Reserve(t.Context(), "other"); err == nil {
				t.Fatal("storage failure admitted another job")
			}
		})
	}
}
func TestAdmissionIsPersistedBeforeEstimateAndUnsignedRecoveryIsSafe(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var saved rt.Transaction
	f.estimate = func(context.Context) (uint64, error) {
		if !saved.Admitted || saved.Nonce != 0 || len(saved.Attempts) != 0 {
			t.Error("estimate preceded durable admission")
		}
		return 0, errors.New("cannot simulate")
	}
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if !r.NotSubmitted || r.Pending || r.Err == nil || len(f.sent()) != 0 {
		t.Fatalf("unsigned failure = %+v", r)
	}
	fresh := driver(t, f)
	reserve(t, fresh)
	r = fresh.Resume(t.Context(), saved, persist(&saved))
	if !r.NotSubmitted || r.Pending || len(f.sent()) != 0 {
		t.Fatalf("unsigned resume = %+v", r)
	}
	if err := fresh.Release(t.Context(), "job"); err != nil {
		t.Fatal(err)
	}
}
func TestAmbiguousSendRetriesExactBytesBeforeReplacement(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var saved rt.Transaction
	f.onSend = func(tx *types.Transaction, n int) error {
		if len(saved.Attempts) == 0 {
			t.Error("broadcast preceded persistence")
		}
		if n == 1 {
			return context.DeadlineExceeded
		}
		if n == 2 {
			f.include(tx, 1, types.ReceiptStatusSuccessful)
		}
		return nil
	}
	r := d.Execute(t.Context(), intent(), persist(&saved))
	sent := f.sent()
	if r.Err != nil || r.Execution == nil || len(sent) != 2 || sent[0].Hash() != sent[1].Hash() || len(saved.Attempts) != 1 {
		t.Fatalf("result=%+v sends=%d attempts=%d", r, len(sent), len(saved.Attempts))
	}
	if saved.Terminal != "confirmed" || saved.Execution == nil {
		t.Fatal("canonical evidence not persisted")
	}
	if err := d.Release(t.Context(), "job"); err != nil {
		t.Fatal(err)
	}
}
func TestReplacementPersistsSameNoncePayloadAndOlderVariantWins(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var saved rt.Transaction
	var first *types.Transaction
	f.onSend = func(tx *types.Transaction, n int) error {
		if len(saved.Attempts) != n {
			t.Errorf("send %d has %d persisted attempts", n, len(saved.Attempts))
		}
		if n == 1 {
			first = tx
			f.mu.Lock()
			f.base = 10_000_000_000
			f.mu.Unlock()
			f.mine(1)
		} else {
			if tx.Nonce() != first.Nonce() || *tx.To() != *first.To() || !bytes.Equal(tx.Data(), first.Data()) || tx.Value().Sign() != 0 {
				t.Error("replacement mutated intent")
			}
			if tx.GasFeeCap().Cmp(bumpFee(first.GasFeeCap())) < 0 || tx.GasTipCap().Cmp(bumpFee(first.GasTipCap())) < 0 {
				t.Error("replacement fee bump too small")
			}
			f.include(first, 1, types.ReceiptStatusSuccessful)
		}
		return nil
	}
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if r.Err != nil || r.Execution == nil || r.Execution.TxHash != first.Hash() || len(saved.Attempts) != 2 {
		t.Fatalf("older variant result=%+v attempts=%d", r, len(saved.Attempts))
	}
}
func signedRecord(t *testing.T, d *Driver) rt.Transaction {
	t.Helper()
	var tx rt.Transaction
	r := d.Execute(t.Context(), intent(), persist(&tx))
	if !r.Pending || len(tx.Attempts) == 0 {
		t.Fatalf("pending setup = %+v", r)
	}
	return tx
}
func TestSignedResumeObservesWithoutSigningOrBroadcast(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	tx := signedRecord(t, d)
	before := len(f.sent())
	fresh := driver(t, f)
	reserve(t, fresh)
	r := fresh.Resume(t.Context(), tx, persist(&tx))
	if !r.Pending || len(f.sent()) != before {
		t.Fatalf("resume replayed: %+v", r)
	}
	if err := fresh.Release(t.Context(), "job"); err == nil {
		t.Fatal("pending signed record released")
	}
	if err := fresh.Reserve(t.Context(), "another"); err == nil {
		t.Fatal("pending ownership not fenced")
	}
}
func TestForeignMinedNonceRetainsOwnership(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var saved rt.Transaction
	f.onSend = func(*types.Transaction, int) error {
		f.mu.Lock()
		f.nonce = 1
		f.mu.Unlock()
		return errors.New("nonce too low")
	}
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if !r.Pending || r.Execution != nil || !errors.Is(r.Err, ErrNonceConsumed) || len(f.sent()) != 1 {
		t.Fatalf("foreign nonce=%+v sends=%d", r, len(f.sent()))
	}
	if err := d.Release(t.Context(), "job"); err == nil {
		t.Fatal("foreign consumption released owned nonce")
	}
}
func TestCanonicalDepthAndReorgDoNotRelease(t *testing.T) {
	for _, kind := range []string{"insufficient depth", "orphan receipt", "removed log"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			reserve(t, d)
			var saved rt.Transaction
			f.onSend = func(tx *types.Transaction, _ int) error {
				n := uint64(1)
				if kind == "insufficient depth" {
					n = 3
				}
				f.include(tx, n, 1)
				if kind == "orphan receipt" {
					f.mu.Lock()
					f.receipts[tx.Hash()].BlockHash = common.HexToHash("0xdead")
					f.mu.Unlock()
				}
				if kind == "removed log" {
					f.mu.Lock()
					r := f.receipts[tx.Hash()]
					r.Logs = []*types.Log{{Removed: true, TxHash: tx.Hash(), BlockHash: r.BlockHash, BlockNumber: n}}
					f.mu.Unlock()
				}
				return nil
			}
			r := d.Execute(t.Context(), intent(), persist(&saved))
			if !r.Pending || r.Execution != nil || saved.Terminal != "" {
				t.Fatalf("invalid evidence accepted: %+v", r)
			}
		})
	}
}
func TestReceiptPersistenceFailureBlocksRelease(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	f.onSend = func(tx *types.Transaction, _ int) error { f.include(tx, 1, 1); return nil }
	r := d.Execute(t.Context(), intent(), func(tx rt.Transaction) error {
		if tx.Execution != nil {
			return errors.New("fsync failed")
		}
		return nil
	})
	if !r.Pending || r.Execution != nil || r.Err == nil {
		t.Fatalf("unpersisted evidence=%+v", r)
	}
	if err := d.Release(t.Context(), "job"); err == nil {
		t.Fatal("receipt persistence failure released")
	}
}
func TestEstimateAndShutdownAreBounded(t *testing.T) {
	for _, kind := range []string{"estimate", "drain"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			reserve(t, d)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if kind == "estimate" {
				f.estimate = func(ctx context.Context) (uint64, error) { <-ctx.Done(); return 0, ctx.Err() }
			} else {
				f.onSend = func(*types.Transaction, int) error { cancel(); return nil }
			}
			start := time.Now()
			var saved rt.Transaction
			r := d.Execute(ctx, intent(), persist(&saved))
			if time.Since(start) > time.Second {
				t.Fatal("unbounded operation")
			}
			if r.Err == nil {
				t.Fatalf("expected bound result: %+v", r)
			}
			if kind == "estimate" && !r.NotSubmitted {
				t.Fatalf("estimate signed unexpectedly: %+v", r)
			}
			if kind == "drain" && (!r.Pending || len(saved.Attempts) != 1) {
				t.Fatalf("cancel released ownership: %+v", r)
			}
		})
	}
}
func TestReserveFencesConcurrentJobsAndObservers(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	entered := make(chan struct{})
	unblock := make(chan struct{})
	f.estimate = func(context.Context) (uint64, error) { close(entered); <-unblock; return 0, errors.New("stop") }
	done := make(chan rt.TransactionResult, 1)
	go func() { done <- d.Execute(t.Context(), intent(), func(rt.Transaction) error { return nil }) }()
	<-entered
	if err := d.Reserve(t.Context(), "other"); err == nil {
		t.Fatal("concurrent job acquired signer")
	}
	r := d.Execute(t.Context(), intent(), func(rt.Transaction) error { return nil })
	if !r.Pending || r.Err == nil {
		t.Fatalf("second observer entered: %+v", r)
	}
	if err := d.Release(t.Context(), "job"); err == nil {
		t.Fatal("active job released")
	}
	close(unblock)
	<-done
}

type alteringSigner struct{ signer.Signer }

func (s alteringSigner) SignTx(ctx context.Context, tx *types.Transaction, chain *big.Int) (*types.Transaction, error) {
	// A host signer that rewrites its argument in place must still be rejected.
	changed := types.NewTx(&types.DynamicFeeTx{ChainID: chain, Nonce: tx.Nonce(), To: tx.To(), Data: tx.Data(), Value: tx.Value(), Gas: tx.Gas() + 1, GasFeeCap: tx.GasFeeCap(), GasTipCap: tx.GasTipCap()})
	raw, _ := changed.MarshalBinary()
	if err := tx.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return s.Signer.SignTx(ctx, tx, chain)
}
func TestSignerCannotChangeQuotedGasInPlace(t *testing.T) {
	f := newFixture()
	d, err := New(f, f, alteringSigner{testSigner(t)}, big.NewInt(31337), config())
	if err != nil {
		t.Fatal(err)
	}
	reserve(t, d)
	var saved rt.Transaction
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if !r.NotSubmitted || r.Err == nil || len(saved.Attempts) != 0 || len(f.sent()) != 0 {
		t.Fatalf("signer changed gas: %+v attempts=%d sends=%d", r, len(saved.Attempts), len(f.sent()))
	}
}
func TestInitialEstimateCannotOutliveMinedNonce(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	f.estimate = func(context.Context) (uint64, error) { f.mu.Lock(); f.nonce = 1; f.mu.Unlock(); return 21_000, nil }
	var saved rt.Transaction
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if !r.NotSubmitted || !errors.Is(r.Err, ErrNonceConsumed) || len(saved.Attempts) != 0 || len(f.sent()) != 0 {
		t.Fatalf("signed obsolete nonce after estimate: %+v attempts=%d sends=%d", r, len(saved.Attempts), len(f.sent()))
	}
}
func TestResumeTerminalReceiptMustBeReverifiedBeforeRelease(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var saved rt.Transaction
	f.onSend = func(tx *types.Transaction, _ int) error { f.include(tx, 1, 1); return nil }
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if r.Execution == nil {
		t.Fatal(r.Err)
	}
	f.mu.Lock()
	f.receipts = make(map[common.Hash]*types.Receipt)
	f.mu.Unlock()
	fresh := driver(t, f)
	reserve(t, fresh)
	r = fresh.Resume(t.Context(), saved, persist(&saved))
	if !r.Pending || r.Execution != nil {
		t.Fatalf("reorg recovery=%+v", r)
	}
	if err := fresh.Release(t.Context(), "job"); err == nil {
		t.Fatal("unverified restored terminal released signer")
	}
}

func TestMinedNonceReceiptReadRaceStillFindsOwnedReceipt(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	var sent *types.Transaction
	f.onSend = func(tx *types.Transaction, _ int) error {
		sent = tx
		f.mu.Lock()
		f.nonce = 1
		f.mu.Unlock()
		return nil
	}
	f.onNonce = func(nonce uint64) {
		if nonce > 0 {
			f.include(sent, 1, 1)
		}
	}
	var saved rt.Transaction
	r := d.Execute(t.Context(), intent(), persist(&saved))
	if r.Err != nil || r.Execution == nil || r.Execution.TxHash != sent.Hash() {
		t.Fatalf("receipt became available after nonce read: %+v", r)
	}
}

type nextBlockFixture struct {
	*fixture
	nextErr                error
	nextCalls, latestCalls int
}

func (f *nextBlockFixture) EstimateGasNextBlock(_ context.Context, call ethereum.CallMsg, parent *types.Header, slot time.Duration) (uint64, error) {
	f.nextCalls++
	if call.To == nil || *call.To != intent().To || !bytes.Equal(call.Data, intent().Data) || parent.Number.Uint64() != 3 || slot != 12*time.Second {
		return 0, errors.New("lost next-block estimate identity")
	}
	return 30_000, f.nextErr
}
func (f *nextBlockFixture) EstimateGas(ctx context.Context, call ethereum.CallMsg) (uint64, error) {
	f.latestCalls++
	return f.fixture.EstimateGas(ctx, call)
}
func TestNextBlockGasFallbackOnlyForUnsupportedOverrides(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		gas      uint64
		fallback int
	}{{"next block", nil, 31_500, 0}, {"unsupported", testRPCError{code: -32602, message: "invalid params"}, 23_100, 1}, {"revert", testRPCError{code: 3, message: "execution reverted"}, 0, 0}, {"transport", context.DeadlineExceeded, 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &nextBlockFixture{fixture: newFixture(), nextErr: tc.err}
			d, err := New(f, f.fixture, testSigner(t), big.NewInt(31337), config())
			if err != nil {
				t.Fatal(err)
			}
			reserve(t, d)
			f.onSend = func(tx *types.Transaction, _ int) error { f.include(tx, 1, 1); return nil }
			var saved rt.Transaction
			r := d.Execute(t.Context(), intent(), persist(&saved))
			sent := f.sent()
			if tc.gas == 0 {
				if !r.NotSubmitted || r.Err == nil || len(sent) != 0 {
					t.Fatalf("failed estimate signed: %+v", r)
				}
			} else if r.Err != nil || len(sent) != 1 || sent[0].Gas() != tc.gas {
				t.Fatalf("gas result=%+v sends=%v", r, sent)
			}
			if f.nextCalls != 1 || f.latestCalls != tc.fallback {
				t.Fatalf("next=%d latest=%d", f.nextCalls, f.latestCalls)
			}
		})
	}
}
func TestFeeCeilingAndUnavailableWindowReplacement(t *testing.T) {
	for _, kind := range []string{"ceiling blocks admission", "unavailable fees bump", "replacement persistence blocks send"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			reserve(t, d)
			var saved rt.Transaction
			var first *types.Transaction
			if kind == "ceiling blocks admission" {
				f.base = 200_000_000_000
			} else {
				f.onSend = func(tx *types.Transaction, n int) error {
					if n == 1 {
						first = tx
						f.mu.Lock()
						f.feeErr = true
						f.mu.Unlock()
					} else {
						f.include(first, 1, 1)
					}
					return nil
				}
			}
			r := d.Execute(t.Context(), intent(), func(tx rt.Transaction) error {
				if kind == "replacement persistence blocks send" && len(tx.Attempts) == 2 {
					return errors.New("replacement fsync failed")
				}
				saved = tx
				return nil
			})
			sent := f.sent()
			switch kind {
			case "ceiling blocks admission":
				if !r.NotSubmitted || !errors.Is(r.Err, ErrFeeLimitReached) || len(sent) != 0 {
					t.Fatalf("cap result=%+v sends=%d", r, len(sent))
				}
			case "unavailable fees bump":
				if r.Err != nil || r.Execution == nil || len(sent) != 2 || sent[1].GasFeeCap().Cmp(bumpFee(sent[0].GasFeeCap())) != 0 {
					t.Fatalf("fallback=%+v sends=%d", r, len(sent))
				}
			case "replacement persistence blocks send":
				if !r.Pending || r.Err == nil || len(sent) != 1 || len(saved.Attempts) != 1 {
					t.Fatalf("replacement persisted late: %+v sends=%d", r, len(sent))
				}
				if err := d.Release(t.Context(), "job"); err == nil {
					t.Fatal("replacement storage failure released")
				}
			}
		})
	}
}
func TestSignedPersistenceFailureIsNeverUnsignedSafe(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	reserve(t, d)
	r := d.Execute(t.Context(), intent(), func(tx rt.Transaction) error {
		if len(tx.Attempts) > 0 {
			return errors.New("rename may already have completed")
		}
		return nil
	})
	if !r.Pending || r.NotSubmitted || r.Err == nil || len(f.sent()) != 0 {
		t.Fatalf("signed uncertain journal=%+v", r)
	}
}

func TestStallKeepsGasWhenRawEstimateStillFitsSignedLimit(t *testing.T) {
	f := newFixture()
	cfg := config()
	cfg.Horizon.BlockTime = time.Millisecond
	cfg.PendingTimeout = 200 * time.Millisecond
	d, err := New(f, f, testSigner(t), big.NewInt(31337), cfg)
	if err != nil {
		t.Fatal(err)
	}
	reserve(t, d)
	calls := 0
	f.estimate = func(context.Context) (uint64, error) {
		calls++
		if calls == 1 {
			return 21_000, nil
		}
		return 22_000, nil
	}
	var first *types.Transaction
	f.onSend = func(tx *types.Transaction, n int) error {
		if n == 1 {
			first = tx
			go func() { time.Sleep(6 * time.Millisecond); f.mine(3) }()
		} else {
			f.include(first, 1, 1)
		}
		return nil
	}
	var saved rt.Transaction
	r := d.Execute(t.Context(), intent(), persist(&saved))
	sent := f.sent()
	if r.Err != nil || len(sent) != 2 || sent[0].Hash() != sent[1].Hash() || len(saved.Attempts) != 1 {
		t.Fatalf("unnecessary gas replacement: %+v sends=%d attempts=%d", r, len(sent), len(saved.Attempts))
	}
}
