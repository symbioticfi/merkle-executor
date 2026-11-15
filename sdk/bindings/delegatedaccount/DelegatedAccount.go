// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package delegatedaccount

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

// DelegatedAccountMetaData contains all meta data concerning the DelegatedAccount contract.
var DelegatedAccountMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"execute\",\"stateMutability\":\"payable\",\"inputs\":[{\"name\":\"mode\",\"type\":\"bytes32\"},{\"name\":\"executionCalldata\",\"type\":\"bytes\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"supportsExecutionMode\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"mode\",\"type\":\"bytes32\"}],\"outputs\":[{\"name\":\"supported\",\"type\":\"bool\"}]}]",
	ID:  "DelegatedAccount",
}

// DelegatedAccount is an auto generated Go binding around an Ethereum contract.
type DelegatedAccount struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *DelegatedAccount) GetABI() abi.ABI {
	return c.abi
}

// NewDelegatedAccount creates a new instance of DelegatedAccount.
func NewDelegatedAccount() *DelegatedAccount {
	parsed, err := DelegatedAccountMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &DelegatedAccount{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *DelegatedAccount) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9ae5c53.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function execute(bytes32 mode, bytes executionCalldata) payable returns()
func (delegatedAccount *DelegatedAccount) PackExecute(mode [32]byte, executionCalldata []byte) []byte {
	enc, err := delegatedAccount.abi.Pack("execute", mode, executionCalldata)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9ae5c53.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function execute(bytes32 mode, bytes executionCalldata) payable returns()
func (delegatedAccount *DelegatedAccount) TryPackExecute(mode [32]byte, executionCalldata []byte) ([]byte, error) {
	return delegatedAccount.abi.Pack("execute", mode, executionCalldata)
}

// PackSupportsExecutionMode is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd03c7914.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function supportsExecutionMode(bytes32 mode) view returns(bool supported)
func (delegatedAccount *DelegatedAccount) PackSupportsExecutionMode(mode [32]byte) []byte {
	enc, err := delegatedAccount.abi.Pack("supportsExecutionMode", mode)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSupportsExecutionMode is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd03c7914.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function supportsExecutionMode(bytes32 mode) view returns(bool supported)
func (delegatedAccount *DelegatedAccount) TryPackSupportsExecutionMode(mode [32]byte) ([]byte, error) {
	return delegatedAccount.abi.Pack("supportsExecutionMode", mode)
}

// UnpackSupportsExecutionMode is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd03c7914.
//
// Solidity: function supportsExecutionMode(bytes32 mode) view returns(bool supported)
func (delegatedAccount *DelegatedAccount) UnpackSupportsExecutionMode(data []byte) (bool, error) {
	out, err := delegatedAccount.abi.Unpack("supportsExecutionMode", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}
