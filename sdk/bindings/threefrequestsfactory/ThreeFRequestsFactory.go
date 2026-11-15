// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package threefrequestsfactory

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

// ThreeFRequestsFactoryMetaData contains all meta data concerning the ThreeFRequestsFactory contract.
var ThreeFRequestsFactoryMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"isRequest\",\"inputs\":[{\"name\":\"request\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\"}]",
	ID:  "ThreeFRequestsFactory",
}

// ThreeFRequestsFactory is an auto generated Go binding around an Ethereum contract.
type ThreeFRequestsFactory struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ThreeFRequestsFactory) GetABI() abi.ABI {
	return c.abi
}

// NewThreeFRequestsFactory creates a new instance of ThreeFRequestsFactory.
func NewThreeFRequestsFactory() *ThreeFRequestsFactory {
	parsed, err := ThreeFRequestsFactoryMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ThreeFRequestsFactory{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ThreeFRequestsFactory) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackIsRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9adfc20.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isRequest(address request) view returns(bool)
func (threeFRequestsFactory *ThreeFRequestsFactory) PackIsRequest(request common.Address) []byte {
	enc, err := threeFRequestsFactory.abi.Pack("isRequest", request)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9adfc20.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isRequest(address request) view returns(bool)
func (threeFRequestsFactory *ThreeFRequestsFactory) TryPackIsRequest(request common.Address) ([]byte, error) {
	return threeFRequestsFactory.abi.Pack("isRequest", request)
}

// UnpackIsRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe9adfc20.
//
// Solidity: function isRequest(address request) view returns(bool)
func (threeFRequestsFactory *ThreeFRequestsFactory) UnpackIsRequest(data []byte) (bool, error) {
	out, err := threeFRequestsFactory.abi.Unpack("isRequest", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}
