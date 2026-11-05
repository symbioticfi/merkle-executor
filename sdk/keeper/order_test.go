package keeper

import (
	"context"
	"slices"
	"testing"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type orderPlanner struct {
	*recoveryPlanner
	visited []string
}

func (p *orderPlanner) PrepareExecution(ctx context.Context, strategies []Strategy) (ExecutionPlan, error) {
	return prepareFixture(ctx, strategies, p.Prepare)
}

func (p *orderPlanner) Prepare(_ context.Context, s Strategy) (Plan, error) {
	p.visited = append(p.visited, s.Config.Name)
	return Plan{}, provider.ErrNotReady
}

func TestConfiguredPrioritySurvivesPendingRecovery(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "configured order", true: "pending first"}[pending], func(t *testing.T) {
			k, store, base, sender, _ := recoveryHarness(t)
			k.Mode = "simulate"
			p := &orderPlanner{recoveryPlanner: base}
			k.Planner = p
			prototype := k.Strategies[0]
			names := []string{"claims", "swap", "withdraw", "leverage", "deleverage", "3f", "ll"}
			k.Strategies = nil
			for _, name := range names {
				s := prototype
				s.Config.Name = name
				k.Strategies = append(k.Strategies, s)
			}
			want := slices.Clone(names)
			if pending {
				s := k.Strategies[len(k.Strategies)-1]
				sender.pending = "in-flight"
				plan, err := base.Prepare(t.Context(), s)
				if err != nil {
					t.Fatal(err)
				}
				batch, err := newExecution(ExecutionPlan{To: plan.Snapshot.Executor, Members: []PlanMember{{Name: s.Config.Name, Plan: plan}}}, k.Strategies)
				if err != nil {
					t.Fatal(err)
				}
				batch.ID = sender.pending
				if err := store.Save(ExecutionKey, batch); err != nil {
					t.Fatal(err)
				}
				// Simulation preserves configured order and does not recover signed work.
			}
			if err := k.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(p.visited, want) {
				t.Fatalf("order %v, want %v", p.visited, want)
			}
			var configured []string
			for _, strategy := range k.Strategies {
				configured = append(configured, strategy.Config.Name)
			}
			if !slices.Equal(configured, names) {
				t.Fatal("configuration order mutated")
			}
		})
	}
}
