package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type coordinatorPlanner struct {
	mu          sync.Mutex
	plans       map[string]Plan
	prepared    []string
	verifyCalls int
	verify      func(int) error
	prepare     func(context.Context, []Strategy) []PreparationResult
}

func (p *coordinatorPlanner) PrepareAll(ctx context.Context, ss []Strategy) []PreparationResult {
	if p.prepare != nil {
		return p.prepare(ctx, ss)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PreparationResult, len(ss))
	for i, s := range ss {
		p.prepared = append(p.prepared, s.Config.Name)
		plan, ok := p.plans[s.Config.Name]
		if !ok {
			out[i].Err = provider.ErrNotReady
		} else {
			out[i].Plan = plan.Clone()
		}
	}
	return out
}
func (*coordinatorPlanner) Validate(context.Context, Plan) error { return nil }
func (*coordinatorPlanner) ConfirmedSnapshot(_ context.Context, s chainio.Snapshot) (chainio.Snapshot, error) {
	return s, nil
}
func (p *coordinatorPlanner) VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.verifyCalls++
	if p.verify != nil {
		return p.verify(p.verifyCalls)
	}
	return nil
}

type coordinatorDriver struct {
	mu      sync.Mutex
	intents []Intent
	execute func(context.Context, Intent) TransactionResult
}

func (d *coordinatorDriver) Execute(ctx context.Context, intent Intent) TransactionResult {
	d.mu.Lock()
	d.intents = append(d.intents, intent)
	d.mu.Unlock()
	if d.execute != nil {
		return d.execute(ctx, intent)
	}
	return TransactionResult{NotSubmitted: true}
}
func (d *coordinatorDriver) calls() []Intent {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Intent(nil), d.intents...)
}
func coordinatorPlan(t *testing.T, executorAddress string) Plan {
	t.Helper()
	recipe := recipes.Recipe{}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	return Plan{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Executor: common.HexToAddress(executorAddress), Account: common.HexToAddress("0x20"), Caller: common.HexToAddress("0x30"), RecipeHash: hash}, Calldata: executor.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)}
}
func coordinatorExecution() *chainio.Execution {
	return &chainio.Execution{TxHash: common.HexToHash("0xa1"), BlockNumber: big.NewInt(11), BlockHash: common.HexToHash("0x11")}
}
func coordinatorHarness(t *testing.T) (*Runtime, *coordinatorPlanner, *coordinatorDriver) {
	t.Helper()
	name := common.HexToAddress("0x10").Hex()
	p := &coordinatorPlanner{plans: map[string]Plan{name: coordinatorPlan(t, name)}}
	d := &coordinatorDriver{}
	r := &Runtime{Planner: p, Driver: d, Strategies: []Strategy{{Config: Config{Name: name, Executor: name, Account: common.HexToAddress("0x20").Hex(), Interval: time.Hour}}}, ChainID: big.NewInt(1), Caller: common.HexToAddress("0x30"), Mode: "send", OperationTimeout: time.Second, ShutdownTimeout: time.Second}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	return r, p, d
}
func addCoordinatorStrategy(t *testing.T, r *Runtime, p *coordinatorPlanner, name, account string) {
	t.Helper()
	s := Strategy{Config: Config{Name: name, Executor: common.HexToAddress(name).Hex(), Account: common.HexToAddress(account).Hex(), Interval: time.Hour}}
	r.Strategies = append(r.Strategies, s)
	plan := coordinatorPlan(t, s.Config.Executor)
	plan.Snapshot.Account = common.HexToAddress(account)
	p.plans[name] = plan
}

func TestCoordinatorValidatesCurrentConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Runtime)
	}{
		{"chain", func(r *Runtime) { r.ChainID = nil }},
		{"caller", func(r *Runtime) { r.Caller = common.Address{} }},
		{"mode", func(r *Runtime) { r.Mode = "other" }},
		{"driver", func(r *Runtime) { r.Driver = nil }},
		{"timeout", func(r *Runtime) { r.OperationTimeout = 0 }},
		{"name", func(r *Runtime) { r.Strategies[0].Config.Name = " " }},
		{"duplicate", func(r *Runtime) { r.Strategies = append(r.Strategies, r.Strategies[0]) }},
		{"executor", func(r *Runtime) { r.Strategies[0].Config.Executor = "bad" }},
		{"account", func(r *Runtime) { r.Strategies[0].Config.Account = common.Address{}.Hex() }},
		{"interval", func(r *Runtime) { r.Strategies[0].Config.Interval = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, d := coordinatorHarness(t)
			tc.change(r)
			if err := r.Start(context.Background()); err == nil {
				t.Fatal("invalid current configuration accepted")
			}
			if len(d.calls()) != 0 {
				t.Fatal("configuration failure submitted work")
			}
		})
	}
}

func TestCoordinatorFreshRuntimeStartsEmpty(t *testing.T) {
	r, _, _ := coordinatorHarness(t)
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.Snapshot()) != 1 {
		t.Fatal("missing in-process workflow")
	}
	fresh, _, _ := coordinatorHarness(t)
	if err := fresh.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fresh.Snapshot()) != 0 {
		t.Fatal("new process inherited workflow state")
	}
}

func TestCoordinatorNotSubmittedReleasesAccountAndRotatesFairly(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	addCoordinatorStrategy(t, r, p, "0x40", "0x20")
	for range 4 {
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	calls := d.calls()
	if len(calls) != 4 {
		t.Fatal(len(calls))
	}
	for i, intent := range calls {
		expected := common.HexToAddress(r.Strategies[i%2].Config.Executor)
		if intent.To != expected {
			t.Fatal("unfair signer admission", i, intent.To, expected)
		}
		if i > 0 && intent.ID == calls[i-1].ID {
			t.Fatal("new attempt reused previous workflow identity")
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.owner != "" || len(r.accounts) != 0 {
		t.Fatal("prebroadcast outcome retained ownership")
	}
}

func TestCoordinatorWorkerSurvivesTickCancellation(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	r.OperationTimeout = 10 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	d.execute = func(ctx context.Context, _ Intent) TransactionResult {
		close(started)
		select {
		case <-release:
			return TransactionResult{NotSubmitted: true}
		case <-ctx.Done():
			return TransactionResult{Err: ctx.Err()}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	tick := make(chan error, 1)
	go func() { tick <- r.Tick(ctx) }()
	<-started
	cancel()
	if err := <-tick; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.calls()) != 1 || r.Snapshot()[0].Phase != "pending" {
		t.Fatal("canceled Tick replaced the active sender")
	}
	close(release)
}

func TestCoordinatorAmbiguousOrInvalidResultRetainsSignerAndAccount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result TransactionResult
	}{
		{"ambiguous", TransactionResult{Err: errors.New("broadcast uncertain")}},
		{"empty", TransactionResult{}},
		{"revert-without-evidence", TransactionResult{Reverted: true}},
		{"invalid-evidence", TransactionResult{Execution: &chainio.Execution{}}},
		{"error-with-evidence", TransactionResult{Execution: coordinatorExecution(), Err: errors.New("canonical evidence changed")}},
		{"contradictory", TransactionResult{NotSubmitted: true, Execution: coordinatorExecution()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, p, d := coordinatorHarness(t)
			addCoordinatorStrategy(t, r, p, "0x40", "0x50")
			d.execute = func(context.Context, Intent) TransactionResult { return tc.result }
			for range 3 {
				if err := r.Tick(context.Background()); err == nil {
					t.Fatal("ambiguous ownership not reported")
				}
			}
			if len(d.calls()) != 1 {
				t.Fatal("ambiguous result allocated another send")
			}
			w := r.Snapshot()[0]
			if w.Phase != "pending" || w.Execution != nil {
				t.Fatal("ambiguous workflow became terminal", w)
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.owner == "" || r.accounts[w.Plan.Snapshot.Account] != w.Name {
				t.Fatal("ambiguous signer/account ownership lost")
			}
		})
	}
}

func TestCoordinatorCanonicalOutcomesAndStickyRevert(t *testing.T) {
	for _, reverted := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "revert"}[reverted], func(t *testing.T) {
			r, p, d := coordinatorHarness(t)
			addCoordinatorStrategy(t, r, p, "0x40", "0x20")
			d.execute = func(context.Context, Intent) TransactionResult {
				return TransactionResult{Reverted: reverted, Execution: coordinatorExecution()}
			}
			err := r.Tick(context.Background())
			if (err != nil) != reverted {
				t.Fatal(err)
			}
			first := r.Snapshot()[0]
			phase := "settled"
			if reverted {
				phase = "failed"
			}
			if first.Phase != phase || first.Execution == nil || first.FinishedAt.IsZero() {
				t.Fatal(first)
			}
			_ = r.Tick(context.Background())
			if len(d.calls()) != 2 || d.calls()[1].To != common.HexToAddress("0x40") {
				t.Fatal("canonical outcome did not release signer/account")
			}
			_ = r.Tick(context.Background())
			if len(d.calls()) != 2 {
				t.Fatal("terminal interval/failure reexecuted")
			}
		})
	}
}

type progressOperation struct {
	mu      sync.Mutex
	states  []string
	done    bool
	advance func(context.Context, chainio.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error)
}

func (*progressOperation) ID() string                          { return "stable" }
func (*progressOperation) Requirements() provider.Requirements { return provider.Requirements{} }
func (*progressOperation) Prepare(context.Context, provider.Preparation) (provider.Output, error) {
	return provider.Output{}, nil
}
func (o *progressOperation) Advance(ctx context.Context, s chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	if o.advance != nil {
		return o.advance(ctx, s, out, state)
	}
	if string(out.Data) != `{"order":"external-id"}` || s.Execution == nil {
		return provider.Progress{}, errors.New("external or canonical identity missing")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.states = append(o.states, string(state))
	return provider.Progress{Done: o.done, Reference: "external-id", State: json.RawMessage(`{"cursor":24}`)}, nil
}
func attachCoordinatorOperation(r *Runtime, p *coordinatorPlanner, op *progressOperation) {
	integration := &testIntegration{name: "operation", ops: []provider.Operation{op}}
	r.Strategies[0].Integrations = []provider.Integration{integration}
	name := r.Strategies[0].Config.Name
	plan := p.plans[name]
	plan.Operations = []provider.Record{{Integration: integration.ID(), Policy: integration.Policy(), ID: op.ID(), Output: provider.Output{Data: json.RawMessage(`{"order":"external-id"}`)}, Progress: provider.Progress{State: json.RawMessage(`{"cursor":23}`)}}}
	p.plans[name] = plan
}

func TestCoordinatorExternalProgressRetainsIdentityWithoutRepreparation(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	op := &progressOperation{}
	attachCoordinatorOperation(r, p, op)
	d.execute = func(context.Context, Intent) TransactionResult {
		return TransactionResult{Execution: coordinatorExecution()}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := r.Snapshot()[0]
	if first.Phase != "authorized" {
		t.Fatal("unfinished external work released account")
	}
	op.done = true
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	last := r.Snapshot()[0]
	if first.ID != last.ID || last.Phase != "settled" || len(d.calls()) != 1 || len(p.prepared) != 1 {
		t.Fatal("external progress recompiled/resubmitted the workflow")
	}
	if len(op.states) != 2 || op.states[0] != `{"cursor":23}` || op.states[1] != `{"cursor":24}` {
		t.Fatal("external progress identity changed", op.states)
	}
	if p.verifyCalls != 4 {
		t.Fatal("canonical verification did not surround external progress", p.verifyCalls)
	}
}

func TestCoordinatorReconcilesExternalWorkWhileSignerOccupied(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	r.OperationTimeout = 10 * time.Millisecond
	op := &progressOperation{}
	attachCoordinatorOperation(r, p, op)
	addCoordinatorStrategy(t, r, p, "0x40", "0x50")
	release := make(chan struct{})
	d.execute = func(ctx context.Context, intent Intent) TransactionResult {
		if intent.To == common.HexToAddress("0x10") {
			return TransactionResult{Execution: coordinatorExecution()}
		}
		select {
		case <-release:
			return TransactionResult{NotSubmitted: true}
		case <-ctx.Done():
			return TransactionResult{Err: ctx.Err()}
		}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	op.done = true
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.Snapshot()[0].Phase != "settled" || len(d.calls()) != 2 {
		t.Fatal("occupied signer blocked external progress")
	}
	close(release)
}

func TestCoordinatorAccountOwnershipPreventsConflictingPreparation(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	op := &progressOperation{}
	attachCoordinatorOperation(r, p, op)
	addCoordinatorStrategy(t, r, p, "0x40", "0x20")
	d.execute = func(context.Context, Intent) TransactionResult {
		return TransactionResult{Execution: coordinatorExecution()}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := len(p.prepared)
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(p.prepared) != before || len(d.calls()) != 1 {
		t.Fatal("authorized account admitted a conflicting workflow")
	}
}

func TestCoordinatorCanonicalChangeDoesNotAcceptExternalProgress(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	op := &progressOperation{done: true}
	attachCoordinatorOperation(r, p, op)
	p.verify = func(call int) error {
		if call%2 == 0 {
			return errors.New("canonical block changed")
		}
		return nil
	}
	d.execute = func(context.Context, Intent) TransactionResult {
		return TransactionResult{Execution: coordinatorExecution()}
	}
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("canonical change ignored")
	}
	w := r.Snapshot()[0]
	if w.Phase != "authorized" || w.Plan.Operations[0].Progress.Done || string(w.Plan.Operations[0].Progress.State) != `{"cursor":23}` {
		t.Fatal("progress accepted after canonical change")
	}
}

func TestCoordinatorSimulateNeverAdvancesExternalWork(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	op := &progressOperation{done: true}
	attachCoordinatorOperation(r, p, op)
	r.Mode = "simulate"
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.calls()) != 0 || len(op.states) != 0 || len(r.Snapshot()) != 0 || len(p.prepared) != 1 {
		t.Fatal("simulation admitted/advanced external work")
	}
}

func TestCoordinatorSnapshotClonesExecutionAndOperationData(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	op := &progressOperation{}
	attachCoordinatorOperation(r, p, op)
	evidence := coordinatorExecution()
	evidence.Logs = []types.Log{{TxHash: evidence.TxHash, BlockHash: evidence.BlockHash, BlockNumber: 11, Topics: []common.Hash{{1}}, Data: []byte{2}}}
	d.execute = func(context.Context, Intent) TransactionResult { return TransactionResult{Execution: evidence} }
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := r.Snapshot()[0]
	changed := r.Snapshot()[0]
	changed.Plan.Calldata[0] ^= 255
	changed.Plan.Snapshot.ChainID.SetInt64(55)
	changed.Plan.Operations[0].Output.Data[0] = 'x'
	changed.Plan.Operations[0].Progress.State[0] = 'x'
	changed.Execution.BlockNumber.SetInt64(55)
	changed.Execution.Logs[0].Topics[0] = common.Hash{}
	changed.Execution.Logs[0].Data[0] = 55
	after := r.Snapshot()[0]
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		t.Fatal("snapshot mutation changed live workflow")
	}
}

func TestCoordinatorCloseTimeoutKeepsWaitingOnRepeatedClose(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	r.OperationTimeout = 10 * time.Millisecond
	r.ShutdownTimeout = 10 * time.Millisecond
	canceled := make(chan struct{})
	release := make(chan struct{})
	d.execute = func(ctx context.Context, _ Intent) TransactionResult {
		<-ctx.Done()
		close(canceled)
		<-release
		return TransactionResult{Err: ctx.Err()}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err == nil {
		t.Fatal("Close returned before sender drained")
	}
	<-canceled
	if err := r.Close(); err == nil {
		t.Fatal("repeated Close forgot the active worker")
	}
	close(release)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("closed runtime restarted")
	}
}

func TestCoordinatorPreparationAndReconciliationHaveDeadlines(t *testing.T) {
	r, p, d := coordinatorHarness(t)
	r.OperationTimeout = 10 * time.Millisecond
	p.prepare = func(ctx context.Context, ss []Strategy) []PreparationResult {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("preparation has no deadline")
		}
		<-ctx.Done()
		return []PreparationResult{{Err: ctx.Err()}}
	}
	if err := r.Tick(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("preparation cancellation lost", err)
	}
	p.prepare = nil
	op := &progressOperation{advance: func(ctx context.Context, _ chainio.Snapshot, _ provider.Output, _ json.RawMessage) (provider.Progress, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("reconciliation has no deadline")
		}
		<-ctx.Done()
		return provider.Progress{}, ctx.Err()
	}}
	attachCoordinatorOperation(r, p, op)
	// Start has captured the configured integrations already.
	r.mu.Lock()
	r.strategies[r.Strategies[0].Config.Name] = r.Strategies[0]
	r.mu.Unlock()
	d.execute = func(context.Context, Intent) TransactionResult {
		return TransactionResult{Execution: coordinatorExecution()}
	}
	if err := r.Tick(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("reconciliation cancellation lost", err)
	}
	if r.Snapshot()[0].Phase != "authorized" {
		t.Fatal("timed out reconciliation lost account ownership")
	}
}

func TestCoordinatorRejectsPreparedPlanOutsideConfiguredIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Plan)
	}{
		{"chain", func(p *Plan) { p.Snapshot.ChainID = big.NewInt(2) }},
		{"caller", func(p *Plan) { p.Snapshot.Caller = common.HexToAddress("0x31") }},
		{"executor", func(p *Plan) { p.Snapshot.Executor = common.HexToAddress("0x11") }},
		{"account", func(p *Plan) { p.Snapshot.Account = common.HexToAddress("0x21") }},
		{"calldata", func(p *Plan) { p.Calldata = []byte{1} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, p, d := coordinatorHarness(t)
			name := r.Strategies[0].Config.Name
			plan := p.plans[name]
			tc.change(&plan)
			p.plans[name] = plan
			if err := r.Tick(context.Background()); err == nil {
				t.Fatal("invalid prepared plan accepted")
			}
			if len(d.calls()) != 0 || len(r.Snapshot()) != 0 {
				t.Fatal("invalid plan reached admission")
			}
		})
	}
}

func TestCoordinatorCanceledTickCannotAdmitNewWork(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Tick(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
	if len(d.calls()) != 0 || len(r.Snapshot()) != 0 {
		t.Fatal("canceled tick admitted work")
	}
}

func TestCoordinatorReconciliationBoundsActiveOperations(t *testing.T) {
	r, p, _ := coordinatorHarness(t)
	r.MaxConcurrent = 2
	for _, name := range []string{"0x40", "0x50", "0x60"} {
		addCoordinatorStrategy(t, r, p, name, name)
	}
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var countMu sync.Mutex
	active, peak := 0, 0
	for i := range r.Strategies {
		op := &progressOperation{advance: func(ctx context.Context, _ chainio.Snapshot, _ provider.Output, _ json.RawMessage) (provider.Progress, error) {
			countMu.Lock()
			active++
			peak = max(peak, active)
			countMu.Unlock()
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
			}
			countMu.Lock()
			active--
			countMu.Unlock()
			return provider.Progress{Done: true}, nil
		}}
		integration := &testIntegration{name: "operation", ops: []provider.Operation{op}}
		r.Strategies[i].Integrations = []provider.Integration{integration}
		name := r.Strategies[i].Config.Name
		plan := p.plans[name]
		plan.Operations = []provider.Record{{Integration: integration.ID(), Policy: integration.Policy(), ID: op.ID()}}
		p.plans[name] = plan
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	for _, s := range r.Strategies {
		r.putLocked(Workflow{Name: s.Config.Name, ID: s.Config.Name, Phase: "authorized", Plan: p.plans[s.Config.Name], Execution: coordinatorExecution()})
	}
	r.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- r.reconcile(context.Background(), map[string]bool{}) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("bounded workers did not start")
		}
	}
	select {
	case <-started:
		close(release)
		t.Fatal("reconciliation exceeded concurrency bound")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if peak != 2 {
		t.Fatal("unexpected reconciliation concurrency", peak)
	}
	for _, w := range r.Snapshot() {
		if w.Phase != "settled" {
			t.Fatal("bounded worker skipped workflow", w.Name)
		}
	}
}

func TestCoordinatorCloseCancelsActiveTickBeforeWaiting(t *testing.T) {
	for _, phase := range []string{"preparation", "sender"} {
		t.Run(phase, func(t *testing.T) {
			r, p, d := coordinatorHarness(t)
			r.OperationTimeout = time.Minute
			started := make(chan struct{})
			if phase == "preparation" {
				p.prepare = func(ctx context.Context, _ []Strategy) []PreparationResult {
					close(started)
					<-ctx.Done()
					return []PreparationResult{{Err: ctx.Err()}}
				}
			} else {
				d.execute = func(ctx context.Context, _ Intent) TransactionResult {
					close(started)
					<-ctx.Done()
					return TransactionResult{Err: ctx.Err()}
				}
			}
			tick := make(chan error, 1)
			go func() { tick <- r.Tick(context.Background()) }()
			<-started
			closed := make(chan error, 1)
			go func() { closed <- r.Close() }()
			select {
			case err := <-closed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("Close waited for Tick before canceling process work")
			}
			<-tick
		})
	}
}
