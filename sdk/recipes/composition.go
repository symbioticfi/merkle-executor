package recipes

import (
	"bytes"
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
	method := executorABI.GetABI().Methods["execute"]
	if len(calldata) < 4 || !bytes.Equal(calldata[:4], method.ID) {
		return nil, errors.New("invalid prepared execute calldata")
	}
	values, err := method.Inputs.Unpack(calldata[4:])
	if err != nil {
		return nil, errors.New("invalid prepared execute arguments")
	}

	steps := values[1].([][]byte)
	for _, step := range steps {
		if len(step) > 0 && step[0] == Call {
			return nil, errors.New("direct Call steps are unsupported with exclusive managed targets")
		}
	}
	inspection, err := Inspect(Recipe{Queries: values[0].([][]byte), Steps: steps})
	if err != nil {
		return nil, err
	}
	runtime := values[3].([][]byte)
	if len(runtime) != len(inspection.Slots) {
		return nil, errors.New("prepared runtime slot count mismatch")
	}
	targets := make(map[string]common.Address)
	manage := accountABI.GetABI().Methods["manage"]
	for _, slot := range inspection.Slots {
		if slot.Operation != Manage || len(runtime[slot.Index]) == 0 {
			continue
		}
		decoded, err := manage.Inputs.Unpack(runtime[slot.Index])
		if err != nil {
			return nil, errors.New("invalid prepared managed actions")
		}
		actions := *abi.ConvertType(decoded[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
		if len(actions) != len(slot.Leaves) {
			return nil, errors.New("prepared managed action count mismatch")
		}
		for i, action := range actions {
			targets[strconv.FormatUint(uint64(slot.StepIndex), 10)+":"+strconv.Itoa(i)] = action.Target
		}
	}
	return targets, nil
}
