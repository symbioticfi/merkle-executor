// Package threef supplies ready factory request addresses to single-claim recipes.
package threef

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/recipecapability"
	request "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequest"
	factory "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequestsfactory"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func arguments(names ...string) abi.Arguments {
	args := make(abi.Arguments, len(names))
	for i, name := range names {
		args[i].Type, _ = abi.NewType(name, "", nil)
	}
	return args
}

var queryArgs = arguments("address", "bytes", "uint256[]", "uint256")
var callArgs = arguments("address", "bytes", "uint256[]")
var dynamicCallArgs = arguments("uint256", "bytes", "uint256[]")
var pairArgs = arguments("uint256", "uint256")
var indexArgs = arguments("uint256")

func packed(args abi.Arguments, values ...any) []byte {
	data, err := args.Pack(values...)
	if err != nil {
		panic(err)
	} // Only fixed, internally typed arguments reach this helper.
	return data
}
func instruction(op byte, args abi.Arguments, values ...any) []byte {
	return append([]byte{op}, packed(args, values...)...)
}
func binding(offset int) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset)), 224), new(big.Int).Lsh(big.NewInt(int64(recipes.Input)), 216))
}

// claimTemplate reproduces the reviewed capability: isRequest(request), then
// burnAll(account, account) on that same request. Only the outer request word varies.
func claimTemplate(owner, requestsFactory common.Address) ([]byte, int) {
	burn, _ := request.NewThreeFRequest().TryPackBurnAll(owner, owner)
	dynamic := instruction(recipes.Call, dynamicCallArgs, new(big.Int), burn, []*big.Int{})
	inner, _ := account.NewRecipeCapability().TryPackExecute(nil, [][]byte{dynamic}, nil)
	innerOffset := bytes.Index(inner, dynamic) + 1
	isRequest, _ := factory.NewThreeFRequestsFactory().TryPackIsRequest(common.Address{})
	query := packed(queryArgs, requestsFactory, isRequest, []*big.Int{binding(4)}, new(big.Int))
	steps := [][]byte{
		instruction(recipes.Read, indexArgs, new(big.Int)),
		instruction(recipes.Equal, pairArgs, new(big.Int).Lsh(big.NewInt(int64(recipes.Result)), 216), big.NewInt(1)),
		instruction(recipes.Call, callArgs, owner, inner, []*big.Int{binding(innerOffset)}),
	}
	data, _ := account.NewRecipeCapability().TryPackExecute([][]byte{query}, steps, []*big.Int{new(big.Int)})
	offset := 4 + int(new(big.Int).SetBytes(data[68:100]).Int64()) + 32
	return data, offset
}

func claimLayout(action recipes.ManagedAction, owner common.Address) (common.Address, int, bool) {
	data := action.Leaf.Data
	method := account.NewRecipeCapability().GetABI().Methods["execute"]
	if action.Leaf.Target != owner || action.Leaf.Value.Sign() != 0 || len(action.Bindings) != 0 || len(data) < 100 || !bytes.Equal(data[:4], method.ID) {
		return common.Address{}, 0, false
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return common.Address{}, 0, false
	}
	queries := args[0].([][]byte)
	if len(queries) != 1 {
		return common.Address{}, 0, false
	}
	query, err := queryArgs.Unpack(queries[0])
	if err != nil {
		return common.Address{}, 0, false
	}
	requestsFactory := query[0].(common.Address)
	if requestsFactory == (common.Address{}) {
		return common.Address{}, 0, false
	}
	expected, offset := claimTemplate(owner, requestsFactory)
	if len(data) != len(expected) || len(action.Leaf.DataMask) != len(expected) {
		return common.Address{}, 0, false
	}
	for i := range data {
		if i >= offset && i < offset+32 {
			if action.Leaf.DataMask[i] != 255 {
				return common.Address{}, 0, false
			}
		} else if data[i] != expected[i] || action.Leaf.DataMask[i] != 0 {
			return common.Address{}, 0, false
		}
	}
	return requestsFactory, offset, true
}
