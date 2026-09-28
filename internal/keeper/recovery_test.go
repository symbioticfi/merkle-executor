package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/state"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
)

type recoveryStore struct {
	*state.Store
	failPhase string
	failDone  bool
}

func (s *recoveryStore) Save(key string, value any) error {
	if job, ok := value.(Job); ok {
		if s.failPhase != "" && job.Phase == s.failPhase {
			s.failPhase = ""
			return errors.New("injected journal write failure")
		}
		if s.failDone && len(job.Progress) > 0 && job.Progress[0].Done {
			s.failDone = false
			return errors.New("injected progress write failure")
		}
	}
	return s.Store.Save(key, value)
}

type recoveryPlanner struct {
	prepared, validated, verified int
	verifyErr                     error
}

func (p *recoveryPlanner) Prepare(_ context.Context, s Strategy) (Plan, error) {
	p.prepared++
	return Plan{Snapshot: provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Timestamp: 100, Account: common.HexToAddress(s.Config.Account), Executor: common.HexToAddress(s.Config.Executor), Caller: common.HexToAddress("0x30")}, Calldata: []byte{1, 2, 3}, Outputs: []provider.Output{{Data: json.RawMessage(`{"uid":"stable-order"}`)}}}, nil
}
func (p *recoveryPlanner) Validate(context.Context, Plan) error { p.validated++; return nil }
func (p *recoveryPlanner) ConfirmedSnapshot(_ context.Context, s provider.Snapshot) (provider.Snapshot, error) {
	return s, nil
}
func (p *recoveryPlanner) VerifyConfirmedSnapshot(context.Context, provider.Snapshot) error {
	p.verified++
	return p.verifyErr
}

type recoverySender struct {
	pending     string
	requests    []txmanager.Request
	broadcasts  int
	reverted    bool
	ackFailures int
	beforeAck   func(string) error
	execution   *provider.Execution
}

func (s *recoverySender) PendingID() string { return s.pending }
func (s *recoverySender) Send(_ context.Context, req txmanager.Request) (txmanager.Result, error) {
	if s.pending == "" {
		s.pending = req.ID
		s.broadcasts++
	}
	if s.pending != req.ID {
		return txmanager.Result{}, errors.New("unexpected competing intent")
	}
	s.requests = append(s.requests, req)
	return txmanager.Result{ID: req.ID, Hash: common.HexToHash("0xfeed"), Confirmed: !s.reverted, Reverted: s.reverted, Execution: s.execution.Clone()}, nil
}
func (s *recoverySender) Acknowledge(id string) error {
	if s.beforeAck != nil {
		if err := s.beforeAck(id); err != nil {
			return err
		}
	}
	if s.ackFailures > 0 {
		s.ackFailures--
		return errors.New("injected acknowledgement failure")
	}
	if s.pending != id {
		return errors.New("unexpected acknowledgement")
	}
	s.pending = ""
	return nil
}

type recoveryProvider struct {
	calls      int
	done       bool
	outputs    [][]byte
	executions []*provider.Execution
}

func (p *recoveryProvider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("planner stub owns preparation")
}
func (p *recoveryProvider) Advance(_ context.Context, snapshot provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	p.calls++
	p.outputs = append(p.outputs, bytes.Clone(out.Data))
	p.executions = append(p.executions, snapshot.Execution.Clone())
	return provider.Progress{Done: p.done, Reference: "stable-order", State: json.RawMessage(`{"phase":"reconciled"}`)}, nil
}

func TestRecoveryExecutionEvidencePersistsBeforeAcknowledgement(t *testing.T) {
	k, store, _, sender, pr := recoveryHarness(t)
	sender.execution = &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Logs: []types.Log{{Address: common.HexToAddress("0x20"), TxHash: common.HexToHash("0xfeed"), BlockNumber: 10, BlockHash: common.HexToHash("0x10"), Data: []byte{1, 2, 3}}}}
	sender.ackFailures = 1
	sender.beforeAck = func(id string) error {
		job := loadRecoveryJob(t, store, "cow")
		if job.ID != id || job.Phase != "authorized" || job.Result.Execution == nil || !bytes.Equal(job.Result.Execution.Logs[0].Data, []byte{1, 2, 3}) {
			return errors.New("acknowledgement preceded durable execution evidence")
		}
		return nil
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("expected acknowledgement failure")
	}
	if pr.calls != 0 {
		t.Fatal("provider advanced before durable acknowledgement")
	}
	sender.execution = nil
	restarted := *k
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(pr.executions) != 1 || pr.executions[0] == nil || pr.executions[0].TxHash != common.HexToHash("0xfeed") || !bytes.Equal(pr.executions[0].Logs[0].Data, []byte{1, 2, 3}) || len(sender.requests) != 1 {
		t.Fatal("restart did not reconcile persisted execution evidence")
	}
}

func TestConfirmedSnapshotRejectsFutureAndOrphanedExecution(t *testing.T) {
	for _, mode := range []string{"future", "orphaned", "reorg-after-advance"} {
		t.Run(mode, func(t *testing.T) {
			c := newConfirmationChain()
			p := &Planner{Chain: c, Confirmations: 2}
			execution := &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), BlockHash: c.canonical[10].Hash()}
			original := provider.Snapshot{ChainID: big.NewInt(1), Account: common.HexToAddress("0x20"), Execution: execution}
			switch mode {
			case "future":
				execution.BlockNumber = big.NewInt(11)
				execution.BlockHash = c.canonical[11].Hash()
			case "orphaned":
				execution.BlockHash = common.HexToHash("0xbad")
			}
			snapshot, err := p.ConfirmedSnapshot(context.Background(), original)
			if mode != "reorg-after-advance" {
				if err == nil {
					t.Fatal("invalid receipt allowed into Advance snapshot")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			c.canonical[10] = types.CopyHeader(c.canonical[10])
			c.canonical[10].Extra = []byte("reorganized receipt")
			if err := p.VerifyConfirmedSnapshot(context.Background(), snapshot); err == nil {
				t.Fatal("orphaned receipt could advance durable provider state")
			}
		})
	}
}

func recoveryHarness(t *testing.T) (*Keeper, *recoveryStore, *recoveryPlanner, *recoverySender, *recoveryProvider) {
	t.Helper()
	disk, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = disk.Close() })
	store := &recoveryStore{Store: disk}
	planner := &recoveryPlanner{}
	sender := &recoverySender{}
	pr := &recoveryProvider{done: true}
	strategy := Strategy{Config: config.Strategy{Name: "cow", Executor: "0x0000000000000000000000000000000000000010", Account: "0x0000000000000000000000000000000000000020", Tree: "approved.json", Interval: time.Hour}, Providers: []provider.Provider{pr}}
	k := &Keeper{Store: store, Planner: planner, Sender: sender, Strategies: []Strategy{strategy}, Mode: "send", OperationTimeout: time.Second, Log: logr.Discard()}
	return k, store, planner, sender, pr
}
func loadRecoveryJob(t *testing.T, s *recoveryStore, name string) Job {
	t.Helper()
	var job Job
	found, err := s.Load(key(name), &job)
	if err != nil || !found {
		t.Fatalf("load job: %v %v", found, err)
	}
	return job
}

func TestRecoveryFailedAuthorizationSaveRetainsExactTransaction(t *testing.T) {
	k, store, planner, sender, pr := recoveryHarness(t)
	store.failPhase = "authorized"
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("injected authorization persistence failure ignored")
	}
	job := loadRecoveryJob(t, store, "cow")
	if job.Phase != "prepared" || sender.PendingID() != job.ID || pr.calls != 0 {
		t.Fatalf("unsafe failure boundary: %+v calls=%d", job, pr.calls)
	}
	restarted := *k
	if err := restarted.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 2 || sender.broadcasts != 1 || sender.requests[0].ID != sender.requests[1].ID || !bytes.Equal(sender.requests[0].Data, sender.requests[1].Data) {
		t.Fatal("restart replaced already confirmed transaction")
	}
	if planner.prepared != 1 || planner.validated != 1 || pr.calls != 1 || loadRecoveryJob(t, store, "cow").Phase != "settled" {
		t.Fatal("restart did not resume retained job")
	}
}

func TestRecoveryAcknowledgementFollowsDurableAuthorization(t *testing.T) {
	k, store, _, sender, pr := recoveryHarness(t)
	sender.ackFailures = 1
	sender.beforeAck = func(id string) error {
		job := loadRecoveryJob(t, store, "cow")
		if job.ID != id || job.Phase != "authorized" {
			return errors.New("acknowledgement preceded authorized checkpoint")
		}
		return nil
	}
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("injected acknowledgement failure ignored")
	}
	if loadRecoveryJob(t, store, "cow").Phase != "authorized" || pr.calls != 0 {
		t.Fatal("external progress preceded acknowledgement")
	}
	restarted := *k
	if err := restarted.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 1 || pr.calls != 1 || sender.PendingID() != "" {
		t.Fatal("acknowledgement recovery resubmitted recipe")
	}
}

func TestRecoveryProviderPersistenceFailureReusesDeterministicOutput(t *testing.T) {
	k, store, planner, sender, pr := recoveryHarness(t)
	store.failDone = true
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("injected provider persistence failure ignored")
	}
	job := loadRecoveryJob(t, store, "cow")
	if job.Phase != "authorized" || job.Progress[0].Done {
		t.Fatal("failed save committed completion")
	}
	restarted := *k
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pr.calls != 2 || !bytes.Equal(pr.outputs[0], pr.outputs[1]) || planner.prepared != 1 || sender.broadcasts != 1 {
		t.Fatal("recovery changed external operation identity")
	}
	if loadRecoveryJob(t, store, "cow").Phase != "settled" {
		t.Fatal("provider recovery did not finish")
	}
}

func TestRecoveryReorgCannotCommitProviderCompletion(t *testing.T) {
	k, store, planner, _, _ := recoveryHarness(t)
	planner.verifyErr = errors.New("snapshot reorganized")
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("provider snapshot reorg ignored")
	}
	job := loadRecoveryJob(t, store, "cow")
	if job.Phase != "authorized" || job.Progress[0].Done {
		t.Fatal("reorganized provider result was committed")
	}
}

func TestRecoveryConfirmedRevertRemainsHalted(t *testing.T) {
	k, store, planner, sender, pr := recoveryHarness(t)
	sender.reverted = true
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("confirmed revert treated as success")
	}
	restarted := *k
	if err := restarted.Tick(context.Background()); err == nil {
		t.Fatal("failed strategy restarted automatically")
	}
	if job := loadRecoveryJob(t, store, "cow"); job.Phase != "failed" || !job.Result.Reverted || planner.prepared != 1 || sender.broadcasts != 1 || pr.calls != 0 {
		t.Fatal("reverted operation was retried or advanced")
	}
}

func TestRecoveryRemovedExternalPendingStrategyIsRejected(t *testing.T) {
	k, store, _, sender, pr := recoveryHarness(t)
	pr.done = false
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if loadRecoveryJob(t, store, "cow").Phase != "authorized" || sender.PendingID() != "" {
		t.Fatal("expected external-only pending lifecycle")
	}
	restarted := *k
	replacement := k.Strategies[0]
	replacement.Config.Name = "replacement"
	restarted.Strategies = []Strategy{replacement}
	if err := restarted.CheckRecovery(); err == nil {
		t.Fatal("removed strategy orphaned external-pending work")
	}
}

func TestRecoveryAccountReservationIgnoresAddressLetterCase(t *testing.T) {
	k, _, planner, sender, pr := recoveryHarness(t)
	pr.done = false
	k.Strategies[0].Config.Account = strings.ToLower(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Hex())
	other := k.Strategies[0]
	other.Config.Name = "other"
	k.Strategies = append(k.Strategies, other)
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if planner.prepared != 1 || sender.broadcasts != 1 {
		t.Fatal("lowercase account bypassed external-pending reservation")
	}
}

func TestRecoveryRemovedTerminalOwnerCannotLeaveSenderOrphaned(t *testing.T) {
	k, _, _, sender, _ := recoveryHarness(t)
	sender.reverted = true
	sender.ackFailures = 1
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("expected acknowledgement failure")
	}
	if sender.PendingID() == "" {
		t.Fatal("expected retained terminal transaction")
	}
	restarted := *k
	replacement := k.Strategies[0]
	replacement.Config.Name = "replacement"
	restarted.Strategies = []Strategy{replacement}
	if err := restarted.CheckRecovery(); err == nil {
		t.Fatal("removed terminal owner passed startup with occupied sender")
	}
}

func TestRecoveryUnsignedPreparedDataIsRefreshedBeforeSigning(t *testing.T) {
	k, store, planner, sender, _ := recoveryHarness(t)
	strategy := k.Strategies[0]
	old, err := planner.Prepare(context.Background(), strategy)
	if err != nil {
		t.Fatal(err)
	}
	old.Calldata = []byte{9, 9, 9}
	job := Job{ID: "expired-unsigned-order", ConfigHash: strategy.Config.Fingerprint(), Phase: "prepared", Plan: old, Progress: make([]provider.Progress, len(strategy.Providers))}
	if err := store.Save(key(strategy.Config.Name), job); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if planner.prepared != 2 || len(sender.requests) != 1 || bytes.Equal(sender.requests[0].Data, old.Calldata) {
		t.Fatal("stale unsigned runtime data was signed even though it still simulated")
	}
}

type confirmationChain struct {
	canonical map[uint64]*types.Header
	byHash    map[common.Hash]*types.Header
	head      *types.Header
	onNumber  func(*big.Int)
}

func newConfirmationChain() *confirmationChain {
	c := &confirmationChain{canonical: map[uint64]*types.Header{}, byHash: map[common.Hash]*types.Header{}}
	for i := uint64(10); i <= 12; i++ {
		h := &types.Header{Number: new(big.Int).SetUint64(i), Time: i * 10}
		if c.head != nil {
			h.ParentHash = c.head.Hash()
		}
		c.canonical[i] = h
		c.byHash[h.Hash()] = h
		c.head = h
	}
	return c
}
func (c *confirmationChain) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if c.onNumber != nil {
		c.onNumber(n)
	}
	h := c.head
	if n != nil {
		h = c.canonical[n.Uint64()]
	}
	if h == nil {
		return nil, nil
	}
	return types.CopyHeader(h), nil
}
func (c *confirmationChain) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	h := c.byHash[hash]
	if h == nil {
		return nil, ethereum.NotFound
	}
	return types.CopyHeader(h), nil
}
func (c *confirmationChain) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, errors.New("unexpected contract call")
}
func (c *confirmationChain) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return nil, errors.New("unexpected log query")
}

func TestConfirmedSnapshotRequiresExactDescendantDepth(t *testing.T) {
	c := newConfirmationChain()
	p := &Planner{Chain: c, Confirmations: 2}
	s, err := p.ConfirmedSnapshot(context.Background(), provider.Snapshot{Account: common.HexToAddress("0x20")})
	if err != nil || s.BlockNumber.Cmp(big.NewInt(10)) != 0 || s.BlockHash != c.canonical[10].Hash() || s.Timestamp != 100 {
		t.Fatalf("confirmation snapshot: %+v %v", s, err)
	}
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	c.canonical[12] = types.CopyHeader(c.canonical[12])
	c.canonical[12].Extra = []byte("fork")
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err != nil {
		t.Fatal("same confirmed ancestor should survive tip-only reorg", err)
	}
	c.canonical[12].ParentHash = common.HexToHash("0xbad")
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err == nil {
		t.Fatal("unrelated confirmation witness accepted")
	}
}

func TestConfirmedSnapshotRejectsHeadReorgAndMalformedHeaders(t *testing.T) {
	for _, mode := range []string{"nil", "wrong-number", "fork-during-read", "wrong-parent"} {
		t.Run(mode, func(t *testing.T) {
			c := newConfirmationChain()
			switch mode {
			case "nil":
				c.head = nil
			case "wrong-number":
				c.canonical[12] = &types.Header{Number: big.NewInt(11)}
			case "fork-during-read":
				c.onNumber = func(n *big.Int) {
					if n != nil {
						h := types.CopyHeader(c.canonical[12])
						h.Extra = []byte("new canonical witness")
						c.canonical[12] = h
					}
				}
			case "wrong-parent":
				c.head.ParentHash = common.HexToHash("0xbad")
			}
			p := &Planner{Chain: c, Confirmations: 2}
			if _, err := p.ConfirmedSnapshot(context.Background(), provider.Snapshot{}); err == nil {
				t.Fatal("invalid confirmation snapshot accepted")
			}
		})
	}
}
