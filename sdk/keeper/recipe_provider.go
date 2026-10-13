package keeper

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"math/big"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// RecipeRequirements declares authorized input ownership and the numerical
// expressions needed before obtaining external runtime data. This method is
// pure: every provider is checked before any quote or reward HTTP request.
type RecipeRequirements struct {
	Values           []recipes.Value
	Inputs           []uint
	Actions          []string
	ExclusiveTargets map[string]common.Address
}

// RecipeProvider derives its configuration from authenticated program bytes.
// The SDK invokes this only after validating the active root, chain and account.
// Inputs, results, recipe bytes and permissions are copied for each invocation.
// Recovery uses the persisted Output and never invokes these methods.
type RecipeProvider interface {
	RecipeRequirements(provider.Snapshot, recipes.Recipe, *recipes.Tree) (RecipeRequirements, error)
	PrepareRecipe(context.Context, provider.Snapshot, recipes.Recipe, *recipes.Tree, map[uint]*big.Int) (provider.Output, error)
}
