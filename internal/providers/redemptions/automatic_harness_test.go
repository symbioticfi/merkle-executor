package redemptions

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Test adapter keeps behavioral fixtures at the compile/operation boundary.
type automaticTest struct{ *automatic }

func testAutomatic(p provider.Integration) *automaticTest { return &automaticTest{p.(*automatic)} }
func (p *automaticTest) RecipeRequirements(s chainio.Snapshot, r recipes.Recipe, t *recipes.Tree) (provider.Requirements, error) {
	ops, err := p.Compile(provider.Program{Snapshot: s, Recipe: r, Tree: t})
	if err != nil {
		return provider.Requirements{}, err
	}
	total := provider.Requirements{ExclusiveTargets: map[string]common.Address{}}
	for _, op := range ops {
		req := op.Requirements()
		total.Actions = append(total.Actions, req.Actions...)
		total.Inputs = append(total.Inputs, req.Inputs...)
		total.Values = append(total.Values, req.Values...)
		for key, target := range req.ExclusiveTargets {
			total.ExclusiveTargets[key] = target
		}
	}
	return total, nil
}
func (p *automaticTest) PrepareRecipe(ctx context.Context, s chainio.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) (provider.Output, error) {
	program := provider.Program{Snapshot: s, Recipe: r, Tree: t}
	ops, err := p.Compile(program)
	if err != nil {
		return provider.Output{}, err
	}
	if len(ops) == 0 {
		return derived.Empty(PolicyVersion)
	}
	if len(ops) != 1 {
		panic("fixture must prepare individual routes")
	}
	return ops[0].Prepare(ctx, provider.Preparation{Program: program, Results: results, Reader: p.deps.Chain})
}
func (p *automaticTest) record(out provider.Output) (provider.Record, error) {
	var envelope routeEnvelope
	inner, err := provider.DecodeOperation(p.Policy(), out, &envelope)
	if err != nil {
		return provider.Record{}, err
	}
	runtime, err := New(envelope.Config, p.deps)
	if err != nil {
		return provider.Record{}, err
	}
	req := provider.OutputRequirements(inner, runtime.ExclusiveTargets())
	return provider.Record{Integration: p.ID(), Policy: p.Policy(), ID: provider.OperationID(req.Actions), Requirements: req, Output: out}, nil
}
func (p *automaticTest) ExclusiveOutputTargets(out provider.Output) (map[string]common.Address, error) {
	record, err := p.record(out)
	if err != nil {
		return nil, err
	}
	op, err := p.Restore(record)
	if err != nil {
		return nil, err
	}
	return op.Requirements().ExclusiveTargets, nil
}
func (p *automaticTest) Advance(ctx context.Context, s chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	var legacy derived.Envelope
	if json.Unmarshal(out.Data, &legacy) == nil && legacy.Version == 1 && !legacy.Matched {
		var cfg any
		_, _, err := derived.Decode(PolicyVersion, out, &cfg)
		return provider.Progress{Done: err == nil}, err
	}
	record, err := p.record(out)
	if err != nil {
		return provider.Progress{}, err
	}
	op, err := p.Restore(record)
	if err != nil {
		return provider.Progress{}, err
	}
	return op.Advance(ctx, s, out, state)
}
func decodeTestBundle(out provider.Output, bundle *durableRoutes) (provider.Output, bool, error) {
	var envelope routeEnvelope
	inner, err := provider.DecodeOperation(PolicyVersion+"/operation-v2", out, &envelope)
	if err != nil {
		return provider.Output{}, false, err
	}
	bundle.Account = envelope.Account
	bundle.Routes = []durableRoute{{Config: envelope.Config, Output: inner}}
	return inner, true, nil
}
func encodeTestBundle(policy string, bundle durableRoutes, inner provider.Output) (provider.Output, error) {
	route := bundle.Routes[0]
	return provider.EncodeOperation(policy+"/operation-v2", routeEnvelope{Config: route.Config, Account: bundle.Account}, route.Output)
}
