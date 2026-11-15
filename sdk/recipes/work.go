package recipes

import (
	"github.com/go-errors/errors"
)

// HasWork classifies the results of a successful full original execute call.
// It does not simulate execution or prove success itself. Manage is conditional
// on its committed gate; Call and Approve are unconditional, including zero
// approvals and calls with zero-valued arguments. Every instruction and the
// complete result vector are checked before returning a work decision.
func HasWork(calldata, returned []byte) (bool, error) {
	return hasWork(calldata, returned, true)
}

func hasWork(calldata, returned []byte, includeApprovals bool) (bool, error) {
	calls, results, err := executionResults(calldata, returned)
	if err != nil {
		return false, err
	}
	work := false
	for i, call := range calls {
		active, err := executionWork(call, results[i], includeApprovals)
		if err != nil {
			return false, err
		}
		work = work || active
	}
	return work, nil
}

func executionWork(call, returned []byte, includeApprovals bool) (bool, error) {
	decoded, err := decodeExecution(call)
	if err != nil {
		return false, err
	}
	recipe, inputs := decoded.recipe, decoded.inputs
	values, err := stepResults(returned, len(recipe.Steps))
	if err != nil {
		return false, err
	}
	skipped := make(map[uint]bool)
	work := false
	for _, slot := range decoded.inspection.Slots {
		if slot.Operation == Approve {
			work = work || includeApprovals
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
