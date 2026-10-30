package recipes

import (
	"bytes"
	"math/big"

	"github.com/go-errors/errors"
)

// Multicall repeats one committed recipe using its executor's native batching.
// Numerical witnesses cannot be shared between state-changing invocations.
func Multicall(calls [][]byte) ([]byte, error) {
	if len(calls) == 0 {
		return nil, errors.New("empty executor batch")
	}
	var identity []byte
	for _, call := range calls {
		method := executorABI.GetABI().Methods["execute"]
		if len(call) < 4 || !bytes.Equal(call[:4], method.ID) {
			return nil, errors.New("batch entries must be execute calls")
		}
		args, err := method.Inputs.Unpack(call[4:])
		if err != nil {
			return nil, errors.New("invalid batch execution")
		}
		r := Recipe{Queries: args[0].([][]byte), Steps: args[1].([][]byte)}
		hash, err := HashRecipe(r)
		if err != nil {
			return nil, err
		}
		if identity != nil && !bytes.Equal(identity, hash[:]) {
			return nil, errors.New("batch must repeat the same committed recipe")
		}
		identity = hash[:]
		inspection, err := Inspect(r)
		if err != nil {
			return nil, err
		}
		if len(args[2].([]*big.Int)) != 0 || len(args[3].([][]byte)) != len(inspection.Slots) {
			return nil, errors.New("claim batch requires complete runtime and no numerical inputs")
		}
		claim := false
		for _, slot := range inspection.Slots {
			claim = claim || (slot.Operation == Manage && slot.Gate.Source == Literal && slot.Gate.Value.Sign() > 0 && len(slot.Leaves) > 0)
		}
		if !claim {
			return nil, errors.New("batch requires an unconditional permissioned call")
		}
	}
	return executorABI.TryPackMulticall(calls)
}

func multicallEntries(data []byte) ([][]byte, bool, error) {
	method := executorABI.GetABI().Methods["multicall"]
	if len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
		return nil, false, nil
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, true, errors.New("invalid executor multicall")
	}
	calls := args[0].([][]byte)
	canonical, err := Multicall(calls)
	if err != nil || !bytes.Equal(canonical, data) {
		return nil, true, errors.New("invalid repeated recipe batch")
	}
	return calls, true, nil
}
