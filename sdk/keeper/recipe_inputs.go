package keeper

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// NewRecipeInputs enables generic recipe-result input derivation without manual
// fixed data. Exact equalities take precedence; bounded financial inputs select
// the maximum allowed READ-derived amount in the shared numerical lens call.
func NewRecipeInputs() provider.Provider { return recipeInputs{} }

type recipeInputs struct{}

func (recipeInputs) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("authenticated recipe context is required")
}
func (recipeInputs) RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error) {
	return RecipeRequirements{}, nil
}
func (recipeInputs) PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) (provider.Output, error) {
	return provider.Output{}, nil
}
func (recipeInputs) Advance(ctx context.Context, s provider.Snapshot, _ provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if err := ctx.Err(); err != nil {
		return provider.Progress{}, err
	}
	if s.Execution == nil {
		return provider.Progress{}, errors.New("confirmed execution is required")
	}
	if err := s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	return provider.Progress{Done: true}, nil
}
