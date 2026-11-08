package keeper

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestExecutionCompositionIncludesRepeatedNativeCalls(t *testing.T) {
	for _, exclusive := range []bool{false, true} {
		for _, collision := range []bool{false, true} {
			t.Run(fmt.Sprintf("exclusive=%t/collision=%t", exclusive, collision), func(t *testing.T) {
				target, native := common.Address{1}, common.Address{2}
				if collision {
					native = target
				}
				first := &testProvider{}
				var runtime Provider = first
				if exclusive {
					runtime = &exclusiveProvider{claims: map[string]common.Address{"0:0": target}}
				}
				strategies := []Strategy{{Config: Config{Name: "claim"}, Providers: []Provider{runtime}}, {Config: Config{Name: "native"}, Providers: []Provider{&testProvider{}}}}
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
