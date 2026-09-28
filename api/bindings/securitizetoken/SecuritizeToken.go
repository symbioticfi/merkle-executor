// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package securitizetoken

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

// SecuritizeTokenMetaData contains all meta data concerning the SecuritizeToken contract.
var SecuritizeTokenMetaData = bind.MetaData{
	ABI: "[{\"name\":\"decimals\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"}]",
	ID:  "SecuritizeToken",
}

// SecuritizeToken is an auto generated Go binding around an Ethereum contract.
type SecuritizeToken struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *SecuritizeToken) GetABI() abi.ABI {
	return c.abi
}

// NewSecuritizeToken creates a new instance of SecuritizeToken.
func NewSecuritizeToken() *SecuritizeToken {
	parsed, err := SecuritizeTokenMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &SecuritizeToken{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *SecuritizeToken) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackDecimals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x313ce567.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function decimals() view returns(uint8)
func (securitizeToken *SecuritizeToken) PackDecimals() []byte {
	enc, err := securitizeToken.abi.Pack("decimals")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDecimals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x313ce567.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function decimals() view returns(uint8)
func (securitizeToken *SecuritizeToken) TryPackDecimals() ([]byte, error) {
	return securitizeToken.abi.Pack("decimals")
}

// UnpackDecimals is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (securitizeToken *SecuritizeToken) UnpackDecimals(data []byte) (uint8, error) {
	out, err := securitizeToken.abi.Unpack("decimals", data)
	if err != nil {
		return *new(uint8), err
	}
	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)
	return out0, nil
}
