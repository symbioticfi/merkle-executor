package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const RewardAPI = "https://api.merkl.xyz"
const automaticPolicy = "merkl/authenticated-claim-v1"

type Automatic struct{ deps registry.Deps }
type recipeClaim struct {
	action       recipes.ManagedAction
	cfg          Config
	requirements keeper.RecipeRequirements
}

func NewAutomatic(deps registry.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("automatic Merkl requires chain reader")
	}
	return &Automatic{deps: deps}, nil
}
func (*Automatic) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("Merkl requires authenticated recipe")
}
func rewardHTTP() protocolio.HTTPConfig {
	return protocolio.HTTPConfig{URL: RewardAPI, MaxResponseBytes: 8 << 20}
}

func (p *Automatic) derive(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (*recipeClaim, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	method := binding.NewMerklDistributor().GetABI().Methods["claim"]
	var result *recipeClaim
	for _, action := range actions {
		data := action.Leaf.Data
		if len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
			continue
		}
		if result != nil {
			return nil, errors.New("multiple Merkl claim actions require separate executors")
		}
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, errors.New("invalid Merkl claim template")
		}
		users, tokens, amounts, proofs := args[0].([]common.Address), args[1].([]common.Address), args[2].([]*big.Int), args[3].([][][32]byte)
		if len(tokens) < 1 || len(tokens) > 32 || len(users) != len(tokens) || len(amounts) != len(tokens) || len(proofs) != len(tokens) {
			return nil, errors.New("Merkl claim arrays must have the same bounded length")
		}
		expected, err := binding.NewMerklDistributor().TryPackClaim(users, tokens, amounts, proofs)
		if err != nil || !bytes.Equal(expected, data) {
			return nil, errors.New("noncanonical Merkl claim array layout is unsupported")
		}
		cfg := Config{HTTP: rewardHTTP(), Multicall3: protocolio.StandardMulticall3, Distributor: action.Leaf.Target.Hex(), Action: action.Key, CalldataBytes: len(data), MinClaimable: "1"}
		allowed := make(map[int]bool)
		amountOffsets := make(map[int]bool)
		array := 4 + int(new(big.Int).SetBytes(data[68:100]).Uint64())
		for i, token := range tokens {
			if users[i] != s.Account || token == (common.Address{}) {
				return nil, errors.New("Merkl claim must pin authenticated user and nonzero token")
			}
			cfg.Tokens = append(cfg.Tokens, token.Hex())
			cfg.ProofLengths = append(cfg.ProofLengths, len(proofs[i]))
			at := array + 32 + 32*i
			amountOffsets[at] = true
			for n := at; n < at+32; n++ {
				allowed[n] = true
			}
		}
		proofArray := 4 + int(new(big.Int).SetBytes(data[100:132]).Uint64())
		for i, proof := range proofs {
			relative := new(big.Int).SetBytes(data[proofArray+32+32*i : proofArray+64+32*i]).Uint64()
			at := proofArray + 32 + int(relative) + 32
			for n := at; n < at+32*len(proof); n++ {
				allowed[n] = true
			}
		}
		for i := range data {
			if allowed[i] {
				continue
			}
			if err := action.Pinned(i, 1); err != nil {
				return nil, errors.New("Merkl claim user, tokens, selector and proof layout must be pinned")
			}
		}
		requirements := keeper.RecipeRequirements{Actions: []string{action.Key}}
		for _, b := range action.Bindings {
			if !amountOffsets[int(b.Offset)] {
				return nil, errors.New("Merkl supports numerical bindings only for cumulative amounts")
			}
			switch b.Value.Source {
			case recipes.Input:
				requirements.Inputs = append(requirements.Inputs, uint(b.Value.Value.Uint64()))
			case recipes.Result:
				if err := recipes.ReadDependent(r, b.Value); err != nil {
					return nil, err
				}
				requirements.Values = append(requirements.Values, b.Value)
			case recipes.Literal:
			default:
				return nil, errors.New("invalid Merkl amount source")
			}
		}
		// Cumulative inputs can be sparse; BindOutput fills them from the exact
		// root-verified claim bytes rather than guessing claimable delta mappings.
		result = &recipeClaim{action: action, cfg: cfg, requirements: requirements}
	}
	return result, nil
}
func (p *Automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	claim, err := p.derive(s, r, t)
	if err != nil || claim == nil {
		return keeper.RecipeRequirements{}, err
	}
	return claim.requirements, nil
}
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) (provider.Output, error) {
	claim, err := p.derive(s, r, t)
	if err != nil {
		return provider.Output{}, err
	}
	if claim == nil {
		return derived.Empty(automaticPolicy)
	}
	runtime, err := New(claim.cfg, p.deps)
	if err != nil {
		return provider.Output{}, err
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return provider.Output{}, err
	}
	if err = derived.BindOutput(derived.SignTemplate{Action: claim.action}, out.Actions[claim.cfg.Action], results, &out); err != nil {
		return provider.Output{}, err
	}
	return derived.Encode(automaticPolicy, claim.cfg, out, true)
}
func (p *Automatic) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	var cfg Config
	inner, matched, err := derived.Decode(automaticPolicy, out, &cfg)
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true}, nil
	}
	if !reflect.DeepEqual(cfg.HTTP, rewardHTTP()) || cfg.Multicall3 != protocolio.StandardMulticall3 || cfg.MinClaimable != "1" || cfg.SkipIfEmpty || cfg.DedicatedClaimGroup || len(cfg.AmountInputs) != 0 || len(cfg.ClaimableInputs) != 0 {
		return provider.Progress{}, errors.New("persisted Merkl automatic constants differ")
	}
	runtime, err := New(cfg, p.deps)
	if err != nil {
		return provider.Progress{}, err
	}
	if len(inner.Actions) != 1 || len(inner.SkipSteps) != 0 {
		return provider.Progress{}, errors.New("persisted Merkl claim shape differs")
	}
	return runtime.Advance(ctx, s, inner, state)
}
