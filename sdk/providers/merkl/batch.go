package merkl

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
)

// Discovery selects unclaimed rewards; each execution still contains exactly
// one self-claim and the distributor verifies its proof on chain.
func (p *Automatic) prepareAll(ctx context.Context, s provider.Snapshot, claim recipeClaim, entries []wire.MerklEntry) ([]provider.Output, error) {
	var tokens []string
	for _, entry := range entries {
		if entry.Chain.ID != s.ChainID.Int64() {
			continue
		}
		for _, reward := range entry.Rewards {
			token := common.HexToAddress(reward.Token.Address)
			allowed := claim.layout.tokens[0]
			if allowed == (common.Address{}) || token == allowed {
				tokens = append(tokens, token.Hex())
			}
		}
	}
	sort.Strings(tokens)
	var runtimes []*Provider
	var requests []protocolio.ReadRequest
	for i, token := range tokens {
		if i > 0 && tokens[i-1] == token {
			return nil, errors.New("Merkl duplicate reward token")
		}
		cfg := claim.cfg
		cfg.Tokens = []string{token}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return nil, err
		}
		data, err := runtime.batchPayload(s.Account)
		if err != nil {
			return nil, err
		}
		requests = append(requests, protocolio.ContractRead(runtime.multicall, data, s.BlockNumber, false))
		runtimes = append(runtimes, runtime)
	}
	reads, err := protocolio.ReadGroup(ctx, p.deps.Chain, requests...)
	if err != nil {
		return nil, err
	}
	var outputs []provider.Output
	for i, runtime := range runtimes {
		prepared, err := runtime.prepareRewards(s, entries, func() ([]multicall.Multicall3Result, error) {
			raw, err := reads.At(i).CallData()
			if err != nil {
				return nil, err
			}
			return runtime.decodeBatch(raw)
		})
		if err != nil {
			return nil, err
		}
		var info map[string]string
		if err := json.Unmarshal(prepared.Data, &info); err != nil {
			return nil, err
		}
		amount, ok := new(big.Int).SetString(info["claimable"], 10)
		if !ok {
			return nil, errors.New("invalid prepared Merkl amount")
		}
		if amount.Sign() == 0 {
			continue
		}
		payload := claim.layout.payload(claim.action.Leaf.Data, prepared.Actions[claim.cfg.Action])
		if err := derived.BindOutput(derived.SignTemplate{Action: claim.action}, payload, nil, &prepared); err != nil {
			return nil, err
		}
		prepared.Actions[claim.cfg.Action] = payload
		encoded, err := derived.Encode(automaticPolicy, automaticConfig{Claims: []Config{runtime.cfg}}, prepared, true)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, encoded...)
	}
	if len(outputs) == 0 {
		return nil, errors.Errorf("%w: no unclaimed Merkl rewards", provider.ErrNotReady)
	}
	return outputs, nil
}
