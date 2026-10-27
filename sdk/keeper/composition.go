package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// ValidateComposition rejects ambiguous receipt attribution without changing the
// committed program. It also runs on recovered jobs, using their durable bytes.
func ValidateComposition(calldata []byte, providers []provider.Provider) error {
	return validateComposition(calldata, providers, nil)
}

func ValidatePlanComposition(calldata []byte, providers []provider.Provider, outputs []provider.Output) error {
	if len(providers) != len(outputs) {
		return errors.New("provider output count differs from prepared plan")
	}
	return validateComposition(calldata, providers, outputs)
}

func validateComposition(calldata []byte, providers []provider.Provider, outputs []provider.Output) error {
	owners := make(map[common.Address]int)
	claims := make(map[string]common.Address)
	for i, p := range providers {
		var targets map[string]common.Address
		if exclusive, ok := p.(provider.ExclusiveOutputTargetProvider); ok {
			if outputs == nil {
				return errors.New("derived exclusive targets require persisted provider outputs")
			}
			var err error
			targets, err = exclusive.ExclusiveOutputTargets(outputs[i])
			if err != nil {
				return err
			}
		} else if exclusive, ok := p.(provider.ExclusiveTargetProvider); ok {
			targets = exclusive.ExclusiveTargets()
		}
		if err := addTargetClaims(targets, i, owners, claims); err != nil {
			return err
		}
	}
	if len(claims) == 0 {
		return nil
	}
	targets, err := recipes.ManagedTargets(calldata)
	if err != nil {
		return err
	}
	return validateTargetClaims(targets, owners, claims)
}

// ValidateRecipeComposition checks pure authenticated target ownership before
// any provider resolution or HTTP. Immutable calldata is checked again later.
func ValidateRecipeComposition(recipe recipes.Recipe, tree *recipes.Tree, requirements []RecipeRequirements) error {
	owners := map[common.Address]int{}
	claims := map[string]common.Address{}
	for i, requirement := range requirements {
		if err := addTargetClaims(requirement.ExclusiveTargets, i, owners, claims); err != nil {
			return err
		}
	}
	if len(claims) == 0 {
		return nil
	}
	for _, step := range recipe.Steps {
		if len(step) > 0 && step[0] == recipes.Call {
			return errors.New("direct Call steps are unsupported with exclusive managed targets")
		}
	}
	actions, err := recipes.ManagedActions(recipe, tree)
	if err != nil {
		return err
	}
	targets := map[string]common.Address{}
	for _, action := range actions {
		targets[action.Key] = action.Leaf.Target
	}
	return validateTargetClaims(targets, owners, claims)
}

func validateTargetClaims(targets map[string]common.Address, owners map[common.Address]int, claims map[string]common.Address) error {
	for key, target := range claims {
		if targets[key] != target {
			return errors.New("exclusive managed action is missing or targets another contract")
		}
	}
	for key, target := range targets {
		if _, ok := owners[target]; ok && claims[key] != target {
			return errors.New("unowned managed call to an exclusive target")
		}
	}
	return nil
}

func addTargetClaims(targets map[string]common.Address, owner int, owners map[common.Address]int, claims map[string]common.Address) error {
	for key, target := range targets {
		if target == (common.Address{}) {
			return errors.New("exclusive managed target is zero")
		}
		if existing, ok := owners[target]; ok && existing != owner {
			return errors.New("multiple providers require exclusive managed target")
		}
		if _, ok := claims[key]; ok {
			return errors.New("duplicate exclusive managed action")
		}
		owners[target] = owner
		claims[key] = target
	}
	return nil
}
