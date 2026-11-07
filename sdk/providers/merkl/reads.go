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

// rewardReads shares only immutable results from this preparation's pinned block.
// Getter failures remain local to the claim consuming them, in recipe order.
type rewardReads struct {
	ctx      context.Context
	deps     provider.Deps
	snapshot provider.Snapshot
	calls    []multicall.Multicall3Call3
	indices  map[rewardReadKey]int
	group    *protocolio.ReadResults
	chunks   map[int][]multicall.Multicall3Result
}

func newRewardReads(ctx context.Context, deps provider.Deps, s provider.Snapshot, claims []recipeClaim, entries []wire.MerklEntry) *rewardReads {
	reads := &rewardReads{ctx: ctx, deps: deps, snapshot: s, indices: map[rewardReadKey]int{}, chunks: map[int][]multicall.Multicall3Result{}}
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
		for _, call := range rewardCalls(common.HexToAddress(claim.cfg.Distributor), s.Account, tokens) {
			key := rewardReadKey{call.Target, string(call.CallData)}
			if _, ok := reads.indices[key]; ok {
				continue
			}
			reads.indices[key] = len(reads.calls)
			// Inspect each result when its claim consumes it. A later failed
			// getter must not override an earlier claim's validation branch.
			call.AllowFailure = true
			reads.calls = append(reads.calls, call)
		}
	}
	return reads
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
	if r.group == nil {
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
		r.group = group
	}
	var results []multicall.Multicall3Result
	for _, call := range rewardCalls(p.distributor, r.snapshot.Account, p.tokens) {
		index, ok := r.indices[rewardReadKey{call.Target, string(call.CallData)}]
		if !ok {
			return nil, errors.New("Merkl getter missing from preparation read plan")
		}
		chunk := index / protocolio.MaxReadBatch
		values, ok := r.chunks[chunk]
		if !ok {
			raw, err := r.group.At(chunk).CallData()
			if err != nil {
				return nil, err
			}
			values, err = multicall.NewMulticall3().UnpackAggregate3(raw)
			if err != nil {
				return nil, errors.New("invalid Merkl aggregate response")
			}
			if len(values) != min(protocolio.MaxReadBatch, len(r.calls)-chunk*protocolio.MaxReadBatch) {
				return nil, errors.New("Merkl aggregate result count mismatch")
			}
			r.chunks[chunk] = values
		}
		results = append(results, values[index%protocolio.MaxReadBatch])
	}
	for _, result := range results {
		if !result.Success {
			return nil, errors.New("Merkl aggregate getter failed")
		}
	}
	return results, nil
}
