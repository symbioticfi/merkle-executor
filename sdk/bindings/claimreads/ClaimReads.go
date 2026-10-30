// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package claimreads

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

// ClaimReadsMetaData contains all meta data concerning the ClaimReads contract.
var ClaimReadsMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"getUserNextRequestId\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"withdrawPeriod\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getWithdrawalRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"},{\"name\":\"arg1\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"},{\"name\":\"value1\",\"type\":\"uint256\"},{\"name\":\"value2\",\"type\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"strategy\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"epochNumber\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"userDepositsEpochs\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"},{\"name\":\"arg1\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"userWithdrawalsEpochs\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"},{\"name\":\"arg1\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"canWithdraw\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"balancesOf\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint128\"},{\"name\":\"value1\",\"type\":\"uint128\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxDeposit\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxMint\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxWithdraw\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxRedeem\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimableDepositRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"},{\"name\":\"arg1\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimableRedeemRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"},{\"name\":\"arg1\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimableCancelDepositRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"},{\"name\":\"arg1\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimableCancelRedeemRequest\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"uint256\"},{\"name\":\"arg1\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"depositEscrowBalance\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"},{\"name\":\"arg1\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemEscrowBalance\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"withdrawsRequests\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"instantWithdrawsRequests\",\"inputs\":[{\"name\":\"arg0\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "ClaimReads",
}

// ClaimReads is an auto generated Go binding around an Ethereum contract.
type ClaimReads struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ClaimReads) GetABI() abi.ABI {
	return c.abi
}

// NewClaimReads creates a new instance of ClaimReads.
func NewClaimReads() *ClaimReads {
	parsed, err := ClaimReadsMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ClaimReads{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ClaimReads) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackBalanceOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x70a08231.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function balanceOf(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackBalanceOf(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("balanceOf", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackBalanceOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x70a08231.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function balanceOf(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackBalanceOf(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("balanceOf", arg0)
}

// UnpackBalanceOf is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x70a08231.
//
// Solidity: function balanceOf(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackBalanceOf(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("balanceOf", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackBalancesOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6392a51f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function balancesOf(address arg0) view returns(uint128 value0, uint128 value1)
func (claimReads *ClaimReads) PackBalancesOf(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("balancesOf", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackBalancesOf is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6392a51f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function balancesOf(address arg0) view returns(uint128 value0, uint128 value1)
func (claimReads *ClaimReads) TryPackBalancesOf(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("balancesOf", arg0)
}

// BalancesOfOutput serves as a container for the return parameters of contract
// method BalancesOf.
type BalancesOfOutput struct {
	Value0 *big.Int
	Value1 *big.Int
}

// UnpackBalancesOf is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6392a51f.
//
// Solidity: function balancesOf(address arg0) view returns(uint128 value0, uint128 value1)
func (claimReads *ClaimReads) UnpackBalancesOf(data []byte) (BalancesOfOutput, error) {
	out, err := claimReads.abi.Unpack("balancesOf", data)
	outstruct := new(BalancesOfOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Value0 = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Value1 = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackCanWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb51459fe.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function canWithdraw() view returns(bool value0)
func (claimReads *ClaimReads) PackCanWithdraw() []byte {
	enc, err := claimReads.abi.Pack("canWithdraw")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCanWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb51459fe.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function canWithdraw() view returns(bool value0)
func (claimReads *ClaimReads) TryPackCanWithdraw() ([]byte, error) {
	return claimReads.abi.Pack("canWithdraw")
}

// UnpackCanWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb51459fe.
//
// Solidity: function canWithdraw() view returns(bool value0)
func (claimReads *ClaimReads) UnpackCanWithdraw(data []byte) (bool, error) {
	out, err := claimReads.abi.Unpack("canWithdraw", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackClaimableCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaabb5922.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableCancelDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackClaimableCancelDepositRequest(arg0 *big.Int, arg1 common.Address) []byte {
	enc, err := claimReads.abi.Pack("claimableCancelDepositRequest", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaabb5922.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableCancelDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackClaimableCancelDepositRequest(arg0 *big.Int, arg1 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("claimableCancelDepositRequest", arg0, arg1)
}

// UnpackClaimableCancelDepositRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xaabb5922.
//
// Solidity: function claimableCancelDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackClaimableCancelDepositRequest(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("claimableCancelDepositRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackClaimableCancelRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb04a5e05.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableCancelRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackClaimableCancelRedeemRequest(arg0 *big.Int, arg1 common.Address) []byte {
	enc, err := claimReads.abi.Pack("claimableCancelRedeemRequest", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableCancelRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb04a5e05.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableCancelRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackClaimableCancelRedeemRequest(arg0 *big.Int, arg1 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("claimableCancelRedeemRequest", arg0, arg1)
}

// UnpackClaimableCancelRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb04a5e05.
//
// Solidity: function claimableCancelRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackClaimableCancelRedeemRequest(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("claimableCancelRedeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackClaimableDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x995ea21a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackClaimableDepositRequest(arg0 *big.Int, arg1 common.Address) []byte {
	enc, err := claimReads.abi.Pack("claimableDepositRequest", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x995ea21a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackClaimableDepositRequest(arg0 *big.Int, arg1 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("claimableDepositRequest", arg0, arg1)
}

// UnpackClaimableDepositRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x995ea21a.
//
// Solidity: function claimableDepositRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackClaimableDepositRequest(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("claimableDepositRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackClaimableRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xeaed1d07.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackClaimableRedeemRequest(arg0 *big.Int, arg1 common.Address) []byte {
	enc, err := claimReads.abi.Pack("claimableRedeemRequest", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xeaed1d07.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackClaimableRedeemRequest(arg0 *big.Int, arg1 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("claimableRedeemRequest", arg0, arg1)
}

// UnpackClaimableRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xeaed1d07.
//
// Solidity: function claimableRedeemRequest(uint256 arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackClaimableRedeemRequest(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("claimableRedeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackDepositEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x47805e71.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function depositEscrowBalance(address arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackDepositEscrowBalance(arg0 common.Address, arg1 common.Address) []byte {
	enc, err := claimReads.abi.Pack("depositEscrowBalance", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDepositEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x47805e71.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function depositEscrowBalance(address arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackDepositEscrowBalance(arg0 common.Address, arg1 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("depositEscrowBalance", arg0, arg1)
}

// UnpackDepositEscrowBalance is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x47805e71.
//
// Solidity: function depositEscrowBalance(address arg0, address arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackDepositEscrowBalance(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("depositEscrowBalance", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochNumber is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf4145a83.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochNumber() view returns(uint256 value0)
func (claimReads *ClaimReads) PackEpochNumber() []byte {
	enc, err := claimReads.abi.Pack("epochNumber")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochNumber is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf4145a83.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochNumber() view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackEpochNumber() ([]byte, error) {
	return claimReads.abi.Pack("epochNumber")
}

// UnpackEpochNumber is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf4145a83.
//
// Solidity: function epochNumber() view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackEpochNumber(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("epochNumber", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackGetUserNextRequestId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8d29d30e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getUserNextRequestId(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackGetUserNextRequestId(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("getUserNextRequestId", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetUserNextRequestId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8d29d30e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getUserNextRequestId(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackGetUserNextRequestId(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("getUserNextRequestId", arg0)
}

// UnpackGetUserNextRequestId is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8d29d30e.
//
// Solidity: function getUserNextRequestId(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackGetUserNextRequestId(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("getUserNextRequestId", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackGetWithdrawalRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2847e64.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getWithdrawalRequest(address arg0, uint256 arg1) view returns(uint256 value0, uint256 value1, bool value2)
func (claimReads *ClaimReads) PackGetWithdrawalRequest(arg0 common.Address, arg1 *big.Int) []byte {
	enc, err := claimReads.abi.Pack("getWithdrawalRequest", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetWithdrawalRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2847e64.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getWithdrawalRequest(address arg0, uint256 arg1) view returns(uint256 value0, uint256 value1, bool value2)
func (claimReads *ClaimReads) TryPackGetWithdrawalRequest(arg0 common.Address, arg1 *big.Int) ([]byte, error) {
	return claimReads.abi.Pack("getWithdrawalRequest", arg0, arg1)
}

// GetWithdrawalRequestOutput serves as a container for the return parameters of contract
// method GetWithdrawalRequest.
type GetWithdrawalRequestOutput struct {
	Value0 *big.Int
	Value1 *big.Int
	Value2 bool
}

// UnpackGetWithdrawalRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf2847e64.
//
// Solidity: function getWithdrawalRequest(address arg0, uint256 arg1) view returns(uint256 value0, uint256 value1, bool value2)
func (claimReads *ClaimReads) UnpackGetWithdrawalRequest(data []byte) (GetWithdrawalRequestOutput, error) {
	out, err := claimReads.abi.Unpack("getWithdrawalRequest", data)
	outstruct := new(GetWithdrawalRequestOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Value0 = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Value1 = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.Value2 = *abi.ConvertType(out[2], new(bool)).(*bool)
	return *outstruct, nil
}

// PackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function instantWithdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackInstantWithdrawsRequests(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("instantWithdrawsRequests", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function instantWithdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackInstantWithdrawsRequests(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("instantWithdrawsRequests", arg0)
}

// UnpackInstantWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9bf5bbe7.
//
// Solidity: function instantWithdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackInstantWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("instantWithdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackIsRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9adfc20.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isRequest(address arg0) view returns(bool value0)
func (claimReads *ClaimReads) PackIsRequest(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("isRequest", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe9adfc20.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isRequest(address arg0) view returns(bool value0)
func (claimReads *ClaimReads) TryPackIsRequest(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("isRequest", arg0)
}

// UnpackIsRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe9adfc20.
//
// Solidity: function isRequest(address arg0) view returns(bool value0)
func (claimReads *ClaimReads) UnpackIsRequest(data []byte) (bool, error) {
	out, err := claimReads.abi.Unpack("isRequest", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackMaxDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x402d267d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxDeposit(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackMaxDeposit(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("maxDeposit", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x402d267d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxDeposit(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackMaxDeposit(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("maxDeposit", arg0)
}

// UnpackMaxDeposit is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x402d267d.
//
// Solidity: function maxDeposit(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackMaxDeposit(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("maxDeposit", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackMaxMint is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc63d75b6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxMint(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackMaxMint(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("maxMint", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxMint is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc63d75b6.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxMint(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackMaxMint(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("maxMint", arg0)
}

// UnpackMaxMint is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc63d75b6.
//
// Solidity: function maxMint(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackMaxMint(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("maxMint", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackMaxRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd905777e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxRedeem(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackMaxRedeem(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("maxRedeem", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd905777e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxRedeem(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackMaxRedeem(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("maxRedeem", arg0)
}

// UnpackMaxRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd905777e.
//
// Solidity: function maxRedeem(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackMaxRedeem(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("maxRedeem", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackMaxWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xce96cb77.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxWithdraw(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackMaxWithdraw(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("maxWithdraw", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xce96cb77.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxWithdraw(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackMaxWithdraw(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("maxWithdraw", arg0)
}

// UnpackMaxWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xce96cb77.
//
// Solidity: function maxWithdraw(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackMaxWithdraw(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("maxWithdraw", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8c15336b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemEscrowBalance(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackRedeemEscrowBalance(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("redeemEscrowBalance", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8c15336b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemEscrowBalance(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackRedeemEscrowBalance(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("redeemEscrowBalance", arg0)
}

// UnpackRedeemEscrowBalance is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8c15336b.
//
// Solidity: function redeemEscrowBalance(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackRedeemEscrowBalance(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("redeemEscrowBalance", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackStrategy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa8c62e76.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function strategy() view returns(address value0)
func (claimReads *ClaimReads) PackStrategy() []byte {
	enc, err := claimReads.abi.Pack("strategy")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackStrategy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa8c62e76.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function strategy() view returns(address value0)
func (claimReads *ClaimReads) TryPackStrategy() ([]byte, error) {
	return claimReads.abi.Pack("strategy")
}

// UnpackStrategy is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa8c62e76.
//
// Solidity: function strategy() view returns(address value0)
func (claimReads *ClaimReads) UnpackStrategy(data []byte) (common.Address, error) {
	out, err := claimReads.abi.Unpack("strategy", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackUserDepositsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x670a9a6f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function userDepositsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackUserDepositsEpochs(arg0 common.Address, arg1 *big.Int) []byte {
	enc, err := claimReads.abi.Pack("userDepositsEpochs", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUserDepositsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x670a9a6f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function userDepositsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackUserDepositsEpochs(arg0 common.Address, arg1 *big.Int) ([]byte, error) {
	return claimReads.abi.Pack("userDepositsEpochs", arg0, arg1)
}

// UnpackUserDepositsEpochs is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x670a9a6f.
//
// Solidity: function userDepositsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackUserDepositsEpochs(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("userDepositsEpochs", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackUserWithdrawalsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b223e52.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function userWithdrawalsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) PackUserWithdrawalsEpochs(arg0 common.Address, arg1 *big.Int) []byte {
	enc, err := claimReads.abi.Pack("userWithdrawalsEpochs", arg0, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUserWithdrawalsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b223e52.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function userWithdrawalsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackUserWithdrawalsEpochs(arg0 common.Address, arg1 *big.Int) ([]byte, error) {
	return claimReads.abi.Pack("userWithdrawalsEpochs", arg0, arg1)
}

// UnpackUserWithdrawalsEpochs is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5b223e52.
//
// Solidity: function userWithdrawalsEpochs(address arg0, uint256 arg1) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackUserWithdrawalsEpochs(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("userWithdrawalsEpochs", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackWithdrawPeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x12eb4f9a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function withdrawPeriod() view returns(uint256 value0)
func (claimReads *ClaimReads) PackWithdrawPeriod() []byte {
	enc, err := claimReads.abi.Pack("withdrawPeriod")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdrawPeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x12eb4f9a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdrawPeriod() view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackWithdrawPeriod() ([]byte, error) {
	return claimReads.abi.Pack("withdrawPeriod")
}

// UnpackWithdrawPeriod is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x12eb4f9a.
//
// Solidity: function withdrawPeriod() view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackWithdrawPeriod(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("withdrawPeriod", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xacfd7dce.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function withdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) PackWithdrawsRequests(arg0 common.Address) []byte {
	enc, err := claimReads.abi.Pack("withdrawsRequests", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xacfd7dce.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) TryPackWithdrawsRequests(arg0 common.Address) ([]byte, error) {
	return claimReads.abi.Pack("withdrawsRequests", arg0)
}

// UnpackWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xacfd7dce.
//
// Solidity: function withdrawsRequests(address arg0) view returns(uint256 value0)
func (claimReads *ClaimReads) UnpackWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := claimReads.abi.Unpack("withdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
