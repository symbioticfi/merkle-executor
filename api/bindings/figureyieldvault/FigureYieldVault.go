// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package figureyieldvault

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

// FigureYieldVaultMetaData contains all meta data concerning the FigureYieldVault contract.
var FigureYieldVaultMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"asset\",\"inputs\":[],\"outputs\":[{\"name\":\"asset\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"convertToAssets\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pendingRedemptions\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"timestamp\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"requestRedeem\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	ID:  "FigureYieldVault",
}

// FigureYieldVault is an auto generated Go binding around an Ethereum contract.
type FigureYieldVault struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *FigureYieldVault) GetABI() abi.ABI {
	return c.abi
}

// NewFigureYieldVault creates a new instance of FigureYieldVault.
func NewFigureYieldVault() *FigureYieldVault {
	parsed, err := FigureYieldVaultMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &FigureYieldVault{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *FigureYieldVault) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x38d52e0f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function asset() view returns(address asset)
func (figureYieldVault *FigureYieldVault) PackAsset() []byte {
	enc, err := figureYieldVault.abi.Pack("asset")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x38d52e0f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function asset() view returns(address asset)
func (figureYieldVault *FigureYieldVault) TryPackAsset() ([]byte, error) {
	return figureYieldVault.abi.Pack("asset")
}

// UnpackAsset is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x38d52e0f.
//
// Solidity: function asset() view returns(address asset)
func (figureYieldVault *FigureYieldVault) UnpackAsset(data []byte) (common.Address, error) {
	out, err := figureYieldVault.abi.Unpack("asset", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackConvertToAssets is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07a2d13a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function convertToAssets(uint256 shares) view returns(uint256 assets)
func (figureYieldVault *FigureYieldVault) PackConvertToAssets(shares *big.Int) []byte {
	enc, err := figureYieldVault.abi.Pack("convertToAssets", shares)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackConvertToAssets is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07a2d13a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function convertToAssets(uint256 shares) view returns(uint256 assets)
func (figureYieldVault *FigureYieldVault) TryPackConvertToAssets(shares *big.Int) ([]byte, error) {
	return figureYieldVault.abi.Pack("convertToAssets", shares)
}

// UnpackConvertToAssets is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x07a2d13a.
//
// Solidity: function convertToAssets(uint256 shares) view returns(uint256 assets)
func (figureYieldVault *FigureYieldVault) UnpackConvertToAssets(data []byte) (*big.Int, error) {
	out, err := figureYieldVault.abi.Unpack("convertToAssets", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackPendingRedemptions is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe50fe9b9.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function pendingRedemptions(address user) view returns(uint256 shares, uint256 assets, uint256 timestamp)
func (figureYieldVault *FigureYieldVault) PackPendingRedemptions(user common.Address) []byte {
	enc, err := figureYieldVault.abi.Pack("pendingRedemptions", user)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPendingRedemptions is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe50fe9b9.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function pendingRedemptions(address user) view returns(uint256 shares, uint256 assets, uint256 timestamp)
func (figureYieldVault *FigureYieldVault) TryPackPendingRedemptions(user common.Address) ([]byte, error) {
	return figureYieldVault.abi.Pack("pendingRedemptions", user)
}

// PendingRedemptionsOutput serves as a container for the return parameters of contract
// method PendingRedemptions.
type PendingRedemptionsOutput struct {
	Shares    *big.Int
	Assets    *big.Int
	Timestamp *big.Int
}

// UnpackPendingRedemptions is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe50fe9b9.
//
// Solidity: function pendingRedemptions(address user) view returns(uint256 shares, uint256 assets, uint256 timestamp)
func (figureYieldVault *FigureYieldVault) UnpackPendingRedemptions(data []byte) (PendingRedemptionsOutput, error) {
	out, err := figureYieldVault.abi.Unpack("pendingRedemptions", data)
	outstruct := new(PendingRedemptionsOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Shares = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Assets = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.Timestamp = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaa2f892d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function requestRedeem(uint256 shares) returns()
func (figureYieldVault *FigureYieldVault) PackRequestRedeem(shares *big.Int) []byte {
	enc, err := figureYieldVault.abi.Pack("requestRedeem", shares)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaa2f892d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function requestRedeem(uint256 shares) returns()
func (figureYieldVault *FigureYieldVault) TryPackRequestRedeem(shares *big.Int) ([]byte, error) {
	return figureYieldVault.abi.Pack("requestRedeem", shares)
}
