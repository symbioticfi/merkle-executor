package chainio

import (
	"context"
	stderrors "errors"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type recordingBackend struct {
	mu           sync.Mutex
	groups       [][]ReadRequest
	active, peak atomic.Int64
	wait         <-chan struct{}
	malformed    bool
}

func (b *recordingBackend) BatchRead(ctx context.Context, qs []ReadRequest) ([]ReadResult, error) {
	n := b.active.Add(1)
	defer b.active.Add(-1)
	for old := b.peak.Load(); n > old; old = b.peak.Load() {
		if b.peak.CompareAndSwap(old, n) {
			break
		}
	}
	b.mu.Lock()
	b.groups = append(b.groups, qs)
	b.mu.Unlock()
	if b.wait != nil {
		select {
		case <-b.wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if b.malformed {
		return nil, nil
	}
	out := make([]ReadResult, len(qs))
	for i, q := range qs {
		out[i].Data = append([]byte{1}, q.Call.Data...)
	}
	return out, nil
}
func testLimits() Limits {
	return Limits{MaxMembers: 256, ReadTimeout: time.Second, DefaultCallGas: 1, DefaultOutputBytes: 1, MaxRequestBytes: 32 << 20, MaxResponseBytes: 32 << 20, MaxCalldataBytes: 32 << 20, MaxGas: 30_000_000, MaxOutputBytes: 32 << 20, MaxConcurrent: 2, MaxQueuedRequests: 4096, MaxQueuedBytes: 64 << 20, CollectionWindow: 20 * time.Millisecond}
}
func startScheduler(t *testing.T, b BatchReader, l Limits) *Scheduler {
	t.Helper()
	s, e := NewScheduler(b, l)
	if e != nil {
		t.Fatal(e)
	}
	go func() { _ = s.Run(t.Context()) }()
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func request(i byte) ReadRequest {
	to := common.HexToAddress("0x1234")
	return ReadRequest{Kind: ReadCall, Call: ethereum.CallMsg{To: &to, Data: []byte{i}}, Block: big.NewInt(10), ChainID: "1", ExpectedOutputBytes: 32}
}
func TestSchedulerSharesNumberedPinAcrossOwners(t *testing.T) {
	for _, owners := range []int{1, 16, 64, 256, 1024} {
		t.Run(big.NewInt(int64(owners)).String(), func(t *testing.T) {
			b := new(recordingBackend)
			s := startScheduler(t, b, testLimits())
			ready := make(chan struct{})
			var wg sync.WaitGroup
			for range owners {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-ready
					r, e := s.BatchRead(t.Context(), []ReadRequest{request(1)})
					if e != nil || len(r) != 1 {
						t.Errorf("read: %v", e)
					}
				}()
			}
			close(ready)
			wg.Wait()
			b.mu.Lock()
			defer b.mu.Unlock()
			total := 0
			for _, g := range b.groups {
				total += len(g)
			}
			if total != 1 {
				t.Fatalf("owners=%d executed %d unique reads", owners, total)
			}
		})
	}
}
func TestSchedulerExactContextsAndLatest(t *testing.T) {
	b := new(recordingBackend)
	s := startScheduler(t, b, testLimits())
	q := request(1)
	caller := q
	caller.Call.From = common.HexToAddress("0x99")
	override := caller
	override.CodeOverride = []byte{1}
	otherCode := override
	otherCode.CodeOverride = []byte{2}
	otherChain := q
	otherChain.ChainID = "2"
	latest := q
	latest.Block = nil
	pending := q
	pending.Block = big.NewInt(-1)
	qs := []ReadRequest{q, q, caller, override, override, otherCode, otherChain, latest, latest, pending, pending}
	r, e := s.BatchRead(t.Context(), qs)
	if e != nil {
		t.Fatal(e)
	}
	r[0].Data[0] = 9
	if r[1].Data[0] != 1 {
		t.Fatal("shared mutable result")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) != 1 || len(b.groups[0]) != 9 {
		t.Fatalf("groups: %v", b.groups)
	}
}
func TestSchedulerBoundsChunksAndConcurrency(t *testing.T) {
	b := new(recordingBackend)
	l := testLimits()
	l.MaxMembers = 2
	l.MaxCalldataBytes = 2
	l.MaxGas = 2
	l.MaxOutputBytes = 64
	s := startScheduler(t, b, l)
	qs := make([]ReadRequest, 16)
	for i := range qs {
		qs[i] = request(byte(i))
		qs[i].Call.Gas = 1
	}
	if _, e := s.BatchRead(t.Context(), qs); e != nil {
		t.Fatal(e)
	}
	if b.peak.Load() > 2 {
		t.Fatal("concurrency exceeded")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) != 8 {
		t.Fatalf("chunks=%d", len(b.groups))
	}
	for _, g := range b.groups {
		if len(g) > 2 {
			t.Fatal("oversized group")
		}
	}
}
func TestSchedulerCancellationKeepsOtherOwners(t *testing.T) {
	gate := make(chan struct{})
	b := &recordingBackend{wait: gate}
	s := startScheduler(t, b, testLimits())
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, e := s.BatchRead(ctx, []ReadRequest{request(1)}); done <- e }()
	other := make(chan error, 1)
	go func() { _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); other <- e }()
	for b.active.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if e := <-done; !stderrors.Is(e, context.Canceled) {
		t.Fatalf("cancel: %v", e)
	}
	close(gate)
	if e := <-other; e != nil {
		t.Fatal(e)
	}
}
func TestSchedulerMalformedGroupHasNoFallback(t *testing.T) {
	b := &recordingBackend{malformed: true}
	s := startScheduler(t, b, testLimits())
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); e == nil {
		t.Fatal("accepted missing member")
	}
	if len(b.groups) != 1 {
		t.Fatal("retried malformed batch")
	}
}
func TestSchedulerRejectsQueueAndSingleReadBudget(t *testing.T) {
	for _, mutate := range []func(*Limits){func(l *Limits) { l.MaxQueuedBytes = 1 }, func(l *Limits) { l.MaxRequestBytes = 1 }, func(l *Limits) { l.MaxGas = 1 }, func(l *Limits) { l.MaxOutputBytes = 1 }} {
		l := testLimits()
		mutate(&l)
		b := new(recordingBackend)
		s := startScheduler(t, b, l)
		q := request(1)
		q.Call.Gas = 2
		if _, e := s.BatchRead(t.Context(), []ReadRequest{q}); e == nil {
			t.Fatal("accepted oversized read")
		}
		if len(b.groups) != 0 {
			t.Fatal("dispatched oversized read")
		}
	}
}

func TestSchedulerFreshClosingReadIsNotOpeningCache(t *testing.T) {
	b := new(recordingBackend)
	s := startScheduler(t, b, testLimits())
	for range 2 {
		if _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); e != nil {
			t.Fatal(e)
		}
	}
	if len(b.groups) != 2 {
		t.Fatal("closing read reused opening result")
	}
}
func TestSchedulerQueuedRequestBoundAndProgress(t *testing.T) {
	gate := make(chan struct{})
	b := &recordingBackend{wait: gate}
	l := testLimits()
	l.MaxQueuedRequests = 1
	s := startScheduler(t, b, l)
	done := make(chan error, 1)
	go func() { _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); done <- e }()
	for b.active.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(2)}); e == nil {
		t.Fatal("accepted full queue")
	}
	close(gate)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(2)}); e != nil {
		t.Fatal("budget not released", e)
	}
}
func TestSchedulerRejectsActualOutputBudget(t *testing.T) {
	b := new(recordingBackend)
	l := testLimits()
	l.MaxOutputBytes = 1
	s := startScheduler(t, b, l)
	q := request(1)
	q.ExpectedOutputBytes = 1
	if _, e := s.BatchRead(t.Context(), []ReadRequest{q}); e == nil {
		t.Fatal("accepted output larger than actual budget")
	}
}
func TestSchedulerDoesNotStarveNewReadyOwner(t *testing.T) {
	gate := make(chan struct{})
	b := &fairBackend{first: gate, started: make(chan struct{})}
	l := testLimits()
	l.MaxMembers = 1
	l.MaxConcurrent = 1
	l.CollectionWindow = 0
	s := startScheduler(t, b, l)
	first := make(chan error, 1)
	qs := make([]ReadRequest, 10)
	for i := range qs {
		qs[i] = request(byte(i + 1))
	}
	go func() { _, e := s.BatchRead(t.Context(), qs); first <- e }()
	<-b.started
	second := make(chan error, 1)
	go func() { _, e := s.BatchRead(t.Context(), []ReadRequest{request(99)}); second <- e }()
	time.Sleep(time.Millisecond)
	close(gate)
	if e := <-second; e != nil {
		t.Fatal(e)
	}
	if e := <-first; e != nil {
		t.Fatal(e)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, id := range b.order {
		if id == 99 {
			if i >= 3 {
				t.Fatalf("new owner waited behind %d old partitions", i)
			}
			return
		}
	}
	t.Fatal("missing second owner")
}

type fairBackend struct {
	mu      sync.Mutex
	first   <-chan struct{}
	started chan struct{}
	once    sync.Once
	order   []byte
}

func (b *fairBackend) BatchRead(ctx context.Context, qs []ReadRequest) ([]ReadResult, error) {
	b.once.Do(func() {
		if b.started == nil {
			b.started = make(chan struct{})
		}
		close(b.started)
		select {
		case <-b.first:
		case <-ctx.Done():
		}
	})
	b.mu.Lock()
	b.order = append(b.order, qs[0].Call.Data[0])
	b.mu.Unlock()
	return []ReadResult{{Data: []byte{1}}}, nil
}

func TestSchedulerBoundsEncodedOverrideRequest(t *testing.T) {
	b := new(recordingBackend)
	l := testLimits()
	l.MaxRequestBytes = 2500
	s := startScheduler(t, b, l)
	q := request(1)
	q.Call.From = common.HexToAddress("0x99")
	q.CodeOverride = make([]byte, 1500)
	if _, e := s.BatchRead(t.Context(), []ReadRequest{q}); e == nil {
		t.Fatal("accepted override whose hexadecimal wire request exceeds bound")
	}
}
func TestSchedulerResultCannotExceedReservedOutput(t *testing.T) {
	b := new(recordingBackend)
	s := startScheduler(t, b, testLimits())
	q := request(1)
	q.ExpectedOutputBytes = 1
	if _, e := s.BatchRead(t.Context(), []ReadRequest{q}); e == nil {
		t.Fatal("accepted result larger than reserved output budget")
	}
}

func TestSchedulerCancelsAbandonedPartitionAndReleasesCapacity(t *testing.T) {
	b := &abandonedBackend{started: make(chan struct{}), canceled: make(chan struct{})}
	l := testLimits()
	l.MaxConcurrent = 1
	l.MaxQueuedRequests = 1
	l.CollectionWindow = 0
	s := startScheduler(t, b, l)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, e := s.BatchRead(ctx, []ReadRequest{request(1)}); done <- e }()
	<-b.started
	cancel()
	if e := <-done; !stderrors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	select {
	case <-b.canceled:
	case <-time.After(time.Second):
		t.Fatal("abandoned partition did not cancel backend")
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, e := s.BatchRead(t.Context(), []ReadRequest{request(2)})
		if e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue reservation not released", e)
		}
		time.Sleep(time.Millisecond)
	}
}

type abandonedBackend struct {
	started, canceled chan struct{}
	calls             atomic.Int64
}

func (b *abandonedBackend) BatchRead(ctx context.Context, qs []ReadRequest) ([]ReadResult, error) {
	if b.calls.Add(1) == 1 {
		close(b.started)
		<-ctx.Done()
		close(b.canceled)
		return nil, ctx.Err()
	}
	return []ReadResult{{Data: []byte{1}}}, nil
}
func TestSchedulerRejectsRPCWireResponseLargerThanConfiguredBound(t *testing.T) {
	raw := `[{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"` + strings.Repeat("x", 600) + `"}}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(raw)) }))
	defer server.Close()
	b := RPCBatchReader{Call: func(ctx context.Context, elems []rpc.BatchElem) error {
		return CallHTTPBatch(ctx, server.Client(), server.URL, elems)
	}}
	l := testLimits()
	l.MaxResponseBytes = 500
	s := startScheduler(t, b, l)
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); e == nil {
		t.Fatal("accepted raw response exceeding configured bound")
	}
}
func TestSchedulerChargesAggregateCalldataBeforeTransport(t *testing.T) {
	var dispatched atomic.Int64
	b := RPCBatchReader{Multicall: common.HexToAddress("0x99"), Codec: oversizedAggregateCodec{}, Call: func(_ context.Context, elems []rpc.BatchElem) error { dispatched.Add(1); return nil }}
	l := testLimits()
	l.MaxCalldataBytes = 8
	s := startScheduler(t, b, l)
	a, c := request(1), request(2)
	a.Call.Data = []byte{1, 2, 3, 4}
	c.Call.Data = []byte{5, 6, 7, 8}
	a.MulticallSafe = true
	c.MulticallSafe = true
	if _, e := s.BatchRead(t.Context(), []ReadRequest{a, c}); e == nil {
		t.Fatal("accepted aggregate calldata exceeding configured bound")
	}
	if dispatched.Load() != 0 {
		t.Fatal("oversized aggregate reached transport")
	}
}

type oversizedAggregateCodec struct{}

func (oversizedAggregateCodec) Encode([]AggregateCall) ([]byte, error) { return make([]byte, 452), nil }
func (oversizedAggregateCodec) Decode([]byte) ([]AggregateResult, error) {
	return []AggregateResult{{Success: true, Data: []byte{1}}, {Success: true, Data: []byte{1}}}, nil
}

func TestSchedulerReadDeadlineBoundsLiveOwners(t *testing.T) {
	b := &abandonedBackend{started: make(chan struct{}), canceled: make(chan struct{})}
	l := testLimits()
	l.ReadTimeout = 10 * time.Millisecond
	s := startScheduler(t, b, l)
	if _, e := s.BatchRead(t.Context(), []ReadRequest{request(1)}); !stderrors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("backend deadline: %v", e)
	}
	select {
	case <-b.canceled:
	default:
		t.Fatal("backend remained alive after I/O deadline")
	}
}

func TestSchedulerSkipsCanceledPendingPartitions(t *testing.T) {
	b := &abandonedBackend{started: make(chan struct{}), canceled: make(chan struct{})}
	l := testLimits()
	l.MaxMembers = 1
	l.MaxConcurrent = 1
	l.MaxQueuedRequests = 2
	l.CollectionWindow = 0
	s := startScheduler(t, b, l)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, e := s.BatchRead(ctx, []ReadRequest{request(1), request(2)}); done <- e }()
	<-b.started
	cancel()
	<-done
	<-b.canceled
	deadline := time.Now().Add(time.Second)
	for {
		_, e := s.BatchRead(t.Context(), []ReadRequest{request(3)})
		if e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled pending stage retained capacity", e)
		}
		time.Sleep(time.Millisecond)
	}
	if b.calls.Load() != 2 {
		t.Fatalf("canceled pending partition reached backend; total calls=%d", b.calls.Load())
	}
}

func TestSchedulerDedupRetainsLargestWireResponseReservation(t *testing.T) {
	b := new(recordingBackend)
	l := testLimits()
	l.MaxResponseBytes = 500
	s := startScheduler(t, b, l)
	a, c := request(1), request(1)
	a.ExpectedOutputBytes = 2
	c.ExpectedOutputBytes = 200
	other := request(2)
	other.ExpectedOutputBytes = 2
	if _, e := s.BatchRead(t.Context(), []ReadRequest{a, c, other}); e != nil {
		t.Fatal(e)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) != 2 {
		t.Fatalf("wire partition ignored larger shared reservation: groups=%d", len(b.groups))
	}
}
