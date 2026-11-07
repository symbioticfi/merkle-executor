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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type coordinatorPlanner struct {
	mu       sync.Mutex
	plans    map[string]Plan
	prepares int
}

func (p *coordinatorPlanner) PrepareAll(_ context.Context, ss []Strategy) []PreparationResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PreparationResult, len(ss))
	for i, s := range ss {
		p.prepares++
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
func (*coordinatorPlanner) VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error {
	return nil
}

type coordinatorDriver struct {
	mu                            sync.Mutex
	owner                         string
	executions, resumes, releases int
	wait                          chan struct{}
	terminal                      bool
	beforeRelease                 func() error
	execute                       func(context.Context, Intent, PersistTransaction) TransactionResult
	resume                        func(context.Context, Transaction, PersistTransaction) TransactionResult
}

func (d *coordinatorDriver) Reserve(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.owner != "" && d.owner != id {
		return errors.New("busy")
	}
	d.owner = id
	return nil
}
func (d *coordinatorDriver) Release(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.beforeRelease != nil {
		if err := d.beforeRelease(); err != nil {
			return err
		}
	}
	if d.owner != id {
		return errors.New("wrong owner")
	}
	d.owner = ""
	d.releases++
	return nil
}
func (d *coordinatorDriver) Execute(ctx context.Context, intent Intent, persist PersistTransaction) TransactionResult {
	d.mu.Lock()
	d.executions++
	d.mu.Unlock()
	if d.execute != nil {
		return d.execute(ctx, intent, persist)
	}
	tx := Transaction{Intent: intent, Sender: common.HexToAddress("0x30"), ChainID: big.NewInt(1), Admitted: true}
	if err := persist(tx); err != nil {
		return TransactionResult{Err: err}
	}
	if d.wait != nil {
		select {
		case <-d.wait:
		case <-ctx.Done():
			return TransactionResult{Pending: true, Err: ctx.Err()}
		}
	}
	if !d.terminal {
		return TransactionResult{NotSubmitted: true}
	}
	return TransactionResult{Pending: true}
}
func (d *coordinatorDriver) Resume(ctx context.Context, tx Transaction, persist PersistTransaction) TransactionResult {
	d.mu.Lock()
	d.resumes++
	d.mu.Unlock()
	if d.resume != nil {
		return d.resume(ctx, tx, persist)
	}
	if d.wait != nil {
		select {
		case <-d.wait:
		case <-ctx.Done():
			return TransactionResult{Pending: true, Err: ctx.Err()}
		}
	}
	return TransactionResult{NotSubmitted: len(tx.Attempts) == 0, Pending: len(tx.Attempts) > 0}
}

type coordinatorStore struct {
	*state.Store
	fail      bool
	after     bool
	lists     int
	failPhase string
}

func (s *coordinatorStore) SaveNamespace(n, k string, v any) error {
	if w, ok := v.(Workflow); ok && s.failPhase != "" && w.Phase == s.failPhase {
		return errors.New("injected phase checkpoint failure")
	}
	if s.fail {
		if s.after {
			_ = s.Store.SaveNamespace(n, k, v)
		}
		return errors.New("injected uncertain checkpoint")
	}
	return s.Store.SaveNamespace(n, k, v)
}
func (s *coordinatorStore) ListNamespace(n string) ([]json.RawMessage, error) {
	s.lists++
	return s.Store.ListNamespace(n)
}
func coordinatorPlan(t *testing.T, name string) Plan {
	t.Helper()
	var steps [][]byte
	recipe := recipes.Recipe{Steps: steps}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	return Plan{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Executor: common.HexToAddress(name), Account: common.HexToAddress("0x20"), Caller: common.HexToAddress("0x30"), RecipeHash: hash}, Calldata: executor.NewRecipeExecutor().PackExecute(nil, steps, nil, nil)}
}
func coordinatorHarness(t *testing.T) (*Runtime, *coordinatorStore, *coordinatorDriver) {
	t.Helper()
	st, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &coordinatorStore{Store: st}
	d := &coordinatorDriver{}
	name := common.HexToAddress("0x10").Hex()
	p := &coordinatorPlanner{plans: map[string]Plan{name: coordinatorPlan(t, name)}}
	r := &Runtime{Store: s, Planner: p, Driver: d, Strategies: []Strategy{{Config: Config{Name: name, Executor: name, Account: common.HexToAddress("0x20").Hex(), Interval: time.Hour}}}, ChainID: big.NewInt(1), Caller: common.HexToAddress("0x30"), Mode: "send", OperationTimeout: time.Second}
	t.Cleanup(func() { _ = r.Close(); _ = st.Close() })
	return r, s, d
}

func TestCoordinatorPersistsReservationBeforeDriverAndNeverRescansIdleHistory(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.beforeRelease = func() error {
		var w Workflow
		found, err := s.LoadNamespace(Namespace, WorkflowKey(r.Strategies[0].Config.Name), &w)
		if err != nil || !found || w.Transaction != nil {
			return errors.New("unsigned release preceded durable checkpoint")
		}
		return nil
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 1 || d.releases != 1 {
		t.Fatal(d.executions, d.releases)
	}
	before := s.lists
	for range 3 {
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if s.lists != before {
		t.Fatal("history rescanned per tick")
	}
}
func TestCoordinatorWorkerOutlivesWaitAndHasSingleObserver(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	r.OperationTimeout = 20 * time.Millisecond
	d.wait = make(chan struct{})
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	calls, owner := d.executions, d.owner
	d.mu.Unlock()
	if calls != 1 || owner == "" {
		t.Fatal("worker replaced or reservation lost", calls, owner)
	}
	close(d.wait)
}
func TestCoordinatorStorageUncertaintyStopsAdmissionAfterRename(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(big.NewInt(int64(btoi(after))).String(), func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			if err := r.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			s.fail, s.after = true, after
			if err := r.Tick(context.Background()); err == nil {
				t.Fatal("uncertain write ignored")
			}
			s.fail = false
			if err := r.Tick(context.Background()); err == nil {
				t.Fatal("poisoned runtime admitted work")
			}
			if d.executions != 0 {
				t.Fatal("network after uncertain storage")
			}
		})
	}
}
func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}
func TestCoordinatorRejectsRemovedActiveWorkflowBeforeReservation(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	w := Workflow{Version: 2, IdentityVersion: 2, Name: "removed", ID: "saved", ConfigHash: "legacy", Phase: "prepared", Plan: coordinatorPlan(t, "0x11")}
	if err := s.SaveNamespace(Namespace, WorkflowKey(w.Name), w); err != nil {
		t.Fatal(err)
	}
	restarted := &Runtime{Store: s, Planner: r.Planner, Driver: d, ChainID: r.ChainID, Caller: r.Caller, Mode: "send", OperationTimeout: r.OperationTimeout}
	if err := restarted.Start(context.Background()); err == nil {
		t.Fatal("removed active owner accepted")
	}
	if d.owner != "" {
		t.Fatal("reservation before complete recovery validation")
	}
}

func signedCoordinatorWorkflow(t *testing.T, r *Runtime, name, phase string) Workflow {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	r.Caller = crypto.PubkeyToAddress(key.PublicKey)
	plan := coordinatorPlan(t, name)
	plan.Snapshot.Caller = r.Caller
	if p, ok := r.Planner.(*coordinatorPlanner); ok {
		p.plans[name] = plan.Clone()
	}
	intent := Intent{ID: "durable-" + name, To: plan.Snapshot.Executor, Data: plan.Calldata}
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: r.ChainID, Nonce: 7, To: &intent.To, Gas: 100000, GasFeeCap: big.NewInt(10), GasTipCap: big.NewInt(1), Data: intent.Data})
	signed, err := types.SignTx(unsigned, types.LatestSignerForChainID(r.ChainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tx := Transaction{Intent: intent, Sender: r.Caller, ChainID: new(big.Int).Set(r.ChainID), Nonce: 7, Admitted: true, Attempts: []hexutil.Bytes{raw}}
	w := Workflow{Version: 2, IdentityVersion: 2, Name: name, ID: intent.ID, ConfigHash: r.Strategies[0].Config.Fingerprint(), Phase: phase, Plan: plan, Transaction: &tx, Reserved: phase == "pending"}
	if phase != "pending" {
		tx.Terminal = "confirmed"
		if phase == "failed" {
			tx.Terminal = "reverted"
		}
		tx.Execution = &chainio.Execution{TxHash: signed.Hash(), BlockNumber: big.NewInt(11), BlockHash: common.HexToHash("0x11")}
		if phase == "failed" || phase == "settled" {
			w.FinishedAt = time.Now().UTC()
		}
	}
	return w
}

func TestCoordinatorReceiptAndPhaseAreOneCheckpointBeforeRelease(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(big.NewInt(int64(btoi(fail))).String(), func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			name := r.Strategies[0].Config.Name
			w := signedCoordinatorWorkflow(t, r, name, "authorized")
			d.execute = func(_ context.Context, intent Intent, persist PersistTransaction) TransactionResult {
				tx := cloneTransaction(*w.Transaction)
				tx.Intent = intent
				tx.Terminal = ""
				tx.Execution = nil
				if err := persist(tx); err != nil {
					return TransactionResult{Err: err}
				}
				tx.Terminal = "confirmed"
				tx.Execution = w.Transaction.Execution.Clone()
				if err := persist(tx); err != nil {
					return TransactionResult{Err: err}
				}
				return TransactionResult{Execution: tx.Execution}
			}
			d.beforeRelease = func() error {
				var saved Workflow
				if found, err := s.LoadNamespace(Namespace, WorkflowKey(name), &saved); err != nil || !found || saved.Phase != "authorized" || saved.Transaction.Execution == nil {
					return errors.New("release preceded complete canonical checkpoint")
				}
				return nil
			}
			if err := r.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			if fail {
				s.failPhase = "authorized"
			}
			err := r.Tick(context.Background())
			if fail {
				if err == nil || d.releases != 0 {
					t.Fatal("failed receipt persistence released signer", err, d.releases)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if d.releases != 1 || r.Snapshot()[0].Phase != "settled" {
					t.Fatal("canonical workflow not completed", r.Snapshot())
				}
			}
		})
	}
}

func TestCoordinatorSignedRestartNeverReplansOrExecutes(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "pending")
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(name): w}); err != nil {
		t.Fatal(err)
	}
	d.resume = func(_ context.Context, saved Transaction, _ PersistTransaction) TransactionResult {
		if saved.Nonce != 7 || len(saved.Attempts) != 1 || !bytes.Equal(saved.Attempts[0], w.Transaction.Attempts[0]) {
			t.Error("signed identity changed")
		}
		return TransactionResult{Pending: true}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 0 || d.resumes != 1 || r.Planner.(*coordinatorPlanner).prepares != 0 {
		t.Fatal("signed recovery replayed/recompiled work")
	}
}

func TestCoordinatorSavedTerminalRevalidationFailureRetainsReservation(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "authorized")
	w.Reserved = true
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(name): w}); err != nil {
		t.Fatal(err)
	}
	d.resume = func(context.Context, Transaction, PersistTransaction) TransactionResult {
		return TransactionResult{Pending: true, Err: errors.New("canonical receipt changed")}
	}
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("changed canonical receipt ignored")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.releases != 0 || d.owner != w.ID {
		t.Fatal("saved terminal record released before fresh driver proof")
	}
}

func TestCoordinatorTerminalIntervalAndStickyFailure(t *testing.T) {
	for _, phase := range []string{"settled", "failed"} {
		t.Run(phase, func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			name := r.Strategies[0].Config.Name
			w := signedCoordinatorWorkflow(t, r, name, phase)
			if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(name): w}); err != nil {
				t.Fatal(err)
			}
			err := r.Tick(context.Background())
			if (err != nil) != (phase == "failed") {
				t.Fatal(phase, err)
			}
			if d.executions != 0 || r.Planner.(*coordinatorPlanner).prepares != 0 {
				t.Fatal("terminal checkpoint reexecuted")
			}
		})
	}
}

type checkpointOperation struct {
	state json.RawMessage
	calls int
	done  bool
}

func (*checkpointOperation) ID() string                          { return "stable" }
func (*checkpointOperation) Requirements() provider.Requirements { return provider.Requirements{} }
func (*checkpointOperation) Prepare(context.Context, provider.Preparation) (provider.Output, error) {
	return provider.Output{}, nil
}
func (o *checkpointOperation) Advance(_ context.Context, _ chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	o.calls++
	if !bytes.Equal(state, o.state) || string(out.Data) != `{"order":"external-id"}` {
		return provider.Progress{}, errors.New("external identity/cursor changed")
	}
	return provider.Progress{Done: o.done, Reference: "external-id", State: append(json.RawMessage(nil), state...)}, nil
}
func TestCoordinatorReconcilesExternalWorkWhileSignerOccupied(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	r.OperationTimeout = 20 * time.Millisecond
	name := r.Strategies[0].Config.Name
	authorized := signedCoordinatorWorkflow(t, r, name, "authorized")
	op := &checkpointOperation{state: json.RawMessage(`{"cursor":23}`)}
	integration := &testIntegration{name: "operation", ops: []provider.Operation{op}}
	r.Strategies[0].Integrations = []provider.Integration{integration}
	authorized.Plan.Operations = []provider.Record{{Integration: integration.ID(), Policy: integration.Policy(), ID: op.ID(), Output: provider.Output{Data: json.RawMessage(`{"order":"external-id"}`)}, Progress: provider.Progress{State: op.state, Reference: "external-id"}}}
	other := r.Strategies[0]
	other.Config.Name = "other"
	other.Config.Executor = common.HexToAddress("0x40").Hex()
	other.Config.Account = common.HexToAddress("0x50").Hex()
	other.Integrations = nil
	r.Strategies = append(r.Strategies, other)
	pending := authorized.Clone()
	pending.Name = "other"
	pending.ID = "other-pending"
	pending.ConfigHash = other.Config.Fingerprint()
	pending.Plan = coordinatorPlan(t, other.Config.Executor)
	pending.Plan.Snapshot.Caller = r.Caller
	pending.Plan.Snapshot.Account = common.HexToAddress(other.Config.Account)
	pending.Phase = "pending"
	pending.Reserved = true
	pending.Transaction = &Transaction{Intent: Intent{ID: pending.ID, To: pending.Plan.Snapshot.Executor, Data: pending.Plan.Calldata}, Sender: r.Caller, ChainID: r.ChainID, Admitted: true}
	d.wait = make(chan struct{})
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(name): authorized, WorkflowKey("other"): pending}); err != nil {
		t.Fatal(err)
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if op.calls == 0 || d.executions != 0 || d.owner != pending.ID {
		t.Fatal("external reconciliation blocked by occupied signer")
	}
	close(d.wait)
}
