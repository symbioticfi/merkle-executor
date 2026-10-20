package merkl

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type capabilityFrame struct {
	queries, steps       [][]byte
	inputs               []*big.Int
	inputBase, callIndex int
}

var queryArguments = layoutArgs("address", "bytes", "uint256[]", "uint256")
var pairArguments = layoutArgs("uint256", "uint256")
var indexArguments = layoutArgs("uint256")

// validateClaimGuard admits the frontend's atomic root/recipient/ledger/cash
// checks. Extra outer checked-operation wrappers remain byte-for-byte pinned.
func validateClaimGuard(action recipes.ManagedAction, frame *capabilityFrame, layout *claimLayout, account common.Address, resolve func(int) (int, error)) error {
	if frame == nil {
		return nil
	}
	if len(layout.tokens) != 1 || len(frame.inputs) != 4 || len(frame.queries) != 5 || len(frame.steps) != 20 || frame.callIndex != 11 {
		return errors.New("unsupported Merkl claim guard layout")
	}
	for i := 0; i < 3; i++ {
		at, err := resolve(frame.inputBase + 32*i)
		if err != nil {
			return err
		}
		if err = action.Pinned(at, 32); err != nil {
			return errors.New("Merkl root and recipient guards must be pinned")
		}
	}
	if frame.inputs[0].Sign() == 0 || frame.inputs[1].BitLen() > 160 || frame.inputs[2].BitLen() > 160 {
		return errors.New("invalid Merkl root or recipient guard")
	}
	a := binding.NewMerklDistributor()
	token := layout.tokens[0]
	balance := append(crypto.Keccak256([]byte("balanceOf(address)"))[:4], common.LeftPadBytes(account.Bytes(), 32)...)
	queries := []struct {
		target common.Address
		data   []byte
	}{
		{layout.distributor, a.PackGetMerkleRoot()},
		{layout.distributor, a.PackClaimRecipient(account, token)},
		{layout.distributor, a.PackClaimRecipient(account, common.Address{})},
		{layout.distributor, a.PackClaimed(account, token)},
		{token, balance},
	}
	for i, query := range queries {
		expected, _ := queryArguments.Pack(query.target, query.data, []*big.Int{}, new(big.Int))
		if !bytes.Equal(expected, frame.queries[i]) {
			return errors.New("Merkl root, recipient, ledger and balance queries must match claim")
		}
	}
	value := func(source uint8, n int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(source)), 216), big.NewInt(n))
	}
	input := func(n int64) *big.Int { return value(recipes.Input, n) }
	result := func(n int64) *big.Int { return value(recipes.Result, n) }
	read := func(n int64) []byte {
		raw, _ := indexArguments.Pack(big.NewInt(n))
		return append([]byte{recipes.Read}, raw...)
	}
	binary := func(op uint8, a, b *big.Int) []byte {
		raw, _ := pairArguments.Pack(a, b)
		return append([]byte{op}, raw...)
	}
	amountOffset := 4 + int(new(big.Int).SetBytes(layout.data[68:100]).Uint64()) + 32
	bindingWord := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(amountOffset)), 224), input(3))
	call, _ := nestedCallArgs.Pack(layout.distributor, layout.data, []*big.Int{bindingWord})
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 208), big.NewInt(1))
	steps := [][]byte{
		read(0), binary(recipes.Equal, result(0), input(0)), read(1), binary(recipes.Equal, result(2), input(1)), read(2), binary(recipes.Equal, result(4), input(2)),
		read(3), binary(recipes.Add, result(6), big.NewInt(1)), binary(recipes.LessOrEqual, result(7), input(3)), binary(recipes.LessOrEqual, input(3), maximum),
		read(4), append([]byte{recipes.Call}, call...), read(3), binary(recipes.Equal, result(12), input(3)), read(4), binary(recipes.Sub, result(14), result(10)),
		binary(recipes.Sub, input(3), result(6)), binary(recipes.Equal, result(15), result(16)), read(0), binary(recipes.Equal, result(18), input(0)),
	}
	for i, expected := range steps {
		if !bytes.Equal(expected, frame.steps[i]) {
			return errors.New("Merkl atomic claim guard instructions must be pinned canonical checks")
		}
	}
	return nil
}
