// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package securitizelegacyofframp

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

// SecuritizeLegacyOffRampMetaData contains all meta data concerning the SecuritizeLegacyOffRamp contract.
var SecuritizeLegacyOffRampMetaData = bind.MetaData{
	ABI: "[{\"name\":\"asset\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"name\":\"liquidity\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"name\":\"paused\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"name\":\"redeem\",\"type\":\"function\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"name\":\"settlement\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractISettlement\"}],\"stateMutability\":\"view\"}]",
	ID:  "SecuritizeLegacyOffRamp",
}

// SecuritizeLegacyOffRamp is an auto generated Go binding around an Ethereum contract.
type SecuritizeLegacyOffRamp struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *SecuritizeLegacyOffRamp) GetABI() abi.ABI {
	return c.abi
}

// NewSecuritizeLegacyOffRamp creates a new instance of SecuritizeLegacyOffRamp.
func NewSecuritizeLegacyOffRamp() *SecuritizeLegacyOffRamp {
	parsed, err := SecuritizeLegacyOffRampMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &SecuritizeLegacyOffRamp{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *SecuritizeLegacyOffRamp) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x38d52e0f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function asset() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) PackAsset() []byte {
	enc, err := securitizeLegacyOffRamp.abi.Pack("asset")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x38d52e0f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function asset() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) TryPackAsset() ([]byte, error) {
	return securitizeLegacyOffRamp.abi.Pack("asset")
}

// UnpackAsset is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x38d52e0f.
//
// Solidity: function asset() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) UnpackAsset(data []byte) (common.Address, error) {
	out, err := securitizeLegacyOffRamp.abi.Unpack("asset", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1a686502.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function liquidity() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) PackLiquidity() []byte {
	enc, err := securitizeLegacyOffRamp.abi.Pack("liquidity")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1a686502.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function liquidity() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) TryPackLiquidity() ([]byte, error) {
	return securitizeLegacyOffRamp.abi.Pack("liquidity")
}

// UnpackLiquidity is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x1a686502.
//
// Solidity: function liquidity() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) UnpackLiquidity(data []byte) (common.Address, error) {
	out, err := securitizeLegacyOffRamp.abi.Unpack("liquidity", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackPaused is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5c975abb.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function paused() view returns(bool)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) PackPaused() []byte {
	enc, err := securitizeLegacyOffRamp.abi.Pack("paused")
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
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) TryPackPaused() ([]byte, error) {
	return securitizeLegacyOffRamp.abi.Pack("paused")
}

// UnpackPaused is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) UnpackPaused(data []byte) (bool, error) {
	out, err := securitizeLegacyOffRamp.abi.Unpack("paused", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xdb006a75.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeem(uint256 amount) returns()
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) PackRedeem(amount *big.Int) []byte {
	enc, err := securitizeLegacyOffRamp.abi.Pack("redeem", amount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xdb006a75.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeem(uint256 amount) returns()
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) TryPackRedeem(amount *big.Int) ([]byte, error) {
	return securitizeLegacyOffRamp.abi.Pack("redeem", amount)
}

// PackSettlement is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x51160630.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function settlement() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) PackSettlement() []byte {
	enc, err := securitizeLegacyOffRamp.abi.Pack("settlement")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSettlement is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x51160630.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function settlement() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) TryPackSettlement() ([]byte, error) {
	return securitizeLegacyOffRamp.abi.Pack("settlement")
}

// UnpackSettlement is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x51160630.
//
// Solidity: function settlement() view returns(address)
func (securitizeLegacyOffRamp *SecuritizeLegacyOffRamp) UnpackSettlement(data []byte) (common.Address, error) {
	out, err := securitizeLegacyOffRamp.abi.Unpack("settlement", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}
