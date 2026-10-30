package recipes

import (
	"bytes"
	"math/big"

	"github.com/go-errors/errors"
)

// HasWork classifies the results of a successful full original execute call.
// It does not simulate execution or prove success itself. Manage is conditional
// on its committed gate; Call and Approve are unconditional, including zero
// approvals and calls with zero-valued arguments. Every instruction and the
// complete result vector are checked before returning a work decision.
func HasWork(calldata, returned []byte) (bool, error) {
	if calls, batch, err := multicallEntries(calldata); batch {
		if err != nil {
			return false, err
		}
		if len(returned) != 0 {
			return false, errors.New("executor multicall must return no data")
		}
		return len(calls) > 0, nil
	}
	method := executorABI.GetABI().Methods["execute"]
	if len(calldata) < 4 || !bytes.Equal(calldata[:4], method.ID) {
		return false, errors.New("expected executor execute calldata")
	}
	decoded, err := method.Inputs.Unpack(calldata[4:])
	if err != nil {
		return false, errors.New("invalid executor execute calldata")
	}
	recipe := Recipe{Queries: decoded[0].([][]byte), Steps: decoded[1].([][]byte)}
	inspection, err := Inspect(recipe)
	if err != nil {
		return false, err
	}
	inputs := decoded[2].([]*big.Int)
	if len(decoded[3].([][]byte)) != len(inspection.Slots) {
		return false, errors.New("execution runtime slot count mismatch")
	}
	output, err := method.Outputs.Unpack(returned)
	if err != nil || len(output) != 1 {
		return false, errors.New("invalid full execution result")
	}
	values := output[0].([]*big.Int)
	if len(values) != len(recipe.Steps) {
		return false, errors.New("full execution result count mismatch")
	}
	skipped := make(map[uint]bool)
	work := false
	for _, slot := range inspection.Slots {
		if slot.Operation == Approve {
			work = true
			continue
		}
		gate := slot.Gate.Value
		switch slot.Gate.Source {
		case Input:
			if slot.Gate.Value.Uint64() >= uint64(len(inputs)) {
				return false, errors.New("execution gate input out of bounds")
			}
			gate = inputs[slot.Gate.Value.Uint64()]
		case Result:
			gate = values[slot.Gate.Value.Uint64()]
		}
		if gate.Sign() == 0 {
			skipped[slot.StepIndex] = true
		} else if len(slot.Leaves) != 0 {
			work = true
		}
	}
	// Only bindings inside a zero-gated Manage may refer to omitted inputs.
	_, required, err := inspect(recipe, skipped)
	if err != nil {
		return false, err
	}
	for _, index := range required {
		if index >= uint(len(inputs)) {
			return false, errors.New("execution input out of bounds")
		}
	}
	for _, step := range recipe.Steps {
		if step[0] == Call {
			work = true
		}
	}
	return work, nil
}
