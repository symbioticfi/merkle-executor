package recipes

import (
	"bytes"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	erc20 "github.com/symbioticfi/merkle-executor/api/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/api/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/internal/parse"
)

// Assemble builds a complete execute call without evaluating the program. Explicit
// skipped Manage slots remain empty; full simulation must prove their gates zero.
// actions keys are absolute "step:action" indices, not runtime slot indices.
func Assemble(recipe Recipe, tree *Tree, inputs map[uint]string, actions map[string]hexutil.Bytes, skipSteps []uint) ([]byte, []common.Hash, error) {
	skipped := make(map[uint]bool, len(skipSteps))
	for _, step := range skipSteps {
		if step >= uint(len(recipe.Steps)) || recipe.Steps[step].Operation != Manage {
			return nil, nil, errors.Errorf("skip step %d is not Manage", step)
		}
		if skipped[step] {
			return nil, nil, errors.Errorf("duplicate skip step %d", step)
		}
		skipped[step] = true
	}
	inspection, required, err := inspect(recipe, skipped)
	if err != nil {
		return nil, nil, err
	}
	numerical, err := assembleInputs(required, inspection.Inputs, inputs)
	if err != nil {
		return nil, nil, err
	}
	runtime := make([][]byte, len(inspection.Slots))
	used := make(map[common.Hash]bool)
	usedActions := make(map[string]bool)
	var leaves []common.Hash
	for _, slot := range inspection.Slots {
		if skipped[slot.StepIndex] {
			runtime[slot.Index] = []byte{}
			continue
		}
		switch slot.Operation {
		case Manage:
			var payload []byte
			payload, err = assembleManage(slot, tree, actions, usedActions)
			runtime[slot.Index] = payload
		case Approve:
			runtime[slot.Index], err = assembleApproval(slot, tree)
		}
		if err != nil {
			return nil, nil, errors.Errorf("step %d runtime: %w", slot.StepIndex, err)
		}
		for _, leaf := range slot.Leaves {
			if !used[leaf] {
				used[leaf] = true
				leaves = append(leaves, leaf)
			}
		}
	}
	for key := range actions {
		if !usedActions[key] {
			return nil, nil, errors.Errorf("unused action replacement %q", key)
		}
	}
	calldata, err := executorABI.TryPackExecute(recipe.Queries, recipe.Steps, numerical, runtime)
	if err != nil {
		return nil, nil, errors.Errorf("encode execute: %w", err)
	}
	return calldata, leaves, nil
}

func assembleInputs(required, allowed []uint, inputs map[uint]string) ([]*big.Int, error) {
	size := uint(0)
	if len(required) > 0 {
		size = required[len(required)-1] + 1
	}
	seen := make(map[uint]bool, len(allowed))
	for _, index := range allowed {
		seen[index] = true
	}
	for _, index := range required {
		if _, ok := inputs[index]; !ok {
			return nil, errors.Errorf("input %d is required", index)
		}
	}
	for index := range inputs {
		if !seen[index] {
			return nil, errors.Errorf("unused input %d", index)
		}
		if index >= size {
			size = index + 1
		}
	}
	result := make([]*big.Int, size)
	for i := range result {
		result[i] = new(big.Int)
	}
	for index, text := range inputs {
		value, err := parse.Uint256(text)
		if err != nil {
			return nil, errors.Errorf("input %d: %w", index, err)
		}
		result[index] = value
	}
	return result, nil
}

func assembleManage(slot RuntimeSlot, tree *Tree, replacements map[string]hexutil.Bytes, used map[string]bool) ([]byte, error) {
	actions := make([]account.IMerkleAccountAction, len(slot.Leaves))
	for i, hash := range slot.Leaves {
		leaf, err := tree.lookup(hash)
		if err != nil {
			return nil, err
		}
		if err = validateBindings(slot.Bindings[i], len(leaf.Data), func(Value) error { return nil }); err != nil {
			return nil, err
		}
		data := bytes.Clone(leaf.Data)
		key := strconv.FormatUint(uint64(slot.StepIndex), 10) + ":" + strconv.Itoa(i)
		if replacement, ok := replacements[key]; ok {
			if len(replacement) != len(data) {
				return nil, errors.Errorf("replacement %s changes calldata length", key)
			}
			for at, b := range replacement {
				if (b^data[at]) & ^leaf.DataMask[at] != 0 {
					return nil, errors.Errorf("replacement %s changes pinned calldata at byte %d", key, at)
				}
			}
			data = bytes.Clone(replacement)
			used[key] = true
		}
		actions[i] = account.IMerkleAccountAction{Target: leaf.Target, Value: new(big.Int), Data: data, DataMask: bytes.Clone(leaf.DataMask), Proof: rawHashes(leaf.Proof)}
	}
	encoded, err := accountABI.TryPackManage(actions)
	if err != nil {
		return nil, errors.Errorf("encode managed actions: %w", err)
	}
	return encoded[4:], nil
}

func assembleApproval(slot RuntimeSlot, tree *Tree) ([]byte, error) {
	leaf, err := tree.lookup(slot.Leaves[0])
	if err != nil {
		return nil, err
	}
	if leaf.Target != slot.Token || len(leaf.DataMask) != 68 {
		return nil, errors.New("approval leaf has wrong token or calldata length")
	}
	// Preserve the stored amount preimage, including any pinned bits. The recipe
	// supplies its live amount on-chain; full simulation checks both that amount
	// and the optional allowance-reset call against the permission.
	amount := new(big.Int).SetBytes(leaf.Data[36:])
	data, err := erc20.NewERC20().TryPackApprove(slot.Spender, amount)
	if err != nil {
		return nil, errors.Errorf("encode approval: %w", err)
	}
	formed := Leaf{Target: slot.Token, Value: new(big.Int), Data: data, DataMask: leaf.DataMask}
	hash, err := HashLeaf(formed)
	if err != nil {
		return nil, err
	}
	if hash != slot.Leaves[0] {
		return nil, errors.New("approval leaf does not match committed token and spender")
	}
	return permissionArgs.Pack(leaf.DataMask, rawHashes(leaf.Proof))
}
