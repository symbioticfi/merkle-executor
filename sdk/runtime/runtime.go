package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Planning interface {
	PrepareAll(context.Context, []Strategy) []PreparationResult
	Validate(context.Context, Plan) error
	ConfirmedSnapshot(context.Context, chainio.Snapshot) (chainio.Snapshot, error)
	VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error
}

type driverWorker struct {
	done chan struct{}
	err  error
}

// Runtime owns workflows and the signer lane for this process only.
type Runtime struct {
	Planner          Planning
	Driver           Driver
	Strategies       []Strategy
	Mode             string
	OperationTimeout time.Duration
	ShutdownTimeout  time.Duration
	MaxConcurrent    int
	Log              logr.Logger
	Observe          func(strategy, outcome string)
	ChainID          *big.Int
	Caller           common.Address

	tickMu          sync.Mutex
	mu              sync.Mutex
	started, closed bool
	process         context.Context
	cancel          context.CancelFunc
	workflows       map[string]Workflow
	strategies      map[string]Strategy
	accounts        map[common.Address]string
	owner           string
	worker          *driverWorker
	blockedErr      error
	next            int
}

func (r *Runtime) putLocked(w Workflow) {
	r.workflows[w.Name] = w.Clone()
	if w.ownsAccount() {
		r.accounts[w.Plan.Snapshot.Account] = w.Name
	} else if r.accounts[w.Plan.Snapshot.Account] == w.Name {
		delete(r.accounts, w.Plan.Snapshot.Account)
	}
}

func (r *Runtime) Snapshot() []Workflow {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Workflow, 0, len(r.workflows))
	for _, w := range r.workflows {
		out = append(out, w.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Runtime) global(err error) error {
	if r.Observe != nil {
		r.Observe("_global", "error")
	}
	r.Log.Error(errors.New("runtime iteration failed"), "runtime iteration failed")
	return err
}

// Start validates the current configuration and creates empty process-local state.
func (r *Runtime) Start(ctx context.Context) error {
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	return r.start(ctx)
}

func (r *Runtime) start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("runtime closed")
	}
	if r.started {
		return nil
	}
	if r.Planner == nil || r.ChainID == nil || r.ChainID.Sign() <= 0 || r.ChainID.BitLen() > 256 || r.Caller == (common.Address{}) || r.OperationTimeout <= 0 || (r.Mode != "send" && r.Mode != "simulate") || r.Mode == "send" && r.Driver == nil {
		return errors.New("runtime dependencies, identity, mode and timeout required")
	}
	strategies := make(map[string]Strategy, len(r.Strategies))
	for _, s := range r.Strategies {
		if s.Config.Name == "" || strings.TrimSpace(s.Config.Name) != s.Config.Name {
			return errors.New("strategy name required")
		}
		if _, exists := strategies[s.Config.Name]; exists {
			return errors.New("duplicate strategy identity")
		}
		if !common.IsHexAddress(s.Config.Executor) || common.HexToAddress(s.Config.Executor) == (common.Address{}) || !common.IsHexAddress(s.Config.Account) || common.HexToAddress(s.Config.Account) == (common.Address{}) || s.Config.Interval <= 0 {
			return errors.New("strategy executor, account and positive interval required")
		}
		strategies[s.Config.Name] = s
	}
	r.strategies = strategies
	r.workflows = make(map[string]Workflow)
	r.accounts = make(map[common.Address]string)
	r.process, r.cancel = context.WithCancel(context.WithoutCancel(ctx))
	r.started = true
	return nil
}

func (r *Runtime) Tick(ctx context.Context) error {
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	if err := r.start(ctx); err != nil {
		return r.global(err)
	}
	operationContext, cancelOperations := context.WithCancel(ctx)
	stop := context.AfterFunc(r.process, cancelOperations)
	defer stop()
	defer cancelOperations()
	ctx = operationContext
	var failures []error
	r.mu.Lock()
	if worker := r.worker; worker != nil {
		select {
		case <-worker.done:
			r.worker = nil
			if worker.err != nil {
				failures = append(failures, worker.err)
			}
		default:
		}
	}
	r.mu.Unlock()
	reconciled := map[string]bool{}
	if r.Mode != "simulate" {
		if err := r.reconcile(ctx, reconciled); err != nil {
			failures = append(failures, err)
		}
	}
	r.mu.Lock()
	eligible := make([]Strategy, 0, len(r.Strategies))
	eligibleIndices := make([]int, 0, len(r.Strategies))
	for i := range r.Strategies {
		index := (i + r.next) % len(r.Strategies)
		s := r.Strategies[index]
		w, found := r.workflows[s.Config.Name]
		if found && (w.ownsAccount() || w.Phase == "failed" || w.Phase == "settled" && time.Since(w.FinishedAt) < s.Config.Interval) {
			continue
		}
		account := common.HexToAddress(s.Config.Account)
		if owner := r.accounts[account]; owner != "" && owner != s.Config.Name {
			continue
		}
		eligible = append(eligible, s)
		eligibleIndices = append(eligibleIndices, index)
	}
	r.mu.Unlock()
	if len(eligible) > 0 {
		operation, cancel := context.WithTimeout(ctx, r.OperationTimeout)
		results := r.Planner.PrepareAll(operation, eligible)
		cancel()
		if len(results) != len(eligible) {
			return r.global(errors.New("planner returned incomplete preparation group"))
		}
		for i, result := range results {
			s := eligible[i]
			outcome := "ok"
			if errors.Is(result.Err, provider.ErrNotReady) {
				outcome = "idle"
			} else if result.Err != nil {
				outcome = "error"
				failures = append(failures, errors.Errorf("strategy %s preparation failed: %w", s.Config.Name, result.Err))
			}
			if r.Observe != nil {
				r.Observe(s.Config.Name, outcome)
			}
			if result.Err != nil || r.Mode == "simulate" {
				continue
			}
			r.mu.Lock()
			free := !r.closed && r.owner == "" && r.worker == nil
			r.mu.Unlock()
			if !free {
				continue
			}
			operation, cancel = context.WithTimeout(ctx, r.OperationTimeout)
			err := r.Planner.Validate(operation, result.Plan)
			cancel()
			if err == nil {
				err = r.validatePlan(result.Plan, s)
			}
			if err != nil {
				if !errors.Is(err, provider.ErrNotReady) {
					failures = append(failures, err)
				}
				continue
			}
			if err = ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}
			var random [16]byte
			if _, err = rand.Read(random[:]); err != nil {
				return r.global(err)
			}
			w := Workflow{Name: s.Config.Name, ID: s.Config.Name + "-" + hex.EncodeToString(random[:]), Phase: "pending", Plan: result.Plan.Clone(), StartedAt: time.Now().UTC()}
			r.mu.Lock()
			if r.closed {
				r.mu.Unlock()
				return errors.New("runtime closed")
			}
			r.putLocked(w)
			r.owner = w.Name
			r.launchLocked(w)
			r.next = (eligibleIndices[i] + 1) % len(r.Strategies)
			r.mu.Unlock()
		}
	}
	r.mu.Lock()
	worker := r.worker
	r.mu.Unlock()
	if worker != nil {
		wait, cancel := context.WithTimeout(ctx, r.OperationTimeout)
		select {
		case <-worker.done:
			r.mu.Lock()
			if r.worker == worker {
				r.worker = nil
			}
			r.mu.Unlock()
			if worker.err != nil {
				failures = append(failures, worker.err)
			}
		case <-wait.Done():
		}
		cancel()
	}
	if r.Mode != "simulate" {
		if err := r.reconcile(ctx, reconciled); err != nil {
			failures = append(failures, err)
		}
	}
	r.mu.Lock()
	if r.blockedErr != nil {
		failures = append(failures, r.blockedErr)
	}
	for _, w := range r.workflows {
		if w.Phase == "failed" {
			failures = append(failures, errors.Errorf("strategy %s halted on confirmed revert", w.Name))
		}
	}
	r.mu.Unlock()
	return errors.Join(failures...)
}

func (r *Runtime) validatePlan(plan Plan, s Strategy) error {
	if plan.Snapshot.ChainID == nil || plan.Snapshot.ChainID.Cmp(r.ChainID) != 0 || plan.Snapshot.Caller != r.Caller || plan.Snapshot.Executor != common.HexToAddress(s.Config.Executor) || plan.Snapshot.Account != common.HexToAddress(s.Config.Account) {
		return errors.New("prepared plan differs from strategy or deployment identity")
	}
	_, err := RestoreOperations(plan, s.Integrations)
	return err
}

func (r *Runtime) launchLocked(w Workflow) {
	worker := &driverWorker{done: make(chan struct{})}
	r.worker = worker
	go func() {
		defer close(worker.done)
		result := r.Driver.Execute(r.process, Intent{ID: w.ID, To: w.Plan.Snapshot.Executor, Data: w.Plan.Calldata})
		r.mu.Lock()
		defer r.mu.Unlock()
		current := r.workflows[w.Name].Clone()
		worker.err = result.Err
		switch {
		case result.NotSubmitted && result.Execution == nil && !result.Reverted:
			current.Phase = "prepared"
			r.owner = ""
		case !result.NotSubmitted && result.Err == nil && result.Execution != nil && result.Execution.Validate() == nil:
			current.Execution = result.Execution.Clone()
			current.Phase = "authorized"
			if result.Reverted {
				current.Phase = "failed"
				current.FinishedAt = time.Now().UTC()
			}
			r.owner = ""
		default:
			if worker.err == nil {
				worker.err = errors.New("sender returned without conclusive submission evidence")
			}
			r.blockedErr = worker.err
		}
		r.putLocked(current)
	}()
}

func (r *Runtime) reconcile(ctx context.Context, seen map[string]bool) error {
	r.mu.Lock()
	var names []string
	for name, w := range r.workflows {
		if w.Phase == "authorized" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	r.mu.Unlock()
	sort.Strings(names)
	work := make(chan string)
	var wg sync.WaitGroup
	var failuresMu sync.Mutex
	var failures []error
	for range min(len(names), max(1, r.MaxConcurrent)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range work {
				operation, cancel := context.WithTimeout(ctx, r.OperationTimeout)
				if err := r.reconcileOne(operation, name); err != nil {
					failuresMu.Lock()
					failures = append(failures, err)
					failuresMu.Unlock()
				}
				cancel()
			}
		}()
	}
	for _, name := range names {
		select {
		case work <- name:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	return errors.Join(failures...)
}

func (r *Runtime) reconcileOne(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	w := r.workflows[name].Clone()
	s := r.strategies[name]
	r.mu.Unlock()
	operations, err := RestoreOperations(w.Plan, s.Integrations)
	if err != nil {
		return err
	}
	snapshot := w.Plan.Snapshot
	snapshot.Execution = w.Execution.Clone()
	snapshot, err = r.Planner.ConfirmedSnapshot(ctx, snapshot)
	if err != nil {
		return err
	}
	for i, op := range operations {
		if w.Plan.Operations[i].Progress.Done {
			continue
		}
		if err = r.Planner.VerifyConfirmedSnapshot(ctx, snapshot); err != nil {
			return err
		}
		progress, err := op.Advance(ctx, snapshot, w.Plan.Operations[i].Output.Clone(), w.Plan.Operations[i].Progress.Clone().State)
		if err != nil {
			return err
		}
		if err = r.Planner.VerifyConfirmedSnapshot(ctx, snapshot); err != nil {
			return err
		}
		w.Plan.Operations[i].Progress = progress.Clone()
		r.mu.Lock()
		r.putLocked(w)
		r.mu.Unlock()
	}
	for _, record := range w.Plan.Operations {
		if !record.Progress.Done {
			return nil
		}
	}
	w.Phase = "settled"
	w.FinishedAt = time.Now().UTC()
	r.mu.Lock()
	r.putLocked(w)
	r.mu.Unlock()
	return nil
}

// Close cancels process work and waits before the host may close its RPC client.
func (r *Runtime) Close() error {
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	r.mu.Lock()
	worker := r.worker
	r.mu.Unlock()
	if worker == nil {
		return nil
	}
	timeout := r.ShutdownTimeout
	if timeout <= 0 {
		timeout = r.OperationTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-worker.done:
		return nil
	case <-timer.C:
		return errors.New("runtime worker drain timed out")
	}
}
