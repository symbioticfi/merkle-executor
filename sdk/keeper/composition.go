package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"strconv"
)

// ValidatePlanComposition checks receipt attribution against immutable call data.
func ValidatePlanComposition(calldata []byte, providers []Provider, outputs [][]provider.Output) error {
	if len(providers) != len(outputs) {
		return errors.New("provider output count differs from prepared plan")
	}
	for _, group := range outputs {
		if len(group) == 0 {
			return errors.New("provider returned no execution outputs")
		}
	}
	owners := make(map[common.Address]int)
	claims := make(map[string]common.Address)
	for i, p := range providers {
		targets, err := exclusiveTargets(p, outputs, i)
		if err != nil {
			return err
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

func exclusiveTargets(p Provider, outputs [][]provider.Output, i int) (map[string]common.Address, error) {
	if exclusive, ok := p.(provider.ExclusiveOutputTargetProvider); ok {
		if i >= len(outputs) || len(outputs[i]) == 0 {
			return nil, errors.New("derived exclusive targets require prepared provider outputs")
		}
		all := map[string]common.Address{}
		for j, output := range outputs[i] {
			targets, err := exclusive.ExclusiveOutputTargets(output)
			if err != nil {
				return nil, err
			}
			for key, target := range targets {
				if len(outputs[i]) > 1 {
					key = "cycle" + strconv.Itoa(j) + "/" + key
				}
				all[key] = target
			}
		}
		return all, nil
	}
	return nil, nil
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

// executionComposition inspects each candidate once. Rejected additions leave
// the accepted prefix intact, including when receipt attribution conflicts.
type executionComposition struct {
	targets map[common.Address]bool
	owners  map[common.Address]bool
	opaque  bool
}

func newExecutionComposition() *executionComposition {
	return &executionComposition{targets: map[common.Address]bool{}, owners: map[common.Address]bool{}}
}

func (c *executionComposition) add(plan Plan, strategy Strategy) error {
	if err := validatePlanMember(plan, strategy); err != nil {
		return err
	}
	if len(strategy.Providers) != len(plan.Outputs) {
		return errors.New("provider output count differs from prepared plan")
	}
	owners, claims := map[common.Address]int{}, map[string]common.Address{}
	for i, runtime := range strategy.Providers {
		if len(plan.Outputs[i]) == 0 {
			return errors.New("provider returned no execution outputs")
		}
		exclusive, err := exclusiveTargets(runtime, plan.Outputs, i)
		if err != nil {
			return err
		}
		if err = addTargetClaims(exclusive, i, owners, claims); err != nil {
			return err
		}
	}
	targets, targetErr := recipes.ManagedTargets(plan.Calldata)
	if len(owners) != 0 || len(c.owners) != 0 {
		if targetErr != nil {
			return targetErr
		}
		if err := validateTargetClaims(targets, owners, claims); err != nil {
			return err
		}
		for _, target := range targets {
			if c.owners[target] {
				return errors.New("managed call conflicts with an earlier exclusive receipt target")
			}
		}
		for target := range owners {
			if c.opaque || c.targets[target] {
				return errors.New("exclusive receipt target conflicts with an earlier call")
			}
		}
	}
	for target := range owners {
		c.owners[target] = true
	}
	for _, target := range targets {
		c.targets[target] = true
	}
	c.opaque = c.opaque || targetErr != nil
	return nil
}
