package service

import (
	"github.com/go-errors/errors"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlanerfq"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	"github.com/symbioticfi/merkle-executor/internal/providers/securitize"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

// DefaultIntegrations registers the supported protocols at the host boundary.
// This order also describes the v1 provider slots; v2 identities never depend on it.
func DefaultIntegrations(deps registry.Deps) ([]provider.Integration, []rt.LegacyConverter, error) {
	constructors := []func(registry.Deps) (provider.Integration, error){
		cowswap.NewAutomatic, merkl.NewAutomatic, liquidlanerfq.NewAutomatic,
		redemptions.NewAutomatic, securitize.NewAutomatic,
	}
	converters := []rt.LegacyConverter{
		func(s chainio.Snapshot, o provider.Output, p provider.Progress) ([]provider.Record, error) {
			return cowswap.ImportLegacy(deps, s, o, p)
		},
		func(s chainio.Snapshot, o provider.Output, p provider.Progress) ([]provider.Record, error) {
			return merkl.ImportLegacy(deps, s, o, p)
		},
		func(s chainio.Snapshot, o provider.Output, p provider.Progress) ([]provider.Record, error) {
			return liquidlanerfq.ImportLegacy(deps, s, o, p)
		},
		func(s chainio.Snapshot, o provider.Output, p provider.Progress) ([]provider.Record, error) {
			return redemptions.ImportLegacy(deps, s, o, p)
		},
		func(s chainio.Snapshot, o provider.Output, p provider.Progress) ([]provider.Record, error) {
			return securitize.ImportLegacy(deps, s, o, p)
		},
		nil,
	}
	registered := make([]provider.Integration, 0, len(constructors))
	for _, construct := range constructors {
		integration, err := construct(deps)
		if err != nil {
			return nil, nil, errors.Errorf("register integration: %w", err)
		}
		registered = append(registered, integration)
	}
	return registered, converters, nil
}
