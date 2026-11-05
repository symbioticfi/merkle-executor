package keeper

import (
	"math/big"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Repeated runtime data is safe only for a standalone call: repeating a larger
// recipe could repeat its funding or other financial effects too.
func repeatActions(recipe recipes.Recipe, tree *recipes.Tree, first []byte, outputs []provider.Output) ([]byte, error) {
	var repeated *provider.Output
	for i := range outputs {
		if len(outputs[i].Repeats) == 0 {
			continue
		}
		if repeated != nil {
			return nil, errors.New("multiple providers repeat recipe actions")
		}
		repeated = &outputs[i]
	}
	if repeated == nil {
		return first, nil
	}
	actions, err := recipes.ManagedActions(recipe, tree)
	if err != nil {
		return nil, err
	}
	if len(recipe.Queries) != 0 || len(recipe.Steps) != 1 || len(actions) != 1 || len(actions[0].Bindings) != 0 || len(actions[0].Slot.Leaves) != 1 || actions[0].Slot.Gate.Source != recipes.Literal || actions[0].Slot.Gate.Value.Cmp(big.NewInt(1)) != 0 {
		return nil, errors.New("repeated actions require one unconditional managed call without reads or bindings")
	}
	key := actions[0].Key
	for i := range outputs {
		out := &outputs[i]
		if len(out.Inputs) != 0 || len(out.Results) != 0 || len(out.SkipSteps) != 0 || (out != repeated && len(out.Actions) != 0) {
			return nil, errors.New("repeated actions cannot mix other runtime effects")
		}
	}
	if len(repeated.Actions) != 1 || repeated.Actions[key] == nil {
		return nil, errors.New("repeated provider must own the single managed call")
	}
	calls := [][]byte{first}
	for _, variant := range repeated.Repeats {
		if len(variant) != 1 || variant[key] == nil {
			return nil, errors.New("repeated action must replace its owned calldata")
		}
		call, _, err := recipes.Assemble(recipe, tree, nil, variant, nil)
		if err != nil {
			return nil, errors.Errorf("assemble repeated action: %w", err)
		}
		calls = append(calls, call)
	}
	return recipes.Multicall(calls)
}
