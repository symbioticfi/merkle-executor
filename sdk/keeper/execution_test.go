package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type batchPlannerFixture struct {
	*recoveryPlanner
	preparedBatches int
}

func (p *batchPlannerFixture) PrepareExecution(ctx context.Context, strategies []Strategy) (ExecutionPlan, error) {
	p.preparedBatches++
	batch := ExecutionPlan{To: common.HexToAddress("0x30")}
	for _, strategy := range strategies {
		plan, err := p.Prepare(ctx, strategy)
		if err != nil {
			return batch, err
		}
		plan.Calldata = executor.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)
		plan.Recipe = recipes.Snapshot{Account: plan.Snapshot.Account, Executor: plan.Snapshot.Executor, Caller: plan.Snapshot.Caller}
		batch.Members = append(batch.Members, PlanMember{Name: strategy.Config.Name, Plan: plan})
	}
	return batch, nil
}
func batchHarness(t *testing.T) (*Keeper, *recoveryStore, *batchPlannerFixture, *recoverySender, []*recoveryProvider) {
	t.Helper()
	k, store, planner, sender, first := recoveryHarness(t)
	p := &batchPlannerFixture{recoveryPlanner: planner}
	k.Planner = p
	second := &recoveryProvider{done: true}
	strategy := k.Strategies[0]
	strategy.Config.Name, strategy.Config.Executor = "withdraw", common.Address{0x11}.Hex()
	strategy.Providers = []Provider{second}
	k.Strategies = append(k.Strategies, strategy)
	return k, store, p, sender, []*recoveryProvider{first, second}
}

func TestBatchPersistsEveryMemberBeforeAcknowledgement(t *testing.T) {
	for _, failure := range []string{"", "member save", "acknowledgement"} {
		t.Run(failure, func(t *testing.T) {
			k, store, p, sender, providers := batchHarness(t)
			if failure == "member save" {
				store.failPhase = "authorized"
			}
			if failure == "acknowledgement" {
				sender.ackFailures = 1
			}
			sender.beforeAck = func(id string) error {
				for _, strategy := range k.Strategies {
					job := loadRecoveryJob(t, store, strategy.Config.Name)
					if job.ID != id || job.Phase != "authorized" {
						t.Fatal("batch acknowledged before every member was durable")
					}
				}
				return nil
			}
			err := k.Tick(t.Context())
			if failure != "" {
				if err == nil {
					t.Fatal("persistence failure ignored")
				}
				for _, provider := range providers {
					if provider.calls != 0 {
						t.Fatal("provider advanced before batch committed")
					}
				}
				if err = k.CheckRecovery(); err != nil {
					t.Fatal(err)
				}
				err = k.Tick(t.Context())
			}
			if err != nil {
				t.Fatal(err)
			}
			if sender.broadcasts != 1 || p.preparedBatches != 1 {
				t.Fatalf("batch re-prepared or resubmitted: %+v %+v", sender, p)
			}
			if len(sender.requests) != 1 || sender.requests[0].To != common.HexToAddress("0x30") {
				t.Fatal("batch was not sent to keeper account once")
			}
			for _, provider := range providers {
				if provider.calls != 1 {
					t.Fatal("member was not reconciled exactly once")
				}
			}
		})
	}
}

func TestBatchRecoveryRejectsRemovedOrChangedMember(t *testing.T) {
	for _, changed := range []bool{false, true} {
		k, store, _, sender, _ := batchHarness(t)
		store.failPhase = "authorized"
		if err := k.Tick(t.Context()); err == nil {
			t.Fatal("expected persistence failure")
		}
		if sender.PendingID() == "" {
			t.Fatal("signed batch was lost")
		}
		if changed {
			k.Strategies[1].Config.Revision = "changed"
		} else {
			k.Strategies = k.Strategies[:1]
		}
		if err := k.CheckRecovery(); err == nil {
			t.Fatal("active batch member changed without rejection")
		}
	}
}

func TestBatchKeepsExternalSettlementAccountReserved(t *testing.T) {
	k, _, p, sender, providers := batchHarness(t)
	providers[0].done = false
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	k.Strategies[1].Config.Interval = time.Nanosecond
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if sender.broadcasts != 1 || p.preparedBatches != 1 {
		t.Fatal("new work spent reserved account funds")
	}
}

func TestBatchRevertMarksAllMembersFailed(t *testing.T) {
	k, store, _, sender, providers := batchHarness(t)
	sender.reverted = true
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("batch revert ignored")
	}
	for _, strategy := range k.Strategies {
		if job := loadRecoveryJob(t, store, strategy.Config.Name); job.Phase != "failed" {
			t.Fatal("batch member not failed")
		}
	}
	for _, provider := range providers {
		if provider.calls != 0 {
			t.Fatal("reverted member advanced")
		}
	}
}

func TestBatchSimulationPreservesPendingWork(t *testing.T) {
	k, store, _, sender, providers := batchHarness(t)
	store.failPhase = "authorized"
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("expected persistence failure")
	}
	var before ExecutionJob
	if found, err := store.Load(ExecutionKey, &before); err != nil || !found {
		t.Fatal("missing pending batch", err)
	}
	want, _ := json.Marshal(before)
	k.Mode = "simulate"
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	var after ExecutionJob
	if found, err := store.Load(ExecutionKey, &after); err != nil || !found {
		t.Fatal("simulation discarded pending batch", err)
	}
	got, _ := json.Marshal(after)
	if !bytes.Equal(got, want) || len(sender.requests) != 1 || sender.PendingID() != before.ID {
		t.Fatal("simulation changed submission state")
	}
	for _, provider := range providers {
		if provider.calls != 0 {
			t.Fatal("simulation advanced a provider")
		}
	}
}

func TestBatchRejectsMismatchedMemberIdentity(t *testing.T) {
	k, store, _, sender, _ := batchHarness(t)
	store.failPhase = "authorized"
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("expected persistence failure")
	}
	var batch ExecutionJob
	if _, err := store.Load(ExecutionKey, &batch); err != nil {
		t.Fatal(err)
	}
	batch.Members[0].Plan.Recipe.Account = common.Address{42}
	if err := store.Save(ExecutionKey, batch); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckRecovery(); err == nil {
		t.Fatal("mismatched member account accepted")
	}
	if sender.PendingID() != batch.ID {
		t.Fatal("corruption released pending transaction")
	}
}

func TestSignedBatchRecoveryDoesNotPrepareAgain(t *testing.T) {
	k, store, p, sender, _ := batchHarness(t)
	store.failPhase = "authorized"
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("expected persistence failure")
	}
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if p.preparedBatches != 1 || len(sender.requests) != 1 {
		t.Fatal("recovery prepared or sent a second transaction")
	}
}

func TestBatchProtectsTransactionWideReceiptOwnership(t *testing.T) {
	first, other := common.Address{1}, common.Address{2}
	for _, tc := range []struct {
		name            string
		target          common.Address
		exclusive, want bool
	}{
		{"separate targets", other, true, true},
		{"shared receipt target", first, true, false},
		{"unowned call to receipt target", first, false, false},
		{"unrelated native call", other, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			providers := []Provider{&recoveryProvider{}}
			if tc.exclusive {
				providers[0] = &exclusiveProvider{claims: map[string]common.Address{"0:0": tc.target}}
			}
			strategies := []Strategy{
				{Config: Config{Name: "one"}, Providers: []Provider{&exclusiveProvider{claims: map[string]common.Address{"0:0": first}}}},
				{Config: Config{Name: "two"}, Providers: providers},
			}
			batch := ExecutionPlan{Members: []PlanMember{
				{Name: "one", Plan: Plan{Calldata: compositionCalldata(t, first), Outputs: [][]provider.Output{{{}}}}},
				{Name: "two", Plan: Plan{Calldata: compositionCalldata(t, tc.target), Outputs: [][]provider.Output{{{}}}}},
			}}
			if err := validateExecutionComposition(batch, strategies); (err == nil) != tc.want {
				t.Fatalf("receipt ownership: %v", err)
			}
		})
	}
}

func TestUnsignedExecutionIsReprepared(t *testing.T) {
	for _, delegated := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "delegated"}[delegated], func(t *testing.T) {
			k, store, _, sender, _ := batchHarness(t)
			if !delegated {
				k.Strategies = k.Strategies[:1]
				k.Planner = &recoveryPlanner{}
			}
			plan, err := k.Planner.PrepareExecution(t.Context(), k.Strategies)
			if err != nil {
				t.Fatal(err)
			}
			// A structurally valid old input must not survive an unsigned restart.
			plan.Members[0].Plan.Calldata = executor.NewRecipeExecutor().PackExecute(nil, nil, []*big.Int{big.NewInt(99)}, nil)
			old, err := newExecution(plan, k.Strategies)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(ExecutionKey, old); err != nil {
				t.Fatal(err)
			}
			if err := k.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(sender.requests) != 1 || sender.requests[0].ID == old.ID || bytes.Equal(sender.requests[0].Data, old.Data) {
				t.Fatal("unsigned execution was reused instead of prepared from current inputs")
			}
		})
	}
}

func TestExecutionCompositionIncludesRepeatedNativeCalls(t *testing.T) {
	for _, exclusive := range []bool{false, true} {
		for _, collision := range []bool{false, true} {
			t.Run(fmt.Sprintf("exclusive=%t/collision=%t", exclusive, collision), func(t *testing.T) {
				target, native := common.Address{1}, common.Address{2}
				if collision {
					native = target
				}
				first := &recoveryProvider{}
				var runtime Provider = first
				if exclusive {
					runtime = &exclusiveProvider{claims: map[string]common.Address{"0:0": target}}
				}
				strategies := []Strategy{{Config: Config{Name: "claim"}, Providers: []Provider{runtime}}, {Config: Config{Name: "native"}, Providers: []Provider{&recoveryProvider{}}}}
				repeated, err := recipes.Multicall([][]byte{compositionCalldata(t, native), compositionCalldata(t, native)})
				if err != nil {
					t.Fatal(err)
				}
				plan := ExecutionPlan{Members: []PlanMember{{Name: "claim", Plan: Plan{Calldata: compositionCalldata(t, target), Outputs: [][]provider.Output{{{}}}}}, {Name: "native", Plan: Plan{Calldata: repeated, Outputs: [][]provider.Output{{{}}}}}}}
				err = validateExecutionComposition(plan, strategies)
				if (err != nil) != (exclusive && collision) {
					t.Fatalf("unexpected composition result: %v", err)
				}
			})
		}
	}
}
