package recipes

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
)

func pack(t *testing.T, args abi.Arguments, values ...any) []byte {
	t.Helper()
	data, err := args.Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func literal(n int64) *big.Int { return big.NewInt(n) }
func input(n int64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(n))
}
func result(n int64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(n))
}

func binding(offset uint32, value *big.Int) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(offset)), 224), value)
}

func step(operation uint8, data []byte) []byte { return append([]byte{operation}, data...) }

func singleTree(t *testing.T) *Tree {
	t.Helper()
	data := erc20.NewERC20().PackApprove(common.HexToAddress("0x1234"), new(big.Int))
	mask := make([]byte, len(data))
	for i := 36; i < len(mask); i++ {
		mask[i] = 0xff
	}
	leaf := Leaf{Target: common.HexToAddress("0x5678"), Value: new(big.Int), Data: data, DataMask: mask}
	var err error
	leaf.Hash, err = HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	return &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}
}

func manageStep(t *testing.T, leaf common.Hash, gate *big.Int, bindings []*big.Int) []byte {
	return step(Manage, pack(t, manageArgs, gate, [][32]byte{leaf}, [][]*big.Int{bindings}))
}

func TestIndependentABIGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/core-abi.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]string
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	recipe, err := DecodeRecipe(common.FromHex(golden["recipe"]))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if hash.Hex() != golden["recipeHash"] {
		t.Fatalf("recipe hash %s", hash)
	}
	if hexutil.Encode(executorABI.PackSetRecipe(recipe.Queries, recipe.Steps)) != golden["setRecipe"] || executorABI.GetABI().Events["SetRecipe"].ID.Hex() != golden["setRecipeTopic"] {
		t.Fatal("Solidity setRecipe ABI or event mismatch")
	}
	if len(recipe.Steps) != 3 || recipe.Steps[2][0] != Call {
		t.Fatal("Call=11 lost")
	}
	encoded, leaves, err := Assemble(recipe, nil, map[uint]string{0: "17"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hexutil.Encode(encoded) != golden["execute"] || len(leaves) != 0 {
		t.Fatal("Solidity execute ABI mismatch")
	}
}

func TestAllOperationsAndReferenceRules(t *testing.T) {
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	query := pack(t, queryArgs, leaf.Target, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))
	recipe := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}
	for op := Add; op <= LessOrEqual; op++ {
		data := pack(t, binaryArgs, result(0), literal(1))
		if op == MulDiv {
			data = pack(t, mulDivArgs, result(0), literal(1), literal(1))
		}
		recipe.Steps = append(recipe.Steps, step(op, data))
	}
	recipe.Steps = append(recipe.Steps, manageStep(t, leaf.Hash, literal(1), nil))
	recipe.Steps = append(recipe.Steps, step(Approve, pack(t, approveArgs, literal(2), leaf.Target, common.HexToAddress("0x1234"), leaf.Hash)))
	recipe.Steps = append(recipe.Steps, step(Call, pack(t, callArgs, leaf.Target, []byte{1, 2, 3, 4}, []*big.Int{})))
	inspection, err := Inspect(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Slots) != 2 || inspection.Slots[0].StepIndex != 9 || inspection.Slots[1].StepIndex != 10 {
		t.Fatal("wrong runtime layout")
	}
	if _, _, err = Assemble(recipe, tree, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []*big.Int{result(1), input(65536), new(big.Int).Lsh(big.NewInt(3), 216)} {
		invalid := Recipe{Steps: [][]byte{step(Add, pack(t, binaryArgs, bad, literal(0)))}}
		if _, err = Inspect(invalid); err == nil {
			t.Fatalf("bad reference accepted: %+v", bad)
		}
	}
	invalid := Recipe{Steps: [][]byte{step(Equal, pack(t, binaryArgs, literal(1), literal(1))), step(Add, pack(t, binaryArgs, result(0), literal(1)))}}
	if _, err = Inspect(invalid); err == nil {
		t.Fatal("assertion result accepted")
	}
	if _, err = Inspect(Recipe{Steps: [][]byte{{12}}}); err == nil {
		t.Fatal("unknown opcode accepted")
	}
}

func TestMaskedReplacementAndSkippedGroup(t *testing.T) {
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	recipe := Recipe{Steps: [][]byte{manageStep(t, leaf.Hash, literal(1), []*big.Int{binding(36, literal(19))})}}
	replacement := bytes.Clone(leaf.Data)
	replacement[len(replacement)-1] = 17
	data, used, err := Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": replacement}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != 1 || used[0] != leaf.Hash {
		t.Fatal("used leaf missing")
	}
	decoded, err := executorABI.GetABI().Methods["execute"].Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatal(err)
	}
	runtime := decoded[3].([][]byte)
	unpacked, err := accountABI.GetABI().Methods["manage"].Inputs.Unpack(runtime[0])
	if err != nil {
		t.Fatal(err)
	}
	actions := *abi.ConvertType(unpacked[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
	// On-chain bindings must still run; the keeper does not overwrite 17 with 19.
	if actions[0].Data[len(replacement)-1] != 17 {
		t.Fatal("keeper interpreted binding")
	}
	replacement[4] ^= 1
	if _, _, err = Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": replacement}, nil); err == nil {
		t.Fatal("pinned recipient changed")
	}
	if _, _, err = Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": replacement[:len(replacement)-1]}, nil); err == nil {
		t.Fatal("short replacement accepted")
	}
	data, used, err = Assemble(recipe, nil, nil, nil, []uint{0})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = executorABI.GetABI().Methods["execute"].Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatal(err)
	}
	runtime = decoded[3].([][]byte)
	if len(runtime) != 1 || len(runtime[0]) != 0 || len(used) != 0 {
		t.Fatal("skipped slot lost or acquired role")
	}
	if _, _, err = Assemble(recipe, tree, nil, nil, []uint{0, 0}); err == nil {
		t.Fatal("duplicate skip accepted")
	}
	if _, _, err = Assemble(recipe, tree, nil, map[string]hexutil.Bytes{"0:0": leaf.Data}, []uint{0}); err == nil {
		t.Fatal("skipped replacement accepted")
	}
}

func TestApprovalPinsSpenderAndUsesPermissionPayload(t *testing.T) {
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	recipe := Recipe{Steps: [][]byte{step(Approve, pack(t, approveArgs, input(2), leaf.Target, common.HexToAddress("0x1234"), leaf.Hash))}}
	data, _, err := Assemble(recipe, tree, map[uint]string{2: "123"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := executorABI.GetABI().Methods["execute"].Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatal(err)
	}
	values := decoded[2].([]*big.Int)
	if len(values) != 3 || values[0].Sign() != 0 || values[2].Uint64() != 123 {
		t.Fatal("sparse input indexing failed")
	}
	permission, err := permissionArgs.Unpack(decoded[3].([][]byte)[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(permission[0].([]byte), leaf.DataMask) {
		t.Fatal("approval mask mismatch")
	}
	recipe.Steps[0] = step(Approve, pack(t, approveArgs, literal(1), leaf.Target, common.HexToAddress("0xdead"), leaf.Hash))
	if _, _, err = Assemble(recipe, tree, nil, nil, nil); err == nil {
		t.Fatal("foreign spender accepted")
	}
}

func TestApprovalSupportsPinnedAndPartialAmountMasks(t *testing.T) {
	for _, maskByte := range []byte{0, 0x0f, 0xff} {
		tree := singleTree(t)
		leaf := &tree.Leaves[0]
		leaf.Data[len(leaf.Data)-1] = 17
		for i := 36; i < len(leaf.DataMask); i++ {
			leaf.DataMask[i] = maskByte
		}
		var err error
		leaf.Hash, err = HashLeaf(*leaf)
		if err != nil {
			t.Fatal(err)
		}
		tree.Root = leaf.Hash
		recipe := Recipe{Steps: [][]byte{step(Approve, pack(t, approveArgs, literal(17), leaf.Target, common.HexToAddress("0x1234"), leaf.Hash))}}
		if _, _, err = Assemble(recipe, tree, nil, nil, nil); err != nil {
			t.Fatalf("mask %x: %v", maskByte, err)
		}
	}
}

func TestSkippedBindingInputsAreOptional(t *testing.T) {
	tree := singleTree(t)
	recipe := Recipe{Steps: [][]byte{manageStep(t, tree.Leaves[0].Hash, input(0), []*big.Int{binding(36, input(2))})}}
	for _, values := range []map[uint]string{{0: "0"}, {0: "0", 2: "42"}} {
		encoded, _, err := Assemble(recipe, nil, values, nil, []uint{0})
		if err != nil {
			t.Fatalf("skipped binding input: %v", err)
		}
		decoded, err := executorABI.GetABI().Methods["execute"].Inputs.Unpack(encoded[4:])
		if err != nil {
			t.Fatal(err)
		}
		expected := 1
		if _, ok := values[2]; ok {
			expected = 3
		}
		if len(decoded[2].([]*big.Int)) != expected {
			t.Fatal("omitted trailing input inflated calldata")
		}
	}
	if _, _, err := Assemble(recipe, nil, nil, nil, []uint{0}); err == nil {
		t.Fatal("skipped gate input must still be required")
	}
	if _, _, err := Assemble(recipe, tree, map[uint]string{0: "0"}, nil, nil); err == nil {
		t.Fatal("keeper must not infer a skip from the numerical gate")
	}
	recipe.Steps = append(recipe.Steps, step(Add, pack(t, binaryArgs, input(2), literal(0))))
	if _, _, err := Assemble(recipe, nil, map[uint]string{0: "0"}, nil, []uint{0}); err == nil {
		t.Fatal("input referenced outside skipped group became optional")
	}
}

func TestMalformedPayloadsReturnErrors(t *testing.T) {
	if _, err := Inspect(Recipe{Steps: [][]byte{nil}}); err == nil {
		t.Fatal("empty step accepted")
	}
	if _, _, err := Assemble(Recipe{Steps: [][]byte{nil}}, nil, nil, nil, []uint{0}); err == nil {
		t.Fatal("empty step accepted as skipped Manage")
	}
	for op := Read; op <= Call; op++ {
		for _, length := range []int{0, 1, 31} {
			if _, err := Inspect(Recipe{Steps: [][]byte{step(op, make([]byte, length))}}); err == nil {
				t.Fatalf("operation %d accepted truncated payload %d", op, length)
			}
		}
	}
	for _, encoded := range [][]byte{nil, {0}, make([]byte, 31)} {
		if _, err := DecodeRecipe(encoded); err == nil {
			t.Fatal("truncated recipe accepted")
		}
	}
}

func TestPackedOperandAndBindingWidths(t *testing.T) {
	maxLiteral := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(1))
	for _, test := range []struct {
		name   string
		word   *big.Int
		source uint8
		value  *big.Int
	}{
		{"max literal", maxLiteral, Literal, maxLiteral},
		{"max input index", input(MaxInputIndex), Input, big.NewInt(MaxInputIndex)},
		{"result with high bytes", binding(0xffffffff, result(1)), Result, big.NewInt(1)},
		{"literal with high bytes", binding(0xffffffff, maxLiteral), Literal, maxLiteral},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := new(big.Int).Set(test.word)
			decoded := decodeValue(test.word)
			if decoded.Source != test.source || decoded.Value.Cmp(test.value) != 0 || test.word.Cmp(original) != 0 {
				t.Fatal("packed operand decoded incorrectly or mutated")
			}
		})
	}
	bindings := decodeBindings([]*big.Int{binding(0xffffffff, input(2))})
	if bindings[0].Offset != 0xffffffff || bindings[0].Value.Source != Input || bindings[0].Value.Value.Uint64() != 2 {
		t.Fatal("packed binding lost offset or operand")
	}
	if err := validateBindings(bindings, 68, func(Value) error { return nil }); err == nil {
		t.Fatal("binding end overflowed uint32")
	}
	for _, source := range []uint8{Input, Result} {
		word := new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(source)), 216), maxLiteral)
		if _, err := Inspect(Recipe{Steps: [][]byte{step(Add, pack(t, binaryArgs, word, literal(0)))}}); err == nil {
			t.Fatal("uint216 index truncated to host width")
		}
	}
}

func TestFullWidthInputAndCommittedBytes(t *testing.T) {
	// Solidity ignores the high four operand bytes and trailing ABI data, but
	// both remain part of the recipe commitment and must be submitted unchanged.
	data := append(pack(t, binaryArgs, binding(0xffffffff, input(0)), literal(0)), 1, 2, 3)
	recipe := Recipe{Steps: [][]byte{step(Add, data)}}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	encoded, _, err := Assemble(recipe, nil, map[uint]string{0: maximum.String()}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := executorABI.GetABI().Methods["execute"].Inputs.Unpack(encoded[4:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded[1].([][]byte)[0], recipe.Steps[0]) || decoded[2].([]*big.Int)[0].Cmp(maximum) != 0 {
		t.Fatal("committed instruction or uint256 input changed")
	}
}

func TestBindingsAndPayloadFailures(t *testing.T) {
	for _, bindings := range [][]*big.Int{
		{binding(3, literal(0))},
		{binding(4, literal(0)), binding(35, literal(0))},
		{binding(36, literal(0))},
	} {
		recipe := Recipe{Steps: [][]byte{step(Call, pack(t, callArgs, common.Address{}, make([]byte, 36), bindings))}}
		if _, err := Inspect(recipe); err == nil {
			t.Fatal("bad binding accepted")
		}
	}
	data := pack(t, binaryArgs, literal(1), literal(2))
	if _, err := Inspect(Recipe{Steps: [][]byte{step(Add, data[:63])}}); err == nil {
		t.Fatal("truncated payload accepted")
	}
	if _, err := Inspect(Recipe{Steps: [][]byte{step(Add, append(data, 1, 2, 3))}}); err != nil {
		t.Fatalf("Solidity-valid trailing bytes rejected: %v", err)
	}
	if _, err := Inspect(Recipe{Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}); err == nil {
		t.Fatal("missing query accepted")
	}
}
