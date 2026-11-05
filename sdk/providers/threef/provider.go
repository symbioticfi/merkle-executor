package threef

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	request "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequest"
	factory "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequestsfactory"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const policy = "threef/claim-v1"

type Provider struct{ chain provider.Reader }
type config struct {
	Key      string
	Factory  common.Address
	Offset   int
	Template hexutil.Bytes
}

func NewAutomatic(deps provider.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("3F requires chain reader")
	}
	return &Provider{chain: deps.Chain}, nil
}
func (*Provider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("3F requires authenticated recipe")
}
func derive(s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree) (config, bool, error) {
	actions, err := recipes.ManagedActions(r, tree)
	if err != nil {
		return config{}, false, err
	}
	if len(r.Queries) != 0 || len(r.Steps) != 1 || len(actions) != 1 {
		return config{}, false, nil
	}
	a := actions[0]
	if a.Slot.Gate.Source != recipes.Literal || a.Slot.Gate.Value.Cmp(big.NewInt(1)) != 0 || len(a.Slot.Leaves) != 1 {
		return config{}, false, nil
	}
	f, at, ok := claimLayout(a, s.Account)
	return config{Key: a.Key, Factory: f, Offset: at, Template: a.Leaf.Data}, ok, nil
}
func (*Provider) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	c, ok, err := derive(s, r, t)
	if !ok || err != nil {
		return keeper.RecipeRequirements{}, err
	}
	return keeper.RecipeRequirements{Actions: []string{c.Key}}, nil
}
func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree, _ map[uint]*big.Int) (provider.Output, error) {
	c, ok, err := derive(s, r, tree)
	if err != nil {
		return provider.Output{}, err
	}
	if !ok {
		return derived.Empty(policy)
	}
	if err := protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	count, err := p.count(ctx, s, c.Factory)
	if err != nil {
		return provider.Output{}, err
	}
	var out provider.Output
	binding := request.NewThreeFRequest()
	balanceData, _ := binding.TryPackBalancesOf(s.Account)
	readyData, _ := binding.TryPackCanWithdraw()
	// Scan from the factory's first request, including rights transferred later.
	for start := uint64(0); start < count; start += protocolio.MaxReadBatch {
		end := min(start+protocolio.MaxReadBatch, count)
		var reads []protocolio.ReadRequest
		for i := start; i < end; i++ {
			reads = append(reads, protocolio.ContractRead(requestAt(c.Factory, i), balanceData, s.BlockNumber, true))
		}
		balances, err := protocolio.ReadGroup(ctx, p.chain, reads...)
		if err != nil {
			return provider.Output{}, err
		}
		var owned []common.Address
		reads = nil
		for i := start; i < end; i++ {
			data, err := balances.At(int(i - start)).CallData()
			if err != nil {
				return provider.Output{}, err
			}
			balance, err := binding.UnpackBalancesOf(data)
			if err != nil {
				return provider.Output{}, errors.New("invalid 3F balancesOf response")
			}
			if balance.PtShares.Sign() == 0 && balance.YtShares.Sign() == 0 {
				continue
			}
			address := requestAt(c.Factory, i)
			owned = append(owned, address)
			reads = append(reads, protocolio.ContractRead(address, readyData, s.BlockNumber, true))
		}
		ready, err := protocolio.ReadGroup(ctx, p.chain, reads...)
		if err != nil {
			return provider.Output{}, err
		}
		for i, address := range owned {
			data, err := ready.At(i).CallData()
			if err != nil {
				return provider.Output{}, err
			}
			allowed, err := binding.UnpackCanWithdraw(data)
			if err != nil {
				return provider.Output{}, errors.New("invalid 3F canWithdraw response")
			}
			if !allowed {
				continue
			}
			payload := bytes.Clone(c.Template)
			copy(payload[c.Offset:c.Offset+32], common.LeftPadBytes(address.Bytes(), 32))
			variant := map[string]hexutil.Bytes{c.Key: payload}
			if out.Actions == nil {
				out.Actions = variant
			} else {
				out.Repeats = append(out.Repeats, variant)
			}
		}
	}
	if out.Actions == nil {
		return provider.Output{}, errors.Errorf("%w: no ready 3F requests", provider.ErrNotReady)
	}
	return derived.Encode(policy, c, out, true)
}
func requestAt(f common.Address, index uint64) common.Address {
	return crypto.CreateAddress(f, 7+3*index)
}
func (p *Provider) count(ctx context.Context, s provider.Snapshot, f common.Address) (uint64, error) {
	binding := factory.NewThreeFRequestsFactory()
	exists := func(i uint64) (bool, error) {
		data, _ := binding.TryPackIsRequest(requestAt(f, i))
		raw, err := protocolio.CallData(ctx, p.chain, s, f, data)
		if err != nil {
			return false, err
		}
		ok, err := binding.UnpackIsRequest(raw)
		if err != nil {
			return false, errors.New("invalid 3F isRequest response")
		}
		return ok, nil
	}
	ok, err := exists(0)
	if err != nil || !ok {
		return 0, err
	}
	// The factory creates a request, PT and YT at consecutive CREATE nonces.
	// EIP-2681 bounds the search without truncating the factory's inventory.
	const end = (math.MaxUint64-9)/3 + 1
	low, high := uint64(1), uint64(1)
	for high < end {
		ok, err := exists(high)
		if err != nil {
			return 0, err
		}
		if !ok {
			break
		}
		low = high + 1
		if high > end/2 {
			high = end
		} else {
			high *= 2
		}
	}
	for low < high {
		mid := low + (high-low)/2
		ok, err := exists(mid)
		if err != nil {
			return 0, err
		}
		if ok {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low, nil
}
func (*Provider) Advance(_ context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	var c config
	inner, matched, err := derived.Decode(policy, out, &c)
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true}, nil
	}
	if err := s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	expected, offset := claimTemplate(s.Account, c.Factory)
	if c.Offset != offset || len(c.Template) != len(expected) {
		return provider.Progress{}, errors.New("invalid persisted 3F template")
	}
	copy(expected[offset:offset+32], c.Template[offset:offset+32])
	if !bytes.Equal(c.Template, expected) || len(inner.Inputs)+len(inner.Results)+len(inner.SkipSteps) != 0 {
		return provider.Progress{}, errors.New("invalid persisted 3F claim")
	}
	variants := append([]map[string]hexutil.Bytes{inner.Actions}, inner.Repeats...)
	seen := map[common.Address]bool{}
	for _, variant := range variants {
		data := variant[c.Key]
		if len(variant) != 1 || len(data) != len(expected) {
			return provider.Progress{}, errors.New("invalid persisted 3F action")
		}
		address, err := derived.WordAddress(data[offset : offset+32])
		if err != nil || seen[address] {
			return provider.Progress{}, errors.New("invalid persisted 3F request")
		}
		seen[address] = true
		copy(expected[offset:offset+32], data[offset:offset+32])
		if !bytes.Equal(data, expected) {
			return provider.Progress{}, errors.New("persisted 3F action changed pinned data")
		}
	}
	return provider.Progress{Done: true}, nil
}
