package keeper

import (
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type countedCompositionProvider struct {
	*testProvider
	calls int
}

func (p *countedCompositionProvider) ExclusiveOutputTargets(provider.Output) (map[string]common.Address, error) {
	p.calls++
	return nil, nil
}

func TestExecutionCompositionInspectsEveryCandidateOnce(t *testing.T) {
	for _, count := range []int{10, 100} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			runtime := &countedCompositionProvider{testProvider: &testProvider{}}
			composition := newExecutionComposition()
			var strategies []Strategy
			batch := ExecutionPlan{}
			for i := range count {
				strategy := Strategy{Config: Config{Name: strconv.Itoa(i)}, Providers: []Provider{runtime}}
				plan := Plan{Calldata: compositionCalldata(t, common.Address{1}), Outputs: [][]provider.Output{{{}}}}
				if err := composition.add(plan, strategy); err != nil {
					t.Fatal(err)
				}
				strategies = append(strategies, strategy)
				batch.Members = append(batch.Members, PlanMember{Name: strategy.Config.Name, Plan: plan})
			}
			if runtime.calls != count {
				t.Fatalf("candidate inspections=%d want%d", runtime.calls, count)
			}
			runtime.calls = 0
			if err := validateExecutionComposition(batch, strategies); err != nil {
				t.Fatal(err)
			}
			if runtime.calls != count {
				t.Fatalf("final inspections=%d want%d", runtime.calls, count)
			}
		})
	}
}

func TestRejectedCompositionCandidateLeavesAcceptedPrefixUnchanged(t *testing.T) {
	target := common.Address{1}
	plan := Plan{Calldata: compositionCalldata(t, target), Outputs: [][]provider.Output{{{}}}}
	native := Strategy{Providers: []Provider{&testProvider{}}}
	composition := newExecutionComposition()
	if err := composition.add(plan, native); err != nil {
		t.Fatal(err)
	}
	exclusive := Strategy{Providers: []Provider{&exclusiveProvider{claims: map[string]common.Address{"0:0": target}}}}
	if err := composition.add(plan, exclusive); err == nil {
		t.Fatal("exclusive candidate ignored existing native call")
	}
	if err := composition.add(plan, native); err != nil {
		t.Fatalf("rejected candidate poisoned accepted ownership: %v", err)
	}
}
