package protocolio

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// PreparationPin is provisional configured identity at a selected block. It has
// no recipe commitment or permission root and grants no authority. ChainID is a
// canonical positive decimal string so the entire pin is immutable by value.
type PreparationPin struct {
	ChainID                   string
	BlockNumber               uint64
	BlockHash                 common.Hash
	Timestamp                 uint64
	Executor, Account, Caller common.Address
}

func (p PreparationPin) Validate() error {
	id, ok := new(big.Int).SetString(p.ChainID, 10)
	if !ok || id.Sign() <= 0 || id.String() != p.ChainID || p.BlockHash == (common.Hash{}) || p.Executor == (common.Address{}) || p.Account == (common.Address{}) || p.Caller == (common.Address{}) {
		return errors.New("invalid provisional preparation identity")
	}
	return nil
}
func (p PreparationPin) Matches(s provider.Snapshot) bool {
	return s.ChainID != nil && s.ChainID.String() == p.ChainID && s.BlockNumber != nil && s.BlockNumber.IsUint64() && s.BlockNumber.Uint64() == p.BlockNumber && s.BlockHash == p.BlockHash && s.Timestamp == p.Timestamp && s.Executor == p.Executor && s.Account == p.Account && s.Caller == p.Caller
}

// ReadAccessor lazily consumes only the assigned preparation results.
type ReadAccessor func(int) ReadResult

// PreparationPlan contains independent calls at the provisional numeric pin.
// Resolve runs only after discovery and configured tree validation. It should
// classify API eligibility before consuming results, preserving lazy errors.
type PreparationPlan struct {
	Requests []ReadRequest
	Resolve  func(context.Context, provider.Snapshot, ReadAccessor) (provider.Output, error)
}

// StagedProvider optionally plans pure local reads without I/O or persistence.
// Its ordinary Prepare method remains usable by hosts that do not stage reads.
type StagedProvider interface {
	PlanPreparation(PreparationPin) (PreparationPlan, error)
}

type preparedProvider struct {
	provider provider.Provider
	plan     PreparationPlan
	err      error
	offset   int
}

// Preparation is local to one attempt and one goroutine, never a provider cache.
type Preparation struct {
	pin       PreparationPin
	providers []preparedProvider
	requests  []ReadRequest
}

func PlanPreparation(providers []provider.Provider, pin PreparationPin) *Preparation {
	p := &Preparation{pin: pin}
	for _, pr := range providers {
		item := preparedProvider{provider: pr, offset: len(p.requests)}
		if staged, ok := pr.(StagedProvider); ok {
			item.err = pin.Validate()
			if item.err == nil {
				item.plan, item.err = staged.PlanPreparation(pin)
			}
			if item.err == nil {
				item.err = validatePreparationPlan(pin, item.plan)
			}
			if item.err == nil {
				p.requests = append(p.requests, item.plan.Requests...)
			}
		}
		p.providers = append(p.providers, item)
	}
	return p
}
func validatePreparationPlan(pin PreparationPin, plan PreparationPlan) error {
	if plan.Resolve == nil {
		return errors.New("staged preparation resolver required")
	}
	for _, request := range plan.Requests {
		if request.Kind != ReadCall || request.Block == nil || !request.Block.IsUint64() || request.Block.Uint64() != pin.BlockNumber {
			return errors.New("staged read must use the provisional numeric pin")
		}
	}
	return nil
}
func (p *Preparation) Requests() []ReadRequest { return p.requests }
func (p *Preparation) Resolve(ctx context.Context, index int, snapshot provider.Snapshot, read ReadAccessor) (provider.Output, error) {
	if index < 0 || index >= len(p.providers) || !p.pin.Matches(snapshot) {
		return provider.Output{}, errors.New("staged preparation identity mismatch")
	}
	item := p.providers[index]
	if item.err != nil {
		return provider.Output{}, item.err
	}
	if item.plan.Resolve == nil {
		return item.provider.Prepare(ctx, snapshot)
	}
	return item.plan.Resolve(ctx, snapshot, func(i int) ReadResult {
		if i < 0 || i >= len(item.plan.Requests) {
			return ReadResult{Err: errors.New("staged result index outside provider plan")}
		}
		return read(item.offset + i)
	})
}
