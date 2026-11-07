package protocolio

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// ManagedCall is a zero-value account call whose successful execution a provider
// must establish before declaring its request or claim phase complete.
type ManagedCall struct {
	Target common.Address `json:"target"`
	Data   hexutil.Bytes  `json:"data"`
}

// ExecutedCalls requires the expected calls in order in the account's successful
// Manage events. A matching event cannot be reused for another expected call.
// Call chainio.VerifyExecution first to establish canonical confirmation.
// Matching is local to this invocation. Receipt-dependent providers must declare
// complete provider.Requirements.ExclusiveTargets claims and runtime composition validation to avoid
// claiming another provider's or a static action's evidence, including on restart.
func ExecutedCalls(s chainio.Snapshot, expected []ManagedCall) error {
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
