// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package accessmanager

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

// AccessManagerMetaData contains all meta data concerning the AccessManager contract.
var AccessManagerMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"canCall\",\"inputs\":[{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"selector\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"immediate\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"delay\",\"type\":\"uint32\",\"internalType\":\"uint32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"execute\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint32\",\"internalType\":\"uint32\"}],\"stateMutability\":\"payable\"}]",
	ID:  "AccessManager",
}

// AccessManager is an auto generated Go binding around an Ethereum contract.
type AccessManager struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *AccessManager) GetABI() abi.ABI {
	return c.abi
}

// NewAccessManager creates a new instance of AccessManager.
func NewAccessManager() *AccessManager {
	parsed, err := AccessManagerMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &AccessManager{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *AccessManager) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackCanCall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb7009613.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function canCall(address caller, address target, bytes4 selector) view returns(bool immediate, uint32 delay)
func (accessManager *AccessManager) PackCanCall(caller common.Address, target common.Address, selector [4]byte) []byte {
	enc, err := accessManager.abi.Pack("canCall", caller, target, selector)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCanCall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb7009613.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function canCall(address caller, address target, bytes4 selector) view returns(bool immediate, uint32 delay)
func (accessManager *AccessManager) TryPackCanCall(caller common.Address, target common.Address, selector [4]byte) ([]byte, error) {
	return accessManager.abi.Pack("canCall", caller, target, selector)
}

// CanCallOutput serves as a container for the return parameters of contract
// method CanCall.
type CanCallOutput struct {
	Immediate bool
	Delay     uint32
}

// UnpackCanCall is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb7009613.
//
// Solidity: function canCall(address caller, address target, bytes4 selector) view returns(bool immediate, uint32 delay)
func (accessManager *AccessManager) UnpackCanCall(data []byte) (CanCallOutput, error) {
	out, err := accessManager.abi.Unpack("canCall", data)
	outstruct := new(CanCallOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Immediate = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.Delay = *abi.ConvertType(out[1], new(uint32)).(*uint32)
	return *outstruct, nil
}

// PackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1cff79cd.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function execute(address target, bytes data) payable returns(uint32)
func (accessManager *AccessManager) PackExecute(target common.Address, data []byte) []byte {
	enc, err := accessManager.abi.Pack("execute", target, data)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1cff79cd.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function execute(address target, bytes data) payable returns(uint32)
func (accessManager *AccessManager) TryPackExecute(target common.Address, data []byte) ([]byte, error) {
	return accessManager.abi.Pack("execute", target, data)
}

// UnpackExecute is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x1cff79cd.
//
// Solidity: function execute(address target, bytes data) payable returns(uint32)
func (accessManager *AccessManager) UnpackExecute(data []byte) (uint32, error) {
	out, err := accessManager.abi.Unpack("execute", data)
	if err != nil {
		return *new(uint32), err
	}
	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)
	return out0, nil
}
