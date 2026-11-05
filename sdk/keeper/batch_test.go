package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type batchPlannerFixture struct {
	*recoveryPlanner
	preparedBatches, validatedBatches int
}

func (p *batchPlannerFixture) PrepareBatch(ctx context.Context, strategies []Strategy) (BatchPlan, error) {
	p.preparedBatches++
	var batch BatchPlan
	for _, strategy := range strategies {
		plan, err := p.Prepare(ctx, strategy)
		if err != nil {
			return batch, err
		}
		plan.Calldata = executor.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)
		plan.Recipe = recipes.Snapshot{Account: plan.Snapshot.Account, Executor: plan.Snapshot.Executor, Caller: plan.Snapshot.Caller}
		batch.Members = append(batch.Members, BatchMember{Name: strategy.Config.Name, Plan: plan})
	}
	return batch, nil
}
func (p *batchPlannerFixture) ValidateBatch(context.Context, BatchPlan) error {
	p.validatedBatches++
	return nil
}

func batchHarness(t *testing.T) (*Keeper, *recoveryStore, *batchPlannerFixture, *recoverySender, []*recoveryProvider) {
	t.Helper()
	k, store, planner, sender, first := recoveryHarness(t)
	p := &batchPlannerFixture{recoveryPlanner: planner}
	k.Planner, k.Batching = p, true
	second := &recoveryProvider{done: true}
	strategy := k.Strategies[0]
	strategy.Config.Name, strategy.Config.Executor = "withdraw", common.Address{0x11}.Hex()
	strategy.Providers = []provider.Provider{second}
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
			if sender.broadcasts != 1 || p.preparedBatches != 1 || p.validatedBatches != 1 {
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
	var before BatchJob
	if found, err := store.Load(BatchKey, &before); err != nil || !found {
		t.Fatal("missing pending batch", err)
	}
	want, _ := json.Marshal(before)
	k.Mode = "simulate"
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	var after BatchJob
	if found, err := store.Load(BatchKey, &after); err != nil || !found {
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
	var batch BatchJob
	if _, err := store.Load(BatchKey, &batch); err != nil {
		t.Fatal(err)
	}
	batch.Members[0].Job.Plan.Recipe.Account = common.Address{42}
	if err := store.Save(BatchKey, batch); err != nil {
		t.Fatal(err)
	}
	if err := k.CheckRecovery(); err == nil {
		t.Fatal("mismatched member account accepted")
	}
	if sender.PendingID() != batch.ID {
		t.Fatal("corruption released pending transaction")
	}
}

func TestBatchCannotBeAbandonedByDisablingDelegation(t *testing.T) {
	k, store, _, sender, _ := batchHarness(t)
	store.failPhase = "authorized"
	if err := k.Tick(t.Context()); err == nil {
		t.Fatal("expected persistence failure")
	}
	k.Batching = false
	for _, pending := range []string{sender.pending, ""} {
		sender.pending = pending
		if err := k.CheckRecovery(); err == nil {
			t.Fatal("unfinished batch ignored when delegation disabled")
		}
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
			providers := []provider.Provider{&recoveryProvider{}}
			if tc.exclusive {
				providers[0] = &exclusiveProvider{claims: map[string]common.Address{"0:0": tc.target}}
			}
			strategies := []Strategy{
				{Config: Config{Name: "one"}, Providers: []provider.Provider{&exclusiveProvider{claims: map[string]common.Address{"0:0": first}}}},
				{Config: Config{Name: "two"}, Providers: providers},
			}
			batch := BatchPlan{Members: []BatchMember{
				{Name: "one", Plan: Plan{Calldata: compositionCalldata(t, first), Outputs: make([]provider.Output, 1)}},
				{Name: "two", Plan: Plan{Calldata: compositionCalldata(t, tc.target), Outputs: make([]provider.Output, 1)}},
			}}
			if err := validateBatchComposition(batch, strategies); (err == nil) != tc.want {
				t.Fatalf("receipt ownership: %v", err)
			}
		})
	}
}
