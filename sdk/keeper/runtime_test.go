package keeper

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/transaction"
)

type testProvider struct{}

func (*testProvider) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return RecipeRequirements{}, nil
}
func (*testProvider) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) ([]provider.Output, error) {
	return []provider.Output{{}}, nil
}

type runtimePlanner struct {
	plans         map[string]Plan
	visits        [][]string
	failures      map[string]error
	err, guardErr error
}

func (p *runtimePlanner) PrepareExecution(_ context.Context, strategies []Strategy) (ExecutionPlan, error) {
	var names []string
	for _, s := range strategies {
		names = append(names, s.Config.Name)
	}
	p.visits = append(p.visits, names)
	if p.err != nil {
		return ExecutionPlan{}, p.err
	}
	for _, s := range strategies {
		if plan, ok := p.plans[s.Config.Name]; ok {
			return ExecutionPlan{To: plan.Snapshot.Executor, Members: []PlanMember{{Name: s.Config.Name, Plan: plan}}, Failures: p.failures}, nil
		}
	}
	return ExecutionPlan{}, provider.ErrNotReady
}
func (*runtimePlanner) ConfirmedSnapshot(_ context.Context, s provider.Snapshot) (provider.Snapshot, error) {
	return s, nil
}
func (p *runtimePlanner) VerifyConfirmedSnapshot(context.Context, provider.Snapshot) error {
	return p.guardErr
}

type runtimeSender struct {
	busy               bool
	requests           []transaction.Request
	result             *transaction.Result
	polls              int
	submitErr, pollErr error
}

func (s *runtimeSender) Submit(_ context.Context, r transaction.Request) error {
	s.requests = append(s.requests, r)
	s.busy = true
	return s.submitErr
}
func (s *runtimeSender) Poll(context.Context) (*transaction.Result, error) {
	s.polls++
	if s.pollErr != nil {
		return nil, s.pollErr
	}
	if s.result != nil {
		s.busy = false
		r := s.result
		s.result = nil
		return r, nil
	}
	return nil, nil
}

type immediateSender struct {
	runtimeSender
	afterSubmit func()
	afterPoll   func()
}

func (s *immediateSender) Submit(ctx context.Context, request transaction.Request) error {
	err := s.runtimeSender.Submit(ctx, request)
	s.result = confirmedResult()
	if s.afterSubmit != nil {
		s.afterSubmit()
	}
	return err
}

func (s *immediateSender) Poll(ctx context.Context) (*transaction.Result, error) {
	result, err := s.runtimeSender.Poll(ctx)
	if result != nil && s.afterPoll != nil {
		s.afterPoll()
	}
	return result, err
}

func addRuntimeStrategy(t *testing.T, k *Keeper, planner *runtimePlanner, name string, account, executor byte) {
	t.Helper()
	strategy := k.Strategies[0]
	strategy.Config.Name = name
	strategy.Config.Account = common.Address{account}.Hex()
	strategy.Config.Executor = common.Address{executor}.Hex()
	plan := freezeExecution(ExecutionPlan{Members: []PlanMember{{Plan: planner.plans["one"]}}}).Members[0].Plan
	plan.Snapshot.Account, plan.Recipe.Account = common.Address{account}, common.Address{account}
	plan.Snapshot.Executor, plan.Recipe.Executor = common.Address{executor}, common.Address{executor}
	k.Strategies = append(k.Strategies, strategy)
	planner.plans[name] = plan
}

func TestRAMDrainsImmediatelyConfirmedExecutorsInOneTick(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Hour} {
		t.Run(interval.String(), func(t *testing.T) {
			k, planner, _ := runtimeHarness(t)
			k.Strategies[0].Config.Interval = interval
			addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
			addRuntimeStrategy(t, k, planner, "three", 0x12, 0x22)
			sender := &immediateSender{}
			k.Sender = sender
			if err := k.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(sender.requests) != 3 || sender.polls != 3 || k.Pending() {
				t.Fatalf("sends=%d polls=%d pending=%v", len(sender.requests), sender.polls, k.Pending())
			}
			for i, name := range []string{"one", "two", "three"} {
				if len(planner.visits[i]) == 0 || planner.visits[i][0] != name {
					t.Fatalf("priority changed: %v", planner.visits)
				}
			}
		})
	}
}

func TestRAMDrainRechecksIdleExecutorAfterConfirmedEffects(t *testing.T) {
	k, planner, _ := runtimeHarness(t)
	k.Strategies[0].Config.Interval = 0
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	first := planner.plans["one"]
	delete(planner.plans, "one")
	sender := &immediateSender{}
	sender.afterPoll = func() { planner.plans["one"] = first }
	k.Sender = sender
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 2 || sender.polls != 2 || k.Pending() {
		t.Fatalf("sends=%d polls=%d pending=%v", len(sender.requests), sender.polls, k.Pending())
	}
	if sender.requests[0].To != planner.plans["two"].Snapshot.Executor || sender.requests[1].To != first.Snapshot.Executor {
		t.Fatal("idle executor did not run after the other executor confirmed")
	}
	if len(planner.visits) != 2 || len(planner.visits[1]) != 1 || planner.visits[1][0] != "one" {
		t.Fatalf("confirmed executor was retried or idle executor was skipped: %v", planner.visits)
	}
}

func TestRAMDrainStopsOnPendingWithoutRepeatedPolling(t *testing.T) {
	k, planner, sender := runtimeHarness(t)
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 1 || sender.polls != 1 || len(planner.visits) != 1 || !k.Pending() {
		t.Fatalf("pending work spun: sends=%d polls=%d prepares=%d", len(sender.requests), sender.polls, len(planner.visits))
	}
}

func TestRAMDrainCancellationRetainsSubmission(t *testing.T) {
	k, planner, _ := runtimeHarness(t)
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sender := &immediateSender{afterSubmit: cancel}
	k.Sender = sender
	if err := k.Tick(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if len(sender.requests) != 1 || sender.polls != 0 || !k.Pending() {
		t.Fatal("canceled drain consumed or replaced signed ownership")
	}
}

func TestRAMReportsFailedCandidateWithoutBlockingHealthyWork(t *testing.T) {
	for _, mode := range []string{"send", "simulate"} {
		t.Run(mode, func(t *testing.T) {
			k, planner, _ := runtimeHarness(t)
			k.Mode = mode
			addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
			delete(planner.plans, "two")
			failure := errors.New("second executor unavailable")
			planner.failures = map[string]error{"two": failure}
			sender := &immediateSender{}
			k.Sender = sender
			outcomes := map[string]string{}
			k.Observe = func(name, outcome string) { outcomes[name] = outcome }
			if err := k.Tick(t.Context()); !errors.Is(err, failure) {
				t.Fatalf("candidate failure lost: %v", err)
			}
			if outcomes["one"] != "ok" || outcomes["two"] != "error" {
				t.Fatalf("incorrect member outcomes: %v", outcomes)
			}
			wantSends := 1
			if mode == "simulate" {
				wantSends = 0
			}
			if len(sender.requests) != wantSends || len(planner.visits) != 1 || k.Pending() {
				t.Fatalf("healthy work blocked or candidate retried: sends=%d prepares=%d pending=%v", len(sender.requests), len(planner.visits), k.Pending())
			}
		})
	}
}

func TestRAMDrainAdvancesWaitingExternalTaskOnce(t *testing.T) {
	k, planner, pending := runtimeHarness(t)
	plan := planner.plans["one"]
	calls := 0
	plan.Outputs[0][0].Task = provider.TaskFunc(func(context.Context, provider.Snapshot) (provider.Progress, error) {
		calls++
		return provider.Progress{}, nil
	})
	planner.plans["one"] = plan
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	addRuntimeStrategy(t, k, planner, "three", 0x12, 0x22)
	for _, name := range []string{"two", "three"} {
		planner.plans[name].Outputs[0][0].Task = nil
	}
	sender := &immediateSender{runtimeSender: *pending}
	sender.result = confirmedResult()
	k.Sender = sender
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(sender.requests) != 3 || !k.Pending() {
		t.Fatalf("task calls=%d sends=%d pending=%v", calls, len(sender.requests), k.Pending())
	}
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(sender.requests) != 3 {
		t.Fatal("waiting task was starved or repeated during a drain")
	}
}
func (s *runtimeSender) Busy() bool { return s.busy }
func runtimeHarness(t *testing.T) (*Keeper, *runtimePlanner, *runtimeSender) {
	t.Helper()
	account, executor, caller := common.HexToAddress("0x10"), common.HexToAddress("0x20"), common.HexToAddress("0x30")
	plan := Plan{Snapshot: provider.Snapshot{Account: account, Executor: executor, Caller: caller}, Recipe: recipes.Snapshot{Account: account, Executor: executor, Caller: caller}, Calldata: compositionCalldata(t, common.HexToAddress("0x40")), Outputs: [][]provider.Output{{{}}}}
	p := &runtimePlanner{plans: map[string]Plan{"one": plan}}
	s := &runtimeSender{}
	k := &Keeper{Planner: p, Sender: s, Mode: "send", OperationTimeout: time.Second, Log: logr.Discard(), Strategies: []Strategy{{Config: Config{Name: "one", Executor: executor.Hex(), Account: account.Hex(), Interval: time.Hour}, Providers: []Provider{&testProvider{}}}}}
	return k, p, s
}
func confirmedResult() *transaction.Result {
	hash := common.HexToHash("0x123")
	return &transaction.Result{Hash: hash, Execution: &provider.Execution{TxHash: hash, BlockNumber: big.NewInt(4), BlockHash: common.HexToHash("0x456")}}
}
func TestRAMRejectsAmbiguousStrategyNamesBeforeWork(t *testing.T) {
	for _, mode := range []string{"send", "simulate"} {
		for _, name := range []string{"empty", "duplicate"} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				k, p, sender := runtimeHarness(t)
				k.Mode = mode
				if name == "empty" {
					k.Strategies[0].Config.Name = ""
				} else {
					k.Strategies = append(k.Strategies, k.Strategies[0])
				}
				if err := k.Tick(t.Context()); err == nil {
					t.Fatal("ambiguous settlement identity accepted")
				}
				if len(p.visits) != 0 || len(sender.requests) != 0 || k.Pending() {
					t.Fatal("invalid strategy names started work")
				}
			})
		}
	}
}
func TestRAMPendingRetainsOneImmutableExecution(t *testing.T) {
	for _, failure := range []string{"pending", "broadcast timeout", "receipt error"} {
		t.Run(failure, func(t *testing.T) {
			k, p, s := runtimeHarness(t)
			if failure == "broadcast timeout" {
				s.submitErr = context.DeadlineExceeded
			}
			_ = k.Tick(t.Context())
			if failure == "receipt error" {
				s.pollErr = errors.New("RPC unavailable")
			}
			_ = k.Tick(t.Context())
			if !k.Pending() || len(p.visits) != 1 || len(s.requests) != 1 {
				t.Fatalf("lost ownership: pending=%v prepares=%d sends=%d", k.Pending(), len(p.visits), len(s.requests))
			}
		})
	}
}
func TestRAMSettlementReservesAccountAndRetriesTask(t *testing.T) {
	k, p, s := runtimeHarness(t)
	plan := p.plans["one"]
	calls := 0
	plan.Outputs[0][0].Task = provider.TaskFunc(func(_ context.Context, s provider.Snapshot) (provider.Progress, error) {
		calls++
		if s.Execution == nil {
			t.Fatal("task started before canonical authorization")
		}
		return provider.Progress{Done: calls > 1}, nil
	})
	p.plans["one"] = plan
	other := k.Strategies[0]
	other.Config.Name = "two"
	other.Config.Executor = common.HexToAddress("0x21").Hex()
	k.Strategies = append(k.Strategies, other)
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("published before confirmation")
	}
	s.result = confirmedResult()
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(p.visits) != 1 || !k.Pending() {
		t.Fatalf("reservation lost: calls=%d prepares=%d", calls, len(p.visits))
	}
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || k.Pending() {
		t.Fatalf("task did not settle: %d", calls)
	}
}
func TestRAMRevertHaltsEverySelectedMember(t *testing.T) {
	k, p, s := runtimeHarness(t)
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.result = confirmedResult()
	s.result.Reverted = true
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("revert hidden")
	}
	_ = k.Tick(t.Context())
	if len(p.visits) != 1 || len(s.requests) != 1 || k.Pending() {
		t.Fatal("reverted recipe repeated")
	}
}
func TestRAMIdleAndInvalidPreparationRemainDistinct(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{provider.ErrNotReady, "idle"}, {errors.New("bad proof"), "error"}} {
		k, p, s := runtimeHarness(t)
		p.err = tc.err
		outcome := ""
		k.Observe = func(_, o string) { outcome = o }
		err := k.Tick(t.Context())
		if (err == nil) != (tc.want == "idle") || outcome != tc.want || len(s.requests) != 0 {
			t.Fatalf("outcome=%s err=%v", outcome, err)
		}
	}
}

func TestRAMPendingTransactionDoesNotStarveExternalSettlement(t *testing.T) {
	k, p, sender := runtimeHarness(t)
	plan := p.plans["one"]
	calls := 0
	plan.Outputs[0][0].Task = provider.TaskFunc(func(context.Context, provider.Snapshot) (provider.Progress, error) {
		calls++
		return provider.Progress{}, nil
	})
	p.plans["one"] = plan
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	other := k.Strategies[0]
	other.Config.Name = "two"
	other.Config.Account = common.HexToAddress("0x11").Hex()
	other.Config.Executor = common.HexToAddress("0x21").Hex()
	second := freezeExecution(ExecutionPlan{Members: []PlanMember{{Plan: plan}}}).Members[0].Plan
	second.Snapshot.Account, second.Recipe.Account = common.HexToAddress(other.Config.Account), common.HexToAddress(other.Config.Account)
	second.Snapshot.Executor, second.Recipe.Executor = common.HexToAddress(other.Config.Executor), common.HexToAddress(other.Config.Executor)
	second.Outputs[0][0].Task = nil
	p.plans["two"] = second
	k.Strategies = append(k.Strategies, other)
	sender.result = confirmedResult()
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 2 || calls != 1 {
		t.Fatal("second account not admitted")
	}
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(sender.requests) != 2 {
		t.Fatal("pending signer starved earlier external task")
	}
}
