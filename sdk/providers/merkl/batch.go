package merkl

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
)

// Discovery selects unclaimed rewards; each execution still contains exactly
// one self-claim and the distributor verifies its proof on chain.
func (p *Automatic) prepareAll(ctx context.Context, s provider.Snapshot, claim recipeClaim, entries rewardSet) ([]provider.Output, error) {
	tokens := entries.tokens()
	reads := newRewardReads(ctx, p.deps, s, []recipeClaim{claim}, tokens)
	var outputs []provider.Output
	for _, token := range tokens {
		allowed := claim.layout.tokens[0]
		if allowed != (common.Address{}) && token != allowed.Hex() {
			continue
		}
		cfg := claim.cfg
		cfg.Tokens = []string{token}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return nil, err
		}
		prepared, claimable, err := runtime.prepareRewards(s, entries, func() ([]multicall.Multicall3Result, error) {
			return reads.forProvider(runtime)
		})
		if err != nil {
			return nil, err
		}
		if claimable.Sign() == 0 {
			continue
		}
		payload := claim.layout.payload(claim.action.Leaf.Data, prepared.Actions[claim.cfg.Action])
		if err := derived.BindOutput(derived.SignTemplate{Action: claim.action}, payload, nil, &prepared); err != nil {
			return nil, err
		}
		prepared.Actions[claim.cfg.Action] = payload
		outputs = append(outputs, prepared)
	}
	if len(outputs) == 0 {
		return nil, errors.Errorf("%w: no unclaimed Merkl rewards", provider.ErrNotReady)
	}
	return outputs, nil
}
