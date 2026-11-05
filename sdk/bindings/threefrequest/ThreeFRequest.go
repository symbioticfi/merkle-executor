// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package threefrequest

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

// ThreeFRequestMetaData contains all meta data concerning the ThreeFRequest contract.
var ThreeFRequestMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"balancesOf\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"ptShares\",\"type\":\"uint256\"},{\"name\":\"ytShares\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"canWithdraw\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"burnAll\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\"},{\"name\":\"receiver\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"ptShares\",\"type\":\"uint256\"},{\"name\":\"ytShares\",\"type\":\"uint256\"},{\"name\":\"pAssets\",\"type\":\"uint256\"},{\"name\":\"yAssets\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\"}]",
	ID:  "ThreeFRequest",
}

// ThreeFRequest is an auto generated Go binding around an Ethereum contract.
type ThreeFRequest struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ThreeFRequest) GetABI() abi.ABI {
	return c.abi
}

// NewThreeFRequest creates a new instance of ThreeFRequest.
func NewThreeFRequest() *ThreeFRequest {
	parsed, err := ThreeFRequestMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ThreeFRequest{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ThreeFRequest) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackBalancesOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6392a51f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function balancesOf(address account) view returns(uint256 ptShares, uint256 ytShares)
func (threeFRequest *ThreeFRequest) PackBalancesOf(account common.Address) []byte {
	enc, err := threeFRequest.abi.Pack("balancesOf", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackBalancesOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6392a51f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function balancesOf(address account) view returns(uint256 ptShares, uint256 ytShares)
func (threeFRequest *ThreeFRequest) TryPackBalancesOf(account common.Address) ([]byte, error) {
	return threeFRequest.abi.Pack("balancesOf", account)
}

// BalancesOfOutput serves as a container for the return parameters of contract
// method BalancesOf.
type BalancesOfOutput struct {
	PtShares *big.Int
	YtShares *big.Int
}

// UnpackBalancesOf is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6392a51f.
//
// Solidity: function balancesOf(address account) view returns(uint256 ptShares, uint256 ytShares)
func (threeFRequest *ThreeFRequest) UnpackBalancesOf(data []byte) (BalancesOfOutput, error) {
	out, err := threeFRequest.abi.Unpack("balancesOf", data)
	outstruct := new(BalancesOfOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.PtShares = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.YtShares = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackBurnAll is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xdd19a0d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function burnAll(address owner, address receiver) returns(uint256 ptShares, uint256 ytShares, uint256 pAssets, uint256 yAssets)
func (threeFRequest *ThreeFRequest) PackBurnAll(owner common.Address, receiver common.Address) []byte {
	enc, err := threeFRequest.abi.Pack("burnAll", owner, receiver)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackBurnAll is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xdd19a0d8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function burnAll(address owner, address receiver) returns(uint256 ptShares, uint256 ytShares, uint256 pAssets, uint256 yAssets)
func (threeFRequest *ThreeFRequest) TryPackBurnAll(owner common.Address, receiver common.Address) ([]byte, error) {
	return threeFRequest.abi.Pack("burnAll", owner, receiver)
}

// BurnAllOutput serves as a container for the return parameters of contract
// method BurnAll.
type BurnAllOutput struct {
	PtShares *big.Int
	YtShares *big.Int
	PAssets  *big.Int
	YAssets  *big.Int
}

// UnpackBurnAll is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xdd19a0d8.
//
// Solidity: function burnAll(address owner, address receiver) returns(uint256 ptShares, uint256 ytShares, uint256 pAssets, uint256 yAssets)
func (threeFRequest *ThreeFRequest) UnpackBurnAll(data []byte) (BurnAllOutput, error) {
	out, err := threeFRequest.abi.Unpack("burnAll", data)
	outstruct := new(BurnAllOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.PtShares = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.YtShares = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.PAssets = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.YAssets = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackCanWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb51459fe.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function canWithdraw() view returns(bool)
func (threeFRequest *ThreeFRequest) PackCanWithdraw() []byte {
	enc, err := threeFRequest.abi.Pack("canWithdraw")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCanWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb51459fe.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function canWithdraw() view returns(bool)
func (threeFRequest *ThreeFRequest) TryPackCanWithdraw() ([]byte, error) {
	return threeFRequest.abi.Pack("canWithdraw")
}

// UnpackCanWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb51459fe.
//
// Solidity: function canWithdraw() view returns(bool)
func (threeFRequest *ThreeFRequest) UnpackCanWithdraw(data []byte) (bool, error) {
	out, err := threeFRequest.abi.Unpack("canWithdraw", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}
