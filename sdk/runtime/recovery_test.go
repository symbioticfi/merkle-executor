package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr/funcr"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestPreflightRejectsWrongCheckpointFilenameAndExtraDeployment(t *testing.T) {
	for _, kind := range []string{"wrong-key", "extra-deployment", "forged-deployment"} {
		t.Run(kind, func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			if err := r.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "wrong-key":
				w := Workflow{Version: 2, Name: "saved", ID: "saved", Phase: "prepared"}
				if err := s.SaveNamespace(Namespace, "incorrect-key", w); err != nil {
					t.Fatal(err)
				}
			case "extra-deployment":
				if err := s.SaveNamespace(Namespace, "other", Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}); err != nil {
					t.Fatal(err)
				}
			case "forged-deployment":
				if err := s.SaveNamespace(Namespace, "other", map[string]any{"kind": "deployment", "version": 2, "caller": r.Caller, "chainId": 2}); err != nil {
					t.Fatal(err)
				}
			}
			if err := Preflight(s, r.ChainID, r.Caller, []string{r.Strategies[0].Config.Name}); err == nil {
				t.Fatal("miskeyed checkpoint accepted")
			}
			if d.executions != 0 || d.resumes != 0 {
				t.Fatal("preflight performed driver I/O")
			}
		})
	}
}

func TestPreflightChecksLegacyOwnershipAndOptionalCallerWithoutMutation(t *testing.T) {
	r, s, _ := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "pending")
	writeLegacyFixture(t, r, s, w, true)
	var before json.RawMessage
	_, _ = s.Load("strategy."+name+".v1", &before)
	if err := Preflight(s, r.ChainID, common.Address{}, []string{name}); err != nil {
		t.Fatal(err)
	}
	if err := Preflight(s, r.ChainID, common.HexToAddress("0x99"), []string{name}); err == nil {
		t.Fatal("foreign caller accepted")
	}
	if err := Preflight(s, r.ChainID, r.Caller, nil); err == nil {
		t.Fatal("removed owner accepted")
	}
	if err := s.Save("strategies.v1", []string{}); err != nil {
		t.Fatal(err)
	}
	if err := Preflight(s, r.ChainID, r.Caller, []string{name}); err == nil {
		t.Fatal("orphan journal accepted")
	}
	var after json.RawMessage
	_, _ = s.Load("strategy."+name+".v1", &after)
	if !bytes.Equal(before, after) {
		t.Fatal("offline preflight changed work")
	}
	var deployment Deployment
	if found, err := s.LoadNamespace(Namespace, "deployment", &deployment); err != nil || found {
		t.Fatal("preflight activated import")
	}
}

func coordinatorExternalFixture(t *testing.T) (*Runtime, *coordinatorStore, *checkpointOperation) {
	t.Helper()
	r, s, _ := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "authorized")
	op := &checkpointOperation{state: json.RawMessage(`{"cursor":23}`), done: true}
	in := &testIntegration{name: "external", ops: []provider.Operation{op}}
	r.Strategies[0].Integrations = []provider.Integration{in}
	w.Plan.Operations = []provider.Record{{Integration: in.ID(), Policy: in.Policy(), ID: op.ID(), Output: provider.Output{Data: json.RawMessage(`{"order":"external-id"}`)}, Progress: provider.Progress{State: op.state, Reference: "external-id"}}}
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(name): w}); err != nil {
		t.Fatal(err)
	}
	return r, s, op
}

func TestCoordinatorPostAdvanceReorgCannotCommitCompletion(t *testing.T) {
	r, _, op := coordinatorExternalFixture(t)
	r.Planner.(*coordinatorPlanner).verify = func(n int) error {
		if n == 2 {
			return errors.New("closing canonical witness changed")
		}
		return nil
	}
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("reorg ignored")
	}
	w := r.Snapshot()[0]
	if op.calls != 1 || w.Phase != "authorized" || w.Plan.Operations[0].Progress.Done {
		t.Fatal("reorg committed provider progress")
	}
}

func TestCoordinatorSimulationNeverPublishesSavedExternalWork(t *testing.T) {
	r, s, op := coordinatorExternalFixture(t)
	r.Mode = "simulate"
	name := r.Strategies[0].Config.Name
	var before json.RawMessage
	if found, err := s.LoadNamespace(Namespace, WorkflowKey(name), &before); err != nil || !found {
		t.Fatal(found, err)
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	var after json.RawMessage
	if found, err := s.LoadNamespace(Namespace, WorkflowKey(name), &after); err != nil || !found {
		t.Fatal(found, err)
	}
	d := r.Driver.(*coordinatorDriver)
	if op.calls != 0 || d.executions != 0 || d.resumes != 0 || !bytes.Equal(before, after) {
		t.Fatal("simulation advanced or rewrote saved external work")
	}
}

func TestCoordinatorProgressSaveFailureRetainsExternalIdentityAcrossRestart(t *testing.T) {
	r, s, op := coordinatorExternalFixture(t)
	s.failDone = true
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("progress failure ignored")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	s.failDone = false
	next := &Runtime{Store: s, Planner: r.Planner, Driver: r.Driver, Strategies: r.Strategies, ChainID: r.ChainID, Caller: r.Caller, Mode: r.Mode, OperationTimeout: r.OperationTimeout}
	defer next.Close()
	if err := next.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	w := next.Snapshot()[0]
	if w.Phase != "settled" || op.calls != 2 || w.Plan.Operations[0].Progress.Reference != "external-id" || string(w.Plan.Operations[0].Progress.State) != string(op.state) {
		t.Fatal("restart changed deterministic external identity", w, op.calls)
	}
}

func TestCoordinatorAccountReservationIgnoresAddressCase(t *testing.T) {
	r, s, op := coordinatorExternalFixture(t)
	op.done = false
	account := common.HexToAddress("0xabcdef0000000000000000000000000000000001")
	first := &r.Strategies[0]
	first.Config.Account = strings.ToLower(account.Hex())
	var w Workflow
	_, _ = s.LoadNamespace(Namespace, WorkflowKey(first.Config.Name), &w)
	w.Plan.Snapshot.Account = account
	w.ConfigHash = first.Config.Fingerprint()
	if err := s.SaveNamespace(Namespace, WorkflowKey(w.Name), w); err != nil {
		t.Fatal(err)
	}
	other := *first
	other.Config.Name = "other"
	other.Config.Executor = common.HexToAddress("0x55").Hex()
	other.Config.Account = strings.ToUpper(account.Hex())
	r.Strategies = append(r.Strategies, other)
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.Planner.(*coordinatorPlanner).prepares != 0 {
		t.Fatal("same account acquired twice using different letter case")
	}
}

func TestCoordinatorReadySelectionIsFair(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	second := r.Strategies[0]
	second.Config.Name = common.HexToAddress("0x11").Hex()
	second.Config.Executor = second.Config.Name
	second.Config.Account = common.HexToAddress("0x21").Hex()
	r.Strategies = append(r.Strategies, second)
	plan := coordinatorPlan(t, second.Config.Name)
	plan.Snapshot.Account = common.HexToAddress(second.Config.Account)
	r.Planner.(*coordinatorPlanner).plans[second.Config.Name] = plan
	var order []common.Address
	d.execute = func(_ context.Context, intent Intent, persist PersistTransaction) TransactionResult {
		order = append(order, intent.To)
		if err := persist(Transaction{Intent: intent, Sender: r.Caller, ChainID: big.NewInt(1), Admitted: true}); err != nil {
			return TransactionResult{Err: err}
		}
		return TransactionResult{NotSubmitted: true}
	}
	for range 4 {
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(order) != 4 || order[0] == order[1] || order[0] != order[2] || order[1] != order[3] {
		t.Fatal("ready strategy starved", order)
	}
}

func TestCoordinatorFairSelectionSkipsExcludedStrategyGaps(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	first := r.Strategies[0]
	for i := range 4 {
		s := first
		s.Config.Name = big.NewInt(int64(i + 100)).String()
		s.Config.Interval = time.Hour
		r.Strategies = append(r.Strategies, s)
	}
	last := first
	last.Config.Name = common.HexToAddress("0x44").Hex()
	last.Config.Executor = last.Config.Name
	last.Config.Account = common.HexToAddress("0x45").Hex()
	r.Strategies = append(r.Strategies, last)
	plan := coordinatorPlan(t, last.Config.Name)
	plan.Snapshot.Account = common.HexToAddress(last.Config.Account)
	r.Planner.(*coordinatorPlanner).plans[last.Config.Name] = plan
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	for _, s := range r.Strategies[1:5] {
		r.workflows[s.Config.Name] = Workflow{Name: s.Config.Name, Phase: "settled", FinishedAt: time.Now().UTC()}
	}
	r.mu.Unlock()
	var order []common.Address
	d.execute = func(_ context.Context, intent Intent, _ PersistTransaction) TransactionResult {
		order = append(order, intent.To)
		return TransactionResult{NotSubmitted: true}
	}
	for range 4 {
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(order) != 4 || order[0] == order[1] || order[0] != order[2] || order[1] != order[3] {
		t.Fatal("filtered gaps biased ready selection", order)
	}
}

func TestCoordinatorRepeatedCloseNeverClaimsUndrainedWorkerCompleted(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	r.OperationTimeout = 10 * time.Millisecond
	r.ShutdownTimeout = 10 * time.Millisecond
	release := make(chan struct{})
	d.execute = func(context.Context, Intent, PersistTransaction) TransactionResult {
		<-release
		return TransactionResult{Pending: true}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err == nil {
		t.Fatal("undrained worker reported closed")
	}
	if err := r.Close(); err == nil {
		t.Fatal("second close reported success before worker drained")
	}
	close(release)
	if err := r.Close(); err != nil {
		t.Fatal("drained worker did not close", err)
	}
}

func TestCoordinatorGlobalStorageObservationIsSanitized(t *testing.T) {
	r, s, _ := coordinatorHarness(t)
	var logs []string
	r.Log = funcr.New(func(prefix, args string) { logs = append(logs, prefix+args) }, funcr.Options{})
	var observations []string
	r.Observe = func(strategy, outcome string) { observations = append(observations, strategy+":"+outcome) }
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.fail = true
	if err := r.Tick(context.Background()); err == nil {
		t.Fatal("storage error ignored")
	}
	joined := strings.Join(logs, " ")
	if strings.Contains(joined, "injected") || !strings.Contains(strings.Join(observations, " "), "_global:error") {
		t.Fatal("global failure leaked details or missed observation", logs, observations)
	}
}
