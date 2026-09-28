// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package noonwithdrawalhandler

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

// WithdrawalHandlerWithdrawalRequest is an auto generated low-level Go binding around an user-defined struct.
type WithdrawalHandlerWithdrawalRequest struct {
	Amount    *big.Int
	Timestamp *big.Int
	Claimed   bool
}

// NoonWithdrawalHandlerMetaData contains all meta data concerning the NoonWithdrawalHandler contract.
var NoonWithdrawalHandlerMetaData = bind.MetaData{
	ABI: "[{\"name\":\"STAKING_VAULT_ROLE\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"name\":\"claimWithdrawal\",\"type\":\"function\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"name\":\"getUserNextRequestId\",\"type\":\"function\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"name\":\"getWithdrawalRequest\",\"type\":\"function\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"tuple\",\"components\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"claimed\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"internalType\":\"structWithdrawalHandler.WithdrawalRequest\"}],\"stateMutability\":\"view\"},{\"name\":\"hasRole\",\"type\":\"function\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"name\":\"usn\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"name\":\"withdrawPeriod\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "NoonWithdrawalHandler",
}

// NoonWithdrawalHandler is an auto generated Go binding around an Ethereum contract.
type NoonWithdrawalHandler struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *NoonWithdrawalHandler) GetABI() abi.ABI {
	return c.abi
}

// NewNoonWithdrawalHandler creates a new instance of NoonWithdrawalHandler.
func NewNoonWithdrawalHandler() *NoonWithdrawalHandler {
	parsed, err := NoonWithdrawalHandlerMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &NoonWithdrawalHandler{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *NoonWithdrawalHandler) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackSTAKINGVAULTROLE is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb0951693.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function STAKING_VAULT_ROLE() view returns(bytes32)
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackSTAKINGVAULTROLE() []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("STAKING_VAULT_ROLE")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSTAKINGVAULTROLE is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb0951693.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function STAKING_VAULT_ROLE() view returns(bytes32)
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackSTAKINGVAULTROLE() ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("STAKING_VAULT_ROLE")
}

// UnpackSTAKINGVAULTROLE is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb0951693.
//
// Solidity: function STAKING_VAULT_ROLE() view returns(bytes32)
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackSTAKINGVAULTROLE(data []byte) ([32]byte, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("STAKING_VAULT_ROLE", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackClaimWithdrawal is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf8444436.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawal(uint256 requestId) returns()
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackClaimWithdrawal(requestId *big.Int) []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("claimWithdrawal", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimWithdrawal is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf8444436.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimWithdrawal(uint256 requestId) returns()
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackClaimWithdrawal(requestId *big.Int) ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("claimWithdrawal", requestId)
}

// PackGetUserNextRequestId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8d29d30e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getUserNextRequestId(address user) view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackGetUserNextRequestId(user common.Address) []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("getUserNextRequestId", user)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetUserNextRequestId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8d29d30e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getUserNextRequestId(address user) view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackGetUserNextRequestId(user common.Address) ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("getUserNextRequestId", user)
}

// UnpackGetUserNextRequestId is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8d29d30e.
//
// Solidity: function getUserNextRequestId(address user) view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackGetUserNextRequestId(data []byte) (*big.Int, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("getUserNextRequestId", data)
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
// Solidity: function getWithdrawalRequest(address user, uint256 requestId) view returns((uint256,uint256,bool))
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackGetWithdrawalRequest(user common.Address, requestId *big.Int) []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("getWithdrawalRequest", user, requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetWithdrawalRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2847e64.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getWithdrawalRequest(address user, uint256 requestId) view returns((uint256,uint256,bool))
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackGetWithdrawalRequest(user common.Address, requestId *big.Int) ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("getWithdrawalRequest", user, requestId)
}

// UnpackGetWithdrawalRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf2847e64.
//
// Solidity: function getWithdrawalRequest(address user, uint256 requestId) view returns((uint256,uint256,bool))
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackGetWithdrawalRequest(data []byte) (WithdrawalHandlerWithdrawalRequest, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("getWithdrawalRequest", data)
	if err != nil {
		return *new(WithdrawalHandlerWithdrawalRequest), err
	}
	out0 := *abi.ConvertType(out[0], new(WithdrawalHandlerWithdrawalRequest)).(*WithdrawalHandlerWithdrawalRequest)
	return out0, nil
}

// PackHasRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x91d14854.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackHasRole(role [32]byte, account common.Address) []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("hasRole", role, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackHasRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x91d14854.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackHasRole(role [32]byte, account common.Address) ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("hasRole", role, account)
}

// UnpackHasRole is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackHasRole(data []byte) (bool, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("hasRole", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackUsn is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x3d511b4f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function usn() view returns(address)
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackUsn() []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("usn")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUsn is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x3d511b4f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function usn() view returns(address)
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackUsn() ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("usn")
}

// UnpackUsn is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x3d511b4f.
//
// Solidity: function usn() view returns(address)
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackUsn(data []byte) (common.Address, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("usn", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackWithdrawPeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x12eb4f9a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function withdrawPeriod() view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) PackWithdrawPeriod() []byte {
	enc, err := noonWithdrawalHandler.abi.Pack("withdrawPeriod")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdrawPeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x12eb4f9a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdrawPeriod() view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) TryPackWithdrawPeriod() ([]byte, error) {
	return noonWithdrawalHandler.abi.Pack("withdrawPeriod")
}

// UnpackWithdrawPeriod is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x12eb4f9a.
//
// Solidity: function withdrawPeriod() view returns(uint256)
func (noonWithdrawalHandler *NoonWithdrawalHandler) UnpackWithdrawPeriod(data []byte) (*big.Int, error) {
	out, err := noonWithdrawalHandler.abi.Unpack("withdrawPeriod", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
