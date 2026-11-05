package merkl

import (
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const RewardAPI = "https://api.merkl.xyz"
const automaticPolicy = "merkl/authenticated-claim-v1"

type Automatic struct{ deps provider.Deps }
type recipeClaim struct {
	action       recipes.ManagedAction
	cfg          Config
	layout       *claimLayout
	requirements keeper.RecipeRequirements
}
type automaticConfig struct {
	Claims []Config `json:"claims"`
}

func NewAutomatic(deps provider.Deps) (provider.Provider, error) {
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

func (p *Automatic) derive(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) ([]recipeClaim, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var result []recipeClaim
	seen := map[string]bool{}
	for _, action := range actions {
		layout, err := claimLayoutOf(action, s.Account)
		if err != nil {
			return nil, err
		}
		if layout == nil {
			continue
		}
		if action.Leaf.Value.Sign() != 0 {
			return nil, errors.New("Merkl claim value must be zero")
		}
		cfg := Config{HTTP: rewardHTTP(), Multicall3: protocolio.StandardMulticall3, Distributor: layout.distributor.Hex(), Action: action.Key, CalldataBytes: len(layout.data), MinClaimable: "1", AllowIdle: true}
		fields := map[int]bool{}
		for i, token := range layout.tokens {
			identity := cfg.Distributor + token.Hex()
			if seen[identity] {
				return nil, errors.New("Merkl configured token occurs in multiple claim actions")
			}
			seen[identity] = true
			cfg.Tokens = append(cfg.Tokens, token.Hex())
			cfg.ProofCapacities = append(cfg.ProofCapacities, len(layout.proofs[i]))
			for _, at := range append([]int{layout.tokenWords[i], layout.amounts[i], layout.lengths[i]}, layout.proofs[i]...) {
				if at < 4 || at+32 > len(action.Leaf.Data) {
					return nil, errors.New("Merkl field exceeds managed calldata")
				}
				if fields[at] {
					return nil, errors.New("Merkl claim fields alias the same input word")
				}
				fields[at] = true
			}
		}
		requirements := keeper.RecipeRequirements{Actions: []string{action.Key}}
		for _, b := range action.Bindings {
			if !fields[int(b.Offset)] {
				return nil, errors.New("Merkl bindings must target reward tokens, cumulative amounts or proof fields")
			}
			switch b.Value.Source {
			case recipes.Input:
				requirements.Inputs = append(requirements.Inputs, uint(b.Value.Value.Uint64()))
			case recipes.Result:
				requirements.Values = append(requirements.Values, b.Value)
			case recipes.Literal:
			default:
				return nil, errors.New("invalid Merkl field source")
			}
		}
		// A dedicated claim's optional gate is the verified cumulative-minus-claimed
		// delta. Shared groups keep their committed gate policy with the recipe.
		if action.Slot.Gate.Source == recipes.Input && len(action.Slot.Leaves) == 1 && len(layout.tokens) == 1 {
			gate := uint(action.Slot.Gate.Value.Uint64())
			cfg.ClaimableInputs = []uint{gate}
			requirements.Inputs = append(requirements.Inputs, gate)
		}
		result = append(result, recipeClaim{action: action, cfg: cfg, layout: layout, requirements: requirements})
	}
	return result, nil
}
func (p *Automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	claims, err := p.derive(s, r, t)
	if err != nil {
		return keeper.RecipeRequirements{}, err
	}
	var req keeper.RecipeRequirements
	seen := map[uint]bool{}
	for _, claim := range claims {
		req.Actions = append(req.Actions, claim.requirements.Actions...)
		req.Values = append(req.Values, claim.requirements.Values...)
		for _, index := range claim.requirements.Inputs {
			if !seen[index] {
				req.Inputs = append(req.Inputs, index)
				seen[index] = true
			}
		}
	}
	return req, nil
}
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) (provider.Output, error) {
	claims, err := p.derive(s, r, t)
	if err != nil {
		return provider.Output{}, err
	}
	if len(claims) == 0 {
		return derived.Empty(automaticPolicy)
	}
	if err = protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	if !s.ChainID.IsInt64() {
		return provider.Output{}, errors.New("Merkl API chain ID exceeds int64")
	}
	client, err := NewClient(rewardHTTP(), p.deps.HTTP, p.deps.LookupSecret)
	if err != nil {
		return provider.Output{}, err
	}
	entries, err := client.Rewards(ctx, s.Account, s.ChainID)
	if err != nil {
		return provider.Output{}, err
	}
	if len(r.Queries) == 0 && len(r.Steps) == 1 && len(claims) == 1 && len(claims[0].action.Bindings) == 0 && len(claims[0].layout.tokens) == 1 && claims[0].action.Slot.Gate.Source == recipes.Literal && claims[0].action.Slot.Gate.Value.Cmp(big.NewInt(1)) == 0 {
		return p.prepareAll(ctx, s, claims[0], entries)
	}
	out := provider.Output{Inputs: map[uint]string{}, Actions: map[string]hexutil.Bytes{}}
	cfg := automaticConfig{}
	anyClaimable := false
	allOptional := true
	used := map[string]bool{}
	for _, claim := range claims {
		allOptional = allOptional && len(claim.cfg.ClaimableInputs) == 1
		selected, prepared, err := p.prepareClaim(ctx, s, claim.cfg, entries, used)
		if err != nil {
			return provider.Output{}, err
		}
		var metadata map[string]string
		if err = json.Unmarshal(prepared.Data, &metadata); err != nil {
			return provider.Output{}, err
		}
		anyClaimable = anyClaimable || metadata["claimable"] != "0"
		payload := claim.layout.payload(claim.action.Leaf.Data, prepared.Actions[claim.cfg.Action])
		prepared.Actions[claim.cfg.Action] = payload
		if err = derived.BindOutput(derived.SignTemplate{Action: claim.action}, payload, results, &prepared); err != nil {
			return provider.Output{}, err
		}
		for index, value := range prepared.Inputs {
			if old, ok := out.Inputs[index]; ok && old != value {
				return provider.Output{}, errors.New("Merkl recipe input has conflicting claim meanings")
			}
			out.Inputs[index] = value
		}
		out.Actions[claim.cfg.Action] = payload
		cfg.Claims = append(cfg.Claims, selected)
		for _, token := range selected.Tokens {
			used[selected.Distributor+token] = true
		}
	}
	if !anyClaimable && !allOptional {
		return provider.Output{}, errors.Errorf("%w: Merkl rewards below minClaimable", provider.ErrNotReady)
	}
	return derived.Encode(automaticPolicy, cfg, out, true)
}

// A runtime reward token is chosen from proven, unclaimed API entries. The recipe
// and permission mask still decide whether the supplied token is authorized.
func (p *Automatic) prepareClaim(ctx context.Context, s provider.Snapshot, cfg Config, entries []wire.MerklEntry, used map[string]bool) (Config, provider.Output, error) {
	candidates := []string{""}
	for _, token := range cfg.Tokens {
		if common.HexToAddress(token) != (common.Address{}) {
			continue
		}
		if len(cfg.Tokens) != 1 {
			return cfg, provider.Output{}, errors.New("Merkl runtime token requires a single-entry claim")
		}
		candidates = nil
		for _, entry := range entries {
			if entry.Chain.ID != s.ChainID.Int64() {
				continue
			}
			for _, reward := range entry.Rewards {
				token := common.HexToAddress(reward.Token.Address).Hex()
				if !used[cfg.Distributor+token] {
					candidates = append(candidates, token)
				}
			}
		}
		sort.Strings(candidates)
	}
	var out provider.Output
	for _, token := range candidates {
		if token != "" {
			cfg.Tokens = []string{token}
		}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return cfg, out, err
		}
		out, err = runtime.prepareRewards(s, entries, func() ([]multicall.Multicall3Result, error) { return runtime.readBatch(ctx, s) })
		if err != nil {
			return cfg, out, err
		}
		var metadata map[string]string
		if err = json.Unmarshal(out.Data, &metadata); err != nil {
			return cfg, out, err
		}
		if metadata["claimable"] != "0" {
			return cfg, out, nil
		}
	}
	if len(candidates) == 0 {
		return cfg, out, errors.Errorf("%w: no Merkl reward token available", provider.ErrNotReady)
	}
	return cfg, out, nil
}
func (p *Automatic) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	var cfg automaticConfig
	inner, matched, err := derived.Decode(automaticPolicy, out, &cfg)
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true}, nil
	}
	if len(cfg.Claims) == 0 || len(inner.SkipSteps) != 0 || (len(inner.Repeats) == 0 && len(inner.Actions) != len(cfg.Claims)) || (len(inner.Repeats) != 0 && (len(inner.Actions) != 1 || len(cfg.Claims) != 1+len(inner.Repeats) || len(inner.Inputs) != 0 || len(inner.Results) != 0)) {
		return provider.Progress{}, errors.New("persisted Merkl claim shape differs")
	}
	seen := map[string]bool{}
	for i, claim := range cfg.Claims {
		actions := inner.Actions
		if len(inner.Repeats) != 0 && i > 0 {
			actions = inner.Repeats[i-1]
		}
		if !reflect.DeepEqual(claim.HTTP, rewardHTTP()) || claim.Multicall3 != protocolio.StandardMulticall3 || claim.MinClaimable != "1" || claim.SkipIfEmpty || claim.DedicatedClaimGroup || len(claim.AmountInputs) != 0 || !claim.AllowIdle || len(claim.ClaimableInputs) > 1 || (seen[claim.Action] && len(inner.Repeats) == 0) || len(actions) != 1 && len(inner.Repeats) != 0 || len(actions[claim.Action]) == 0 {
			return provider.Progress{}, errors.New("persisted Merkl automatic constants differ")
		}
		seen[claim.Action] = true
		runtime, err := New(claim, p.deps)
		if err != nil {
			return provider.Progress{}, err
		}
		if _, err = runtime.Advance(ctx, s, inner, state); err != nil {
			return provider.Progress{}, err
		}
	}
	return provider.Progress{Done: true}, nil
}
