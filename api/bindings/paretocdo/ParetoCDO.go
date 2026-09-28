// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package paretocdo

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

// ParetoCDOMetaData contains all meta data concerning the ParetoCDO contract.
var ParetoCDOMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"AATranche\",\"inputs\":[],\"outputs\":[{\"name\":\"tranche\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"ONE_TRANCHE_TOKEN\",\"inputs\":[],\"outputs\":[{\"name\":\"unit\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimInstantWithdrawRequest\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimWithdrawRequest\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"defaulted\",\"inputs\":[],\"outputs\":[{\"name\":\"isDefaulted\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isEpochRunning\",\"inputs\":[],\"outputs\":[{\"name\":\"running\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isWalletAllowed\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"allowed\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"keyring\",\"inputs\":[],\"outputs\":[{\"name\":\"keyringAddress\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"keyringPolicyId\",\"inputs\":[],\"outputs\":[{\"name\":\"policy\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"oneToken\",\"inputs\":[],\"outputs\":[{\"name\":\"unit\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"requestWithdraw\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"tranche\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"assets\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"strategy\",\"inputs\":[],\"outputs\":[{\"name\":\"receiptToken\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"strategyToken\",\"inputs\":[],\"outputs\":[{\"name\":\"receiptToken\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"token\",\"inputs\":[],\"outputs\":[{\"name\":\"underlying\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"virtualPrice\",\"inputs\":[{\"name\":\"tranche\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"price\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"inputs\":[],\"name\":\"allowInstantWithdraw\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"epochEndDate\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	ID:  "ParetoCDO",
}

// ParetoCDO is an auto generated Go binding around an Ethereum contract.
type ParetoCDO struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *ParetoCDO) GetABI() abi.ABI {
	return c.abi
}

// NewParetoCDO creates a new instance of ParetoCDO.
func NewParetoCDO() *ParetoCDO {
	parsed, err := ParetoCDOMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &ParetoCDO{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *ParetoCDO) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAATranche is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x44966ba0.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function AATranche() view returns(address tranche)
func (paretoCDO *ParetoCDO) PackAATranche() []byte {
	enc, err := paretoCDO.abi.Pack("AATranche")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAATranche is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x44966ba0.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function AATranche() view returns(address tranche)
func (paretoCDO *ParetoCDO) TryPackAATranche() ([]byte, error) {
	return paretoCDO.abi.Pack("AATranche")
}

// UnpackAATranche is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x44966ba0.
//
// Solidity: function AATranche() view returns(address tranche)
func (paretoCDO *ParetoCDO) UnpackAATranche(data []byte) (common.Address, error) {
	out, err := paretoCDO.abi.Unpack("AATranche", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackONETRANCHETOKEN is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9aba356.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function ONE_TRANCHE_TOKEN() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) PackONETRANCHETOKEN() []byte {
	enc, err := paretoCDO.abi.Pack("ONE_TRANCHE_TOKEN")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackONETRANCHETOKEN is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9aba356.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function ONE_TRANCHE_TOKEN() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) TryPackONETRANCHETOKEN() ([]byte, error) {
	return paretoCDO.abi.Pack("ONE_TRANCHE_TOKEN")
}

// UnpackONETRANCHETOKEN is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc9aba356.
//
// Solidity: function ONE_TRANCHE_TOKEN() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) UnpackONETRANCHETOKEN(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("ONE_TRANCHE_TOKEN", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackAllowInstantWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6bd7b301.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function allowInstantWithdraw() view returns(bool)
func (paretoCDO *ParetoCDO) PackAllowInstantWithdraw() []byte {
	enc, err := paretoCDO.abi.Pack("allowInstantWithdraw")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAllowInstantWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6bd7b301.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function allowInstantWithdraw() view returns(bool)
func (paretoCDO *ParetoCDO) TryPackAllowInstantWithdraw() ([]byte, error) {
	return paretoCDO.abi.Pack("allowInstantWithdraw")
}

// UnpackAllowInstantWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6bd7b301.
//
// Solidity: function allowInstantWithdraw() view returns(bool)
func (paretoCDO *ParetoCDO) UnpackAllowInstantWithdraw(data []byte) (bool, error) {
	out, err := paretoCDO.abi.Unpack("allowInstantWithdraw", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackClaimInstantWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x991052b7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimInstantWithdrawRequest() returns()
func (paretoCDO *ParetoCDO) PackClaimInstantWithdrawRequest() []byte {
	enc, err := paretoCDO.abi.Pack("claimInstantWithdrawRequest")
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
func (paretoCDO *ParetoCDO) TryPackClaimInstantWithdrawRequest() ([]byte, error) {
	return paretoCDO.abi.Pack("claimInstantWithdrawRequest")
}

// PackClaimWithdrawRequest is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x33986ffa.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimWithdrawRequest() returns()
func (paretoCDO *ParetoCDO) PackClaimWithdrawRequest() []byte {
	enc, err := paretoCDO.abi.Pack("claimWithdrawRequest")
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
func (paretoCDO *ParetoCDO) TryPackClaimWithdrawRequest() ([]byte, error) {
	return paretoCDO.abi.Pack("claimWithdrawRequest")
}

// PackDefaulted is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69e25ec1.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function defaulted() view returns(bool isDefaulted)
func (paretoCDO *ParetoCDO) PackDefaulted() []byte {
	enc, err := paretoCDO.abi.Pack("defaulted")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDefaulted is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69e25ec1.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function defaulted() view returns(bool isDefaulted)
func (paretoCDO *ParetoCDO) TryPackDefaulted() ([]byte, error) {
	return paretoCDO.abi.Pack("defaulted")
}

// UnpackDefaulted is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x69e25ec1.
//
// Solidity: function defaulted() view returns(bool isDefaulted)
func (paretoCDO *ParetoCDO) UnpackDefaulted(data []byte) (bool, error) {
	out, err := paretoCDO.abi.Unpack("defaulted", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackEpochEndDate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x75d1497b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function epochEndDate() view returns(uint256)
func (paretoCDO *ParetoCDO) PackEpochEndDate() []byte {
	enc, err := paretoCDO.abi.Pack("epochEndDate")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEpochEndDate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x75d1497b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function epochEndDate() view returns(uint256)
func (paretoCDO *ParetoCDO) TryPackEpochEndDate() ([]byte, error) {
	return paretoCDO.abi.Pack("epochEndDate")
}

// UnpackEpochEndDate is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x75d1497b.
//
// Solidity: function epochEndDate() view returns(uint256)
func (paretoCDO *ParetoCDO) UnpackEpochEndDate(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("epochEndDate", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackIsEpochRunning is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc5c75098.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isEpochRunning() view returns(bool running)
func (paretoCDO *ParetoCDO) PackIsEpochRunning() []byte {
	enc, err := paretoCDO.abi.Pack("isEpochRunning")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsEpochRunning is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc5c75098.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isEpochRunning() view returns(bool running)
func (paretoCDO *ParetoCDO) TryPackIsEpochRunning() ([]byte, error) {
	return paretoCDO.abi.Pack("isEpochRunning")
}

// UnpackIsEpochRunning is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc5c75098.
//
// Solidity: function isEpochRunning() view returns(bool running)
func (paretoCDO *ParetoCDO) UnpackIsEpochRunning(data []byte) (bool, error) {
	out, err := paretoCDO.abi.Unpack("isEpochRunning", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackIsWalletAllowed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2e02cd5e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isWalletAllowed(address account) view returns(bool allowed)
func (paretoCDO *ParetoCDO) PackIsWalletAllowed(account common.Address) []byte {
	enc, err := paretoCDO.abi.Pack("isWalletAllowed", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsWalletAllowed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2e02cd5e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isWalletAllowed(address account) view returns(bool allowed)
func (paretoCDO *ParetoCDO) TryPackIsWalletAllowed(account common.Address) ([]byte, error) {
	return paretoCDO.abi.Pack("isWalletAllowed", account)
}

// UnpackIsWalletAllowed is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2e02cd5e.
//
// Solidity: function isWalletAllowed(address account) view returns(bool allowed)
func (paretoCDO *ParetoCDO) UnpackIsWalletAllowed(data []byte) (bool, error) {
	out, err := paretoCDO.abi.Unpack("isWalletAllowed", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackKeyring is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9ed9de94.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function keyring() view returns(address keyringAddress)
func (paretoCDO *ParetoCDO) PackKeyring() []byte {
	enc, err := paretoCDO.abi.Pack("keyring")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackKeyring is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9ed9de94.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function keyring() view returns(address keyringAddress)
func (paretoCDO *ParetoCDO) TryPackKeyring() ([]byte, error) {
	return paretoCDO.abi.Pack("keyring")
}

// UnpackKeyring is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9ed9de94.
//
// Solidity: function keyring() view returns(address keyringAddress)
func (paretoCDO *ParetoCDO) UnpackKeyring(data []byte) (common.Address, error) {
	out, err := paretoCDO.abi.Unpack("keyring", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackKeyringPolicyId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd636b05f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function keyringPolicyId() view returns(uint256 policy)
func (paretoCDO *ParetoCDO) PackKeyringPolicyId() []byte {
	enc, err := paretoCDO.abi.Pack("keyringPolicyId")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackKeyringPolicyId is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd636b05f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function keyringPolicyId() view returns(uint256 policy)
func (paretoCDO *ParetoCDO) TryPackKeyringPolicyId() ([]byte, error) {
	return paretoCDO.abi.Pack("keyringPolicyId")
}

// UnpackKeyringPolicyId is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd636b05f.
//
// Solidity: function keyringPolicyId() view returns(uint256 policy)
func (paretoCDO *ParetoCDO) UnpackKeyringPolicyId(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("keyringPolicyId", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackOneToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa27eccc1.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function oneToken() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) PackOneToken() []byte {
	enc, err := paretoCDO.abi.Pack("oneToken")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackOneToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa27eccc1.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function oneToken() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) TryPackOneToken() ([]byte, error) {
	return paretoCDO.abi.Pack("oneToken")
}

// UnpackOneToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa27eccc1.
//
// Solidity: function oneToken() view returns(uint256 unit)
func (paretoCDO *ParetoCDO) UnpackOneToken(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("oneToken", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRequestWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xccc143b8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function requestWithdraw(uint256 amount, address tranche) returns(uint256 assets)
func (paretoCDO *ParetoCDO) PackRequestWithdraw(amount *big.Int, tranche common.Address) []byte {
	enc, err := paretoCDO.abi.Pack("requestWithdraw", amount, tranche)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRequestWithdraw is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xccc143b8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function requestWithdraw(uint256 amount, address tranche) returns(uint256 assets)
func (paretoCDO *ParetoCDO) TryPackRequestWithdraw(amount *big.Int, tranche common.Address) ([]byte, error) {
	return paretoCDO.abi.Pack("requestWithdraw", amount, tranche)
}

// UnpackRequestWithdraw is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xccc143b8.
//
// Solidity: function requestWithdraw(uint256 amount, address tranche) returns(uint256 assets)
func (paretoCDO *ParetoCDO) UnpackRequestWithdraw(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("requestWithdraw", data)
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
// Solidity: function strategy() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) PackStrategy() []byte {
	enc, err := paretoCDO.abi.Pack("strategy")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackStrategy is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa8c62e76.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function strategy() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) TryPackStrategy() ([]byte, error) {
	return paretoCDO.abi.Pack("strategy")
}

// UnpackStrategy is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa8c62e76.
//
// Solidity: function strategy() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) UnpackStrategy(data []byte) (common.Address, error) {
	out, err := paretoCDO.abi.Unpack("strategy", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackStrategyToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x747efea1.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function strategyToken() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) PackStrategyToken() []byte {
	enc, err := paretoCDO.abi.Pack("strategyToken")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackStrategyToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x747efea1.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function strategyToken() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) TryPackStrategyToken() ([]byte, error) {
	return paretoCDO.abi.Pack("strategyToken")
}

// UnpackStrategyToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x747efea1.
//
// Solidity: function strategyToken() view returns(address receiptToken)
func (paretoCDO *ParetoCDO) UnpackStrategyToken(data []byte) (common.Address, error) {
	out, err := paretoCDO.abi.Unpack("strategyToken", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc0c546a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function token() view returns(address underlying)
func (paretoCDO *ParetoCDO) PackToken() []byte {
	enc, err := paretoCDO.abi.Pack("token")
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
func (paretoCDO *ParetoCDO) TryPackToken() ([]byte, error) {
	return paretoCDO.abi.Pack("token")
}

// UnpackToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xfc0c546a.
//
// Solidity: function token() view returns(address underlying)
func (paretoCDO *ParetoCDO) UnpackToken(data []byte) (common.Address, error) {
	out, err := paretoCDO.abi.Unpack("token", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackVirtualPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9290d427.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function virtualPrice(address tranche) view returns(uint256 price)
func (paretoCDO *ParetoCDO) PackVirtualPrice(tranche common.Address) []byte {
	enc, err := paretoCDO.abi.Pack("virtualPrice", tranche)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackVirtualPrice is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9290d427.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function virtualPrice(address tranche) view returns(uint256 price)
func (paretoCDO *ParetoCDO) TryPackVirtualPrice(tranche common.Address) ([]byte, error) {
	return paretoCDO.abi.Pack("virtualPrice", tranche)
}

// UnpackVirtualPrice is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9290d427.
//
// Solidity: function virtualPrice(address tranche) view returns(uint256 price)
func (paretoCDO *ParetoCDO) UnpackVirtualPrice(data []byte) (*big.Int, error) {
	out, err := paretoCDO.abi.Unpack("virtualPrice", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
