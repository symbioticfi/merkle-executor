// Package signer keeps signing material behind a replaceable interface.
package signer

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
)

type Signer interface {
	Address() common.Address
	SignTx(context.Context, *types.Transaction, *big.Int) (*types.Transaction, error)
}

type local struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// NewFromHexKey constructs a local signer; errors never contain key data.
func NewFromHexKey(value string) (Signer, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return nil, errors.New("signer contains an invalid private key")
	}
	return &local{key: key, address: crypto.PubkeyToAddress(key.PublicKey)}, nil
}

func (s *local) Address() common.Address { return s.address }

func (s *local) SignTx(ctx context.Context, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil || chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("transaction and positive chain ID required")
	}
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), s.key)
	if err != nil {
		return nil, errors.Errorf("sign transaction: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return signed, nil
}
