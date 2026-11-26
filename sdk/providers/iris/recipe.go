package iris

import (
	"bytes"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

var (
	queryArgs   = recipes.Arguments("address", "bytes", "uint256[]", "uint256")
	readArgs    = recipes.Arguments("uint256")
	compareArgs = recipes.Arguments("uint256", "uint256")
	manageArgs  = recipes.Arguments("uint256", "bytes32[]", "uint256[][]")
	withdraw    = crypto.Keccak256([]byte("withdrawBond(address,uint256,address)"))[:4]
)

func matchesClaim(s provider.Snapshot, recipe recipes.Recipe, tree *recipes.Tree) (bool, error) {
	if s.ChainID == nil || s.ChainID.Cmp(big.NewInt(1)) != 0 ||
		s.Account == (common.Address{}) || len(recipe.Queries) != 6 {
		return false, nil
	}
	actions, err := recipes.ManagedActions(recipe, tree)
	if err != nil || len(actions) != 1 {
		return false, err
	}
	action := actions[0]
	leaf := action.Leaf
	if leaf.Target != mainnetCore || leaf.Value == nil || leaf.Value.Sign() != 0 ||
		len(leaf.Data) != 100 || !bytes.Equal(leaf.Data[:4], withdraw) ||
		!bytes.Equal(leaf.Data[68:], common.LeftPadBytes(s.Account.Bytes(), 32)) ||
		action.Pinned(0, 4) != nil || action.Pinned(68, 32) != nil {
		return false, nil
	}

	// Match the Curator template exactly before taking ownership of input 0.
	// The two older encodings include pod bounds, and the oldest also checks bond > 0.
	for _, encoding := range []struct{ podBounds, positiveBond bool }{{}, {true, false}, {true, true}} {
		expected := claimProgram(s.Account, leaf.Hash, encoding.podBounds, encoding.positiveBond)
		if slices.EqualFunc(recipe.Queries, expected.Queries, bytes.Equal) &&
			slices.EqualFunc(recipe.Steps, expected.Steps, bytes.Equal) {
			return true, nil
		}
	}
	return false, nil
}

func claimProgram(account common.Address, leaf common.Hash, podBounds, positiveBond bool) recipes.Recipe {
	var recipe recipes.Recipe
	add := func(op byte, args abi.Arguments, values ...any) {
		recipe.Steps = append(recipe.Steps, append([]byte{op}, pack(args, values...)...))
	}
	operand := func(offset uint32, source byte, value *big.Int) *big.Int {
		prefix := uint64(offset)<<8 | uint64(source)
		return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(prefix), 216), value)
	}
	podInput := operand(0, recipes.Input, big.NewInt(0))
	podBinding := operand(4, recipes.Input, big.NewInt(0))
	if podBounds {
		maxAddress := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 160), big.NewInt(1))
		add(recipes.LessOrEqual, compareArgs, big.NewInt(1), podInput)
		add(recipes.LessOrEqual, compareArgs, podInput, maxAddress)
	}
	read := func(data []byte, offset int64) *big.Int {
		index := big.NewInt(int64(len(recipe.Steps)))
		add(recipes.Read, readArgs, big.NewInt(int64(len(recipe.Queries))))
		recipe.Queries = append(recipe.Queries, pack(queryArgs, mainnetCore, data, []*big.Int{podBinding}, big.NewInt(offset)))
		return index
	}
	loan := read(coreABI.PackGetLoan(common.Address{}), 32)
	add(recipes.Equal, compareArgs, operand(0, recipes.Result, loan), account.Big())
	position := coreABI.PackGetPosition(common.Address{})
	for _, offset := range []int64{128, 64, 224, 288} {
		result := read(position, offset)
		add(recipes.Equal, compareArgs, operand(0, recipes.Result, result), big.NewInt(0))
	}
	bond := read(position, 96)
	if positiveBond {
		add(recipes.LessOrEqual, compareArgs, big.NewInt(1), operand(0, recipes.Result, bond))
	}
	add(recipes.Manage, manageArgs, big.NewInt(1), [][32]byte{leaf},
		[][]*big.Int{{podBinding, operand(36, recipes.Result, bond)}})
	return recipe
}

func pack(args abi.Arguments, values ...any) []byte {
	data, err := args.Pack(values...)
	if err != nil {
		panic(err) // All arguments are internally constructed with fixed ABI types.
	}
	return data
}
