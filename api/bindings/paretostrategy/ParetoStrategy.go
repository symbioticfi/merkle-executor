// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package paretostrategy

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

// ParetoStrategyMetaData contains all meta data concerning the ParetoStrategy contract.
var ParetoStrategyMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"epochNumber\",\"inputs\":[],\"outputs\":[{\"name\":\"epoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"idleCDO\",\"inputs\":[],\"outputs\":[{\"name\":\"idleCdo\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"token\",\"inputs\":[],\"outputs\":[{\"name\":\"underlying\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"withdrawsRequests\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"instantWithdrawsRequests\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"lastWithdrawRequest\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"name\":\"apr0RateByEpoch\",\"type\":\"function\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"name\":\"apr0Users\",\"type\":\"function\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"principal\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"principalEpoch\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"settledPrincipal\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"settledInterest\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "ParetoStrategy",
}

// ParetoStrategy is an auto generated Go binding around an Ethereum contract.
type ParetoStrategy struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ParetoStrategy) GetABI() abi.ABI {
	return c.abi
}

// NewParetoStrategy creates a new instance of ParetoStrategy.
func NewParetoStrategy() *ParetoStrategy {
	parsed, err := ParetoStrategyMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ParetoStrategy{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ParetoStrategy) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackApr0RateByEpoch is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x11259509.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function apr0RateByEpoch(uint256 ) view returns(uint256)
func (paretoStrategy *ParetoStrategy) PackApr0RateByEpoch(arg0 *big.Int) []byte {
	enc, err := paretoStrategy.abi.Pack("apr0RateByEpoch", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackApr0RateByEpoch is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x11259509.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function apr0RateByEpoch(uint256 ) view returns(uint256)
func (paretoStrategy *ParetoStrategy) TryPackApr0RateByEpoch(arg0 *big.Int) ([]byte, error) {
	return paretoStrategy.abi.Pack("apr0RateByEpoch", arg0)
}

// UnpackApr0RateByEpoch is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x11259509.
//
// Solidity: function apr0RateByEpoch(uint256 ) view returns(uint256)
func (paretoStrategy *ParetoStrategy) UnpackApr0RateByEpoch(data []byte) (*big.Int, error) {
	out, err := paretoStrategy.abi.Unpack("apr0RateByEpoch", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackApr0Users is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x97e3f633.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function apr0Users(address ) view returns(uint256 principal, uint256 principalEpoch, uint256 settledPrincipal, uint256 settledInterest)
func (paretoStrategy *ParetoStrategy) PackApr0Users(arg0 common.Address) []byte {
	enc, err := paretoStrategy.abi.Pack("apr0Users", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackApr0Users is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x97e3f633.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function apr0Users(address ) view returns(uint256 principal, uint256 principalEpoch, uint256 settledPrincipal, uint256 settledInterest)
func (paretoStrategy *ParetoStrategy) TryPackApr0Users(arg0 common.Address) ([]byte, error) {
	return paretoStrategy.abi.Pack("apr0Users", arg0)
}

// Apr0UsersOutput serves as a container for the return parameters of contract
// method Apr0Users.
type Apr0UsersOutput struct {
	Principal        *big.Int
	PrincipalEpoch   *big.Int
	SettledPrincipal *big.Int
	SettledInterest  *big.Int
}

// UnpackApr0Users is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x97e3f633.
//
// Solidity: function apr0Users(address ) view returns(uint256 principal, uint256 principalEpoch, uint256 settledPrincipal, uint256 settledInterest)
func (paretoStrategy *ParetoStrategy) UnpackApr0Users(data []byte) (Apr0UsersOutput, error) {
	out, err := paretoStrategy.abi.Unpack("apr0Users", data)
	outstruct := new(Apr0UsersOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Principal = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.PrincipalEpoch = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.SettledPrincipal = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.SettledInterest = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackEpochNumber is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf4145a83.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochNumber() view returns(uint256 epoch)
func (paretoStrategy *ParetoStrategy) PackEpochNumber() []byte {
	enc, err := paretoStrategy.abi.Pack("epochNumber")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochNumber is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf4145a83.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochNumber() view returns(uint256 epoch)
func (paretoStrategy *ParetoStrategy) TryPackEpochNumber() ([]byte, error) {
	return paretoStrategy.abi.Pack("epochNumber")
}

// UnpackEpochNumber is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf4145a83.
//
// Solidity: function epochNumber() view returns(uint256 epoch)
func (paretoStrategy *ParetoStrategy) UnpackEpochNumber(data []byte) (*big.Int, error) {
	out, err := paretoStrategy.abi.Unpack("epochNumber", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackIdleCDO is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd4090fb8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function idleCDO() view returns(address idleCdo)
func (paretoStrategy *ParetoStrategy) PackIdleCDO() []byte {
	enc, err := paretoStrategy.abi.Pack("idleCDO")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIdleCDO is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd4090fb8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function idleCDO() view returns(address idleCdo)
func (paretoStrategy *ParetoStrategy) TryPackIdleCDO() ([]byte, error) {
	return paretoStrategy.abi.Pack("idleCDO")
}

// UnpackIdleCDO is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd4090fb8.
//
// Solidity: function idleCDO() view returns(address idleCdo)
func (paretoStrategy *ParetoStrategy) UnpackIdleCDO(data []byte) (common.Address, error) {
	out, err := paretoStrategy.abi.Unpack("idleCDO", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) PackInstantWithdrawsRequests(account common.Address) []byte {
	enc, err := paretoStrategy.abi.Pack("instantWithdrawsRequests", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackInstantWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9bf5bbe7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) TryPackInstantWithdrawsRequests(account common.Address) ([]byte, error) {
	return paretoStrategy.abi.Pack("instantWithdrawsRequests", account)
}

// UnpackInstantWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9bf5bbe7.
//
// Solidity: function instantWithdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) UnpackInstantWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := paretoStrategy.abi.Unpack("instantWithdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackLastWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x93f88d4c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function lastWithdrawRequest(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) PackLastWithdrawRequest(account common.Address) []byte {
	enc, err := paretoStrategy.abi.Pack("lastWithdrawRequest", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackLastWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x93f88d4c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function lastWithdrawRequest(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) TryPackLastWithdrawRequest(account common.Address) ([]byte, error) {
	return paretoStrategy.abi.Pack("lastWithdrawRequest", account)
}

// UnpackLastWithdrawRequest is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x93f88d4c.
//
// Solidity: function lastWithdrawRequest(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) UnpackLastWithdrawRequest(data []byte) (*big.Int, error) {
	out, err := paretoStrategy.abi.Unpack("lastWithdrawRequest", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc0c546a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function token() view returns(address underlying)
func (paretoStrategy *ParetoStrategy) PackToken() []byte {
	enc, err := paretoStrategy.abi.Pack("token")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc0c546a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function token() view returns(address underlying)
func (paretoStrategy *ParetoStrategy) TryPackToken() ([]byte, error) {
	return paretoStrategy.abi.Pack("token")
}

// UnpackToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xfc0c546a.
//
// Solidity: function token() view returns(address underlying)
func (paretoStrategy *ParetoStrategy) UnpackToken(data []byte) (common.Address, error) {
	out, err := paretoStrategy.abi.Unpack("token", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xacfd7dce.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function withdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) PackWithdrawsRequests(account common.Address) []byte {
	enc, err := paretoStrategy.abi.Pack("withdrawsRequests", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackWithdrawsRequests is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xacfd7dce.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function withdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) TryPackWithdrawsRequests(account common.Address) ([]byte, error) {
	return paretoStrategy.abi.Pack("withdrawsRequests", account)
}

// UnpackWithdrawsRequests is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xacfd7dce.
//
// Solidity: function withdrawsRequests(address account) view returns(uint256)
func (paretoStrategy *ParetoStrategy) UnpackWithdrawsRequests(data []byte) (*big.Int, error) {
	out, err := paretoStrategy.abi.Unpack("withdrawsRequests", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
