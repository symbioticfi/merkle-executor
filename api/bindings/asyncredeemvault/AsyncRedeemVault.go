// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package asyncredeemvault

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

// AsyncRedeemVaultMetaData contains all meta data concerning the AsyncRedeemVault contract.
var AsyncRedeemVaultMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"asset\",\"inputs\":[],\"outputs\":[{\"name\":\"asset\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimableRedeemRequest\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"controller\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"convertToAssets\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxRedeem\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"maxShares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxWithdraw\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"maxAssets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pendingRedeemRequest\",\"inputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"controller\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeem\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"controller\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"requestRedeem\",\"inputs\":[{\"name\":\"shares\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"controller\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"requestId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"}]",
	ID:  "AsyncRedeemVault",
}

// AsyncRedeemVault is an auto generated Go binding around an Ethereum contract.
type AsyncRedeemVault struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *AsyncRedeemVault) GetABI() abi.ABI {
	return c.abi
}

// NewAsyncRedeemVault creates a new instance of AsyncRedeemVault.
func NewAsyncRedeemVault() *AsyncRedeemVault {
	parsed, err := AsyncRedeemVaultMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &AsyncRedeemVault{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *AsyncRedeemVault) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x38d52e0f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function asset() view returns(address asset)
func (asyncRedeemVault *AsyncRedeemVault) PackAsset() []byte {
	enc, err := asyncRedeemVault.abi.Pack("asset")
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
func (asyncRedeemVault *AsyncRedeemVault) TryPackAsset() ([]byte, error) {
	return asyncRedeemVault.abi.Pack("asset")
}

// UnpackAsset is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x38d52e0f.
//
// Solidity: function asset() view returns(address asset)
func (asyncRedeemVault *AsyncRedeemVault) UnpackAsset(data []byte) (common.Address, error) {
	out, err := asyncRedeemVault.abi.Unpack("asset", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackClaimableRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xeaed1d07.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimableRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) PackClaimableRedeemRequest(requestId *big.Int, controller common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("claimableRedeemRequest", requestId, controller)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimableRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xeaed1d07.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimableRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) TryPackClaimableRedeemRequest(requestId *big.Int, controller common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("claimableRedeemRequest", requestId, controller)
}

// UnpackClaimableRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xeaed1d07.
//
// Solidity: function claimableRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) UnpackClaimableRedeemRequest(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("claimableRedeemRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackConvertToAssets is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07a2d13a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function convertToAssets(uint256 shares) view returns(uint256 assets)
func (asyncRedeemVault *AsyncRedeemVault) PackConvertToAssets(shares *big.Int) []byte {
	enc, err := asyncRedeemVault.abi.Pack("convertToAssets", shares)
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
func (asyncRedeemVault *AsyncRedeemVault) TryPackConvertToAssets(shares *big.Int) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("convertToAssets", shares)
}

// UnpackConvertToAssets is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x07a2d13a.
//
// Solidity: function convertToAssets(uint256 shares) view returns(uint256 assets)
func (asyncRedeemVault *AsyncRedeemVault) UnpackConvertToAssets(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("convertToAssets", data)
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
// Solidity: function maxRedeem(address owner) view returns(uint256 maxShares)
func (asyncRedeemVault *AsyncRedeemVault) PackMaxRedeem(owner common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("maxRedeem", owner)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd905777e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxRedeem(address owner) view returns(uint256 maxShares)
func (asyncRedeemVault *AsyncRedeemVault) TryPackMaxRedeem(owner common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("maxRedeem", owner)
}

// UnpackMaxRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd905777e.
//
// Solidity: function maxRedeem(address owner) view returns(uint256 maxShares)
func (asyncRedeemVault *AsyncRedeemVault) UnpackMaxRedeem(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("maxRedeem", data)
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
// Solidity: function maxWithdraw(address owner) view returns(uint256 maxAssets)
func (asyncRedeemVault *AsyncRedeemVault) PackMaxWithdraw(owner common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("maxWithdraw", owner)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xce96cb77.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxWithdraw(address owner) view returns(uint256 maxAssets)
func (asyncRedeemVault *AsyncRedeemVault) TryPackMaxWithdraw(owner common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("maxWithdraw", owner)
}

// UnpackMaxWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xce96cb77.
//
// Solidity: function maxWithdraw(address owner) view returns(uint256 maxAssets)
func (asyncRedeemVault *AsyncRedeemVault) UnpackMaxWithdraw(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("maxWithdraw", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackPendingRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf5a23d8d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function pendingRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) PackPendingRedeemRequest(requestId *big.Int, controller common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("pendingRedeemRequest", requestId, controller)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPendingRedeemRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf5a23d8d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function pendingRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) TryPackPendingRedeemRequest(requestId *big.Int, controller common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("pendingRedeemRequest", requestId, controller)
}

// UnpackPendingRedeemRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf5a23d8d.
//
// Solidity: function pendingRedeemRequest(uint256 requestId, address controller) view returns(uint256 shares)
func (asyncRedeemVault *AsyncRedeemVault) UnpackPendingRedeemRequest(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("pendingRedeemRequest", data)
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
// Solidity: function redeem(uint256 shares, address receiver, address controller) returns(uint256 assets)
func (asyncRedeemVault *AsyncRedeemVault) PackRedeem(shares *big.Int, receiver common.Address, controller common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("redeem", shares, receiver, controller)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xba087652.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeem(uint256 shares, address receiver, address controller) returns(uint256 assets)
func (asyncRedeemVault *AsyncRedeemVault) TryPackRedeem(shares *big.Int, receiver common.Address, controller common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("redeem", shares, receiver, controller)
}

// UnpackRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xba087652.
//
// Solidity: function redeem(uint256 shares, address receiver, address controller) returns(uint256 assets)
func (asyncRedeemVault *AsyncRedeemVault) UnpackRedeem(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("redeem", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7d41c86e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function requestRedeem(uint256 shares, address controller, address owner) returns(uint256 requestId)
func (asyncRedeemVault *AsyncRedeemVault) PackRequestRedeem(shares *big.Int, controller common.Address, owner common.Address) []byte {
	enc, err := asyncRedeemVault.abi.Pack("requestRedeem", shares, controller, owner)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7d41c86e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function requestRedeem(uint256 shares, address controller, address owner) returns(uint256 requestId)
func (asyncRedeemVault *AsyncRedeemVault) TryPackRequestRedeem(shares *big.Int, controller common.Address, owner common.Address) ([]byte, error) {
	return asyncRedeemVault.abi.Pack("requestRedeem", shares, controller, owner)
}

// UnpackRequestRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x7d41c86e.
//
// Solidity: function requestRedeem(uint256 shares, address controller, address owner) returns(uint256 requestId)
func (asyncRedeemVault *AsyncRedeemVault) UnpackRequestRedeem(data []byte) (*big.Int, error) {
	out, err := asyncRedeemVault.abi.Unpack("requestRedeem", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
