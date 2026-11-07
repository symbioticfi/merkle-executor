package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type CheckpointStore interface {
	Load(string, any) (bool, error)
	LoadNamespace(string, string, any) (bool, error)
	SaveNamespace(string, string, any) error
	DeleteNamespace(string, string) error
	ListNamespace(string) ([]json.RawMessage, error)
	ActivateNamespace(string, map[string]any) error
}
type Planning interface {
	PrepareAll(context.Context, []Strategy) []PreparationResult
	Validate(context.Context, Plan) error
	ConfirmedSnapshot(context.Context, chainio.Snapshot) (chainio.Snapshot, error)
	VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error
}
type driverWorker struct {
	name string
	done chan struct{}
	err  error
}
type Runtime struct {
	Store            CheckpointStore
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
	LegacyConverters map[string][]LegacyConverter

	tickMu          sync.Mutex
	mu              sync.Mutex
	started, closed bool
	fault           error
	process         context.Context
	cancel          context.CancelFunc
	workflows       map[string]Workflow
	strategies      map[string]Strategy
	accounts        map[common.Address]string
	owner           string
	worker          *driverWorker
	next            int
}

func (r *Runtime) saveLocked(w Workflow) error {
	if r.fault != nil {
		return r.fault
	}
	if err := r.Store.SaveNamespace(Namespace, WorkflowKey(w.Name), w); err != nil {
		r.fault = errors.Errorf("workflow storage uncertain; restart and reload required: %w", err)
		return r.fault
	}
	r.workflows[w.Name] = w.Clone()
	if w.active() {
		r.accounts[w.Plan.Snapshot.Account] = w.Name
	} else if r.accounts[w.Plan.Snapshot.Account] == w.Name {
		delete(r.accounts, w.Plan.Snapshot.Account)
	}
	return nil
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
	r.Log.Error(errors.New("runtime state or recovery failed"), "runtime iteration failed")
	return err
}

func (r *Runtime) Tick(ctx context.Context) error {
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	if err := r.start(ctx); err != nil {
		return r.global(err)
	}
	r.mu.Lock()
	if r.closed || r.fault != nil {
		err := r.fault
		if err == nil {
			err = errors.New("runtime closed")
		}
		r.mu.Unlock()
		return r.global(err)
	}
	if r.owner != "" && r.worker == nil {
		r.launchLocked(r.owner, true)
	}
	r.mu.Unlock()
	var failures []error
	reconciled := map[string]bool{}
	if r.Mode != "simulate" {
		if err := r.reconcile(ctx, reconciled); err != nil {
			failures = append(failures, err)
		}
	}
	r.mu.Lock()
	if r.fault != nil {
		err := r.fault
		r.mu.Unlock()
		return r.global(err)
	}
	eligible := make([]Strategy, 0, len(r.Strategies))
	eligibleIndices := make([]int, 0, len(r.Strategies))
	for i := range r.Strategies {
		index := (i + r.next) % len(r.Strategies)
		s := r.Strategies[index]
		w, found := r.workflows[s.Config.Name]
		if found && (w.Reserved || w.Phase == "pending" || w.Phase == "authorized" || w.Phase == "failed" || w.Phase == "settled" && time.Since(w.FinishedAt) < s.Config.Interval) {
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
			free := r.owner == "" && r.worker == nil && r.fault == nil
			r.mu.Unlock()
			if !free {
				continue
			}
			operation, cancel = context.WithTimeout(ctx, r.OperationTimeout)
			err := r.Planner.Validate(operation, result.Plan)
			cancel()
			if err != nil {
				if !errors.Is(err, provider.ErrNotReady) {
					failures = append(failures, err)
				}
				continue
			}
			if _, err = RestoreOperations(result.Plan, s.Integrations); err != nil {
				failures = append(failures, err)
				continue
			}
			var random [16]byte
			if _, err = rand.Read(random[:]); err != nil {
				return err
			}
			w := Workflow{Version: 2, IdentityVersion: 2, Name: s.Config.Name, ID: s.Config.Name + "-" + hex.EncodeToString(random[:]), ConfigHash: s.Config.Fingerprint(), Phase: "prepared", Plan: result.Plan.Clone()}
			r.mu.Lock()
			err = r.saveLocked(w)
			r.mu.Unlock()
			if err != nil {
				return r.global(err)
			}
			if err = r.Driver.Reserve(ctx, w.ID); err != nil {
				failures = append(failures, err)
				continue
			}
			r.mu.Lock()
			w.Phase = "pending"
			w.Reserved = true
			w.Transaction = &Transaction{Intent: Intent{ID: w.ID, To: w.Plan.Snapshot.Executor, Data: w.Plan.Calldata}, Sender: r.Caller, ChainID: new(big.Int).Set(r.ChainID), Admitted: true}
			r.owner = w.Name
			err = r.saveLocked(w)
			if err == nil {
				r.launchLocked(w.Name, false)
				r.next = (eligibleIndices[i] + 1) % len(r.Strategies)
			}
			r.mu.Unlock()
			if err != nil {
				return r.global(err)
			}
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
			fault := r.fault
			r.mu.Unlock()
			if fault != nil {
				cancel()
				return r.global(fault)
			}
			if worker.err != nil && !errors.Is(worker.err, context.DeadlineExceeded) && !errors.Is(worker.err, context.Canceled) {
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
	fault := r.fault
	for _, w := range r.workflows {
		if w.Phase == "failed" {
			failures = append(failures, errors.New("strategy halted on confirmed revert; inspect durable workflow"))
		}
	}
	r.mu.Unlock()
	if fault != nil {
		return r.global(fault)
	}
	return errors.Join(failures...)
}

func (r *Runtime) launchLocked(name string, resume bool) {
	w := r.workflows[name].Clone()
	worker := &driverWorker{name: name, done: make(chan struct{})}
	r.worker = worker
	go func() {
		defer close(worker.done)
		persist := func(tx Transaction) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			current := r.workflows[name].Clone()
			if old := current.Transaction; old != nil && len(old.Attempts) > 0 {
				if tx.Nonce != old.Nonce || len(tx.Attempts) < len(old.Attempts) {
					return errors.New("transaction callback discarded owned attempt")
				}
				for i := range old.Attempts {
					if !bytes.Equal(tx.Attempts[i], old.Attempts[i]) {
						return errors.New("transaction callback changed owned attempt")
					}
				}
				if old.Terminal != "" && (tx.Terminal != old.Terminal || !reflect.DeepEqual(tx.Execution.Clone(), old.Execution.Clone())) {
					return errors.New("transaction callback changed terminal evidence")
				}
			}
			current.Transaction = &tx
			current.Phase = "pending"
			if tx.Terminal != "" {
				current.Phase = "authorized"
				if tx.Terminal == "confirmed" && r.workflows[name].Phase == "settled" {
					current.Phase = "settled"
				}
				if tx.Terminal == "reverted" {
					current.Phase = "failed"
					current.FinishedAt = time.Now().UTC()
				}
			}
			if err := validateTransaction(current, r.ChainID, r.Caller); err != nil {
				return err
			}
			return r.saveLocked(current)
		}
		var result TransactionResult
		if resume && w.Transaction == nil {
			if err := r.Driver.Release(r.process, w.ID); err != nil {
				worker.err = err
				return
			}
			r.mu.Lock()
			r.owner = ""
			r.mu.Unlock()
			return
		} else if resume {
			result = r.Driver.Resume(r.process, *w.Transaction, persist)
		} else {
			result = r.Driver.Execute(r.process, w.Transaction.Intent, persist)
		}
		worker.err = result.Err
		r.mu.Lock()
		current := r.workflows[name].Clone()
		if r.fault != nil {
			r.mu.Unlock()
			return
		}
		terminal := current.Transaction != nil && current.Transaction.Terminal != "" && result.Err == nil && !result.Pending && result.Execution != nil && reflect.DeepEqual(result.Execution.Clone(), current.Transaction.Execution.Clone()) && result.Reverted == (current.Transaction.Terminal == "reverted")
		unsigned := result.NotSubmitted && (current.Transaction == nil || len(current.Transaction.Attempts) == 0)
		if unsigned {
			current.Transaction = nil
			current.Phase = "prepared"
			current.Reserved = false
			if err := r.saveLocked(current); err != nil {
				worker.err = err
				r.mu.Unlock()
				return
			}
		}
		r.mu.Unlock()
		if !terminal && !unsigned {
			return
		}
		if err := r.Driver.Release(r.process, current.ID); err != nil {
			worker.err = err
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		current = r.workflows[name].Clone()
		current.Reserved = false
		if err := r.saveLocked(current); err != nil {
			worker.err = err
			return
		}
		r.owner = ""
	}()
}

func (r *Runtime) reconcile(ctx context.Context, seen map[string]bool) error {
	r.mu.Lock()
	var names []string
	for name, w := range r.workflows {
		if w.Phase == "authorized" && !w.Reserved && !seen[name] {
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
	r.mu.Lock()
	w := r.workflows[name].Clone()
	fault := r.fault
	s := r.strategies[name]
	r.mu.Unlock()
	if fault != nil {
		return fault
	}
	operations, err := RestoreOperations(w.Plan, s.Integrations)
	if err != nil {
		return err
	}
	snapshot := w.Plan.Snapshot
	snapshot.Execution = w.Transaction.Execution.Clone()
	snapshot, err = r.Planner.ConfirmedSnapshot(ctx, snapshot)
	if err != nil {
		return err
	}
	for i, op := range operations {
		if w.Plan.Operations[i].Progress.Done {
			continue
		}
		r.mu.Lock()
		fault = r.fault
		r.mu.Unlock()
		if fault != nil {
			return fault
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
		err = r.saveLocked(w)
		r.mu.Unlock()
		if err != nil {
			return err
		}
	}
	for _, record := range w.Plan.Operations {
		if !record.Progress.Done {
			return nil
		}
	}
	w.Phase = "settled"
	w.FinishedAt = time.Now().UTC()
	r.mu.Lock()
	err = r.saveLocked(w)
	r.mu.Unlock()
	return err
}
func (r *Runtime) Close() error {
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
	}
	worker := r.worker
	r.mu.Unlock()
	if worker != nil {
		timeout := r.ShutdownTimeout
		if timeout <= 0 {
			timeout = r.OperationTimeout
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-worker.done:
		case <-timer.C:
			return errors.New("runtime worker drain timed out; state must remain owned")
		}
	}
	return nil
}
