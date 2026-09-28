// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package midasredemption

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

// MidasRedemptionMetaData contains all meta data concerning the MidasRedemption contract.
var MidasRedemptionMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"mToken\",\"inputs\":[],\"outputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"mTokenDataFeed\",\"inputs\":[],\"outputs\":[{\"name\":\"dataFeed\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemRequest\",\"inputs\":[{\"name\":\"tokenOut\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountMTokenIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"redeemRequests\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenOut\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"amountMToken\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"mTokenRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tokenOutRate\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tokensConfig\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"dataFeed\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"fee\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"allowance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"stable\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"accessControl\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"fnPaused\",\"inputs\":[{\"name\":\"selector\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"greenlistEnabled\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"greenlistedRole\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"minAmount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"paused\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"}]",
	ID:  "MidasRedemption",
}

// MidasRedemption is an auto generated Go binding around an Ethereum contract.
type MidasRedemption struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *MidasRedemption) GetABI() abi.ABI {
	return c.abi
}

// NewMidasRedemption creates a new instance of MidasRedemption.
func NewMidasRedemption() *MidasRedemption {
	parsed, err := MidasRedemptionMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &MidasRedemption{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *MidasRedemption) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAccessControl is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x13007d55.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function accessControl() view returns(address)
func (midasRedemption *MidasRedemption) PackAccessControl() []byte {
	enc, err := midasRedemption.abi.Pack("accessControl")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAccessControl is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x13007d55.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function accessControl() view returns(address)
func (midasRedemption *MidasRedemption) TryPackAccessControl() ([]byte, error) {
	return midasRedemption.abi.Pack("accessControl")
}

// UnpackAccessControl is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x13007d55.
//
// Solidity: function accessControl() view returns(address)
func (midasRedemption *MidasRedemption) UnpackAccessControl(data []byte) (common.Address, error) {
	out, err := midasRedemption.abi.Unpack("accessControl", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackFnPaused is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0b5a57bd.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function fnPaused(bytes4 selector) view returns(bool)
func (midasRedemption *MidasRedemption) PackFnPaused(selector [4]byte) []byte {
	enc, err := midasRedemption.abi.Pack("fnPaused", selector)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackFnPaused is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0b5a57bd.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function fnPaused(bytes4 selector) view returns(bool)
func (midasRedemption *MidasRedemption) TryPackFnPaused(selector [4]byte) ([]byte, error) {
	return midasRedemption.abi.Pack("fnPaused", selector)
}

// UnpackFnPaused is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0b5a57bd.
//
// Solidity: function fnPaused(bytes4 selector) view returns(bool)
func (midasRedemption *MidasRedemption) UnpackFnPaused(data []byte) (bool, error) {
	out, err := midasRedemption.abi.Unpack("fnPaused", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackGreenlistEnabled is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x105ed2b2.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function greenlistEnabled() view returns(bool)
func (midasRedemption *MidasRedemption) PackGreenlistEnabled() []byte {
	enc, err := midasRedemption.abi.Pack("greenlistEnabled")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGreenlistEnabled is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x105ed2b2.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function greenlistEnabled() view returns(bool)
func (midasRedemption *MidasRedemption) TryPackGreenlistEnabled() ([]byte, error) {
	return midasRedemption.abi.Pack("greenlistEnabled")
}

// UnpackGreenlistEnabled is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x105ed2b2.
//
// Solidity: function greenlistEnabled() view returns(bool)
func (midasRedemption *MidasRedemption) UnpackGreenlistEnabled(data []byte) (bool, error) {
	out, err := midasRedemption.abi.Unpack("greenlistEnabled", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackGreenlistedRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbbae4086.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function greenlistedRole() view returns(bytes32)
func (midasRedemption *MidasRedemption) PackGreenlistedRole() []byte {
	enc, err := midasRedemption.abi.Pack("greenlistedRole")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGreenlistedRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbbae4086.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function greenlistedRole() view returns(bytes32)
func (midasRedemption *MidasRedemption) TryPackGreenlistedRole() ([]byte, error) {
	return midasRedemption.abi.Pack("greenlistedRole")
}

// UnpackGreenlistedRole is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xbbae4086.
//
// Solidity: function greenlistedRole() view returns(bytes32)
func (midasRedemption *MidasRedemption) UnpackGreenlistedRole(data []byte) ([32]byte, error) {
	out, err := midasRedemption.abi.Unpack("greenlistedRole", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackMToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc3b6f939.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function mToken() view returns(address token)
func (midasRedemption *MidasRedemption) PackMToken() []byte {
	enc, err := midasRedemption.abi.Pack("mToken")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc3b6f939.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function mToken() view returns(address token)
func (midasRedemption *MidasRedemption) TryPackMToken() ([]byte, error) {
	return midasRedemption.abi.Pack("mToken")
}

// UnpackMToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc3b6f939.
//
// Solidity: function mToken() view returns(address token)
func (midasRedemption *MidasRedemption) UnpackMToken(data []byte) (common.Address, error) {
	out, err := midasRedemption.abi.Unpack("mToken", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackMTokenDataFeed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6254afb6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function mTokenDataFeed() view returns(address dataFeed)
func (midasRedemption *MidasRedemption) PackMTokenDataFeed() []byte {
	enc, err := midasRedemption.abi.Pack("mTokenDataFeed")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMTokenDataFeed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6254afb6.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function mTokenDataFeed() view returns(address dataFeed)
func (midasRedemption *MidasRedemption) TryPackMTokenDataFeed() ([]byte, error) {
	return midasRedemption.abi.Pack("mTokenDataFeed")
}

// UnpackMTokenDataFeed is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6254afb6.
//
// Solidity: function mTokenDataFeed() view returns(address dataFeed)
func (midasRedemption *MidasRedemption) UnpackMTokenDataFeed(data []byte) (common.Address, error) {
	out, err := midasRedemption.abi.Unpack("mTokenDataFeed", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackMinAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9b2cb5d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function minAmount() view returns(uint256)
func (midasRedemption *MidasRedemption) PackMinAmount() []byte {
	enc, err := midasRedemption.abi.Pack("minAmount")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMinAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9b2cb5d8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function minAmount() view returns(uint256)
func (midasRedemption *MidasRedemption) TryPackMinAmount() ([]byte, error) {
	return midasRedemption.abi.Pack("minAmount")
}

// UnpackMinAmount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9b2cb5d8.
//
// Solidity: function minAmount() view returns(uint256)
func (midasRedemption *MidasRedemption) UnpackMinAmount(data []byte) (*big.Int, error) {
	out, err := midasRedemption.abi.Unpack("minAmount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackPaused is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5c975abb.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function paused() view returns(bool)
func (midasRedemption *MidasRedemption) PackPaused() []byte {
	enc, err := midasRedemption.abi.Pack("paused")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPaused is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5c975abb.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function paused() view returns(bool)
func (midasRedemption *MidasRedemption) TryPackPaused() ([]byte, error) {
	return midasRedemption.abi.Pack("paused")
}

// UnpackPaused is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (midasRedemption *MidasRedemption) UnpackPaused(data []byte) (bool, error) {
	out, err := midasRedemption.abi.Unpack("paused", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbfc2d46a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemRequest(address tokenOut, uint256 amountMTokenIn) returns(uint256 requestId)
func (midasRedemption *MidasRedemption) PackRedeemRequest(tokenOut common.Address, amountMTokenIn *big.Int) []byte {
	enc, err := midasRedemption.abi.Pack("redeemRequest", tokenOut, amountMTokenIn)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbfc2d46a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemRequest(address tokenOut, uint256 amountMTokenIn) returns(uint256 requestId)
func (midasRedemption *MidasRedemption) TryPackRedeemRequest(tokenOut common.Address, amountMTokenIn *big.Int) ([]byte, error) {
	return midasRedemption.abi.Pack("redeemRequest", tokenOut, amountMTokenIn)
}

// UnpackRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xbfc2d46a.
//
// Solidity: function redeemRequest(address tokenOut, uint256 amountMTokenIn) returns(uint256 requestId)
func (midasRedemption *MidasRedemption) UnpackRedeemRequest(data []byte) (*big.Int, error) {
	out, err := midasRedemption.abi.Unpack("redeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe85ba3e9.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemRequests(uint256 requestId) view returns(address sender, address tokenOut, uint8 status, uint256 amountMToken, uint256 mTokenRate, uint256 tokenOutRate)
func (midasRedemption *MidasRedemption) PackRedeemRequests(requestId *big.Int) []byte {
	enc, err := midasRedemption.abi.Pack("redeemRequests", requestId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe85ba3e9.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemRequests(uint256 requestId) view returns(address sender, address tokenOut, uint8 status, uint256 amountMToken, uint256 mTokenRate, uint256 tokenOutRate)
func (midasRedemption *MidasRedemption) TryPackRedeemRequests(requestId *big.Int) ([]byte, error) {
	return midasRedemption.abi.Pack("redeemRequests", requestId)
}

// RedeemRequestsOutput serves as a container for the return parameters of contract
// method RedeemRequests.
type RedeemRequestsOutput struct {
	Sender       common.Address
	TokenOut     common.Address
	Status       uint8
	AmountMToken *big.Int
	MTokenRate   *big.Int
	TokenOutRate *big.Int
}

// UnpackRedeemRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe85ba3e9.
//
// Solidity: function redeemRequests(uint256 requestId) view returns(address sender, address tokenOut, uint8 status, uint256 amountMToken, uint256 mTokenRate, uint256 tokenOutRate)
func (midasRedemption *MidasRedemption) UnpackRedeemRequests(data []byte) (RedeemRequestsOutput, error) {
	out, err := midasRedemption.abi.Unpack("redeemRequests", data)
	outstruct := new(RedeemRequestsOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Sender = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.TokenOut = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Status = *abi.ConvertType(out[2], new(uint8)).(*uint8)
	outstruct.AmountMToken = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	outstruct.MTokenRate = abi.ConvertType(out[4], new(big.Int)).(*big.Int)
	outstruct.TokenOutRate = abi.ConvertType(out[5], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackTokensConfig is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbc979af6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function tokensConfig(address token) view returns(address dataFeed, uint256 fee, uint256 allowance, bool stable)
func (midasRedemption *MidasRedemption) PackTokensConfig(token common.Address) []byte {
	enc, err := midasRedemption.abi.Pack("tokensConfig", token)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackTokensConfig is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbc979af6.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function tokensConfig(address token) view returns(address dataFeed, uint256 fee, uint256 allowance, bool stable)
func (midasRedemption *MidasRedemption) TryPackTokensConfig(token common.Address) ([]byte, error) {
	return midasRedemption.abi.Pack("tokensConfig", token)
}

// TokensConfigOutput serves as a container for the return parameters of contract
// method TokensConfig.
type TokensConfigOutput struct {
	DataFeed  common.Address
	Fee       *big.Int
	Allowance *big.Int
	Stable    bool
}

// UnpackTokensConfig is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xbc979af6.
//
// Solidity: function tokensConfig(address token) view returns(address dataFeed, uint256 fee, uint256 allowance, bool stable)
func (midasRedemption *MidasRedemption) UnpackTokensConfig(data []byte) (TokensConfigOutput, error) {
	out, err := midasRedemption.abi.Unpack("tokensConfig", data)
	outstruct := new(TokensConfigOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.DataFeed = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Fee = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.Allowance = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.Stable = *abi.ConvertType(out[3], new(bool)).(*bool)
	return *outstruct, nil
}
