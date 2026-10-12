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
	if err := RejectAliases(node); err != nil {
		return err
	}
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

// MaxInputIndex is the shared service bound for a sparse runtime input vector.
const MaxInputIndex = 4095

func InputIndex(index uint) error {
	if index > MaxInputIndex {
		return errors.Errorf("input index exceeds limit of %d", MaxInputIndex)
	}
	return nil
}

// RejectAliases prevents operational values from changing when an anchor moves
// without changing the node identity used by historical configuration hashes.
// Anchor-free and unused-anchor configuration retains its original fingerprint.
func RejectAliases(node yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return errors.New("YAML aliases are not supported in provider configuration")
	}
	for _, child := range node.Content {
		if err := RejectAliases(*child); err != nil {
			return err
		}
	}
	return nil
}
