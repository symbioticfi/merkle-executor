// Package keeper prepares recipes and owns their process-local execution lifecycle.
package keeper

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/transaction"
)

// Sender owns at most one transaction and returns canonical execution evidence.
// Submit errors may leave Busy true: ambiguous submission retains ownership.
type Sender interface {
	Submit(context.Context, transaction.Request) error
	Poll(context.Context) (*transaction.Result, error)
	Busy() bool
}
type Planning interface {
	PrepareExecution(context.Context, []Strategy) (ExecutionPlan, error)
	ConfirmedSnapshot(context.Context, provider.Snapshot) (provider.Snapshot, error)
	VerifyConfirmedSnapshot(context.Context, provider.Snapshot) error
}

type settlement struct {
	plan      Plan
	execution *provider.Execution
	done      [][]bool
}

// Keeper is driven explicitly by the host. It never writes state or starts workers.
// Configuration is fixed for its lifetime; use one Keeper per sender.
type Keeper struct {
	Sender           Sender
	Planner          Planning
	Strategies       []Strategy
	Mode             string
	OperationTimeout time.Duration
	Log              logr.Logger
	Observe          func(strategy, outcome string)

	mu          sync.Mutex
	active      *ExecutionPlan
	settlements map[string]*settlement
	next        map[string]time.Time
	halted      map[string]bool
}

// Pending reports process-owned transaction or external settlement work.
func (k *Keeper) Pending() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active != nil || len(k.settlements) != 0
}

func (k *Keeper) Tick(ctx context.Context) error {
	if !k.mu.TryLock() {
		return errors.New("keeper iteration already running")
	}
	defer k.mu.Unlock()
	if k.Planner == nil || k.OperationTimeout <= 0 {
		return errors.New("planner and positive operation timeout required")
	}
	names := make(map[string]bool, len(k.Strategies))
	for _, strategy := range k.Strategies {
		name := strategy.Config.Name
		if name == "" || names[name] {
			return errors.New("keeper strategy names must be nonempty and unique")
		}
		names[name] = true
	}
	operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
	defer cancel()
	if k.Mode == "simulate" {
		plan, err := k.Planner.PrepareExecution(operation, k.Strategies)
		var failures []error
		for _, s := range k.Strategies {
			memberErr := err
			if memberErr == nil {
				memberErr = plan.Failures[s.Config.Name]
				if memberErr != nil {
					failures = append(failures, memberErr)
				}
			}
			k.observe(s.Config.Name, memberErr)
		}
		if errors.Is(err, provider.ErrNotReady) {
			return nil
		}
		if err == nil {
			k.Log.Info("execution simulation succeeded", "executors", len(plan.Members))
		}
		return errors.Join(append(failures, err)...)
	}
	if k.Mode != "send" || k.Sender == nil {
		return errors.New("send mode requires a sender")
	}
	if k.next == nil {
		k.next = map[string]time.Time{}
		k.halted = map[string]bool{}
		k.settlements = map[string]*settlement{}
	}
	return k.drain(operation)
}

// Wakeup exposes an optional sender progress hint. Hosts still schedule Tick;
// senders without background work return nil and rely on the host's polling.
func (k *Keeper) Wakeup() <-chan struct{} {
	if sender, ok := k.Sender.(interface{ Wakeup() <-chan struct{} }); ok {
		return sender.Wakeup()
	}
	return nil
}

// drain submits each executor at most once and advances each settlement once.
// Confirmed effects may make idle executors ready; a missing receipt yields.
func (k *Keeper) drain(ctx context.Context) error {
	var failures []error
	visited := map[string]bool{}
	advanced := map[*settlement]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if k.active != nil {
			for _, member := range k.active.Members {
				visited[member.Name] = true
			}
			result, err := k.Sender.Poll(ctx)
			if err != nil {
				failures = append(failures, err)
			} else if result != nil {
				if err = k.complete(result); err != nil {
					failures = append(failures, err)
				}
			}
		} else if k.Sender.Busy() {
			return errors.Join(append(failures, errors.New("sender has work without a keeper owner"))...)
		}
		reserved := map[common.Address]bool{}
		for _, strategy := range k.Strategies {
			work := k.settlements[strategy.Config.Name]
			if work == nil {
				continue
			}
			visited[strategy.Config.Name] = true
			if !advanced[work] {
				advanced[work] = true
				err := k.advance(ctx, work)
				if err == nil {
					delete(k.settlements, strategy.Config.Name)
					k.next[strategy.Config.Name] = time.Now().Add(strategy.Config.Interval)
					k.observe(strategy.Config.Name, nil)
				} else if !errors.Is(err, provider.ErrNotReady) {
					failures = append(failures, err)
					k.observe(strategy.Config.Name, err)
				}
			}
			if k.settlements[strategy.Config.Name] != nil {
				reserved[work.plan.Snapshot.Account] = true
			}
		}
		// Earlier external tasks progress even while another account owns the signer.
		if k.active != nil {
			return errors.Join(failures...)
		}
		var candidates []Strategy
		for _, strategy := range k.Strategies {
			if !visited[strategy.Config.Name] && !k.halted[strategy.Config.Name] && !reserved[common.HexToAddress(strategy.Config.Account)] && !time.Now().Before(k.next[strategy.Config.Name]) {
				candidates = append(candidates, strategy)
			}
		}
		if len(candidates) == 0 {
			return errors.Join(failures...)
		}
		plan, err := k.Planner.PrepareExecution(ctx, candidates)
		if err != nil {
			for _, strategy := range candidates {
				k.observe(strategy.Config.Name, err)
			}
			if !errors.Is(err, provider.ErrNotReady) {
				failures = append(failures, err)
			}
			return errors.Join(failures...)
		}
		for _, strategy := range candidates {
			if failure := plan.Failures[strategy.Config.Name]; failure != nil {
				visited[strategy.Config.Name] = true
				k.observe(strategy.Config.Name, failure)
				failures = append(failures, failure)
			}
		}
		if err = validateExecutionComposition(plan, candidates); err != nil {
			return errors.Join(append(failures, err)...)
		}
		to, data, err := plan.Transaction()
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		// Capture ownership before submission: a timeout can follow a successful send.
		frozen := freezeExecution(plan)
		k.active = &frozen
		for _, member := range frozen.Members {
			visited[member.Name] = true
		}
		if err = k.Sender.Submit(ctx, transaction.Request{To: to, Data: data}); err != nil {
			if !k.Sender.Busy() {
				k.active = nil
			}
			return errors.Join(append(failures, err)...)
		}
	}
}

func (k *Keeper) complete(result *transaction.Result) error {
	if result.Hash == (common.Hash{}) || result.Execution == nil || result.Execution.TxHash != result.Hash {
		return errors.New("sender returned invalid execution identity")
	}
	if err := result.Execution.Validate(); err != nil {
		return err
	}
	for _, member := range k.active.Members {
		if result.Reverted {
			k.halted[member.Name] = true
			k.observe(member.Name, errors.New("execution reverted"))
			continue
		}
		work := &settlement{plan: member.Plan, execution: result.Execution.Clone(), done: make([][]bool, len(member.Plan.Outputs))}
		for i, group := range member.Plan.Outputs {
			work.done[i] = make([]bool, len(group))
		}
		k.settlements[member.Name] = work
	}
	k.active = nil
	if result.Reverted {
		return errors.New("executor transaction reverted; selected executors halted for this process")
	}
	return nil
}

func (k *Keeper) advance(ctx context.Context, work *settlement) error {
	hasTask := false
	for _, group := range work.plan.Outputs {
		for _, out := range group {
			hasTask = hasTask || out.Task != nil
		}
	}
	if !hasTask {
		return nil
	}
	original := work.plan.Snapshot
	original.Execution = work.execution.Clone()
	snapshot, err := k.Planner.ConfirmedSnapshot(ctx, original)
	if err != nil {
		return err
	}
	done := true
	for i, group := range work.plan.Outputs {
		for j, out := range group {
			if out.Task == nil || work.done[i][j] {
				continue
			}
			progress, err := out.Task.Advance(ctx, cloneSnapshot(snapshot))
			if err != nil {
				return err
			}
			if err = k.Planner.VerifyConfirmedSnapshot(ctx, snapshot); err != nil {
				return err
			}
			work.done[i][j] = progress.Done
			if progress.Done {
				k.Log.Info("external task finished", "executor", snapshot.Executor.Hex(), "outcome", progress.Outcome, "reference", progress.Reference)
			}
			done = done && progress.Done
		}
	}
	if !done {
		return provider.ErrNotReady
	}
	return nil
}

func (k *Keeper) observe(name string, err error) {
	outcome := "ok"
	if errors.Is(err, provider.ErrNotReady) {
		outcome = "idle"
	} else if err != nil {
		outcome = "error"
		k.Log.Error(err, "strategy iteration failed", "strategy", name)
	}
	if k.Observe != nil {
		k.Observe(name, outcome)
	}
}
