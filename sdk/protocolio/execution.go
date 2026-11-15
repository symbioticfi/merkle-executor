package protocolio

import (
	"context"
	"math/big"

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
