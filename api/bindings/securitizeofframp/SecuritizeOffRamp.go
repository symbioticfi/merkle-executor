// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package securitizeofframp

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

// SecuritizeOffRampMetaData contains all meta data concerning the SecuritizeOffRamp contract.
var SecuritizeOffRampMetaData = bind.MetaData{
	ABI: "[{\"name\":\"RedemptionCompleted\",\"type\":\"event\",\"inputs\":[{\"name\":\"redeemer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"dsTokenValue\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"liquidityValue\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"rate\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"fee\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"liquidityToken\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"name\":\"assetAddress\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"name\":\"availableLiquidity\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"name\":\"calculateLiquidityTokenAmount\",\"type\":\"function\",\"inputs\":[{\"name\":\"assetAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"name\":\"liquidityProvider\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractILiquidityProvider\"}],\"stateMutability\":\"view\"},{\"name\":\"paused\",\"type\":\"function\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"name\":\"redeem\",\"type\":\"function\",\"inputs\":[{\"name\":\"assetAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"minOutputAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	ID:  "SecuritizeOffRamp",
}

// SecuritizeOffRamp is an auto generated Go binding around an Ethereum contract.
type SecuritizeOffRamp struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *SecuritizeOffRamp) GetABI() abi.ABI {
	return c.abi
}

// NewSecuritizeOffRamp creates a new instance of SecuritizeOffRamp.
func NewSecuritizeOffRamp() *SecuritizeOffRamp {
	parsed, err := SecuritizeOffRampMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &SecuritizeOffRamp{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *SecuritizeOffRamp) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAssetAddress is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1ba46cfd.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function assetAddress() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) PackAssetAddress() []byte {
	enc, err := securitizeOffRamp.abi.Pack("assetAddress")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAssetAddress is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1ba46cfd.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function assetAddress() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) TryPackAssetAddress() ([]byte, error) {
	return securitizeOffRamp.abi.Pack("assetAddress")
}

// UnpackAssetAddress is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x1ba46cfd.
//
// Solidity: function assetAddress() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackAssetAddress(data []byte) (common.Address, error) {
	out, err := securitizeOffRamp.abi.Unpack("assetAddress", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackAvailableLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x74375359.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) PackAvailableLiquidity() []byte {
	enc, err := securitizeOffRamp.abi.Pack("availableLiquidity")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAvailableLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x74375359.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) TryPackAvailableLiquidity() ([]byte, error) {
	return securitizeOffRamp.abi.Pack("availableLiquidity")
}

// UnpackAvailableLiquidity is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x74375359.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackAvailableLiquidity(data []byte) (*big.Int, error) {
	out, err := securitizeOffRamp.abi.Unpack("availableLiquidity", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackCalculateLiquidityTokenAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5e345e73.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function calculateLiquidityTokenAmount(uint256 assetAmount) view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) PackCalculateLiquidityTokenAmount(assetAmount *big.Int) []byte {
	enc, err := securitizeOffRamp.abi.Pack("calculateLiquidityTokenAmount", assetAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCalculateLiquidityTokenAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5e345e73.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function calculateLiquidityTokenAmount(uint256 assetAmount) view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) TryPackCalculateLiquidityTokenAmount(assetAmount *big.Int) ([]byte, error) {
	return securitizeOffRamp.abi.Pack("calculateLiquidityTokenAmount", assetAmount)
}

// UnpackCalculateLiquidityTokenAmount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5e345e73.
//
// Solidity: function calculateLiquidityTokenAmount(uint256 assetAmount) view returns(uint256)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackCalculateLiquidityTokenAmount(data []byte) (*big.Int, error) {
	out, err := securitizeOffRamp.abi.Unpack("calculateLiquidityTokenAmount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackLiquidityProvider is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b8bec55.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function liquidityProvider() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) PackLiquidityProvider() []byte {
	enc, err := securitizeOffRamp.abi.Pack("liquidityProvider")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackLiquidityProvider is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b8bec55.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function liquidityProvider() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) TryPackLiquidityProvider() ([]byte, error) {
	return securitizeOffRamp.abi.Pack("liquidityProvider")
}

// UnpackLiquidityProvider is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5b8bec55.
//
// Solidity: function liquidityProvider() view returns(address)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackLiquidityProvider(data []byte) (common.Address, error) {
	out, err := securitizeOffRamp.abi.Unpack("liquidityProvider", data)
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
func (securitizeOffRamp *SecuritizeOffRamp) PackPaused() []byte {
	enc, err := securitizeOffRamp.abi.Pack("paused")
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
func (securitizeOffRamp *SecuritizeOffRamp) TryPackPaused() ([]byte, error) {
	return securitizeOffRamp.abi.Pack("paused")
}

// UnpackPaused is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackPaused(data []byte) (bool, error) {
	out, err := securitizeOffRamp.abi.Unpack("paused", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7cbc2373.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeem(uint256 assetAmount, uint256 minOutputAmount) returns()
func (securitizeOffRamp *SecuritizeOffRamp) PackRedeem(assetAmount *big.Int, minOutputAmount *big.Int) []byte {
	enc, err := securitizeOffRamp.abi.Pack("redeem", assetAmount, minOutputAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7cbc2373.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeem(uint256 assetAmount, uint256 minOutputAmount) returns()
func (securitizeOffRamp *SecuritizeOffRamp) TryPackRedeem(assetAmount *big.Int, minOutputAmount *big.Int) ([]byte, error) {
	return securitizeOffRamp.abi.Pack("redeem", assetAmount, minOutputAmount)
}

// SecuritizeOffRampRedemptionCompleted represents a RedemptionCompleted event raised by the SecuritizeOffRamp contract.
type SecuritizeOffRampRedemptionCompleted struct {
	Redeemer       common.Address
	DsTokenValue   *big.Int
	LiquidityValue *big.Int
	Rate           *big.Int
	Fee            *big.Int
	LiquidityToken common.Address
	Raw            *types.Log // Blockchain specific contextual infos
}

const SecuritizeOffRampRedemptionCompletedEventName = "RedemptionCompleted"

// ContractEventName returns the user-defined event name.
func (SecuritizeOffRampRedemptionCompleted) ContractEventName() string {
	return SecuritizeOffRampRedemptionCompletedEventName
}

// UnpackRedemptionCompletedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RedemptionCompleted(address indexed redeemer, uint256 dsTokenValue, uint256 liquidityValue, uint256 rate, uint256 fee, address indexed liquidityToken)
func (securitizeOffRamp *SecuritizeOffRamp) UnpackRedemptionCompletedEvent(log *types.Log) (*SecuritizeOffRampRedemptionCompleted, error) {
	event := "RedemptionCompleted"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != securitizeOffRamp.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(SecuritizeOffRampRedemptionCompleted)
	if len(log.Data) > 0 {
		if err := securitizeOffRamp.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range securitizeOffRamp.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return nil, err
	}
	out.Raw = log
	return out, nil
}
