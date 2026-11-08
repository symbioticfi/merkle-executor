package merkl

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type rewardReadKey struct {
	target common.Address
	data   string
}

type rewardReadResult struct {
	value multicall.Multicall3Result
	err   error
}

// rewardReads shares only immutable results from this preparation's pinned block.
// Getter failures remain local to the claim consuming them, in recipe order.
type rewardReads struct {
	ctx      context.Context
	deps     provider.Deps
	snapshot provider.Snapshot
	calls    []multicall.Multicall3Call3
	seen     map[rewardReadKey]bool
	results  map[rewardReadKey]rewardReadResult
}

// entries preplans runtime tokens only when every reward will be considered.
func newRewardReads(ctx context.Context, deps provider.Deps, s provider.Snapshot, claims []recipeClaim, entries []wire.MerklEntry) *rewardReads {
	reads := &rewardReads{ctx: ctx, deps: deps, snapshot: s, seen: map[rewardReadKey]bool{}, results: map[rewardReadKey]rewardReadResult{}}
	for _, claim := range claims {
		var tokens []common.Address
		for _, raw := range claim.cfg.Tokens {
			token := common.HexToAddress(raw)
			if token != (common.Address{}) {
				tokens = append(tokens, token)
				continue
			}
			for _, entry := range entries {
				if entry.Chain.ID != s.ChainID.Int64() {
					continue
				}
				for _, reward := range entry.Rewards {
					tokens = append(tokens, common.HexToAddress(reward.Token.Address))
				}
			}
		}
		reads.add(rewardCalls(common.HexToAddress(claim.cfg.Distributor), s.Account, tokens))
	}
	return reads
}

func (r *rewardReads) add(calls []multicall.Multicall3Call3) {
	for _, call := range calls {
		key := rewardReadKey{call.Target, string(call.CallData)}
		if r.seen[key] {
			continue
		}
		r.seen[key] = true
		// Inspect each result when its claim consumes it. A later failed
		// getter must not override an earlier claim's validation branch.
		call.AllowFailure = true
		r.calls = append(r.calls, call)
	}
}

func rewardCalls(distributor, account common.Address, tokens []common.Address) []multicall.Multicall3Call3 {
	binding := binding.NewMerklDistributor()
	calls := make([]multicall.Multicall3Call3, 0, 2+2*len(tokens))
	add := func(data []byte) {
		calls = append(calls, multicall.Multicall3Call3{Target: distributor, CallData: data})
	}
	add(binding.PackGetMerkleRoot())
	add(binding.PackClaimRecipient(account, common.Address{}))
	for _, token := range tokens {
		add(binding.PackClaimed(account, token))
		add(binding.PackClaimRecipient(account, token))
	}
	return calls
}

func (r *rewardReads) forProvider(p *Provider) ([]multicall.Multicall3Result, error) {
	calls := rewardCalls(p.distributor, r.snapshot.Account, p.tokens)
	r.add(calls)
	if len(r.calls) != 0 {
		var requests []protocolio.ReadRequest
		for start := 0; start < len(r.calls); start += protocolio.MaxReadBatch {
			end := min(start+protocolio.MaxReadBatch, len(r.calls))
			payload, err := multicall.NewMulticall3().TryPackAggregate3(r.calls[start:end])
			if err != nil {
				return nil, errors.New("encode Merkl aggregate reads")
			}
			requests = append(requests, protocolio.ContractRead(p.multicall, payload, r.snapshot.BlockNumber, false))
		}
		group, err := protocolio.ReadGroup(r.ctx, r.deps.Chain, requests...)
		if err != nil {
			return nil, err
		}
		for start := 0; start < len(r.calls); start += protocolio.MaxReadBatch {
			count := min(protocolio.MaxReadBatch, len(r.calls)-start)
			raw, err := group.At(start / protocolio.MaxReadBatch).CallData()
			var values []multicall.Multicall3Result
			if err == nil {
				values, err = multicall.NewMulticall3().UnpackAggregate3(raw)
				if err != nil {
					err = errors.New("invalid Merkl aggregate response")
				} else if len(values) != count {
					err = errors.New("Merkl aggregate result count mismatch")
				}
			}
			for i := range count {
				call := r.calls[start+i]
				result := rewardReadResult{err: err}
				if err == nil {
					result.value = values[i]
				}
				r.results[rewardReadKey{call.Target, string(call.CallData)}] = result
			}
		}
		r.calls = nil
	}
	results := make([]multicall.Multicall3Result, 0, len(calls))
	for _, call := range calls {
		result := r.results[rewardReadKey{call.Target, string(call.CallData)}]
		if result.err != nil {
			return nil, result.err
		}
		results = append(results, result.value)
	}
	for _, result := range results {
		if !result.Success {
			return nil, errors.New("Merkl aggregate getter failed")
		}
	}
	return results, nil
}
