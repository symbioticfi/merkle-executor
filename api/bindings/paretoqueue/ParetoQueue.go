// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package paretoqueue

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

// ParetoQueueMetaData contains all meta data concerning the ParetoQueue contract.
var ParetoQueueMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"claimWithdrawRequest\",\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"epochPendingClaims\",\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"epochWithdrawPrice\",\"inputs\":[{\"name\":\"epoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"price\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"idleCDOEpoch\",\"inputs\":[],\"outputs\":[{\"name\":\"idleCdo\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"requestWithdraw\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"strategy\",\"inputs\":[],\"outputs\":[{\"name\":\"creditVault\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tranche\",\"inputs\":[],\"outputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"underlying\",\"inputs\":[],\"outputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"userWithdrawalsEpochs\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"epoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "ParetoQueue",
}

// ParetoQueue is an auto generated Go binding around an Ethereum contract.
type ParetoQueue struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ParetoQueue) GetABI() abi.ABI {
	return c.abi
}

// NewParetoQueue creates a new instance of ParetoQueue.
func NewParetoQueue() *ParetoQueue {
	parsed, err := ParetoQueueMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ParetoQueue{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ParetoQueue) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackClaimWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd2233918.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawRequest(uint256 epoch) returns()
func (paretoQueue *ParetoQueue) PackClaimWithdrawRequest(epoch *big.Int) []byte {
	enc, err := paretoQueue.abi.Pack("claimWithdrawRequest", epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd2233918.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimWithdrawRequest(uint256 epoch) returns()
func (paretoQueue *ParetoQueue) TryPackClaimWithdrawRequest(epoch *big.Int) ([]byte, error) {
	return paretoQueue.abi.Pack("claimWithdrawRequest", epoch)
}

// PackEpochPendingClaims is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9453dc6c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochPendingClaims(uint256 epoch) view returns(uint256 assets)
func (paretoQueue *ParetoQueue) PackEpochPendingClaims(epoch *big.Int) []byte {
	enc, err := paretoQueue.abi.Pack("epochPendingClaims", epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochPendingClaims is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9453dc6c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochPendingClaims(uint256 epoch) view returns(uint256 assets)
func (paretoQueue *ParetoQueue) TryPackEpochPendingClaims(epoch *big.Int) ([]byte, error) {
	return paretoQueue.abi.Pack("epochPendingClaims", epoch)
}

// UnpackEpochPendingClaims is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9453dc6c.
//
// Solidity: function epochPendingClaims(uint256 epoch) view returns(uint256 assets)
func (paretoQueue *ParetoQueue) UnpackEpochPendingClaims(data []byte) (*big.Int, error) {
	out, err := paretoQueue.abi.Unpack("epochPendingClaims", data)
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
// Solidity: function epochWithdrawPrice(uint256 epoch) view returns(uint256 price)
func (paretoQueue *ParetoQueue) PackEpochWithdrawPrice(epoch *big.Int) []byte {
	enc, err := paretoQueue.abi.Pack("epochWithdrawPrice", epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochWithdrawPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x07837637.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochWithdrawPrice(uint256 epoch) view returns(uint256 price)
func (paretoQueue *ParetoQueue) TryPackEpochWithdrawPrice(epoch *big.Int) ([]byte, error) {
	return paretoQueue.abi.Pack("epochWithdrawPrice", epoch)
}

// UnpackEpochWithdrawPrice is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x07837637.
//
// Solidity: function epochWithdrawPrice(uint256 epoch) view returns(uint256 price)
func (paretoQueue *ParetoQueue) UnpackEpochWithdrawPrice(data []byte) (*big.Int, error) {
	out, err := paretoQueue.abi.Unpack("epochWithdrawPrice", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackIdleCDOEpoch is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9343d94e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function idleCDOEpoch() view returns(address idleCdo)
func (paretoQueue *ParetoQueue) PackIdleCDOEpoch() []byte {
	enc, err := paretoQueue.abi.Pack("idleCDOEpoch")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIdleCDOEpoch is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9343d94e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function idleCDOEpoch() view returns(address idleCdo)
func (paretoQueue *ParetoQueue) TryPackIdleCDOEpoch() ([]byte, error) {
	return paretoQueue.abi.Pack("idleCDOEpoch")
}

// UnpackIdleCDOEpoch is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9343d94e.
//
// Solidity: function idleCDOEpoch() view returns(address idleCdo)
func (paretoQueue *ParetoQueue) UnpackIdleCDOEpoch(data []byte) (common.Address, error) {
	out, err := paretoQueue.abi.Unpack("idleCDOEpoch", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRequestWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x745400c9.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function requestWithdraw(uint256 amount) returns()
func (paretoQueue *ParetoQueue) PackRequestWithdraw(amount *big.Int) []byte {
	enc, err := paretoQueue.abi.Pack("requestWithdraw", amount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRequestWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x745400c9.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function requestWithdraw(uint256 amount) returns()
func (paretoQueue *ParetoQueue) TryPackRequestWithdraw(amount *big.Int) ([]byte, error) {
	return paretoQueue.abi.Pack("requestWithdraw", amount)
}

// PackStrategy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa8c62e76.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function strategy() view returns(address creditVault)
func (paretoQueue *ParetoQueue) PackStrategy() []byte {
	enc, err := paretoQueue.abi.Pack("strategy")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackStrategy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa8c62e76.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function strategy() view returns(address creditVault)
func (paretoQueue *ParetoQueue) TryPackStrategy() ([]byte, error) {
	return paretoQueue.abi.Pack("strategy")
}

// UnpackStrategy is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa8c62e76.
//
// Solidity: function strategy() view returns(address creditVault)
func (paretoQueue *ParetoQueue) UnpackStrategy(data []byte) (common.Address, error) {
	out, err := paretoQueue.abi.Unpack("strategy", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackTranche is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6ebc0af1.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function tranche() view returns(address token)
func (paretoQueue *ParetoQueue) PackTranche() []byte {
	enc, err := paretoQueue.abi.Pack("tranche")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackTranche is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6ebc0af1.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function tranche() view returns(address token)
func (paretoQueue *ParetoQueue) TryPackTranche() ([]byte, error) {
	return paretoQueue.abi.Pack("tranche")
}

// UnpackTranche is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6ebc0af1.
//
// Solidity: function tranche() view returns(address token)
func (paretoQueue *ParetoQueue) UnpackTranche(data []byte) (common.Address, error) {
	out, err := paretoQueue.abi.Unpack("tranche", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackUnderlying is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6f307dc3.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function underlying() view returns(address token)
func (paretoQueue *ParetoQueue) PackUnderlying() []byte {
	enc, err := paretoQueue.abi.Pack("underlying")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUnderlying is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6f307dc3.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function underlying() view returns(address token)
func (paretoQueue *ParetoQueue) TryPackUnderlying() ([]byte, error) {
	return paretoQueue.abi.Pack("underlying")
}

// UnpackUnderlying is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6f307dc3.
//
// Solidity: function underlying() view returns(address token)
func (paretoQueue *ParetoQueue) UnpackUnderlying(data []byte) (common.Address, error) {
	out, err := paretoQueue.abi.Unpack("underlying", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackUserWithdrawalsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b223e52.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 amount)
func (paretoQueue *ParetoQueue) PackUserWithdrawalsEpochs(account common.Address, epoch *big.Int) []byte {
	enc, err := paretoQueue.abi.Pack("userWithdrawalsEpochs", account, epoch)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUserWithdrawalsEpochs is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5b223e52.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 amount)
func (paretoQueue *ParetoQueue) TryPackUserWithdrawalsEpochs(account common.Address, epoch *big.Int) ([]byte, error) {
	return paretoQueue.abi.Pack("userWithdrawalsEpochs", account, epoch)
}

// UnpackUserWithdrawalsEpochs is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5b223e52.
//
// Solidity: function userWithdrawalsEpochs(address account, uint256 epoch) view returns(uint256 amount)
func (paretoQueue *ParetoQueue) UnpackUserWithdrawalsEpochs(data []byte) (*big.Int, error) {
	out, err := paretoQueue.abi.Unpack("userWithdrawalsEpochs", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
