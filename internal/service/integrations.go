package service

import (
	"github.com/go-errors/errors"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlanerfq"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	"github.com/symbioticfi/merkle-executor/internal/providers/securitize"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// DefaultIntegrations registers the supported protocols at the host boundary.
func DefaultIntegrations(deps registry.Deps) ([]provider.Integration, error) {
	constructors := []func(registry.Deps) (provider.Integration, error){
		cowswap.NewAutomatic, merkl.NewAutomatic, liquidlanerfq.NewAutomatic,
		redemptions.NewAutomatic, securitize.NewAutomatic,
	}
	registered := make([]provider.Integration, 0, len(constructors))
	for _, construct := range constructors {
		integration, err := construct(deps)
		if err != nil {
			return nil, errors.Errorf("register integration: %w", err)
		}
		registered = append(registered, integration)
	}
	return registered, nil
}
