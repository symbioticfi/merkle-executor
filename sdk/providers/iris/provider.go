// Package iris supplies loan pod inputs for standalone Iris bond claims.
package iris

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/iris"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Iris core d7ca766b, deployments/1.json. Only this core's CREATE sequence is supported.
var mainnetCore = common.HexToAddress("0x77310475712D7a127D694B3e9D186d9f4a423bd9")
var coreABI = binding.NewIris()

type chainReader interface {
	provider.Reader
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
}

type Provider struct{ chain chainReader }

func NewAutomatic(deps provider.Deps) (keeper.Provider, error) {
	chain, ok := deps.Chain.(chainReader)
	if !ok {
		return nil, errors.New("Iris claims require a pinned contract nonce reader")
	}
	return &Provider{chain: chain}, nil
}

func (*Provider) RecipeRequirements(s provider.Snapshot, recipe recipes.Recipe, tree *recipes.Tree) (keeper.RecipeRequirements, error) {
	matched, err := matchesClaim(s, recipe, tree)
	if err != nil || !matched {
		return keeper.RecipeRequirements{}, err
	}
	return keeper.RecipeRequirements{Inputs: []uint{0}}, nil
}

func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, recipe recipes.Recipe, tree *recipes.Tree, _ map[uint]*big.Int) ([]provider.Output, error) {
	matched, err := matchesClaim(s, recipe, tree)
	if err != nil {
		return nil, err
	}
	if !matched {
		return []provider.Output{{}}, nil
	}
	if err := protocolio.Snapshot(s); err != nil {
		return nil, err
	}

	// Core starts at CREATE nonce one and creates exactly one pod per accepted loan.
	count, err := p.chain.NonceAt(ctx, mainnetCore, s.BlockNumber)
	if err != nil {
		return nil, errors.Errorf("read Iris core nonce: %w", err)
	}
	if count == 0 {
		return nil, errors.New("Iris core is unavailable at the recipe block")
	}

	var outputs []provider.Output
	const pageSize = protocolio.MaxReadBatch
	for start := uint64(1); start < count; {
		end := start + min(uint64(pageSize), count-start)
		pods, err := p.completedPage(ctx, s, start, end)
		if err != nil {
			return nil, err
		}
		for _, pod := range pods {
			outputs = append(outputs, provider.Output{Inputs: map[uint]string{0: pod.Big().String()}})
		}
		start = end
	}
	if len(outputs) == 0 {
		return nil, errors.Errorf("%w: no Iris bonds to claim", provider.ErrNotReady)
	}
	return outputs, nil
}

func (p *Provider) completedPage(ctx context.Context, s provider.Snapshot, start, end uint64) ([]common.Address, error) {
	reads := make([]protocolio.ReadRequest, 0, end-start)
	for nonce := start; nonce < end; nonce++ {
		pod := crypto.CreateAddress(mainnetCore, nonce)
		reads = append(reads, protocolio.ContractRead(mainnetCore, coreABI.PackGetLoan(pod), s.BlockNumber, true))
	}
	loans, err := protocolio.ReadGroup(ctx, p.chain, reads...)
	if err != nil {
		return nil, err
	}
	owned := make([]common.Address, 0, len(reads))
	positionReads := make([]protocolio.ReadRequest, 0, len(reads))
	for nonce := start; nonce < end; nonce++ {
		raw, err := loans.At(int(nonce - start)).CallData()
		if err != nil {
			return nil, err
		}
		loan, err := coreABI.UnpackGetLoan(raw)
		if err != nil || loan.Borrower == (common.Address{}) || loan.Solver == (common.Address{}) {
			return nil, errors.New("invalid Iris loan in the core CREATE sequence")
		}
		if loan.Solver == s.Account {
			pod := crypto.CreateAddress(mainnetCore, nonce)
			owned = append(owned, pod)
			positionReads = append(positionReads, protocolio.ContractRead(mainnetCore, coreABI.PackGetPosition(pod), s.BlockNumber, true))
		}
	}

	positions, err := protocolio.ReadGroup(ctx, p.chain, positionReads...)
	if err != nil {
		return nil, err
	}
	var completed []common.Address
	for i, pod := range owned {
		raw, err := positions.At(i).CallData()
		if err != nil {
			return nil, err
		}
		position, err := coreABI.UnpackGetPosition(raw)
		if err != nil || position.LastUpdate == 0 {
			return nil, errors.New("invalid Iris position")
		}
		if position.Bond.Sign() > 0 && position.Debt.Sign() == 0 && position.BondRequirement.Sign() == 0 &&
			position.FixedLeg.Sign() == 0 && position.Surplus.Sign() == 0 {
			completed = append(completed, pod)
		}
	}
	return completed, nil
}
