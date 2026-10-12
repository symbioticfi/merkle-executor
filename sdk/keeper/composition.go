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
	owners := make(map[common.Address]int)
	claims := make(map[string]common.Address)
	for i, p := range providers {
		exclusive, ok := p.(provider.ExclusiveTargetProvider)
		if !ok {
			continue
		}
		for key, target := range exclusive.ExclusiveTargets() {
			if target == (common.Address{}) {
				return errors.New("exclusive managed target is zero")
			}
			if owner, ok := owners[target]; ok && owner != i {
				return errors.New("multiple providers require exclusive managed target")
			}
			if _, ok := claims[key]; ok {
				return errors.New("duplicate exclusive managed action")
			}
			owners[target] = i
			claims[key] = target
		}
	}
	if len(claims) == 0 {
		return nil
	}
	targets, err := recipes.ManagedTargets(calldata)
	if err != nil {
		return err
	}
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
