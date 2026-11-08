package keeper

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type reservationProvider struct {
	task    bool
	witness bool
	calls   int
}

func (*reservationProvider) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return RecipeRequirements{Values: []recipes.Value{{Source: recipes.Literal, Value: big.NewInt(1)}}}, nil
}

func (p *reservationProvider) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) ([]provider.Output, error) {
	p.calls++
	out := provider.Output{}
	if p.witness {
		out.Results = map[uint]string{0: "1"}
	}
	if p.task {
		out.Task = provider.TaskFunc(func(context.Context, provider.Snapshot) (provider.Progress, error) {
			return provider.Progress{Done: true}, nil
		})
	}
	return []provider.Output{out}, nil
}

type reservationReader struct {
	*plannerScaleReader
	target common.Address
	mode   string
	hit    bool
}

func (r *reservationReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if msg.To != nil && *msg.To == r.target && len(msg.Data) >= 4 {
		contract := executor.NewRecipeExecutor().GetABI()
		method, err := contract.MethodById(msg.Data[:4])
		if err == nil && method.Name == "execute" {
			r.hit = true
			switch r.mode {
			case "rejected":
				return nil, errors.New("out of gas")
			case "transport":
				return nil, errors.New("RPC unavailable")
			case "reorg":
				r.header = types.CopyHeader(r.header)
				r.header.Extra = []byte("reorganized during admission")
			}
		}
	}
	return r.plannerScaleReader.CallContract(ctx, msg, block)
}

func idleReservationRecipe(t *testing.T, r *plannerScaleReader, index int) {
	t.Helper()
	args, err := scaleArgs(t, "uint256", "bytes32[]", "uint256[][]").Pack(new(big.Int), [][32]byte{}, [][]*big.Int{})
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, args...)}}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	r.members[index].Recipe = recipe
	r.members[index].Snapshot.RecipeHash = hash
	log := r.logs[r.members[index].Snapshot.Executor]
	log.Topics[1] = hash
	log.Data, err = executor.NewRecipeExecutor().GetABI().Events["SetRecipe"].Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
	if err != nil {
		t.Fatal(err)
	}
	r.logs[log.Address] = log
}

func TestPlannerTaskReservationFollowsAdmittedWork(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tasks []bool
		idle  int
		want  []int
	}{
		{name: "idle task releases account", tasks: []bool{true, true}, idle: 0, want: []int{1}},
		{name: "active task keeps account", tasks: []bool{true, true}, idle: -1, want: []int{0}},
		{name: "idle task after healthy prefix releases account", tasks: []bool{false, true, true}, idle: 1, want: []int{0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, r, strategies, _ := scaleFixture(t, len(tc.tasks))
			r.accepted = map[int]bool{1: true, 2: true, 3: true}
			for i, task := range tc.tasks {
				strategies[i].Providers = []Provider{&reservationProvider{task: task}}
			}
			if tc.idle >= 0 {
				idleReservationRecipe(t, r, tc.idle)
			}
			plan, err := p.PrepareExecution(t.Context(), strategies)
			if err != nil || len(plan.Members) != len(tc.want) {
				t.Fatalf("members=%d, want %d; error=%v", len(plan.Members), len(tc.want), err)
			}
			for i, want := range tc.want {
				if plan.Members[i].Name != strategies[want].Config.Name {
					t.Fatalf("member %d=%s, want %s", i, plan.Members[i].Name, strategies[want].Config.Name)
				}
			}
		})
	}
}

func TestPlannerReleasedTaskReservationPreservesFreshValidation(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 2)
	r.accepted = map[int]bool{1: true, 2: true}
	idleReservationRecipe(t, r, 0)
	strategies[0].Providers = []Provider{&reservationProvider{task: true}}
	strategies[1].Providers = []Provider{&reservationProvider{task: true}}
	r.bad = r.members[1].Snapshot.Executor
	r.badAfterDiscovery = true
	r.deferredError = errors.New("executor metadata changed before admission")
	plan, err := p.PrepareExecution(t.Context(), strategies)
	if !errors.Is(err, r.deferredError) || len(plan.Members) != 0 {
		t.Fatalf("fallback bypassed fresh metadata validation: members=%d error=%v", len(plan.Members), err)
	}
}

func TestPlannerTaskReservationReleasesRejectedCandidate(t *testing.T) {
	for _, mode := range []string{"metadata", "witness", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			p, r, strategies, _ := scaleFixture(t, 2)
			r.accepted = map[int]bool{1: true, 2: true}
			first, next := &reservationProvider{task: true}, &reservationProvider{task: true}
			strategies[0].Providers, strategies[1].Providers = []Provider{first}, []Provider{next}
			switch mode {
			case "metadata":
				r.bad, r.badAfterDiscovery = r.members[0].Snapshot.Executor, true
				r.deferredError = errors.New("task executor metadata unavailable at admission")
			case "witness":
				first.witness = true
			case "rejected":
				p.Chain = &reservationReader{plannerScaleReader: r, target: r.members[0].Snapshot.Executor, mode: mode}
			}
			plan, err := p.PrepareExecution(t.Context(), strategies)
			if err != nil || len(plan.Members) != 1 || plan.Members[0].Name != strategies[1].Config.Name {
				t.Fatalf("rejected task blocked sibling: members=%d error=%v", len(plan.Members), err)
			}
			if plan.Failures[strategies[0].Config.Name] == nil {
				t.Fatalf("rejected task diagnostic was lost: %v", plan.Failures)
			}
		})
	}
}

func TestPlannerTaskReservationPreservesDiscoveryFailure(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 3)
	r.accepted = map[int]bool{1: true, 2: true, 3: true}
	r.bad = r.members[0].Snapshot.Executor
	r.deferredError = errors.New("first executor unavailable during discovery")
	for i := 1; i < len(strategies); i++ {
		strategies[i].Providers = []Provider{&reservationProvider{task: true}}
	}
	idleReservationRecipe(t, r, 1)
	plan, err := p.PrepareExecution(t.Context(), strategies)
	if err != nil || len(plan.Members) != 1 || plan.Members[0].Name != strategies[2].Config.Name || !errors.Is(plan.Failures[strategies[0].Config.Name], r.deferredError) {
		t.Fatalf("idle task hid prior diagnostic or blocked sibling: members=%d failures=%v error=%v", len(plan.Members), plan.Failures, err)
	}
}

func TestPlannerTaskReservationKeepsSharedFailuresFatal(t *testing.T) {
	for _, mode := range []string{"transport", "reorg"} {
		t.Run(mode, func(t *testing.T) {
			p, r, strategies, _ := scaleFixture(t, 3)
			r.accepted = map[int]bool{1: true, 2: true, 3: true}
			r.bad = r.members[0].Snapshot.Executor
			r.deferredError = errors.New("earlier executor discovery failed")
			idleReservationRecipe(t, r, 1)
			first, next := &reservationProvider{task: true}, &reservationProvider{task: true}
			strategies[1].Providers, strategies[2].Providers = []Provider{first}, []Provider{next}
			reader := &reservationReader{plannerScaleReader: r, target: r.members[1].Snapshot.Executor, mode: mode}
			p.Chain = reader
			plan, err := p.PrepareExecution(t.Context(), strategies)
			if err == nil || errors.Is(err, provider.ErrNotReady) || len(plan.Members) != 0 || !reader.hit || next.calls != 0 {
				t.Fatalf("shared admission failure was ignored: members=%d hit=%v fallback calls=%d error=%v", len(plan.Members), reader.hit, next.calls, err)
			}
		})
	}
}
