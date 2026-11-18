package merkl

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const RewardAPI = "https://api.merkl.xyz"

type Automatic struct{ deps provider.Deps }
type recipeClaim struct {
	action       recipes.ManagedAction
	cfg          Config
	layout       *claimLayout
	requirements keeper.RecipeRequirements
}

func NewAutomatic(deps provider.Deps) (keeper.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("automatic Merkl requires chain reader")
	}
	return &Automatic{deps: deps}, nil
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
		cfg := Config{HTTP: rewardHTTP(), Multicall3: protocolio.StandardMulticall3, Distributor: layout.distributor.Hex(), Action: action.Key, CalldataBytes: len(layout.data)}
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
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) ([]provider.Output, error) {
	claims, err := p.derive(s, r, t)
	if err != nil {
		return nil, err
	}
	if len(claims) == 0 {
		return []provider.Output{{}}, nil
	}
	if err = protocolio.Snapshot(s); err != nil {
		return nil, err
	}
	if !s.ChainID.IsInt64() {
		return nil, errors.New("Merkl API chain ID exceeds int64")
	}
	client, err := protocolio.NewHTTP(rewardHTTP(), p.deps.HTTP)
	if err != nil {
		return nil, err
	}
	entries, err := rewards(ctx, client, s.Account, s.ChainID)
	if err != nil {
		return nil, err
	}
	if len(r.Queries) == 0 && len(r.Steps) == 1 && len(claims) == 1 && len(claims[0].action.Bindings) == 0 && len(claims[0].layout.tokens) == 1 && claims[0].action.Slot.Gate.Source == recipes.Literal && claims[0].action.Slot.Gate.Value.Cmp(big.NewInt(1)) == 0 {
		return p.prepareAll(ctx, s, claims[0], entries)
	}
	// Fixed rewards share a read plan. Runtime tokens add their getters only as
	// selection visits them, rather than reading the entire unused inventory.
	reads := newRewardReads(ctx, p.deps, s, claims, nil)
	out := provider.Output{Inputs: map[uint]string{}, Actions: map[string]hexutil.Bytes{}}
	anyClaimable := false
	allOptional := true
	used := map[string]bool{}
	// Runtime slots may only select tokens left available by every committed fixed slot.
	for _, claim := range claims {
		for _, token := range claim.layout.tokens {
			if token != (common.Address{}) {
				used[claim.cfg.Distributor+token.Hex()] = true
			}
		}
	}
	for _, claim := range claims {
		allOptional = allOptional && len(claim.cfg.ClaimableInputs) == 1
		selected, prepared, claimable, err := p.prepareClaim(s, claim.cfg, entries, used, reads)
		var payload []byte
		if errors.Is(err, errAbsentReward) {
			idle, checkErr := canIdleAbsentClaim(r, claim)
			if checkErr != nil {
				return nil, checkErr
			}
			if !idle {
				return nil, err
			}
			// Other recipe effects are independent of this gate and its calldata bindings.
			// Supply only the committed template's words and waive no instruction or proof.
			payload = claim.action.Leaf.Data
			prepared = provider.Output{Inputs: map[uint]string{claim.cfg.ClaimableInputs[0]: "0"}}
		} else {
			if err != nil {
				return nil, err
			}
			anyClaimable = anyClaimable || claimable.Sign() != 0
			payload = claim.layout.payload(claim.action.Leaf.Data, prepared.Actions[claim.cfg.Action])
		}
		if err = derived.BindOutput(derived.SignTemplate{Action: claim.action}, payload, results, &prepared); err != nil {
			return nil, err
		}
		for index, value := range prepared.Inputs {
			if old, ok := out.Inputs[index]; ok && old != value {
				return nil, errors.New("Merkl recipe input has conflicting claim meanings")
			}
			out.Inputs[index] = value
		}
		out.Actions[claim.cfg.Action] = append(hexutil.Bytes(nil), payload...)
		for _, token := range selected.Tokens {
			used[selected.Distributor+token] = true
		}
	}
	if !anyClaimable && !allOptional {
		return nil, errors.Errorf("%w: no unclaimed Merkl rewards", provider.ErrNotReady)
	}
	return []provider.Output{out}, nil
}

// The zero optional gate may flow through recipe arithmetic and assertions
// (the frontend uses it to check the cash delta). It may not drive another call
// or query. Other unwitnessed claim fields must be private to this single call.
func canIdleAbsentClaim(r recipes.Recipe, claim recipeClaim) (bool, error) {
	if len(claim.cfg.ClaimableInputs) != 1 {
		return false, nil
	}
	gate := claim.cfg.ClaimableInputs[0]
	owned := map[uint]bool{}
	for _, index := range claim.requirements.Inputs {
		owned[index] = true
	}
	gateResults := map[uint]bool{}
	for i, step := range r.Steps {
		if uint(i) == claim.action.Step {
			continue
		}
		operands, err := recipes.Operands(r, uint(i))
		if err != nil {
			return false, err
		}
		for _, operand := range operands {
			index := uint(operand.Value.Uint64())
			if operand.Source == recipes.Input && owned[index] && index != gate {
				return false, nil
			}
			if (operand.Source != recipes.Input || index != gate) && (operand.Source != recipes.Result || !gateResults[index]) {
				continue
			}
			switch step[0] {
			case recipes.Add, recipes.Sub, recipes.SaturatingSub, recipes.Min, recipes.Max, recipes.MulDiv:
				gateResults[uint(i)] = true
			case recipes.Equal, recipes.LessOrEqual:
			default:
				return false, nil
			}
		}
	}
	return true, nil
}

// A runtime reward token is chosen from proven, unclaimed API entries. The recipe
// and permission mask still decide whether the supplied token is authorized.
func (p *Automatic) prepareClaim(s provider.Snapshot, cfg Config, entries rewardSet, used map[string]bool, reads *rewardReads) (Config, provider.Output, *big.Int, error) {
	candidates := []string{""}
	for _, token := range cfg.Tokens {
		if common.HexToAddress(token) != (common.Address{}) {
			continue
		}
		if len(cfg.Tokens) != 1 {
			return cfg, provider.Output{}, nil, errors.New("Merkl runtime token requires a single-entry claim")
		}
		candidates = nil
		for _, token := range entries.tokens() {
			if !used[cfg.Distributor+token] {
				candidates = append(candidates, token)
			}
		}
	}
	var out provider.Output
	var claimable *big.Int
	for _, token := range candidates {
		if token != "" {
			cfg.Tokens = []string{token}
		}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return cfg, out, nil, err
		}
		out, claimable, err = runtime.prepareRewards(s, entries, func() ([]multicall.Multicall3Result, error) { return reads.forProvider(runtime) })
		if err != nil || claimable.Sign() != 0 {
			return cfg, out, claimable, err
		}
	}
	if len(candidates) == 0 {
		return cfg, out, nil, errAbsentReward
	}
	return cfg, out, claimable, nil
}
