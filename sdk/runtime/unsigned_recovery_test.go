package runtime

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func addSameAccountStrategy(t *testing.T, r *Runtime) Strategy {
	t.Helper()
	second := r.Strategies[0]
	second.Config.Name = common.HexToAddress("0x11").Hex()
	second.Config.Executor = second.Config.Name
	r.Strategies = append(r.Strategies, second)
	r.Planner.(*coordinatorPlanner).plans[second.Config.Name] = coordinatorPlan(t, second.Config.Name)
	return second
}

func TestUnsignedReleasedWorkflowDoesNotBlockSameAccountExecutor(t *testing.T) {
	r, _, d := coordinatorHarness(t)
	first := r.Strategies[0].Config.Name
	second := addSameAccountStrategy(t, r)
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 1 || d.releases != 1 {
		t.Fatal("unsigned admission not released", d.executions, d.releases)
	}
	delete(r.Planner.(*coordinatorPlanner).plans, first)
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 2 || d.releases != 2 {
		t.Fatal("unsigned idle workflow retained account", d.executions, d.releases, r.accounts[common.HexToAddress(second.Config.Account)])
	}
}

func TestUnsignedPreparedRestartDoesNotRecoverAccountOwnership(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	first := r.Strategies[0]
	second := addSameAccountStrategy(t, r)
	prepared := Workflow{Version: 2, IdentityVersion: 2, Name: first.Config.Name, ID: "unsigned-before-crash", ConfigHash: first.Config.Fingerprint(), Phase: "prepared", Plan: coordinatorPlan(t, first.Config.Name)}
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(prepared.Name): prepared}); err != nil {
		t.Fatal(err)
	}
	delete(r.Planner.(*coordinatorPlanner).plans, first.Config.Name)
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if owner := r.accounts[common.HexToAddress(second.Config.Account)]; owner != "" {
		t.Fatal("unsigned checkpoint recovered stale account owner", owner)
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 1 || d.resumes != 0 {
		t.Fatal("unsigned checkpoint prevented fresh same-account admission", d.executions, d.resumes)
	}
	var retained Workflow
	if found, err := s.LoadNamespace(Namespace, WorkflowKey(first.Config.Name), &retained); err != nil || !found || retained.ID != prepared.ID {
		t.Fatal("unsigned history was deleted or rewritten", found, err)
	}
}

func TestUnsignedOwnershipReleaseUncertaintyRetainsAdmissionsFence(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-replacement", true: "after-replacement"}[after], func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			first := r.Strategies[0].Config.Name
			second := addSameAccountStrategy(t, r)
			d.execute = func(_ context.Context, intent Intent, persist PersistTransaction) TransactionResult {
				if err := persist(Transaction{Intent: intent, Sender: r.Caller, ChainID: r.ChainID, Admitted: true}); err != nil {
					return TransactionResult{Err: err}
				}
				s.fail, s.after = true, after
				return TransactionResult{NotSubmitted: true}
			}
			if err := r.Tick(context.Background()); err == nil {
				t.Fatal("uncertain unsigned release ignored")
			}
			s.fail = false
			if err := r.Tick(context.Background()); err == nil {
				t.Fatal("uncertain ownership allowed another admission")
			}
			if d.executions != 1 || d.releases != 0 || r.owner != first || r.accounts[common.HexToAddress(second.Config.Account)] != first {
				t.Fatal("uncertain write released ownership", d.executions, d.releases, r.owner, r.accounts)
			}
		})
	}
}

func TestUnsignedPendingReservationStillOwnsAccount(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	first := r.Strategies[0]
	second := addSameAccountStrategy(t, r)
	plan := coordinatorPlan(t, first.Config.Name)
	w := Workflow{Version: 2, IdentityVersion: 2, Name: first.Config.Name, ID: "admitted-unsigned", ConfigHash: first.Config.Fingerprint(), Phase: "pending", Reserved: true, Plan: plan}
	w.Transaction = &Transaction{Intent: Intent{ID: w.ID, To: plan.Snapshot.Executor, Data: plan.Calldata}, Sender: r.Caller, ChainID: r.ChainID, Admitted: true}
	if err := s.ActivateNamespace(Namespace, map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: r.ChainID, Caller: r.Caller}, WorkflowKey(w.Name): w}); err != nil {
		t.Fatal(err)
	}
	d.resume = func(context.Context, Transaction, PersistTransaction) TransactionResult {
		return TransactionResult{Pending: true}
	}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.executions != 0 || d.releases != 0 || r.accounts[common.HexToAddress(second.Config.Account)] != first.Config.Name {
		t.Fatal("unresolved unsigned admission lost ownership")
	}
}
