package keeper

import (
	"context"
	"testing"
	"time"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/transaction"
)

type settlementTimeoutPlanner struct{ *runtimePlanner }

func (p settlementTimeoutPlanner) PrepareExecution(ctx context.Context, strategies []Strategy) (ExecutionPlan, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionPlan{}, err
	}
	return p.runtimePlanner.PrepareExecution(ctx, strategies)
}

type settlementTimeoutSender struct {
	runtimeSender
	timeout time.Duration
}

func (s *settlementTimeoutSender) Poll(ctx context.Context) (*transaction.ExecutionResult, error) {
	s.polls++
	deadline, bounded := ctx.Deadline()
	if !bounded || time.Until(deadline) > s.timeout+time.Millisecond {
		return nil, errors.New("poll is missing its operation timeout")
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRAMSettlementTimeoutDoesNotStarveOtherAccounts(t *testing.T) {
	for _, pending := range []bool{true, false} {
		name := "new execution"
		if pending {
			name = "existing settlement"
		}
		t.Run(name, func(t *testing.T) {
			k, planner, _ := runtimeHarness(t)
			k.OperationTimeout = 20 * time.Millisecond
			addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
			k.Planner = settlementTimeoutPlanner{planner}
			sender := &immediateSender{}
			k.Sender = sender
			slow, healthy := 0, 0
			first := planner.plans["one"]
			first.Outputs[0][0].Task = provider.TaskFunc(func(ctx context.Context, _ provider.Snapshot) (provider.Progress, error) {
				slow++
				deadline, bounded := ctx.Deadline()
				if !bounded || time.Until(deadline) > k.OperationTimeout+time.Millisecond {
					t.Fatal("settlement is missing its operation timeout")
				}
				<-ctx.Done()
				return provider.Progress{}, ctx.Err()
			})
			second := planner.plans["two"]
			second.Outputs[0][0].Task = provider.TaskFunc(func(ctx context.Context, _ provider.Snapshot) (provider.Progress, error) {
				if err := ctx.Err(); err != nil {
					return provider.Progress{}, err
				}
				healthy++
				return provider.Progress{Done: true}, nil
			})
			planner.plans["two"] = second
			k.next = map[string]time.Time{}
			k.halted = map[string]bool{}
			k.settlements = map[string]*settlement{
				"one": {plan: first, execution: confirmedResult().Execution, done: [][]bool{{false}}},
			}
			if pending {
				k.settlements["two"] = &settlement{plan: second, execution: confirmedResult().Execution, done: [][]bool{{false}}}
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			for range 3 {
				if err := k.Tick(ctx); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("settlement timeout was lost: %v", err)
				}
			}
			if healthy != 1 || slow != 3 || len(k.settlements) != 1 || k.settlements["one"] == nil {
				t.Fatalf("timed-out task blocked another account or lost ownership: slow=%d healthy=%d pending=%d", slow, healthy, len(k.settlements))
			}
			wantSubmissions := 1
			if pending {
				wantSubmissions = 0
			}
			if len(sender.requests) != wantSubmissions {
				t.Fatalf("submissions=%d, want %d", len(sender.requests), wantSubmissions)
			}
		})
	}
}

func TestRAMSettlementTimeoutPreservesParentCancellation(t *testing.T) {
	k, planner, sender := runtimeHarness(t)
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	k.Planner = settlementTimeoutPlanner{planner}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first := planner.plans["one"]
	first.Outputs[0][0].Task = provider.TaskFunc(func(context.Context, provider.Snapshot) (provider.Progress, error) {
		cancel()
		return provider.Progress{}, ctx.Err()
	})
	k.next = map[string]time.Time{}
	k.halted = map[string]bool{}
	k.settlements = map[string]*settlement{
		"one": {plan: first, execution: confirmedResult().Execution, done: [][]bool{{false}}},
	}
	if err := k.Tick(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("parent cancellation was lost: %v", err)
	}
	if len(sender.requests) != 0 || len(planner.visits) != 0 || !k.Pending() {
		t.Fatal("parent cancellation started other work or discarded settlement ownership")
	}
}

func TestRAMPollTimeoutDoesNotStarveExternalSettlement(t *testing.T) {
	k, planner, _ := runtimeHarness(t)
	k.OperationTimeout = 20 * time.Millisecond
	addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
	sender := &settlementTimeoutSender{runtimeSender: runtimeSender{busy: true}, timeout: k.OperationTimeout}
	k.Sender = sender
	healthy := 0
	first := planner.plans["one"]
	first.Outputs[0][0].Task = provider.TaskFunc(func(ctx context.Context, _ provider.Snapshot) (provider.Progress, error) {
		if err := ctx.Err(); err != nil {
			return provider.Progress{}, err
		}
		healthy++
		return provider.Progress{Done: true}, nil
	})
	second := planner.plans["two"]
	k.active = &ExecutionPlan{To: second.Snapshot.Executor, Members: []PlanMember{{Name: "two", Plan: second}}}
	k.next = map[string]time.Time{}
	k.halted = map[string]bool{}
	k.settlements = map[string]*settlement{
		"one": {plan: first, execution: confirmedResult().Execution, done: [][]bool{{false}}},
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := k.Tick(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("poll timeout was lost: %v", err)
	}
	if healthy != 1 || len(k.settlements) != 0 {
		t.Fatalf("poll timeout blocked another account: healthy=%d pending=%d", healthy, len(k.settlements))
	}
	if !sender.busy || k.active == nil || !k.Pending() || sender.polls != 1 || len(sender.requests) != 0 || len(planner.visits) != 0 {
		t.Fatal("poll timeout changed signed ownership or started new work")
	}
}
