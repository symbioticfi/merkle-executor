// Package liquidlanediscount acquires and verifies connector discount witnesses.
package liquidlanediscount

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidlaneconnector"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const ConsumerAPI = "https://swap.symbiotic.fi"
const automaticPolicy = "liquid-lane-connector-discount/recipe-v1"

type Automatic struct{ deps registry.Deps }

func NewAutomatic(deps registry.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("LL discount requires pinned chain reads")
	}
	return &Automatic{deps: deps}, nil
}
func (*Automatic) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("LL discount requires authenticated recipe")
}
func (*Automatic) derive(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (*request, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var found *request
	for _, action := range actions {
		candidate, err := discountAction(action)
		if err != nil {
			return nil, err
		}
		if candidate == nil {
			continue
		}
		if found != nil {
			return nil, errors.New("multiple LL discount swaps require separate executors")
		}
		if err = candidate.validateGuards(r, s.Account); err != nil {
			return nil, err
		}
		found = candidate
	}
	return found, nil
}
func (p *Automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	request, err := p.derive(s, r, t)
	if err != nil || request == nil {
		return keeper.RecipeRequirements{}, err
	}
	return request.requirements, nil
}
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) (provider.Output, error) {
	request, err := p.derive(s, r, t)
	if err != nil {
		return provider.Output{}, err
	}
	if request == nil {
		return derived.Empty(automaticPolicy)
	}
	if err = protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	if err = protocolio.Canonical(ctx, p.deps.Chain, s); err != nil {
		return provider.Output{}, err
	}
	client, err := protocolio.NewHTTP(protocolio.HTTPConfig{URL: ConsumerAPI, MaxResponseBytes: 64 << 10}, p.deps.HTTP)
	if err != nil {
		return provider.Output{}, err
	}
	witness, err := resolve(ctx, client, request.scope)
	if err != nil {
		return provider.Output{}, err
	}
	swap, err := validateWitness(s, request.scope, witness, func(raw string, digest common.Hash, want common.Address) ([]byte, error) {
		return signatureAt(ctx, p.deps.Chain, s, raw, digest, want)
	})
	if err != nil {
		return provider.Output{}, err
	}
	budget, err := p.fundingLimit(ctx, s, r, t, request)
	if err != nil {
		return provider.Output{}, err
	}
	amount, payment, err := p.selectAmount(ctx, s, request, swap.Discount, budget)
	if err != nil {
		return provider.Output{}, err
	}
	signature, err := hexutil.Decode(witness.ProtocolSignature)
	if err != nil {
		return provider.Output{}, errors.New("invalid LL protocol signature")
	}
	payload, err := binding.NewLiquidLaneConnector().TryPackSwap(swap, signature, request.scope.Recipient, amount)
	if err != nil {
		return provider.Output{}, err
	}
	out := provider.Output{Inputs: map[uint]string{uint(request.paymentInput.Value.Uint64()): payment.String()}, Actions: map[string]hexutil.Bytes{request.action.Key: payload}}
	if err = derived.BindOutput(derived.SignTemplate{Action: request.action}, payload, results, &out); err != nil {
		return provider.Output{}, err
	}
	if err = protocolio.Canonical(ctx, p.deps.Chain, s); err != nil {
		return provider.Output{}, err
	}
	return derived.Encode(automaticPolicy, struct{}{}, out, true)
}
func (*Automatic) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	_, matched, err := derived.Decode(automaticPolicy, out, &struct{}{})
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true}, nil
	}
	if len(out.Actions) != 1 || len(out.Inputs) != 2 || len(out.SkipSteps) != 0 {
		return provider.Progress{}, errors.New("persisted LL discount output shape differs")
	}
	if err = ctx.Err(); err != nil {
		return provider.Progress{}, err
	}
	if err = s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	return provider.Progress{Done: true}, nil
}
