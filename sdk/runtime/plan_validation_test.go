package runtime

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestPlanRestorationAuthenticatesEveryManagedPermission(t *testing.T) {
	leaf := recipes.Leaf{Target: common.HexToAddress("0x40"), Value: new(big.Int), Data: []byte{1, 2, 3, 4}, DataMask: make([]byte, 4)}
	var err error
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	word, _ := abi.NewType("uint256", "", nil)
	hashes, _ := abi.NewType("bytes32[]", "", nil)
	bindings, _ := abi.NewType("uint256[][]", "", nil)
	step, err := (abi.Arguments{{Type: word}, {Type: hashes}, {Type: bindings}}).Pack(big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, step...)}}
	hash, _ := recipes.HashRecipe(recipe)
	calldata, leaves, err := recipes.Assemble(recipe, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := Plan{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Executor: common.HexToAddress("0x11"), Account: common.HexToAddress("0x20"), Caller: common.HexToAddress("0x30"), RecipeHash: hash, Root: leaf.Hash}, Calldata: calldata, Leaves: leaves}
	if err := validatePlanIdentity(original); err != nil {
		t.Fatal("baseline", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*account.IMerkleAccountAction)
	}{
		{"target", func(a *account.IMerkleAccountAction) { a.Target = common.HexToAddress("0x41") }},
		{"value", func(a *account.IMerkleAccountAction) { a.Value = big.NewInt(1) }},
		{"mask", func(a *account.IMerkleAccountAction) { a.DataMask[0] = 255 }},
		{"proof", func(a *account.IMerkleAccountAction) { a.Proof = [][32]byte{common.HexToHash("0x1234")} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := original.Clone()
			e := executor.NewRecipeExecutor()
			args, err := e.GetABI().Methods["execute"].Inputs.Unpack(plan.Calldata[4:])
			if err != nil {
				t.Fatal(err)
			}
			slots := args[3].([][]byte)
			m := account.NewMerkleAccount()
			raw, err := m.GetABI().Methods["manage"].Inputs.Unpack(slots[0])
			if err != nil {
				t.Fatal(err)
			}
			calls := *abi.ConvertType(raw[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
			tc.mutate(&calls[0])
			slots[0] = m.PackManage(calls)[4:]
			plan.Calldata = e.PackExecute(args[0].([][]byte), args[1].([][]byte), args[2].([]*big.Int), slots)
			if err := validatePlanIdentity(plan); err == nil {
				t.Fatal("altered permission accepted")
			}
		})
	}
}

func TestPlanRestorationAuthenticatesApprovalProofWithoutInventingAmount(t *testing.T) {
	token, spender := common.HexToAddress("0x40"), common.HexToAddress("0x50")
	// The leaf's pinned amount preimage differs from the zero-cleanup instruction.
	// Full simulation owns live allowance/reset semantics; offline validation binds
	// the saved proof to the committed leaf without inventing a replacement amount.
	leaf := recipes.Leaf{Target: token, Value: new(big.Int), Data: erc20.NewERC20().PackApprove(spender, big.NewInt(42)), DataMask: make([]byte, 68)}
	leaf.Hash, _ = recipes.HashLeaf(leaf)
	word, _ := abi.NewType("uint256", "", nil)
	address, _ := abi.NewType("address", "", nil)
	hashType, _ := abi.NewType("bytes32", "", nil)
	step, err := (abi.Arguments{{Type: word}, {Type: address}, {Type: address}, {Type: hashType}}).Pack(new(big.Int), token, spender, [32]byte(leaf.Hash))
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Approve}, step...)}}
	hash, _ := recipes.HashRecipe(recipe)
	calldata, leaves, err := recipes.Assemble(recipe, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10"), Executor: common.HexToAddress("0x11"), Account: common.HexToAddress("0x20"), Caller: common.HexToAddress("0x30"), RecipeHash: hash, Root: leaf.Hash}, Calldata: calldata, Leaves: leaves}
	if err := validatePlanIdentity(plan); err != nil {
		t.Fatal("baseline", err)
	}
	e := executor.NewRecipeExecutor()
	args, _ := e.GetABI().Methods["execute"].Inputs.Unpack(calldata[4:])
	slots := args[3].([][]byte)
	maskType, _ := abi.NewType("bytes", "", nil)
	proofType, _ := abi.NewType("bytes32[]", "", nil)
	permissions := abi.Arguments{{Type: maskType}, {Type: proofType}}
	for _, tc := range []struct {
		name  string
		mask  []byte
		proof [][32]byte
	}{
		{"proof", make([]byte, 68), [][32]byte{common.HexToHash("0x1234")}},
		{"mask", make([]byte, 67), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slots[0], _ = permissions.Pack(tc.mask, tc.proof)
			plan.Calldata = e.PackExecute(args[0].([][]byte), args[1].([][]byte), args[2].([]*big.Int), slots)
			if err := validatePlanIdentity(plan); err == nil {
				t.Fatal("altered approval permission accepted")
			}
		})
	}
}
