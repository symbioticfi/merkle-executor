package protocolio

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
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

// ManagedCall is a zero-value account call whose successful execution a provider
// must establish before declaring its request or claim phase complete.
type ManagedCall struct {
	Target common.Address `json:"target"`
	Data   hexutil.Bytes  `json:"data"`
}

// ExecutedCalls requires the expected calls in order in the account's successful
// Manage events. A matching event cannot be reused for another expected call.
// Call Execution first to establish canonical confirmation of this receipt.
func ExecutedCalls(s provider.Snapshot, expected []ManagedCall) error {
	if err := s.Execution.Validate(); err != nil {
		return err
	}
	if s.Account == (common.Address{}) || len(expected) == 0 {
		return errors.New("account and expected managed calls are required")
	}
	a := account.NewMerkleAccount()
	eventID := a.GetABI().Events[account.MerkleAccountManageEventName].ID
	next := 0
	for _, log := range s.Execution.Logs {
		if log.Address != s.Account || len(log.Topics) == 0 || log.Topics[0] != eventID {
			continue
		}
		event, err := a.UnpackManageEvent(&log)
		if err != nil {
			return errors.New("invalid account Manage event")
		}
		for _, action := range event.Actions {
			if action.Value != nil && action.Value.Sign() == 0 && action.Target == expected[next].Target && bytes.Equal(action.Data, expected[next].Data) {
				next++
				if next == len(expected) {
					return nil
				}
			}
		}
	}
	return errors.New("confirmed recipe did not execute the expected managed calls")
}
