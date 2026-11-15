// Package parse holds protocol-neutral address and integer parsing.
package parse

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

func Uint256(s string) (*big.Int, error) {
	if s == "" {
		return nil, errors.New("empty uint256")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil, errors.New("uint256 must be an unsigned decimal string")
		}
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.BitLen() > 256 {
		return nil, errors.New("uint256 out of range")
	}
	return v, nil
}

func Address(s string) (common.Address, error) {
	if !common.IsHexAddress(s) {
		return common.Address{}, errors.New("invalid address")
	}
	v := common.HexToAddress(s)
	if v == (common.Address{}) {
		return v, errors.New("zero address")
	}
	return v, nil
}

// MaxInputIndex is the shared service bound for a sparse runtime input vector.
const MaxInputIndex = 4095

func InputIndex(index uint) error {
	if index > MaxInputIndex {
		return errors.Errorf("input index exceeds limit of %d", MaxInputIndex)
	}
	return nil
}
