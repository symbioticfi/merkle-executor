package threef

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	request "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequest"
	factory "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequestsfactory"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Provider struct {
	chain provider.Reader
	mu    sync.Mutex
	hint  factoryHint
}

type factoryHint struct {
	chainID string
	factory common.Address
	count   uint64
}
type config struct {
	Key      string
	Factory  common.Address
	Offset   int
	Template hexutil.Bytes
}

func NewAutomatic(deps provider.Deps) (keeper.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("3F requires chain reader")
	}
	return &Provider{chain: deps.Chain}, nil
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
func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree, _ map[uint]*big.Int) ([]provider.Output, error) {
	c, ok, err := derive(s, r, tree)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []provider.Output{{}}, nil
	}
	if err := protocolio.Snapshot(s); err != nil {
		return nil, err
	}
	count, err := p.count(ctx, s, c.Factory)
	if err != nil {
		return nil, err
	}
	var outputs []provider.Output
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
			return nil, err
		}
		var owned []common.Address
		reads = nil
		for i := start; i < end; i++ {
			data, err := balances.At(int(i - start)).CallData()
			if err != nil {
				return nil, err
			}
			balance, err := binding.UnpackBalancesOf(data)
			if err != nil {
				return nil, errors.New("invalid 3F balancesOf response")
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
			return nil, err
		}
		for i, address := range owned {
			data, err := ready.At(i).CallData()
			if err != nil {
				return nil, err
			}
			allowed, err := binding.UnpackCanWithdraw(data)
			if err != nil {
				return nil, errors.New("invalid 3F canWithdraw response")
			}
			if !allowed {
				continue
			}
			payload := bytes.Clone(c.Template)
			copy(payload[c.Offset:c.Offset+32], common.LeftPadBytes(address.Bytes(), 32))
			outputs = append(outputs, provider.Output{Actions: map[string]hexutil.Bytes{c.Key: payload}})
		}
	}
	if len(outputs) == 0 {
		return nil, errors.Errorf("%w: no ready 3F requests", provider.ErrNotReady)
	}
	return outputs, nil
}
func requestAt(f common.Address, index uint64) common.Address {
	return crypto.CreateAddress(f, 7+3*index)
}
func (p *Provider) count(ctx context.Context, s provider.Snapshot, f common.Address) (count uint64, err error) {
	p.mu.Lock()
	hint := p.hint
	p.mu.Unlock()
	defer func() {
		if err == nil {
			p.mu.Lock()
			p.hint = factoryHint{chainID: s.ChainID.String(), factory: f, count: count}
			p.mu.Unlock()
		}
	}()

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
	// Membership is append-only on the supported factory. Recheck the cached
	// boundary at every pinned block so a changed chain cannot preserve a stale
	// lower bound. Balances and readiness are never cached.
	const end = (math.MaxUint64-9)/3 + 1
	low, high := uint64(1), uint64(1)
	cached := hint.chainID == s.ChainID.String() && hint.factory == f && hint.count > 0
	if cached {
		valid, e := exists(hint.count - 1)
		if e != nil {
			return 0, e
		}
		cached = valid
		if valid {
			if hint.count == end {
				return end, nil
			}
			next, e := exists(hint.count)
			if e != nil {
				return 0, e
			}
			if !next {
				return hint.count, nil
			}
			low, high = hint.count+1, hint.count+1
		}
	}
	if !cached {
		ok, e := exists(0)
		if e != nil || !ok {
			return 0, e
		}
	}
	// The factory creates a request, PT and YT at consecutive CREATE nonces.
	// EIP-2681 bounds the search without truncating the inventory.
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
