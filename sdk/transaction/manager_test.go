package transaction

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

const testKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

type testBackend struct {
	mined, pending                uint64
	head                          uint64
	headers                       map[uint64]*types.Header
	receipt                       *types.Receipt
	sent                          []*types.Transaction
	sendErr, receiptErr, nonceErr error
	balance                       *big.Int
	balanceErr                    error
	balanceAccount                common.Address
	balanceBlock                  *big.Int
	balanceReads                  int
	afterBalance                  func()
	gas                           uint64
	afterReceipt                  func()
	afterSend                     func()
	receiptMisses                 int
}

func newBackend() *testBackend {
	b := &testBackend{headers: map[uint64]*types.Header{}, gas: 21_000, balance: big.NewInt(1_000_000_000_000_000_000)}
	for i := range uint64(5) {
		h := &types.Header{Number: new(big.Int).SetUint64(i), BaseFee: big.NewInt(1_000_000_000), GasLimit: 30_000_000, Time: 100 + i}
		if i > 0 {
			h.ParentHash = b.headers[i-1].Hash()
		}
		b.headers[i] = h
	}
	return b
}
func (b *testBackend) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	return b.mined, b.nonceErr
}
func (b *testBackend) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return b.pending, b.nonceErr
}
func (b *testBackend) TransactionSenderBalanceAt(_ context.Context, account common.Address, block *big.Int) (*big.Int, error) {
	b.balanceAccount, b.balanceBlock = account, block
	b.balanceReads++
	if b.afterBalance != nil {
		b.afterBalance()
	}
	return b.balance, b.balanceErr
}
func (b *testBackend) FeeHistory(context.Context, uint64, *big.Int, []float64) (*ethereum.FeeHistory, error) {
	return &ethereum.FeeHistory{Reward: [][]*big.Int{{big.NewInt(100_000_000)}}}, nil
}
func (b *testBackend) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n == nil {
		return b.headers[b.head], nil
	}
	if !n.IsUint64() || n.Uint64() > b.head {
		return nil, ethereum.NotFound
	}
	return b.headers[n.Uint64()], nil
}
func (b *testBackend) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	for _, h := range b.headers {
		if h.Hash() == hash {
			return h, nil
		}
	}
	return nil, ethereum.NotFound
}
func (b *testBackend) EstimateGas(context.Context, ethereum.CallMsg) (uint64, error) {
	return b.gas, nil
}
func (b *testBackend) SendTransaction(_ context.Context, tx *types.Transaction) error {
	b.sent = append(b.sent, tx)
	if b.afterSend != nil {
		b.afterSend()
	}
	return b.sendErr
}
func (b *testBackend) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	if b.receiptMisses > 0 {
		b.receiptMisses--
		return nil, ethereum.NotFound
	}
	if b.receiptErr != nil {
		return nil, b.receiptErr
	}
	if b.receipt == nil {
		return nil, ethereum.NotFound
	}
	r := b.receipt
	if b.afterReceipt != nil {
		b.afterReceipt()
	}
	return r, nil
}
func newManager(t *testing.T, b *testBackend) *Manager {
	t.Helper()
	return newManagerWithConfig(t, b, Config{MaxFeeGwei: 30, Confirmations: 2})
}
func newManagerWithConfig(t *testing.T, b *testBackend, cfg Config) *Manager {
	t.Helper()
	s, err := signer.NewFromHexKey(testKey)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(b, s, big.NewInt(31337), cfg, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func testRequest() Request { return Request{To: common.HexToAddress("0xdead"), Data: []byte{1, 2, 3}} }
func include(b *testBackend, status uint64) {
	tx := b.sent[0]
	b.head = 3
	b.mined = tx.Nonce() + 1
	b.pending = b.mined
	b.receipt = &types.Receipt{TxHash: tx.Hash(), BlockNumber: big.NewInt(1), BlockHash: b.headers[1].Hash(), Status: status, TransactionIndex: 2}
	b.receipt.Logs = []*types.Log{{Address: common.HexToAddress("0xbeef"), Topics: []common.Hash{common.HexToHash("0x1234")}, Data: []byte{9}, BlockNumber: 1, BlockHash: b.receipt.BlockHash, TxHash: tx.Hash(), TxIndex: 2, Index: 0}}
}

func TestAmbiguousBroadcastRetainsExactBytesAndNonce(t *testing.T) {
	b := newBackend()
	b.sendErr = errors.New("relay timed out")
	m := newManager(t, b)
	req := testRequest()
	if err := m.Submit(t.Context(), req); err == nil || !m.Busy() {
		t.Fatalf("ambiguous submit: err=%v busy=%v", err, m.Busy())
	}
	req.Data[0] = 99
	if err := m.Submit(t.Context(), testRequest()); !errors.Is(err, ErrBusy) {
		t.Fatalf("second submit: %v", err)
	}
	b.sendErr = nil
	if result, err := m.Poll(t.Context()); result != nil || err != nil || !m.Busy() {
		t.Fatalf("pending poll: result=%+v err=%v busy=%v", result, err, m.Busy())
	}
	if len(b.sent) != 2 || b.sent[0].Hash() != b.sent[1].Hash() || !bytes.Equal(b.sent[1].Data(), []byte{1, 2, 3}) || b.sent[1].Nonce() != 0 {
		t.Fatalf("retry changed signed intent: %+v", b.sent)
	}
}

func TestKnownBroadcastKeepsPendingOwnership(t *testing.T) {
	for _, tc := range []struct {
		message string
		known   bool
	}{
		{"already known", true},
		{"Transaction rejected: transaction already imported", true},
		{"insufficient funds", false},
		{"relay timed out", false},
	} {
		t.Run(tc.message, func(t *testing.T) {
			b := newBackend()
			b.sendErr = errors.New(tc.message)
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); (err == nil) != tc.known || !m.Busy() {
				t.Fatalf("submit err=%v busy=%v", err, m.Busy())
			}
			if result, err := m.Poll(t.Context()); result != nil || (err == nil) != tc.known || !m.Busy() {
				t.Fatalf("pending result=%+v err=%v busy=%v", result, err, m.Busy())
			}
			if len(b.sent) != 2 || b.sent[0].Hash() != b.sent[1].Hash() {
				t.Fatal("retry changed pending transaction identity")
			}
		})
	}
}

func TestSubmitRefusesOutstandingOrInconsistentNonceBeforeSigning(t *testing.T) {
	for _, tc := range []struct {
		name           string
		mined, pending uint64
	}{{"existing pending", 0, 1}, {"inconsistent endpoint", 2, 1}, {"exhausted nonce", math.MaxUint64, math.MaxUint64}} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			b.mined, b.pending = tc.mined, tc.pending
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err == nil || m.Busy() || len(b.sent) != 0 {
				t.Fatalf("unsafe admission: err=%v busy=%v sends=%d", err, m.Busy(), len(b.sent))
			}
		})
	}
}

func TestCanonicalReceiptCompletesAndReleasesOwnership(t *testing.T) {
	for _, status := range []uint64{types.ReceiptStatusSuccessful, types.ReceiptStatusFailed} {
		t.Run(map[uint64]string{0: "reverted", 1: "confirmed"}[status], func(t *testing.T) {
			b := newBackend()
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err != nil {
				t.Fatal(err)
			}
			include(b, status)
			result, err := m.Poll(t.Context())
			if err != nil || result == nil || result.Hash != b.sent[0].Hash() || result.Reverted != (status == 0) || result.Execution == nil || m.Busy() {
				t.Fatalf("terminal poll: result=%+v err=%v busy=%v", result, err, m.Busy())
			}
			if err := result.Execution.Validate(); err != nil {
				t.Fatal(err)
			}
			b.receipt.Logs[0].Data[0] = 42
			if result.Execution.Logs[0].Data[0] != 9 {
				t.Fatal("execution aliases RPC receipt")
			}
			if result, err := m.Poll(t.Context()); err != nil || result != nil {
				t.Fatalf("terminal result repeated: %+v %v", result, err)
			}
			if err := m.Submit(t.Context(), testRequest()); err != nil || b.sent[1].Nonce() != 1 {
				t.Fatalf("next nonce: err=%v", err)
			}
		})
	}
}

func TestReceiptArrivingBeforeAdvancedNonceRetainsOwnedIdentity(t *testing.T) {
	b := newBackend()
	m := newManager(t, b)
	if err := m.Submit(t.Context(), testRequest()); err != nil {
		t.Fatal(err)
	}
	include(b, types.ReceiptStatusSuccessful)
	// Inclusion can happen between the initial receipt lookup and the mined
	// nonce read. Resolve the owned hash before reporting foreign consumption.
	b.receiptMisses = 1
	result, err := m.Poll(t.Context())
	if err != nil || result == nil || result.Hash != b.sent[0].Hash() || m.Busy() || len(b.sent) != 1 {
		t.Fatalf("owned inclusion misclassified: result=%+v err=%v busy=%v sends=%d", result, err, m.Busy(), len(b.sent))
	}
}

func TestInclusionRacingRebroadcastRequiresOwnedCanonicalReceipt(t *testing.T) {
	readFailure := errors.New("receipt read unavailable")
	for _, tc := range []struct {
		name       string
		include    func(*testBackend)
		wantResult bool
		wantErr    error
	}{
		{"confirmed", func(b *testBackend) { include(b, types.ReceiptStatusSuccessful) }, true, nil},
		{"reverted", func(b *testBackend) { include(b, types.ReceiptStatusFailed) }, true, nil},
		{"insufficient depth", func(b *testBackend) {
			include(b, types.ReceiptStatusSuccessful)
			b.head = 2
		}, false, nil},
		{"orphaned", func(b *testBackend) {
			include(b, types.ReceiptStatusSuccessful)
			b.receipt.BlockHash = common.HexToHash("0x99")
			b.receipt.Logs[0].BlockHash = b.receipt.BlockHash
		}, false, nil},
		{"foreign transaction", func(b *testBackend) { b.mined++ }, false, ErrNonceConsumed},
		{"receipt unavailable", func(b *testBackend) { b.receiptErr = readFailure }, false, readFailure},
		{"nonce unavailable", func(b *testBackend) { b.nonceErr = readFailure }, false, readFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err != nil {
				t.Fatal(err)
			}
			b.afterSend = func() {
				tc.include(b)
				b.sendErr = errors.New("Transaction rejected: nonce too low")
			}
			result, err := m.Poll(t.Context())
			if (result != nil) != tc.wantResult || (tc.wantErr == nil && err != nil) || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) || m.Busy() == tc.wantResult {
				t.Fatalf("result=%+v err=%v busy=%v", result, err, m.Busy())
			}
			if len(b.sent) != 2 || b.sent[0].Hash() != b.sent[1].Hash() {
				t.Fatal("rebroadcast changed the owned identity")
			}
			if result != nil && (result.Hash != b.sent[0].Hash() || result.Reverted != (tc.name == "reverted")) {
				t.Fatal("foreign or incorrect execution returned")
			}
		})
	}
}

func TestRebroadcastErrorsWithoutOwnedEvidenceRetainOwnership(t *testing.T) {
	for _, message := range []string{"nonce too low", "insufficient funds", "relay timed out"} {
		t.Run(message, func(t *testing.T) {
			b := newBackend()
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err != nil {
				t.Fatal(err)
			}
			failure := errors.New(message)
			b.sendErr = failure
			if message != "nonce too low" {
				// An unrelated rejection remains visible even when inclusion races it.
				b.afterSend = func() { include(b, types.ReceiptStatusSuccessful) }
			}
			result, err := m.Poll(t.Context())
			if result != nil || !errors.Is(err, failure) || !m.Busy() || len(b.sent) != 2 {
				t.Fatalf("unproven rejection cleared or masked: result=%+v err=%v busy=%v", result, err, m.Busy())
			}
		})
	}
}

func TestNonterminalEvidenceNeverReleasesOwnership(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*testBackend)
		wantErr bool
	}{
		{"insufficient depth", func(b *testBackend) { b.head = 2 }, false},
		{"orphaned receipt", func(b *testBackend) {
			b.receipt.BlockHash = common.HexToHash("0x99")
			b.receipt.Logs[0].BlockHash = b.receipt.BlockHash
		}, false},
		{"invalid receipt hash", func(b *testBackend) { b.receipt.TxHash = common.HexToHash("0x99") }, true},
		{"invalid status", func(b *testBackend) { b.receipt.Status = 2 }, true},
		{"invalid log identity", func(b *testBackend) { b.receipt.Logs[0].TxHash = common.HexToHash("0x99") }, true},
		{"invalid log index", func(b *testBackend) { b.receipt.Logs[0].TxIndex = 0 }, true},
		{"removed log", func(b *testBackend) { b.receipt.Logs[0].Removed = true }, true},
		{"receipt RPC error", func(b *testBackend) { b.receiptErr = errors.New("read unavailable") }, true},
		{"unknown consumed nonce", func(b *testBackend) { b.receipt = nil }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err != nil {
				t.Fatal(err)
			}
			include(b, 1)
			tc.change(b)
			result, err := m.Poll(t.Context())
			if result != nil || (err != nil) != tc.wantErr || !m.Busy() {
				t.Fatalf("unproven terminal: result=%+v err=%v busy=%v", result, err, m.Busy())
			}
			if len(b.sent) != 1 {
				t.Fatal("rebroadcast obsolete transaction")
			}
		})
	}
}

func TestConfirmationRechecksReceiptAndCanonicalWitness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testBackend)
	}{
		{"witness reorg", func(b *testBackend) {
			h := *b.headers[3]
			h.Extra = []byte{1}
			b.headers[3] = &h
		}},
		{"receipt moved", func(b *testBackend) {
			r := *b.receipt
			r.BlockNumber = big.NewInt(2)
			r.BlockHash = b.headers[2].Hash()
			log := *r.Logs[0]
			log.BlockNumber, log.BlockHash = 2, r.BlockHash
			r.Logs = []*types.Log{&log}
			b.receipt = &r
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err != nil {
				t.Fatal(err)
			}
			include(b, 1)
			reads := 0
			b.afterReceipt = func() {
				reads++
				if reads == 2 {
					tc.change(b)
				}
			}
			if tc.name == "receipt moved" {
				// Move the receipt after inclusion was read, before its confirmation recheck.
				b.afterReceipt = func() {
					reads++
					if reads == 1 {
						tc.change(b)
					}
				}
			}
			result, err := m.Poll(t.Context())
			if result != nil || err != nil || !m.Busy() {
				t.Fatalf("changed confirmation accepted: %+v %v", result, err)
			}
		})
	}
}

func TestCancelledWaitRetainsSubmittedTransaction(t *testing.T) {
	b := newBackend()
	m := newManager(t, b)
	if err := m.Submit(t.Context(), testRequest()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := m.Poll(ctx); !errors.Is(err, context.Canceled) || !m.Busy() {
		t.Fatalf("cancelled poll: err=%v busy=%v", err, m.Busy())
	}
	include(b, 1)
	if result, err := m.Poll(t.Context()); result == nil || err != nil {
		t.Fatalf("resume: %+v %v", result, err)
	}
}

func TestSignerCannotChangeTransactionPayload(t *testing.T) {
	b := newBackend()
	s, err := signer.NewFromHexKey(testKey)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(b, changedSigner{s}, big.NewInt(31337), Config{MaxFeeGwei: 30}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Submit(t.Context(), testRequest()); err == nil || m.Busy() || len(b.sent) != 0 {
		t.Fatalf("changed signature accepted: err=%v", err)
	}
}

type changedSigner struct{ signer.Signer }

func (s changedSigner) SignTx(ctx context.Context, tx *types.Transaction, id *big.Int) (*types.Transaction, error) {
	to := *tx.To()
	return s.Signer.SignTx(ctx, types.NewTx(&types.DynamicFeeTx{ChainID: id, Nonce: tx.Nonce(), To: &to, Gas: tx.Gas(), GasFeeCap: tx.GasFeeCap(), GasTipCap: tx.GasTipCap(), Data: []byte{7}}), id)
}

func TestFeeCeilingAndMalformedEstimatesFailBeforeSigning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testBackend)
	}{
		{"expensive base fee", func(b *testBackend) { b.headers[0].BaseFee = big.NewInt(31_000_000_000) }},
		{"missing base fee", func(b *testBackend) { b.headers[0].BaseFee = nil }},
		{"zero gas", func(b *testBackend) { b.gas = 0 }},
		{"overflow gas", func(b *testBackend) { b.gas = math.MaxUint64 }},
		{"block gas limit", func(b *testBackend) { b.gas = 31_000_000 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			tc.change(b)
			m := newManager(t, b)
			if err := m.Submit(t.Context(), testRequest()); err == nil || m.Busy() || len(b.sent) != 0 {
				t.Fatalf("unsafe estimate admitted: %v", err)
			}
		})
	}
}
