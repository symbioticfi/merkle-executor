package keeper

import (
	"testing"

	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestPlannerFitShrinksEnvelopesToConfiguredGasBudget(t *testing.T) {
	for _, kind := range []string{"same executor", "delegated"} {
		t.Run(kind, func(t *testing.T) {
			count := 4
			if kind == "same executor" {
				count = 1
			}
			p, r, strategies, batch := scaleFixture(t, count)
			p.MaxGas = 2_000_000
			r.accepted = map[int]bool{1: true, 2: true, 3: true, 4: true}
			r.gasPerCall = 1_000_000
			if kind == "same executor" {
				entry := batch.Members[0].Plan.Calldata
				var err error
				batch.Members[0].Plan.Calldata, err = recipes.Multicall([][]byte{entry, entry, entry, entry})
				if err != nil {
					t.Fatal(err)
				}
			}
			hash := r.header.Hash()
			plan, err := p.fit(t.Context(), batch, r.members, strategies, nil)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "same executor" {
				entries, err := recipes.Executions(plan.Members[0].Plan.Calldata)
				if err != nil || len(entries) != 2 {
					t.Fatalf("fit retained %d executions: %v", len(entries), err)
				}
			} else if len(plan.Members) != 2 {
				t.Fatalf("fit retained %d executors", len(plan.Members))
			}
			if len(r.gasBudgets) == 0 {
				t.Fatal("missing exact envelope simulation")
			}
			for _, gas := range r.gasBudgets {
				if gas != 2_000_000 {
					t.Fatalf("fit simulated gas %d outside configured budget", gas)
				}
			}
			if r.header.Hash() != hash {
				t.Fatal("gas budgeting changed the canonical header")
			}
		})
	}
}

func TestPlannerDirectAdmissionUsesConfiguredGasBudget(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 1)
	p.Chain = struct{ Backend }{r}
	p.MaxGas = 2_000_000
	if _, err := p.PrepareExecution(t.Context(), strategies); err != nil {
		t.Fatal(err)
	}
	if len(r.gasBudgets) != 1 || r.gasBudgets[0] != 2_000_000 {
		t.Fatalf("direct admission budgets: %v", r.gasBudgets)
	}
}

func TestPlannerRepetitionUsesConfiguredGasBudget(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 1)
	p.MaxGas = 2_000_000
	r.accepted = map[int]bool{1: true, 2: true}
	if _, err := p.PrepareExecution(t.Context(), strategies); err != nil {
		t.Fatal(err)
	}
	if len(r.gasBudgets) < 3 {
		t.Fatalf("missing repetition simulations: %v", r.gasBudgets)
	}
	for _, gas := range r.gasBudgets {
		if gas != 2_000_000 {
			t.Fatalf("repetition simulated gas %d outside configured budget", gas)
		}
	}
}
