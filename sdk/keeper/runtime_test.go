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
			return ExecutionPlan{To: plan.Snapshot.Executor, Members: []PlanMember{{Name: s.Config.Name, Plan: plan}}}, nil
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
	submitErr, pollErr error
}

func (s *runtimeSender) Submit(_ context.Context, r transaction.Request) error {
	s.requests = append(s.requests, r)
	s.busy = true
	return s.submitErr
}
func (s *runtimeSender) Poll(context.Context) (*transaction.Result, error) {
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
