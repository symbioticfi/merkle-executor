package redemptions

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func argumentType(name string) abi.Type {
	t, err := abi.NewType(name, "", nil)
	if err != nil {
		panic(err)
	}
	return t
}

var queryArguments = abi.Arguments{{Type: argumentType("address")}, {Type: argumentType("bytes")}, {Type: argumentType("uint256[]")}, {Type: argumentType("uint256")}}
var readArguments = abi.Arguments{{Type: argumentType("uint256")}}

// Figure's second amount must be the recipe's preview of this exact unwrap.
// A READ after the first Manage remains unsupported by the prefix resolver.
func figureLink(r recipes.Recipe, route selectedRoute, token common.Address) error {
	e := route.Intermediate.Expression
	index := e.Value.Uint64()
	if e.Source != recipes.Result || index >= uint64(len(r.Steps)) || r.Steps[index][0] != recipes.Read {
		return errors.New("Figure request amount must be its linked preview read")
	}
	values, err := readArguments.Unpack(r.Steps[index][1:])
	if err != nil {
		return err
	}
	query := values[0].(*big.Int)
	if !query.IsUint64() || query.Uint64() >= uint64(len(r.Queries)) {
		return errors.New("Figure preview query out of range")
	}
	values, err = queryArguments.Unpack(r.Queries[query.Uint64()])
	if err != nil {
		return err
	}
	data := values[1].([]byte)
	bindings := values[2].([]*big.Int)
	expected := erc4626.NewERC4626().PackPreviewRedeem(new(big.Int))
	if values[0].(common.Address) != token || len(data) != len(expected) || !bytes.Equal(data[:4], expected[:4]) || values[3].(*big.Int).Sign() != 0 || len(bindings) != 1 {
		return errors.New("Figure request amount is not linked to its unwrap token")
	}
	binding := bindings[0]
	at := new(big.Int).Rsh(new(big.Int).Set(binding), 224).Uint64()
	source := uint8(new(big.Int).Rsh(new(big.Int).Set(binding), 216).Uint64())
	value := new(big.Int).And(binding, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(1)))
	if at != 4 || source != route.AmountField.Source || value.Cmp(route.AmountField.Value) != 0 {
		return errors.New("Figure preview amount differs from its unwrap result")
	}
	return nil
}
