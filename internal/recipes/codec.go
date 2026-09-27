package recipes

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/api/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/api/bindings/recipesexecutor"
)

var executorABI = executor.NewRecipesExecutor()
var accountABI = account.NewMerkleAccount()

// These are internal ABI payloads specified by IRecipes, not contract methods.
var valueFields = []abi.ArgumentMarshaling{{Name: "source", Type: "uint8"}, {Name: "value", Type: "uint256"}}
var bindingFields = []abi.ArgumentMarshaling{{Name: "offset", Type: "uint256"}, {Name: "value", Type: "tuple", Components: valueFields}}
var valueType = mustType("tuple", valueFields)
var bindingsType = mustType("tuple[]", bindingFields)
var manageArgs = abi.Arguments{{Type: valueType}, {Type: mustType("bytes32[]", nil)}, {Type: mustType("tuple[][]", bindingFields)}}
var approveArgs = abi.Arguments{{Type: valueType}, {Type: mustType("address", nil)}, {Type: mustType("address", nil)}, {Type: mustType("bytes32", nil)}}
var callArgs = abi.Arguments{{Type: mustType("address", nil)}, {Type: mustType("bytes", nil)}, {Type: bindingsType}}
var queryArgs = append(append(abi.Arguments{}, callArgs...), abi.Argument{Type: mustType("uint256", nil)})
var readArgs = abi.Arguments{{Type: mustType("uint256", nil)}}
var binaryArgs = abi.Arguments{{Type: valueType}, {Type: valueType}}
var mulDivArgs = append(append(abi.Arguments{}, binaryArgs...), abi.Argument{Type: valueType})
var permissionArgs = abi.Arguments{{Type: mustType("bytes", nil)}, {Type: mustType("bytes32[]", nil)}}
var leafArgs = abi.Arguments{{Type: mustType("address", nil)}, {Type: mustType("bytes1", nil)}, {Type: mustType("bytes32", nil)}, {Type: mustType("bytes32", nil)}}

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
	return Recipe{Queries: values[0].([][]byte), Steps: *abi.ConvertType(values[1], new([]Step)).(*[]Step)}, nil
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
		slot := RuntimeSlot{Index: uint(len(result.Slots)), StepIndex: uint(index), Operation: step.Operation}
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
		switch step.Operation {
		case Read:
			var decoded []any
			decoded, err = readArgs.Unpack(step.Data)
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
			bindings := *abi.ConvertType(decoded[2], new([]Binding)).(*[]Binding)
			err = validateBindings(bindings, len(decoded[1].([]byte)), validate)
		case Add, Sub, SaturatingSub, Min, Max, MulDiv, Equal, LessOrEqual:
			args := binaryArgs
			if step.Operation == MulDiv {
				args = mulDivArgs
			}
			var decoded []any
			decoded, err = args.Unpack(step.Data)
			if err != nil {
				break
			}
			for _, raw := range decoded {
				if err = validate(*abi.ConvertType(raw, new(Value)).(*Value)); err != nil {
					break
				}
			}
		case Manage:
			var decoded []any
			decoded, err = manageArgs.Unpack(step.Data)
			if err != nil {
				break
			}
			slot.Gate = *abi.ConvertType(decoded[0], new(Value)).(*Value)
			slot.Leaves = hashes(decoded[1].([][32]byte))
			slot.Bindings = *abi.ConvertType(decoded[2], new([][]Binding)).(*[][]Binding)
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
			decoded, err = approveArgs.Unpack(step.Data)
			if err != nil {
				break
			}
			slot.Amount = *abi.ConvertType(decoded[0], new(Value)).(*Value)
			slot.Token = decoded[1].(common.Address)
			slot.Spender = decoded[2].(common.Address)
			slot.Leaves = []common.Hash{decoded[3].([32]byte)}
			err = validate(slot.Amount)
			result.Slots = append(result.Slots, slot)
		case Call:
			var decoded []any
			decoded, err = callArgs.Unpack(step.Data)
			if err != nil {
				break
			}
			bindings := *abi.ConvertType(decoded[2], new([]Binding)).(*[]Binding)
			err = validateBindings(bindings, len(decoded[1].([]byte)), validate)
		default:
			err = errors.Errorf("unknown operation %d", step.Operation)
		}
		if err != nil {
			return Inspection{}, nil, errors.Errorf("step %d: %w", index, err)
		}
	}
	result.Inputs = sortedInputs(inputs)
	return result, sortedInputs(requiredInputs), nil
}

func sortedInputs(inputs map[uint]struct{}) []uint {
	result := make([]uint, 0, len(inputs))
	for input := range inputs {
		result = append(result, input)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func validateValue(value Value, index int, steps []Step, inputs map[uint]struct{}) error {
	if value.Value == nil || value.Value.Sign() < 0 || value.Value.BitLen() > 256 {
		return errors.New("invalid uint256 value")
	}
	switch value.Source {
	case Literal:
	case Input:
		// Bound host allocation, independently of the numerical input's full uint256 width.
		if !value.Value.IsUint64() || value.Value.Uint64() > MaxInputIndex {
			return errors.New("input index exceeds keeper limit of 65535")
		}
		inputs[uint(value.Value.Uint64())] = struct{}{}
	case Result:
		if !value.Value.IsUint64() || value.Value.Uint64() >= uint64(index) || steps[value.Value.Uint64()].Operation > MulDiv {
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
		if binding.Offset == nil || !binding.Offset.IsUint64() || binding.Offset.Uint64() < end || binding.Offset.Uint64() > ^uint64(0)-32 {
			return errors.New("binding overlaps, changes selector or exceeds bounds")
		}
		end = binding.Offset.Uint64() + 32
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
