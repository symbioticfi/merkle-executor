package recipes

import (
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
)

// ManagedTargets reads every populated Manage slot from immutable execute bytes.
// Gates and numerical bindings cannot change targets, so even zero-gated slots
// participate. Explicit empty skipped slots cannot emit a Manage event. Arbitrary
// Call steps are unsupported: they can invoke account.manage indirectly.
func ManagedTargets(calldata []byte) (map[string]common.Address, error) {
	if isMulticall(calldata) {
		calls, err := batchCalls(calldata)
		if err != nil {
			return nil, err
		}
		targets := map[string]common.Address{}
		for i, call := range calls {
			group, err := ManagedTargets(call)
			if err != nil {
				return nil, err
			}
			for key, target := range group {
				targets["cycle"+strconv.Itoa(i)+"/"+key] = target
			}
		}
		return targets, nil
	}
	decoded, err := decodeExecution(calldata)
	if err != nil {
		return nil, err
	}
	for _, step := range decoded.recipe.Steps {
		if step[0] == Call {
			return nil, errors.New("direct Call steps are unsupported with exclusive managed targets")
		}
	}
	targets := make(map[string]common.Address)
	manage := accountABI.GetABI().Methods["manage"]
	for _, slot := range decoded.inspection.Slots {
		if slot.Operation != Manage || len(decoded.runtime[slot.Index]) == 0 {
			continue
		}
		values, err := manage.Inputs.Unpack(decoded.runtime[slot.Index])
		if err != nil {
			return nil, errors.New("invalid prepared managed actions")
		}
		actions := *abi.ConvertType(values[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
		if len(actions) != len(slot.Leaves) {
			return nil, errors.New("prepared managed action count mismatch")
		}
		for i, action := range actions {
			targets[actionKey(slot.StepIndex, i)] = action.Target
		}
	}
	return targets, nil
}
