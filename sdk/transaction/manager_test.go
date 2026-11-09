package transaction

import (
	"context"
	"math"
	"math/big"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
)

type admissionBackend struct {
	*practicalChain
	gas           uint64
	balance       *big.Int
	balanceErr    error
	gasBudget     uint64
	afterEstimate func()
}

func (b *admissionBackend) EstimateGas(_ context.Context, msg ethereum.CallMsg) (uint64, error) {
	b.gasBudget = msg.Gas
	if b.afterEstimate != nil {
		b.afterEstimate()
	}
	return b.gas, nil
}
func (b *admissionBackend) TransactionSenderBalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error) {
	return b.balance, b.balanceErr
}
func startAdmissionManager(t *testing.T, b Backend, cfg Config) (*Manager, *practicalSigner) {
	t.Helper()
	cfg.PollInterval = time.Millisecond
	cfg.ShutdownTimeout = time.Millisecond
	s := &practicalSigner{Signer: practicalSigning(t, "1")}
	m, err := New(b, s, big.NewInt(1), cfg, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.Start(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	return m, s
}

func TestPassiveConfigurationAndStartup(t *testing.T) {
	for _, fee := range []float64{-1, math.NaN(), math.Inf(1), 1e-20} {
		if _, err := New(newPracticalChain(), practicalSigning(t, "1"), big.NewInt(1), Config{MaxFeeGwei: fee}, logr.Discard()); err == nil {
			t.Fatalf("invalid fee %v accepted", fee)
		}
	}
	m, err := New(newPracticalChain(), practicalSigning(t, "1"), big.NewInt(1), Config{}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if m.Available() || m.LaneReady() {
		t.Fatal("uninitialized manager advertised readiness")
	}
	if err = m.Initialize(t.Context()); !errors.Is(err, ErrFeeLimitReached) {
		t.Fatalf("unconfigured spending initialized: %v", err)
	}
	if _, err = m.MaxFeePerGas(t.Context()); !errors.Is(err, ErrFeeLimitReached) {
		t.Fatalf("unconfigured spending quoted: %v", err)
	}
	result := m.Send(t.Context(), Request{To: common.Address{1}})
	if !result.NotAdmitted || result.Err == nil {
		t.Fatalf("uninitialized request admitted: %+v", result)
	}
}

func TestGasBudgetBoundsEstimationAndHeadroom(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		estimate, block, budget, want uint64
	}{
		{"default headroom", 15_521_169, 30_000_000, 0, DefaultMaxGas},
		{"default estimate overflow", DefaultMaxGas + 1, 30_000_000, 0, 0},
		{"block headroom", 15_521_169, 16_000_000, 0, 16_000_000},
		{"block estimate overflow", 16_000_001, 16_000_000, 0, 0},
		{"configured headroom", 21_000, 30_000_000, 22_500, 22_500},
		{"configured estimate overflow", 25_001, 30_000_000, 25_000, 0},
		{"zero estimate", 0, 30_000_000, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := newPracticalChain()
			chain.headers[0].GasLimit = tc.block
			b := &admissionBackend{practicalChain: chain, gas: tc.estimate, balance: new(big.Int).Lsh(big.NewInt(1), 128)}
			m, s := startAdmissionManager(t, b, Config{MaxFeeGwei: 100, MaxGas: tc.budget})
			sender := m.Sender("test")
			err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}})
			if tc.want == 0 {
				if err == nil || s.calls.Load() != 0 || sender.Busy() {
					t.Fatalf("invalid estimate signed: err=%v signatures=%d", err, s.calls.Load())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tx := practicalAttempt(t, chain, 1)
			budget := tc.budget
			if budget == 0 {
				budget = DefaultMaxGas
			}
			if tx.Gas() != tc.want || b.gasBudget != min(budget, tc.block) {
				t.Fatalf("gas=%d estimate budget=%d, want %d", tx.Gas(), b.gasBudget, tc.want)
			}
		})
	}
}

func TestMaximumFundingAdmissionUsesFinalGasAndFee(t *testing.T) {
	cost := new(big.Int).Mul(big.NewInt(23_100), big.NewInt(41e9))
	readErr := errors.New("RPC unavailable")
	for _, tc := range []struct {
		name    string
		balance *big.Int
		err     error
		ok      bool
	}{
		{"exact", cost, nil, true},
		{"one wei short", new(big.Int).Sub(cost, big.NewInt(1)), nil, false},
		{"missing", nil, nil, false},
		{"negative", big.NewInt(-1), nil, false},
		{"overflow", new(big.Int).Lsh(big.NewInt(1), 256), nil, false},
		{"failed read", cost, readErr, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &admissionBackend{practicalChain: newPracticalChain(), gas: 21_000, balance: tc.balance, balanceErr: tc.err}
			m, s := startAdmissionManager(t, b, Config{MaxFeeGwei: 100})
			sender := m.Sender("test")
			err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}})
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if tx := practicalAttempt(t, b.practicalChain, 1); tx.Cost().Cmp(cost) != 0 {
					t.Fatalf("maximum cost %s", tx.Cost())
				}
			} else if err == nil || sender.Busy() || s.calls.Load() != 0 {
				t.Fatalf("unfunded signing: err=%v calls=%d busy=%v", err, s.calls.Load(), sender.Busy())
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("lost typed funding error: %v", err)
			}
		})
	}
}

func TestFreshAdmissionRefusesUnknownPendingNonce(t *testing.T) {
	chain := newPracticalChain()
	chain.pending = 1
	m, s := startAdmissionManager(t, chain, Config{MaxFeeGwei: 100})
	result := m.Send(t.Context(), Request{To: common.Address{1}})
	if result.Err == nil || result.Uncertain() || s.calls.Load() != 0 {
		t.Fatalf("unknown pending work overwritten: %+v signs=%d", result, s.calls.Load())
	}
}

func TestFreshAdmissionRechecksPendingAfterEstimation(t *testing.T) {
	chain := newPracticalChain()
	b := &admissionBackend{practicalChain: chain, gas: 21_000, balance: new(big.Int).Lsh(big.NewInt(1), 128)}
	b.afterEstimate = func() { chain.mu.Lock(); chain.pending = 1; chain.mu.Unlock() }
	m, s := startAdmissionManager(t, b, Config{MaxFeeGwei: 100})
	result := m.Send(t.Context(), Request{To: common.Address{1}})
	if result.Err == nil || result.Uncertain() || s.calls.Load() != 0 {
		t.Fatalf("pending work appearing during estimate was overwritten: %+v signs=%d", result, s.calls.Load())
	}
}

type rejectingReplacementBackend struct {
	*practicalChain
	mu       sync.Mutex
	original *big.Int
}

func (b *rejectingReplacementBackend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	b.mu.Lock()
	if b.original == nil {
		b.original = tx.GasFeeCap()
	}
	reject := tx.GasFeeCap().Cmp(b.original) > 0
	b.mu.Unlock()
	if reject {
		return errors.New("replacement transaction underpriced")
	}
	return b.practicalChain.SendTransaction(ctx, tx)
}
func TestRejectedReplacementRetainsIdentityWithoutRatchetingAcceptedFees(t *testing.T) {
	chain := newPracticalChain()
	b := &rejectingReplacementBackend{practicalChain: chain}
	signed := make(chan *types.Transaction, 32)
	s := &practicalSigner{Signer: practicalSigning(t, "1")}
	s.sign = func(ctx context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
		out, err := s.Signer.SignTx(ctx, tx, id)
		if out != nil {
			signed <- out
		}
		return out, err
	}
	m, err := New(b, s, big.NewInt(1), Config{MaxFeeGwei: 100, PollInterval: time.Millisecond, ReplacementInterval: 5 * time.Millisecond, ShutdownTimeout: time.Millisecond}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.Start(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	sender := m.Sender("test")
	if err = sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
		t.Fatal(err)
	}
	next := func() *types.Transaction {
		t.Helper()
		select {
		case tx := <-signed:
			return tx
		case <-time.After(time.Second):
			t.Fatal("replacement not signed")
			return nil
		}
	}
	original := next()
	chain.mu.Lock()
	chain.advanceLocked("first replacement block")
	chain.mu.Unlock()
	rejected := next()
	chain.mu.Lock()
	chain.advanceLocked("retry replacement block")
	chain.mu.Unlock()
	retry := next()
	if rejected.GasFeeCap().Cmp(original.GasFeeCap()) <= 0 || retry.Hash() != rejected.Hash() {
		t.Fatal("rejected signature ratcheted accepted fee floor")
	}
	chain.mine(original, types.ReceiptStatusSuccessful, 0)
	result := practicalExecution(t, sender)
	if result.Hash != original.Hash() {
		t.Fatal("replacement rejection lost original receipt")
	}
}

type rejectedInitialBackend struct{ *practicalChain }

func (b *rejectedInitialBackend) SendTransaction(_ context.Context, tx *types.Transaction) error {
	b.mu.Lock()
	b.sent = append(b.sent, tx)
	b.mu.Unlock()
	return errors.New("transaction underpriced")
}
func TestRetainedAttemptBoundFailsClosedWithoutEviction(t *testing.T) {
	b := &rejectedInitialBackend{newPracticalChain()}
	m, s := startAdmissionManager(t, b, Config{MaxFeeGwei: 100})
	for i := range maxAttempts {
		result := m.Send(t.Context(), Request{To: common.Address{1}, Data: []byte{byte(i)}})
		if !result.Uncertain() || result.Err == nil {
			t.Fatalf("signed rejection %d lost identity: %+v", i, result)
		}
	}
	result := m.Send(t.Context(), Request{To: common.Address{2}})
	if !errors.Is(result.Err, ErrBusy) || result.Uncertain() || s.calls.Load() != maxAttempts || m.Available() {
		t.Fatalf("full retained ledger admitted another signature: %+v signs=%d", result, s.calls.Load())
	}
	// A canonical oldest winner still resolves the retained nonce after the bound is reached.
	first := b.attempts()[0]
	b.mine(first, types.ReceiptStatusSuccessful, 0)
	for deadline := time.Now().Add(time.Second); !m.Available(); {
		if time.Now().After(deadline) {
			t.Fatal("retained oldest identity was evicted")
		}
		time.Sleep(time.Millisecond)
	}
}

type captureCancelLog struct{ cancel context.CancelFunc }

func (c captureCancelLog) Init(logr.RuntimeInfo) {}
func (c captureCancelLog) Enabled(int) bool      { return true }
func (c captureCancelLog) Info(_ int, msg string, _ ...any) {
	if msg == "transaction signed" {
		c.cancel()
		time.Sleep(time.Millisecond)
	}
}
func (c captureCancelLog) Error(error, string, ...any)    {}
func (c captureCancelLog) WithValues(...any) logr.LogSink { return c }
func (c captureCancelLog) WithName(string) logr.LogSink   { return c }

type captureContextBackend struct {
	*practicalChain
	broadcastContext chan error
}

func (b *captureContextBackend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	select {
	case b.broadcastContext <- ctx.Err():
	default:
	}
	return b.practicalChain.SendTransaction(ctx, tx)
}
func TestCaptureHandoffDetachesCallerBeforePublishingReady(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		caller, cancelCaller := context.WithCancel(t.Context())
		defer cancelCaller()
		b := &captureContextBackend{practicalChain: newPracticalChain(), broadcastContext: make(chan error, 1)}
		m, err := New(b, practicalSigning(t, "1"), big.NewInt(1), Config{MaxFeeGwei: 100, PollInterval: time.Millisecond, ShutdownTimeout: time.Millisecond}, logr.New(captureCancelLog{cancel: cancelCaller}))
		if err != nil {
			t.Fatal(err)
		}
		if err = m.Initialize(t.Context()); err != nil {
			t.Fatal(err)
		}
		managerCtx, stop := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); m.Start(managerCtx) }()
		defer func() { stop(); <-done }()
		sender := m.Sender("test")
		if err = sender.Submit(caller, ExecutionRequest{To: common.Address{1}}); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if err = <-b.broadcastContext; err != nil {
			t.Fatalf("capture handoff canceled accepted broadcast: %v", err)
		}
		if !sender.Busy() {
			t.Fatal("capture handoff lost strict ticket")
		}
		b.mine(practicalAttempt(t, b.practicalChain, 1), types.ReceiptStatusSuccessful, 0)
		_ = practicalExecution(t, sender)
	})
}
