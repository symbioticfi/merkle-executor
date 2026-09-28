package txmanager

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/state"
)

type fakeChain struct {
	mu                   sync.Mutex
	latest, pending      uint64
	head, receiptHeader  *types.Header
	receipts             map[common.Hash]*types.Receipt
	sent                 []*types.Transaction
	mine                 bool
	status               uint64
	sendErr, simulateErr error
	beforeBroadcast      func(*types.Transaction)
	receiptMutation      func(*types.Receipt)
	estimated, simulated ethereum.CallMsg
}

func newChain() *fakeChain {
	receiptHeader := &types.Header{Number: big.NewInt(10), BaseFee: big.NewInt(1)}
	middle := &types.Header{Number: big.NewInt(11), BaseFee: big.NewInt(1), ParentHash: receiptHeader.Hash()}
	return &fakeChain{head: &types.Header{Number: big.NewInt(12), BaseFee: big.NewInt(1), ParentHash: middle.Hash()}, receiptHeader: receiptHeader, receipts: make(map[common.Hash]*types.Receipt), mine: true, status: types.ReceiptStatusSuccessful}
}
func (b *fakeChain) NonceAt(context.Context, common.Address, *big.Int) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.latest, nil
}
func (b *fakeChain) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pending, nil
}
func (b *fakeChain) EstimateGas(_ context.Context, msg ethereum.CallMsg) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.estimated = msg
	return 90000, nil
}
func (b *fakeChain) CallContract(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.simulated = msg
	return nil, b.simulateErr
}
func (b *fakeChain) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if number != nil && number.Cmp(b.receiptHeader.Number) == 0 {
		return types.CopyHeader(b.receiptHeader), nil
	}
	return types.CopyHeader(b.head), nil
}
func (b *fakeChain) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	middle := &types.Header{Number: big.NewInt(11), BaseFee: big.NewInt(1), ParentHash: b.receiptHeader.Hash()}
	for _, h := range []*types.Header{b.receiptHeader, middle, b.head} {
		if h.Hash() == hash {
			return types.CopyHeader(h), nil
		}
	}
	return nil, ethereum.NotFound
}
func (b *fakeChain) SendTransaction(_ context.Context, tx *types.Transaction) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.beforeBroadcast != nil {
		b.beforeBroadcast(tx)
	}
	b.sent = append(b.sent, tx)
	b.pending = tx.Nonce() + 1
	if b.mine {
		b.latest = tx.Nonce() + 1
		b.receipts[tx.Hash()] = &types.Receipt{TxHash: tx.Hash(), BlockNumber: new(big.Int).Set(b.receiptHeader.Number), BlockHash: b.receiptHeader.Hash(), Status: b.status}
		if b.receiptMutation != nil {
			b.receiptMutation(b.receipts[tx.Hash()])
		}
	}
	return b.sendErr
}

func TestConfirmedExecutionEvidenceSurvivesRestartAndDoesNotAlias(t *testing.T) {
	dir := t.TempDir()
	store, err := state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	chain := newChain()
	chain.receiptMutation = func(r *types.Receipt) {
		r.Logs = []*types.Log{{Address: common.HexToAddress("0x10"), Topics: []common.Hash{common.HexToHash("0x11")}, Data: []byte{1, 2, 3}, TxHash: r.TxHash, BlockHash: r.BlockHash, BlockNumber: r.BlockNumber.Uint64(), Index: 7}}
	}
	s := testSigner(t)
	m := newManager(t, chain, s, store)
	got, err := m.Send(context.Background(), request())
	if err != nil || got.Execution == nil || got.Execution.TxHash != got.Hash || got.Execution.BlockHash != chain.receiptHeader.Hash() || len(got.Execution.Logs) != 1 {
		t.Fatalf("missing execution evidence: %+v %v", got, err)
	}
	got.Execution.BlockNumber.SetInt64(99)
	got.Execution.Logs[0].Data[0] = 99
	got.Execution.Logs[0].Topics[0] = common.Hash{}
	retained := m.retainedResult(request().ID)
	if retained.Execution.BlockNumber.Int64() != 10 || retained.Execution.Logs[0].Data[0] != 1 || retained.Execution.Logs[0].Topics[0] == (common.Hash{}) {
		t.Fatal("provider changed retained transaction evidence")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m = newManager(t, chain, s, store)
	replay, err := m.Send(context.Background(), request())
	if err != nil || replay.Execution == nil || replay.Execution.TxHash != got.Hash || replay.Execution.BlockNumber.Int64() != 10 || !bytes.Equal(replay.Execution.Logs[0].Data, []byte{1, 2, 3}) || len(chain.sent) != 1 {
		t.Fatalf("restart lost confirmed evidence: %+v %v", replay, err)
	}
}

func TestMalformedReceiptLogsNeverBecomeConfirmedEvidence(t *testing.T) {
	for _, mode := range []string{"nil", "hash", "block", "removed", "duplicate", "transaction-index"} {
		t.Run(mode, func(t *testing.T) {
			store, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			chain := newChain()
			chain.receiptMutation = func(r *types.Receipt) {
				log := &types.Log{TxHash: r.TxHash, BlockHash: r.BlockHash, BlockNumber: r.BlockNumber.Uint64()}
				r.Logs = []*types.Log{log}
				switch mode {
				case "nil":
					r.Logs[0] = nil
				case "hash":
					log.TxHash = common.HexToHash("0xbad")
				case "block":
					log.BlockNumber++
				case "removed":
					log.Removed = true
				case "duplicate":
					r.Logs = append(r.Logs, log)
				case "transaction-index":
					log.TxIndex = 1
				}
			}
			m := newManager(t, chain, testSigner(t), store)
			got, err := m.Send(context.Background(), request())
			if err == nil || !got.Pending || got.Confirmed || got.Execution != nil || m.PendingID() != request().ID {
				t.Fatalf("invalid logs became evidence: %+v %v", got, err)
			}
		})
	}
}
func (b *fakeChain) TransactionReceipt(_ context.Context, hash common.Hash) (*types.Receipt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r, ok := b.receipts[hash]; ok {
		copy := *r
		copy.BlockNumber = new(big.Int).Set(r.BlockNumber)
		return &copy, nil
	}
	return nil, ethereum.NotFound
}

func testSigner(t *testing.T) signer.Signer {
	t.Helper()
	t.Setenv("MERKLE_TEST_SENDER", strings.Repeat("0", 63)+"1")
	s, err := signer.FromEnv("MERKLE_TEST_SENDER")
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func newManager(t *testing.T, b *fakeChain, s signer.Signer, store *state.Store) *Manager {
	t.Helper()
	m, err := New(b, s, big.NewInt(1), store, Config{Confirmations: 2, PollInterval: time.Millisecond, MaxFeePerGas: big.NewInt(30), MaxPriorityFeePerGas: big.NewInt(2)}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func request() Request {
	return Request{ID: "strategy:round1", To: common.HexToAddress("0x1001"), Data: []byte{1, 2, 3, 4}}
}
func timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Millisecond)
}

func TestSignedBytesPersistBeforeBroadcastAndConfirmedReplayNeverResigns(t *testing.T) {
	dir := t.TempDir()
	store, err := state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := testSigner(t)
	chain := newChain()
	m := newManager(t, chain, s, store)
	chain.beforeBroadcast = func(tx *types.Transaction) {
		restored := newManager(t, chain, s, store)
		if restored.PendingID() != request().ID {
			t.Fatal("broadcast before durable intent")
		}
	}
	got, err := m.Send(context.Background(), request())
	if err != nil || !got.Confirmed || got.Pending || got.Reverted {
		t.Fatalf("result: %+v %v", got, err)
	}
	if chain.simulated.Gas != 90000 || !bytes.Equal(chain.simulated.Data, request().Data) || chain.simulated.From != s.Address() || chain.simulated.To == nil || *chain.simulated.To != request().To {
		t.Fatal("exact signed call was not simulated")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m = newManager(t, chain, s, store)
	replay, err := m.Send(context.Background(), request())
	if err != nil || !replay.Confirmed || replay.Hash != got.Hash || len(chain.sent) != 1 {
		t.Fatalf("confirmed replay: %+v %v sends=%d", replay, err, len(chain.sent))
	}
	other := request()
	other.ID = "another"
	if _, err := m.Send(context.Background(), other); err == nil {
		t.Fatal("new ID admitted before terminal acknowledgement")
	}
	if err := m.Acknowledge(request().ID); err != nil {
		t.Fatal(err)
	}
	if m.PendingID() != "" {
		t.Fatal("acknowledgement did not release journal")
	}
}

func TestAmbiguousBroadcastRecoversExactHashAfterRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := testSigner(t)
	chain := newChain()
	chain.mine = false
	chain.sendErr = errors.New("connection lost after submission")
	m := newManager(t, chain, s, store)
	ctx, cancel := timeout()
	got, err := m.Send(ctx, request())
	cancel()
	if err == nil || !got.Pending || got.Hash == (common.Hash{}) {
		t.Fatalf("ambiguous result: %+v %v", got, err)
	}
	if err := m.Acknowledge(request().ID); err == nil {
		t.Fatal("pending record acknowledged")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain.mine = true
	chain.sendErr = nil
	m = newManager(t, chain, s, store)
	recovered, err := m.Send(context.Background(), request())
	if err != nil || !recovered.Confirmed || recovered.Hash != got.Hash {
		t.Fatalf("recovery: %+v %v", recovered, err)
	}
	if len(chain.sent) != 2 || chain.sent[0].Hash() != chain.sent[1].Hash() || chain.sent[0].Nonce() != chain.sent[1].Nonce() {
		t.Fatal("recovery replaced original signed operation")
	}
}

func TestCancellationKeepsPendingOwnershipAndRequestIdentity(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	chain.mine = false
	m := newManager(t, chain, testSigner(t), store)
	ctx, cancel := timeout()
	got, err := m.Send(ctx, request())
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || !got.Pending {
		t.Fatalf("timeout: %+v %v", got, err)
	}
	changed := request()
	changed.Data = []byte{5}
	if _, err := m.Send(context.Background(), changed); err == nil {
		t.Fatal("same ID accepted different calldata")
	}
	other := request()
	other.ID = "other"
	if _, err := m.Send(context.Background(), other); err == nil {
		t.Fatal("unresolved nonce did not block other intent")
	}
	if len(chain.sent) != 1 {
		t.Fatal("rejected intent broadcast")
	}
}

func TestReorgedReceiptCannotCompleteUntilCanonicalAndDeepEnough(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	chain.mine = false
	m := newManager(t, chain, testSigner(t), store)
	ctx, cancel := timeout()
	first, _ := m.Send(ctx, request())
	cancel()
	chain.receipts[first.Hash] = &types.Receipt{TxHash: first.Hash, BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0xbad"), Status: 1}
	ctx, cancel = timeout()
	got, err := m.Send(ctx, request())
	cancel()
	if err == nil || !got.Pending || got.Confirmed {
		t.Fatalf("fork receipt accepted: %+v %v", got, err)
	}
	chain.receipts[first.Hash].BlockHash = chain.receiptHeader.Hash()
	chain.head.Number = big.NewInt(11)
	ctx, cancel = timeout()
	got, err = m.Send(ctx, request())
	cancel()
	if err == nil || !got.Pending || got.Confirmed {
		t.Fatalf("insufficient depth accepted: %+v %v", got, err)
	}
	chain.head.Number = big.NewInt(12)
	got, err = m.Send(context.Background(), request())
	if err != nil || !got.Confirmed {
		t.Fatalf("canonical receipt not confirmed: %+v %v", got, err)
	}
}

func TestRevertedReceiptIsTerminalAndReplaysWithoutAnotherTransaction(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	chain.status = types.ReceiptStatusFailed
	m := newManager(t, chain, testSigner(t), store)
	got, err := m.Send(context.Background(), request())
	if err != nil || !got.Reverted || got.Confirmed || got.Pending {
		t.Fatalf("revert: %+v %v", got, err)
	}
	if _, err := m.Send(context.Background(), request()); err != nil || len(chain.sent) != 1 {
		t.Fatalf("revert replay: %v", err)
	}
}

func TestConflictingConfirmationAncestryCannotComplete(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	chain.head.ParentHash = common.HexToHash("0xbad")
	m := newManager(t, chain, testSigner(t), store)
	ctx, cancel := timeout()
	defer cancel()
	got, err := m.Send(ctx, request())
	if err == nil || !got.Pending || got.Confirmed {
		t.Fatalf("unrelated receipt and head were accepted: %+v %v", got, err)
	}
}

func TestSimulationAndUnknownNonceConflictFailBeforeSigning(t *testing.T) {
	for _, mode := range []string{"simulation", "nonce"} {
		t.Run(mode, func(t *testing.T) {
			store, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			chain := newChain()
			if mode == "simulation" {
				chain.simulateErr = errors.New("execution reverted")
			} else {
				chain.pending = 1
			}
			m := newManager(t, chain, testSigner(t), store)
			if _, err := m.Send(context.Background(), request()); err == nil {
				t.Fatal("unsafe call admitted")
			}
			if len(chain.sent) != 0 || m.PendingID() != "" {
				t.Fatal("failed preflight signed or persisted intent")
			}
		})
	}
}

func TestConcurrentSendAndAcknowledgeCannotRaceActiveBroadcast(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	m := newManager(t, chain, testSigner(t), store)
	started, release := make(chan struct{}), make(chan struct{})
	chain.beforeBroadcast = func(*types.Transaction) {
		close(started)
		<-release
	}
	completed := make(chan error, 1)
	go func() {
		_, err := m.Send(context.Background(), request())
		completed <- err
	}()
	<-started
	if m.PendingID() != request().ID {
		t.Error("active ID unavailable while broadcasting")
	}
	if _, err := m.Send(context.Background(), request()); !errors.Is(err, ErrLaneBusy) {
		t.Errorf("concurrent send entered lane: %v", err)
	}
	if err := m.Acknowledge(request().ID); !errors.Is(err, ErrLaneBusy) {
		t.Errorf("acknowledge raced active send: %v", err)
	}
	close(release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if len(chain.sent) != 1 {
		t.Fatal("concurrent send duplicated transaction")
	}
}

func TestPersistenceFailureBlocksBroadcastAndRetainsOwnership(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chain := newChain()
	m := newManager(t, chain, testSigner(t), store)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got, err := m.Send(context.Background(), request())
		if err == nil || !got.Pending || m.PendingID() != request().ID || len(chain.sent) != 0 {
			t.Fatalf("uncertain persistence did not fail closed: %+v %v", got, err)
		}
	}
}

func TestExternalNonceConsumptionCannotReplaceRetainedOperation(t *testing.T) {
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chain := newChain()
	chain.mine = false
	chain.sendErr = errors.New("unknown outcome")
	m := newManager(t, chain, testSigner(t), store)
	first, err := m.Send(context.Background(), request())
	if err == nil || !first.Pending {
		t.Fatalf("expected retained broadcast: %+v %v", first, err)
	}
	chain.latest = 1
	got, err := m.Send(context.Background(), request())
	if !errors.Is(err, ErrNonceConflict) || !got.Pending || got.Hash != first.Hash || len(chain.sent) != 1 {
		t.Fatalf("foreign nonce consumption replaced intent: %+v %v", got, err)
	}
}
