package recipes

import (
	"maps"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
)

var executorABI = executor.NewRecipeExecutor()
var accountABI = account.NewMerkleAccount()

// These are internal ABI payloads specified by IRecipes, not contract methods.
var manageArgs = Arguments("uint256", "bytes32[]", "uint256[][]")
var approveArgs = Arguments("uint256", "address", "address", "bytes32")
var callArgs = Arguments("address", "bytes", "uint256[]")
var queryArgs = Arguments("address", "bytes", "uint256[]", "uint256")
var readArgs = Arguments("uint256")
var binaryArgs = Arguments("uint256", "uint256")
var mulDivArgs = Arguments("uint256", "uint256", "uint256")
var permissionArgs = Arguments("bytes", "bytes32[]")
var leafArgs = Arguments("address", "bytes1", "bytes32", "bytes32")

// Arguments builds an ABI argument list from fixed, internally reviewed type names.
func Arguments(types ...string) abi.Arguments {
	args := make(abi.Arguments, len(types))
	for i, name := range types {
		args[i].Type = mustType(name, nil)
	}
	return args
}

func mustType(name string, fields []abi.ArgumentMarshaling) abi.Type {
	t, err := abi.NewType(name, "", fields)
	if err != nil {
		panic(err)
	}
	return t
}

func HashRecipe(recipe Recipe) (common.Hash, error) {
	encoded, err := executorABI.TryPackSetRecipe(recipe.Queries, recipe.Steps)
	if err != nil {
		return common.Hash{}, errors.Errorf("encode recipe: %w", err)
	}
	return crypto.Keccak256Hash(encoded[4:]), nil
}

func DecodeRecipe(encoded []byte) (Recipe, error) {
	values, err := executorABI.GetABI().Methods["setRecipe"].Inputs.Unpack(encoded)
	if err != nil {
		return Recipe{}, errors.Errorf("decode recipe: %w", err)
	}
	return Recipe{Queries: values[0].([][]byte), Steps: values[1].([][]byte)}, nil
}

func HashLeaf(leaf Leaf) (common.Hash, error) {
	if leaf.Value == nil || leaf.Value.Sign() < 0 || leaf.Value.BitLen() > 96 {
		return common.Hash{}, errors.New("leaf value must fit uint96")
	}
	if len(leaf.Data) != len(leaf.DataMask) {
		return common.Hash{}, errors.New("leaf calldata and mask lengths differ")
	}
	masked := make([]byte, len(leaf.Data))
	for i, b := range leaf.Data {
		masked[i] = b & ^leaf.DataMask[i]
	}
	var value [1]byte
	if leaf.Value.Sign() != 0 {
		value[0] = 0xff
	}
	encoded, err := leafArgs.Pack(leaf.Target, value, crypto.Keccak256Hash(leaf.DataMask), crypto.Keccak256Hash(masked))
	if err != nil {
		return common.Hash{}, errors.Errorf("encode leaf: %w", err)
	}
	return crypto.Keccak256Hash(crypto.Keccak256(encoded)), nil
}

func hashPair(a, b common.Hash) common.Hash {
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	return crypto.Keccak256Hash(a[:], b[:])
}

// Proof verifies the sorted-pair OpenZeppelin proof used by MerkleAccount.
func Proof(root, leaf common.Hash, proof []common.Hash) bool {
	for _, sibling := range proof {
		leaf = hashPair(leaf, sibling)
	}
	return leaf == root
}

// Inspect checks instruction structure and references without interpreting values.
// Original payload bytes are retained: Solidity permits trailing ABI data.
func Inspect(recipe Recipe) (Inspection, error) {
	inspection, _, err := inspect(recipe, nil)
	return inspection, err
}

func inspect(recipe Recipe, skipped map[uint]bool) (Inspection, []uint, error) {
	var result Inspection
	inputs := make(map[uint]struct{})
	requiredInputs := make(map[uint]struct{})
	for index, step := range recipe.Steps {
		if len(step) == 0 {
			return Inspection{}, nil, errors.Errorf("step %d: empty instruction", index)
		}
		operation, data := step[0], step[1:]
		slot := RuntimeSlot{Index: uint(len(result.Slots)), StepIndex: uint(index), Operation: operation}
		var err error
		validate := func(v Value) error {
			if err := validateValue(v, index, recipe.Steps, inputs); err != nil {
				return err
			}
			if v.Source == Input {
				requiredInputs[uint(v.Value.Uint64())] = struct{}{}
			}
			return nil
		}
		switch operation {
		case Read:
			var decoded []any
			decoded, err = readArgs.Unpack(data)
			if err != nil {
				break
			}
			queryIndex := decoded[0].(*big.Int)
			if !queryIndex.IsUint64() || queryIndex.Uint64() >= uint64(len(recipe.Queries)) {
				err = errors.New("query index out of bounds")
				break
			}
			decoded, err = queryArgs.Unpack(recipe.Queries[queryIndex.Uint64()])
			if err != nil {
				break
			}
			bindings := decodeBindings(decoded[2].([]*big.Int))
			err = validateBindings(bindings, len(decoded[1].([]byte)), validate)
		case Add, Sub, SaturatingSub, Min, Max, MulDiv, Equal, LessOrEqual:
			args := binaryArgs
			if operation == MulDiv {
				args = mulDivArgs
			}
			var decoded []any
			decoded, err = args.Unpack(data)
			if err != nil {
				break
			}
			for _, raw := range decoded {
				if err = validate(decodeValue(raw.(*big.Int))); err != nil {
					break
				}
			}
		case Manage:
			var decoded []any
			decoded, err = manageArgs.Unpack(data)
			if err != nil {
				break
			}
			slot.Gate = decodeValue(decoded[0].(*big.Int))
			slot.Leaves = hashes(decoded[1].([][32]byte))
			for _, bindings := range decoded[2].([][]*big.Int) {
				slot.Bindings = append(slot.Bindings, decodeBindings(bindings))
			}
			if len(slot.Leaves) != len(slot.Bindings) {
				err = errors.New("managed leaf/binding counts differ")
				break
			}
			if err = validate(slot.Gate); err != nil {
				break
			}
			bindingValidate := validate
			if skipped[uint(index)] {
				// Skipped bindings are still structurally checked, but their input
				// values are never read by the on-chain interpreter.
				bindingValidate = func(v Value) error { return validateValue(v, index, recipe.Steps, inputs) }
			}
			for _, bindings := range slot.Bindings {
				if err = validateBindings(bindings, -1, bindingValidate); err != nil {
					break
				}
			}
			result.Slots = append(result.Slots, slot)
		case Approve:
			var decoded []any
			decoded, err = approveArgs.Unpack(data)
			if err != nil {
				break
			}
			slot.Amount = decodeValue(decoded[0].(*big.Int))
			slot.Token = decoded[1].(common.Address)
			slot.Spender = decoded[2].(common.Address)
			slot.Leaves = []common.Hash{decoded[3].([32]byte)}
			err = validate(slot.Amount)
			result.Slots = append(result.Slots, slot)
		case Call:
			var decoded []any
			decoded, err = callArgs.Unpack(data)
			if err != nil {
				break
			}
			bindings := decodeBindings(decoded[2].([]*big.Int))
			err = validateBindings(bindings, len(decoded[1].([]byte)), validate)
		default:
			err = errors.Errorf("unknown operation %d", operation)
		}
		if err != nil {
			return Inspection{}, nil, errors.Errorf("step %d: %w", index, err)
		}
	}
	result.Inputs = sortedInputs(inputs)
	return result, sortedInputs(requiredInputs), nil
}

func sortedInputs(inputs map[uint]struct{}) []uint {
	return slices.Sorted(maps.Keys(inputs))
}

// Operands decodes every Value one instruction reads, in ABI order: binding values
// for Read and Call, the gate then bindings for Manage, and the amount for Approve.
func Operands(recipe Recipe, index uint) ([]Value, error) {
	step := recipe.Steps[index]
	var words []*big.Int
	switch step[0] {
	case Read:
		args, err := readArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		query := args[0].(*big.Int)
		if !query.IsUint64() || query.Uint64() >= uint64(len(recipe.Queries)) {
			return nil, errors.New("query index out of bounds")
		}
		if args, err = queryArgs.Unpack(recipe.Queries[query.Uint64()]); err != nil {
			return nil, err
		}
		words = args[2].([]*big.Int)
	case Call:
		args, err := callArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		words = args[2].([]*big.Int)
	case Manage:
		args, err := manageArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		words = []*big.Int{args[0].(*big.Int)}
		for _, bindings := range args[2].([][]*big.Int) {
			words = append(words, bindings...)
		}
	case Approve:
		args, err := approveArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		words = []*big.Int{args[0].(*big.Int)}
	case Add, Sub, SaturatingSub, Min, Max, MulDiv, Equal, LessOrEqual:
		args := binaryArgs
		if step[0] == MulDiv {
			args = mulDivArgs
		}
		values, err := args.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			words = append(words, value.(*big.Int))
		}
	default:
		return nil, errors.Errorf("unknown operation %d", step[0])
	}
	operands := make([]Value, len(words))
	for i, word := range words {
		operands[i] = decodeValue(word)
	}
	return operands, nil
}

// decodeValue matches Solidity's uint8(word >> 216) and uint216(word).
// The upper four bytes are ignored, including in standalone operands.
func decodeValue(word *big.Int) Value {
	var encoded [32]byte
	word.FillBytes(encoded[:])
	return Value{Source: encoded[4], Value: new(big.Int).SetBytes(encoded[5:])}
}

func decodeBindings(words []*big.Int) []Binding {
	bindings := make([]Binding, len(words))
	for i, word := range words {
		bindings[i] = Binding{Offset: uint32(new(big.Int).Rsh(word, 224).Uint64()), Value: decodeValue(word)}
	}
	return bindings
}

func validateValue(value Value, index int, steps [][]byte, inputs map[uint]struct{}) error {
	switch value.Source {
	case Literal:
	case Input:
		// Bound host allocation, independently of the numerical input's full uint256 width.
		if !value.Value.IsUint64() || value.Value.Uint64() > MaxInputIndex {
			return errors.Errorf("input index exceeds limit of %d", MaxInputIndex)
		}
		inputs[uint(value.Value.Uint64())] = struct{}{}
	case Result:
		if !value.Value.IsUint64() || value.Value.Uint64() >= uint64(index) || steps[value.Value.Uint64()][0] > MulDiv {
			return errors.New("result must reference an earlier numerical instruction")
		}
	default:
		return errors.Errorf("unknown value source %d", value.Source)
	}
	return nil
}

func validateBindings(bindings []Binding, dataLength int, validate func(Value) error) error {
	end := uint64(4)
	for _, binding := range bindings {
		if uint64(binding.Offset) < end {
			return errors.New("binding overlaps, changes selector or exceeds bounds")
		}
		end = uint64(binding.Offset) + 32
		if dataLength >= 0 && end > uint64(dataLength) {
			return errors.New("binding outside calldata")
		}
		if err := validate(binding.Value); err != nil {
			return err
		}
	}
	return nil
}

func hashes(raw [][32]byte) []common.Hash {
	result := make([]common.Hash, len(raw))
	for i, h := range raw {
		result[i] = h
	}
	return result
}

func rawHashes(raw []common.Hash) [][32]byte {
	result := make([][32]byte, len(raw))
	for i, h := range raw {
		result[i] = h
	}
	return result
}
