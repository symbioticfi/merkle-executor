package protocolio

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// Execution verifies evidence from the authorizing transaction against the pinned
// confirmed snapshot. A later receipt or an orphaned request cannot authorize progress.
func Execution(ctx context.Context, r provider.Reader, s provider.Snapshot) error {
	if err := Snapshot(s); err != nil {
		return err
	}
	if err := s.Execution.Validate(); err != nil {
		return err
	}
	if s.Execution.BlockNumber.Cmp(s.BlockNumber) > 0 {
		return errors.New("execution receipt is later than the confirmed snapshot")
	}
	reader, ok := r.(interface {
		HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	})
	if !ok {
		return errors.New("execution requires a canonical header reader")
	}
	h, err := reader.HeaderByNumber(ctx, s.Execution.BlockNumber)
	if err != nil || h == nil || h.Number == nil || h.Number.Cmp(s.Execution.BlockNumber) != 0 || h.Hash() != s.Execution.BlockHash {
		return errors.New("execution receipt block is no longer canonical")
	}
	return Canonical(ctx, r, s)
}

// CheckHeader requires a uint64-numbered header, at number when one is given.
func CheckHeader(header *types.Header, number *big.Int) error {
	if header == nil || header.Number == nil || !header.Number.IsUint64() {
		return errors.New("invalid block header")
	}
	if number != nil && header.Number.Cmp(number) != 0 {
		return errors.New("RPC returned another block")
	}
	return nil
}

// Ancestor follows parent hashes from witness down to number.
func Ancestor(ctx context.Context, r interface {
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
}, witness *types.Header, number *big.Int) (*types.Header, error) {
	ancestor := witness
	for ancestor.Number.Cmp(number) > 0 {
		parent, err := r.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, Failure("read confirmation ancestry", err)
		}
		if err = CheckHeader(parent, new(big.Int).Sub(ancestor.Number, big.NewInt(1))); err != nil {
			return nil, err
		}
		if parent.Hash() != ancestor.ParentHash {
			return nil, errors.New("inconsistent confirmation ancestry")
		}
		ancestor = parent
	}
	return ancestor, nil
}
