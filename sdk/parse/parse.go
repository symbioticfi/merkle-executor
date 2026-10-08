// Package parse holds protocol-neutral configuration and integer parsing.
package parse

import (
	"bytes"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"
)

func DecodeStrict(node yaml.Node, out any) error {
	raw, err := yaml.Marshal(&node)
	if err != nil {
		return errors.Errorf("encode config node: %w", err)
	}
	d := yaml.NewDecoder(bytes.NewReader(raw))
	d.KnownFields(true)
	if err := d.Decode(out); err != nil {
		return errors.Errorf("decode config: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected one configuration document")
	}
	return nil
}

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
