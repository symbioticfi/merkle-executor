package keeper

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type scratchProvider struct {
	output []provider.Output
	calls  int
}

func (*scratchProvider) RecipeRequirements(s provider.Snapshot, _ recipes.Recipe, _ *recipes.Tree) (RecipeRequirements, error) {
	s.ChainID.SetInt64(99)
	s.BlockNumber.SetInt64(99)
	return RecipeRequirements{}, nil
}

func (p *scratchProvider) PrepareRecipe(_ context.Context, s provider.Snapshot, _ recipes.Recipe, _ *recipes.Tree, _ map[uint]*big.Int) ([]provider.Output, error) {
	s.ChainID.SetInt64(98)
	s.BlockNumber.SetInt64(98)
	p.calls++
	p.output[0].Data[0] = byte(p.calls)
	return p.output, nil
}

func TestBuildPlanIsolatesProviderSnapshotsAndReusedOutputs(t *testing.T) {
	runtime := &scratchProvider{output: []provider.Output{{Data: []byte{0}}}}
	p := &Planner{ChainID: big.NewInt(1)}
	snapshot := recipes.Snapshot{Executor: common.Address{1}, Account: common.Address{2}, Caller: common.Address{3}, BlockNumber: 12}
	strategy := Strategy{Providers: []Provider{runtime}}
	first, _, err := p.buildPlan(t.Context(), strategy, snapshot, recipes.Recipe{}, &recipes.Tree{})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := p.buildPlan(t.Context(), strategy, snapshot, recipes.Recipe{}, &recipes.Tree{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Snapshot.ChainID.Int64() != 1 || first.Snapshot.BlockNumber.Uint64() != 12 || second.Snapshot.ChainID.Int64() != 1 {
		t.Fatal("provider mutated authenticated plan snapshot")
	}
	if first.Outputs[0][0].Data[0] != 1 || second.Outputs[0][0].Data[0] != 2 {
		t.Fatal("later preparation mutated earlier output")
	}
}

func TestSettlementTasksReceiveIndependentCanonicalEvidence(t *testing.T) {
	k, p, sender := runtimeHarness(t)
	plan := p.plans["one"]
	plan.Snapshot.ChainID = big.NewInt(1)
	plan.Snapshot.BlockNumber = big.NewInt(3)
	calls := 0
	first := provider.TaskFunc(func(_ context.Context, s provider.Snapshot) (provider.Progress, error) {
		calls++
		s.ChainID.SetInt64(99)
		s.Execution.BlockNumber.SetInt64(99)
		s.Execution.Logs[0].Data[0] = 99
		return provider.Progress{Done: true}, nil
	})
	second := provider.TaskFunc(func(_ context.Context, s provider.Snapshot) (provider.Progress, error) {
		calls++
		if s.ChainID.Int64() != 1 || s.Execution.BlockNumber.Int64() != 4 || s.Execution.Logs[0].Data[0] != 1 {
			t.Fatal("earlier task mutated later task's evidence")
		}
		return provider.Progress{Done: true}, nil
	})
	plan.Outputs = [][]provider.Output{{{Task: first}}, {{Task: second}}}
	p.plans["one"] = plan
	k.Strategies[0].Providers = []Provider{&testProvider{}, &testProvider{}}
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	result := confirmedResult()
	result.Execution.Logs = []types.Log{{TxHash: result.Hash, BlockHash: result.Execution.BlockHash, BlockNumber: 4, Index: 1, Data: []byte{1}}}
	sender.result = result
	if err := k.Tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || k.Pending() {
		t.Fatalf("tasks=%d pending=%v", calls, k.Pending())
	}
}
