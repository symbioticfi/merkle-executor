// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package nativeclaims

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

// Struct0 is an auto generated low-level Go binding around an user-defined struct.
type Struct0 struct {
	Amount    *big.Int
	Timestamp *big.Int
	Claimed   bool
}

// NativeClaimsMetaData contains all meta data concerning the NativeClaims contract.
var NativeClaimsMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"getUserNextRequestId\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"withdrawPeriod\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"getWithdrawalRequest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"request\",\"type\":\"tuple\",\"components\":[{\"name\":\"amount\",\"type\":\"uint256\"},{\"name\":\"timestamp\",\"type\":\"uint256\"},{\"name\":\"claimed\",\"type\":\"bool\"}]}]},{\"type\":\"function\",\"name\":\"claimWithdrawal\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"strategy\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"address\"}]},{\"type\":\"function\",\"name\":\"epochNumber\",\"stateMutability\":\"view\",\"inputs\":[],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"userDepositsEpochs\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"epoch\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"userWithdrawalsEpochs\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"epoch\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"epochPrice\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"epochPendingDeposits\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"epochPrefundedDeposits\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"epochWithdrawPrice\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"epochPendingClaims\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimDepositRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"claimWithdrawRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[],\"outputs\":[]},{\"type\":\"function\",\"name\":\"claimWithdrawRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"claimInstantWithdrawRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[],\"outputs\":[]},{\"type\":\"function\",\"name\":\"withdrawsRequests\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"instantWithdrawsRequests\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"apr0Users\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"},{\"name\":\"value1\",\"type\":\"uint256\"},{\"name\":\"value2\",\"type\":\"uint256\"},{\"name\":\"value3\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimDepositEscrow\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"depositEscrowBalance\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimRedeemEscrow\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[]},{\"type\":\"function\",\"name\":\"redeemEscrowBalance\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"deposit\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"mint\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"withdraw\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"redeem\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"maxDeposit\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"maxMint\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"maxWithdraw\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"maxRedeem\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimableDepositRequest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimableRedeemRequest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimCancelDepositRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimCancelRedeemRequest\",\"stateMutability\":\"nonpayable\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimableCancelDepositRequest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]},{\"type\":\"function\",\"name\":\"claimableCancelRedeemRequest\",\"stateMutability\":\"view\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\"},{\"name\":\"account\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"value0\",\"type\":\"uint256\"}]}]",
	ID:  "NativeClaims",
}

// NativeClaims is an auto generated Go binding around an Ethereum contract.
type NativeClaims struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *NativeClaims) GetABI() abi.ABI {
	return c.abi
}

// NewNativeClaims creates a new instance of NativeClaims.
func NewNativeClaims() *NativeClaims {
	parsed, err := NativeClaimsMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &NativeClaims{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *NativeClaims) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackApr0Users is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x97e3f633.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function apr0Users(address account) view returns(uint256 value0, uint256 value1, uint256 value2, uint256 value3)
func (nativeClaims *NativeClaims) PackApr0Users(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("apr0Users", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackApr0Users is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x97e3f633.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function apr0Users(address account) view returns(uint256 value0, uint256 value1, uint256 value2, uint256 value3)
func (nativeClaims *NativeClaims) TryPackApr0Users(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("apr0Users", account)
}

// Apr0UsersOutput serves as a container for the return parameters of contract
// method Apr0Users.
type Apr0UsersOutput struct {
	Value0 *big.Int
	Value1 *big.Int
	Value2 *big.Int
	Value3 *big.Int
}

// UnpackApr0Users is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x97e3f633.
//
// Solidity: function apr0Users(address account) view returns(uint256 value0, uint256 value1, uint256 value2, uint256 value3)
func (nativeClaims *NativeClaims) UnpackApr0Users(data []byte) (Apr0UsersOutput, error) {
	out, err := nativeClaims.abi.Unpack("apr0Users", data)
	outstruct := new(Apr0UsersOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Value0 = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Value1 = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.Value2 = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.Value3 = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackClaimCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69d77a44.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimCancelDepositRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimCancelDepositRequest(requestId *big.Int, account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimCancelDepositRequest", requestId, account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69d77a44.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimCancelDepositRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimCancelDepositRequest(requestId *big.Int, account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimCancelDepositRequest", requestId, account, account0)
}

// UnpackClaimCancelDepositRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x69d77a44.
//
// Solidity: function claimCancelDepositRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimCancelDepositRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimCancelDepositRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackClaimCancelRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x00a06d19.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimCancelRedeemRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimCancelRedeemRequest(requestId *big.Int, account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimCancelRedeemRequest", requestId, account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimCancelRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x00a06d19.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimCancelRedeemRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimCancelRedeemRequest(requestId *big.Int, account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimCancelRedeemRequest", requestId, account, account0)
}

// UnpackClaimCancelRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x00a06d19.
//
// Solidity: function claimCancelRedeemRequest(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimCancelRedeemRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimCancelRedeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackClaimDepositEscrow is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x32ad8e21.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimDepositEscrow(address account, address account) returns()
func (nativeClaims *NativeClaims) PackClaimDepositEscrow(account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimDepositEscrow", account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimDepositEscrow is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x32ad8e21.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimDepositEscrow(address account, address account) returns()
func (nativeClaims *NativeClaims) TryPackClaimDepositEscrow(account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimDepositEscrow", account, account0)
}

// PackClaimDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb975b387.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimDepositRequest(uint256 requestId) returns()
func (nativeClaims *NativeClaims) PackClaimDepositRequest(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("claimDepositRequest", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb975b387.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimDepositRequest(uint256 requestId) returns()
func (nativeClaims *NativeClaims) TryPackClaimDepositRequest(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("claimDepositRequest", requestId)
}

// PackClaimInstantWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x991052b7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimInstantWithdrawRequest() returns()
func (nativeClaims *NativeClaims) PackClaimInstantWithdrawRequest() []byte {
	enc, err := nativeClaims.abi.Pack("claimInstantWithdrawRequest")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimInstantWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x991052b7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimInstantWithdrawRequest() returns()
func (nativeClaims *NativeClaims) TryPackClaimInstantWithdrawRequest() ([]byte, error) {
	return nativeClaims.abi.Pack("claimInstantWithdrawRequest")
}

// PackClaimRedeemEscrow is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x67f5c96b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimRedeemEscrow(address account) returns()
func (nativeClaims *NativeClaims) PackClaimRedeemEscrow(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimRedeemEscrow", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimRedeemEscrow is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x67f5c96b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimRedeemEscrow(address account) returns()
func (nativeClaims *NativeClaims) TryPackClaimRedeemEscrow(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimRedeemEscrow", account)
}

// PackClaimWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x33986ffa.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawRequest() returns()
func (nativeClaims *NativeClaims) PackClaimWithdrawRequest() []byte {
	enc, err := nativeClaims.abi.Pack("claimWithdrawRequest")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x33986ffa.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimWithdrawRequest() returns()
func (nativeClaims *NativeClaims) TryPackClaimWithdrawRequest() ([]byte, error) {
	return nativeClaims.abi.Pack("claimWithdrawRequest")
}

// PackClaimWithdrawRequest0 is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd2233918.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawRequest(uint256 requestId) returns()
func (nativeClaims *NativeClaims) PackClaimWithdrawRequest0(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("claimWithdrawRequest0", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimWithdrawRequest0 is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd2233918.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimWithdrawRequest(uint256 requestId) returns()
func (nativeClaims *NativeClaims) TryPackClaimWithdrawRequest0(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("claimWithdrawRequest0", requestId)
}

// PackClaimWithdrawal is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf8444436.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawal(uint256 requestId) returns()
func (nativeClaims *NativeClaims) PackClaimWithdrawal(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("claimWithdrawal", requestId)
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
func (nativeClaims *NativeClaims) TryPackClaimWithdrawal(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("claimWithdrawal", requestId)
}

// PackClaimableCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaabb5922.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableCancelDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimableCancelDepositRequest(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimableCancelDepositRequest", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableCancelDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaabb5922.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableCancelDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimableCancelDepositRequest(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimableCancelDepositRequest", requestId, account)
}

// UnpackClaimableCancelDepositRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xaabb5922.
//
// Solidity: function claimableCancelDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimableCancelDepositRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimableCancelDepositRequest", data)
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
// Solidity: function claimableCancelRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimableCancelRedeemRequest(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimableCancelRedeemRequest", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableCancelRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb04a5e05.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableCancelRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimableCancelRedeemRequest(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimableCancelRedeemRequest", requestId, account)
}

// UnpackClaimableCancelRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb04a5e05.
//
// Solidity: function claimableCancelRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimableCancelRedeemRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimableCancelRedeemRequest", data)
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
// Solidity: function claimableDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimableDepositRequest(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimableDepositRequest", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableDepositRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x995ea21a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimableDepositRequest(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimableDepositRequest", requestId, account)
}

// UnpackClaimableDepositRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x995ea21a.
//
// Solidity: function claimableDepositRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimableDepositRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimableDepositRequest", data)
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
// Solidity: function claimableRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackClaimableRedeemRequest(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("claimableRedeemRequest", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xeaed1d07.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackClaimableRedeemRequest(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("claimableRedeemRequest", requestId, account)
}

// UnpackClaimableRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xeaed1d07.
//
// Solidity: function claimableRedeemRequest(uint256 requestId, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackClaimableRedeemRequest(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("claimableRedeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e553f65.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function deposit(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackDeposit(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("deposit", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e553f65.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function deposit(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackDeposit(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("deposit", requestId, account)
}

// UnpackDeposit is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6e553f65.
//
// Solidity: function deposit(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackDeposit(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("deposit", data)
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
// Solidity: function depositEscrowBalance(address account, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackDepositEscrowBalance(account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("depositEscrowBalance", account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDepositEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x47805e71.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function depositEscrowBalance(address account, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackDepositEscrowBalance(account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("depositEscrowBalance", account, account0)
}

// UnpackDepositEscrowBalance is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x47805e71.
//
// Solidity: function depositEscrowBalance(address account, address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackDepositEscrowBalance(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("depositEscrowBalance", data)
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
func (nativeClaims *NativeClaims) PackEpochNumber() []byte {
	enc, err := nativeClaims.abi.Pack("epochNumber")
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
func (nativeClaims *NativeClaims) TryPackEpochNumber() ([]byte, error) {
	return nativeClaims.abi.Pack("epochNumber")
}

// UnpackEpochNumber is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf4145a83.
//
// Solidity: function epochNumber() view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochNumber(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochNumber", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochPendingClaims is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9453dc6c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochPendingClaims(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackEpochPendingClaims(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("epochPendingClaims", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochPendingClaims is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9453dc6c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochPendingClaims(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackEpochPendingClaims(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("epochPendingClaims", requestId)
}

// UnpackEpochPendingClaims is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9453dc6c.
//
// Solidity: function epochPendingClaims(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochPendingClaims(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochPendingClaims", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochPendingDeposits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8478de52.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochPendingDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackEpochPendingDeposits(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("epochPendingDeposits", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochPendingDeposits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8478de52.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochPendingDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackEpochPendingDeposits(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("epochPendingDeposits", requestId)
}

// UnpackEpochPendingDeposits is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8478de52.
//
// Solidity: function epochPendingDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochPendingDeposits(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochPendingDeposits", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochPrefundedDeposits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e3b1547.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochPrefundedDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackEpochPrefundedDeposits(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("epochPrefundedDeposits", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochPrefundedDeposits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e3b1547.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochPrefundedDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackEpochPrefundedDeposits(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("epochPrefundedDeposits", requestId)
}

// UnpackEpochPrefundedDeposits is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6e3b1547.
//
// Solidity: function epochPrefundedDeposits(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochPrefundedDeposits(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochPrefundedDeposits", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x81990b6a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackEpochPrice(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("epochPrice", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x81990b6a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackEpochPrice(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("epochPrice", requestId)
}

// UnpackEpochPrice is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x81990b6a.
//
// Solidity: function epochPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochPrice(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochPrice", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackEpochWithdrawPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07837637.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochWithdrawPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackEpochWithdrawPrice(requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("epochWithdrawPrice", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochWithdrawPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07837637.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochWithdrawPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackEpochWithdrawPrice(requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("epochWithdrawPrice", requestId)
}

// UnpackEpochWithdrawPrice is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x07837637.
//
// Solidity: function epochWithdrawPrice(uint256 requestId) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackEpochWithdrawPrice(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("epochWithdrawPrice", data)
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
// Solidity: function getUserNextRequestId(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackGetUserNextRequestId(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("getUserNextRequestId", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetUserNextRequestId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8d29d30e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getUserNextRequestId(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackGetUserNextRequestId(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("getUserNextRequestId", account)
}

// UnpackGetUserNextRequestId is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8d29d30e.
//
// Solidity: function getUserNextRequestId(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackGetUserNextRequestId(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("getUserNextRequestId", data)
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
// Solidity: function getWithdrawalRequest(address account, uint256 requestId) view returns((uint256,uint256,bool) request)
func (nativeClaims *NativeClaims) PackGetWithdrawalRequest(account common.Address, requestId *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("getWithdrawalRequest", account, requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetWithdrawalRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2847e64.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getWithdrawalRequest(address account, uint256 requestId) view returns((uint256,uint256,bool) request)
func (nativeClaims *NativeClaims) TryPackGetWithdrawalRequest(account common.Address, requestId *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("getWithdrawalRequest", account, requestId)
}

// UnpackGetWithdrawalRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf2847e64.
//
// Solidity: function getWithdrawalRequest(address account, uint256 requestId) view returns((uint256,uint256,bool) request)
func (nativeClaims *NativeClaims) UnpackGetWithdrawalRequest(data []byte) (Struct0, error) {
	out, err := nativeClaims.abi.Unpack("getWithdrawalRequest", data)
	if err != nil {
		return *new(Struct0), err
	}
	out0 := *abi.ConvertType(out[0], new(Struct0)).(*Struct0)
	return out0, nil
}

// PackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackInstantWithdrawsRequests(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("instantWithdrawsRequests", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackInstantWithdrawsRequests(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("instantWithdrawsRequests", account)
}

// UnpackInstantWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9bf5bbe7.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackInstantWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("instantWithdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackMaxDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x402d267d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxDeposit(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackMaxDeposit(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("maxDeposit", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxDeposit is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x402d267d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxDeposit(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackMaxDeposit(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("maxDeposit", account)
}

// UnpackMaxDeposit is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x402d267d.
//
// Solidity: function maxDeposit(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackMaxDeposit(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("maxDeposit", data)
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
// Solidity: function maxMint(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackMaxMint(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("maxMint", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxMint is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc63d75b6.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxMint(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackMaxMint(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("maxMint", account)
}

// UnpackMaxMint is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc63d75b6.
//
// Solidity: function maxMint(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackMaxMint(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("maxMint", data)
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
// Solidity: function maxRedeem(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackMaxRedeem(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("maxRedeem", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd905777e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxRedeem(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackMaxRedeem(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("maxRedeem", account)
}

// UnpackMaxRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd905777e.
//
// Solidity: function maxRedeem(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackMaxRedeem(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("maxRedeem", data)
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
// Solidity: function maxWithdraw(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackMaxWithdraw(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("maxWithdraw", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xce96cb77.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxWithdraw(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackMaxWithdraw(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("maxWithdraw", account)
}

// UnpackMaxWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xce96cb77.
//
// Solidity: function maxWithdraw(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackMaxWithdraw(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("maxWithdraw", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackMint is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x94bf804d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function mint(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackMint(requestId *big.Int, account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("mint", requestId, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMint is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x94bf804d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function mint(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackMint(requestId *big.Int, account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("mint", requestId, account)
}

// UnpackMint is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x94bf804d.
//
// Solidity: function mint(uint256 requestId, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackMint(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("mint", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xba087652.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeem(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackRedeem(requestId *big.Int, account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("redeem", requestId, account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xba087652.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeem(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackRedeem(requestId *big.Int, account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("redeem", requestId, account, account0)
}

// UnpackRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xba087652.
//
// Solidity: function redeem(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackRedeem(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("redeem", data)
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
// Solidity: function redeemEscrowBalance(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackRedeemEscrowBalance(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("redeemEscrowBalance", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemEscrowBalance is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8c15336b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemEscrowBalance(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackRedeemEscrowBalance(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("redeemEscrowBalance", account)
}

// UnpackRedeemEscrowBalance is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8c15336b.
//
// Solidity: function redeemEscrowBalance(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackRedeemEscrowBalance(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("redeemEscrowBalance", data)
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
func (nativeClaims *NativeClaims) PackStrategy() []byte {
	enc, err := nativeClaims.abi.Pack("strategy")
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
func (nativeClaims *NativeClaims) TryPackStrategy() ([]byte, error) {
	return nativeClaims.abi.Pack("strategy")
}

// UnpackStrategy is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa8c62e76.
//
// Solidity: function strategy() view returns(address value0)
func (nativeClaims *NativeClaims) UnpackStrategy(data []byte) (common.Address, error) {
	out, err := nativeClaims.abi.Unpack("strategy", data)
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
// Solidity: function userDepositsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackUserDepositsEpochs(account common.Address, epoch *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("userDepositsEpochs", account, epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUserDepositsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x670a9a6f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function userDepositsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackUserDepositsEpochs(account common.Address, epoch *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("userDepositsEpochs", account, epoch)
}

// UnpackUserDepositsEpochs is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x670a9a6f.
//
// Solidity: function userDepositsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackUserDepositsEpochs(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("userDepositsEpochs", data)
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
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackUserWithdrawalsEpochs(account common.Address, epoch *big.Int) []byte {
	enc, err := nativeClaims.abi.Pack("userWithdrawalsEpochs", account, epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUserWithdrawalsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b223e52.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackUserWithdrawalsEpochs(account common.Address, epoch *big.Int) ([]byte, error) {
	return nativeClaims.abi.Pack("userWithdrawalsEpochs", account, epoch)
}

// UnpackUserWithdrawalsEpochs is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5b223e52.
//
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackUserWithdrawalsEpochs(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("userWithdrawalsEpochs", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb460af94.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function withdraw(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) PackWithdraw(requestId *big.Int, account common.Address, account0 common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("withdraw", requestId, account, account0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xb460af94.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdraw(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackWithdraw(requestId *big.Int, account common.Address, account0 common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("withdraw", requestId, account, account0)
}

// UnpackWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xb460af94.
//
// Solidity: function withdraw(uint256 requestId, address account, address account) returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackWithdraw(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("withdraw", data)
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
func (nativeClaims *NativeClaims) PackWithdrawPeriod() []byte {
	enc, err := nativeClaims.abi.Pack("withdrawPeriod")
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
func (nativeClaims *NativeClaims) TryPackWithdrawPeriod() ([]byte, error) {
	return nativeClaims.abi.Pack("withdrawPeriod")
}

// UnpackWithdrawPeriod is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x12eb4f9a.
//
// Solidity: function withdrawPeriod() view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackWithdrawPeriod(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("withdrawPeriod", data)
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
// Solidity: function withdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) PackWithdrawsRequests(account common.Address) []byte {
	enc, err := nativeClaims.abi.Pack("withdrawsRequests", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xacfd7dce.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) TryPackWithdrawsRequests(account common.Address) ([]byte, error) {
	return nativeClaims.abi.Pack("withdrawsRequests", account)
}

// UnpackWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xacfd7dce.
//
// Solidity: function withdrawsRequests(address account) view returns(uint256 value0)
func (nativeClaims *NativeClaims) UnpackWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := nativeClaims.abi.Unpack("withdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
