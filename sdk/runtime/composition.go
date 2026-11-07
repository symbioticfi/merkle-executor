package runtime

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// ValidateComposition rejects ambiguous receipt attribution before protocol I/O.
func ValidateComposition(recipe recipes.Recipe, tree *recipes.Tree, requirements []provider.Requirements) error {
	owners, claims, err := targetClaims(requirements)
	if err != nil || len(claims) == 0 {
		return err
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
	targets := make(map[string]common.Address, len(actions))
	for _, action := range actions {
		targets[action.Key] = action.Leaf.Target
	}
	return checkTargets(targets, owners, claims)
}

// ValidatePlanComposition retains all claims, including completed operations.
func ValidatePlanComposition(calldata []byte, records []provider.Record) error {
	requirements := make([]provider.Requirements, len(records))
	for i, record := range records {
		requirements[i] = record.Requirements
	}
	owners, claims, err := targetClaims(requirements)
	if err != nil || len(claims) == 0 {
		return err
	}
	targets, err := recipes.ManagedTargets(calldata)
	if err != nil {
		return err
	}
	return checkTargets(targets, owners, claims)
}

func targetClaims(requirements []provider.Requirements) (map[common.Address]int, map[string]common.Address, error) {
	owners, claims := map[common.Address]int{}, map[string]common.Address{}
	for i, requirement := range requirements {
		for key, target := range requirement.ExclusiveTargets {
			if target == (common.Address{}) {
				return nil, nil, errors.New("exclusive managed target is zero")
			}
			if owner, exists := owners[target]; exists && owner != i {
				return nil, nil, errors.New("multiple operations require exclusive managed target")
			}
			if _, exists := claims[key]; exists {
				return nil, nil, errors.New("duplicate exclusive managed action")
			}
			owners[target], claims[key] = i, target
		}
	}
	return owners, claims, nil
}

func checkTargets(targets map[string]common.Address, owners map[common.Address]int, claims map[string]common.Address) error {
	for key, target := range claims {
		if targets[key] != target {
			return errors.New("exclusive managed action is missing or targets another contract")
		}
	}
	for key, target := range targets {
		if _, owned := owners[target]; owned && claims[key] != target {
			return errors.New("unowned managed call to an exclusive target")
		}
	}
	return nil
}
