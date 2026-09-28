// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package centrifugeshare

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = bytes.Equal
	_ = errors.New
	_ = big.NewInt
	_ = common.Big1
	_ = types.BloomLookup
	_ = abi.ConvertType
)

// CentrifugeShareMetaData contains all meta data concerning the CentrifugeShare contract.
var CentrifugeShareMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"vault\",\"inputs\":[{\"name\":\"asset\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"vault\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"}]",
	ID:  "CentrifugeShare",
}

// CentrifugeShare is an auto generated Go binding around an Ethereum contract.
type CentrifugeShare struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *CentrifugeShare) GetABI() abi.ABI {
	return c.abi
}

// NewCentrifugeShare creates a new instance of CentrifugeShare.
func NewCentrifugeShare() *CentrifugeShare {
	parsed, err := CentrifugeShareMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &CentrifugeShare{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *CentrifugeShare) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackVault is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf815c03d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function vault(address asset) view returns(address vault)
func (centrifugeShare *CentrifugeShare) PackVault(asset common.Address) []byte {
	enc, err := centrifugeShare.abi.Pack("vault", asset)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackVault is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf815c03d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function vault(address asset) view returns(address vault)
func (centrifugeShare *CentrifugeShare) TryPackVault(asset common.Address) ([]byte, error) {
	return centrifugeShare.abi.Pack("vault", asset)
}

// UnpackVault is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf815c03d.
//
// Solidity: function vault(address asset) view returns(address vault)
func (centrifugeShare *CentrifugeShare) UnpackVault(data []byte) (common.Address, error) {
	out, err := centrifugeShare.abi.Unpack("vault", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}
