// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package noonredeemhandlerv2

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

// NoonRedeemHandlerV2MetaData contains all meta data concerning the NoonRedeemHandlerV2 contract.
var NoonRedeemHandlerV2MetaData = bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"collateral\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"usnAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"collateralAmount\",\"type\":\"uint256\"}],\"name\":\"Redeemed\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"collateral\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"usnAmount\",\"type\":\"uint256\"}],\"name\":\"calculateCollateralAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"collateralDecimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"currentBlockRedeemAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"}],\"name\":\"isWhitelisted\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastRedeemBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"redeemLimitPerBlock\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"collateralAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"usnAmount\",\"type\":\"uint256\"}],\"name\":\"redeemOnchain\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"collateral\",\"type\":\"address\"}],\"name\":\"redeemableCollaterals\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"treasury\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usnToken\",\"outputs\":[{\"internalType\":\"contractERC20Burnable\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	ID:  "NoonRedeemHandlerV2",
}

// NoonRedeemHandlerV2 is an auto generated Go binding around an Ethereum contract.
type NoonRedeemHandlerV2 struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *NoonRedeemHandlerV2) GetABI() abi.ABI {
	return c.abi
}

// NewNoonRedeemHandlerV2 creates a new instance of NoonRedeemHandlerV2.
func NewNoonRedeemHandlerV2() *NoonRedeemHandlerV2 {
	parsed, err := NoonRedeemHandlerV2MetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &NoonRedeemHandlerV2{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *NoonRedeemHandlerV2) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackCalculateCollateralAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x034cc760.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function calculateCollateralAmount(address collateral, uint256 usnAmount) view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackCalculateCollateralAmount(collateral common.Address, usnAmount *big.Int) []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("calculateCollateralAmount", collateral, usnAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCalculateCollateralAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x034cc760.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function calculateCollateralAmount(address collateral, uint256 usnAmount) view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackCalculateCollateralAmount(collateral common.Address, usnAmount *big.Int) ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("calculateCollateralAmount", collateral, usnAmount)
}

// UnpackCalculateCollateralAmount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x034cc760.
//
// Solidity: function calculateCollateralAmount(address collateral, uint256 usnAmount) view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackCalculateCollateralAmount(data []byte) (*big.Int, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("calculateCollateralAmount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackCollateralDecimals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf1ae3c7f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function collateralDecimals(address ) view returns(uint8)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackCollateralDecimals(arg0 common.Address) []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("collateralDecimals", arg0)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCollateralDecimals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf1ae3c7f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function collateralDecimals(address ) view returns(uint8)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackCollateralDecimals(arg0 common.Address) ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("collateralDecimals", arg0)
}

// UnpackCollateralDecimals is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf1ae3c7f.
//
// Solidity: function collateralDecimals(address ) view returns(uint8)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackCollateralDecimals(data []byte) (uint8, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("collateralDecimals", data)
	if err != nil {
		return *new(uint8), err
	}
	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)
	return out0, nil
}

// PackCurrentBlockRedeemAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe38aeafc.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function currentBlockRedeemAmount() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackCurrentBlockRedeemAmount() []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("currentBlockRedeemAmount")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCurrentBlockRedeemAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe38aeafc.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function currentBlockRedeemAmount() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackCurrentBlockRedeemAmount() ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("currentBlockRedeemAmount")
}

// UnpackCurrentBlockRedeemAmount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe38aeafc.
//
// Solidity: function currentBlockRedeemAmount() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackCurrentBlockRedeemAmount(data []byte) (*big.Int, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("currentBlockRedeemAmount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackIsWhitelisted is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x3af32abf.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isWhitelisted(address user) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackIsWhitelisted(user common.Address) []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("isWhitelisted", user)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsWhitelisted is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x3af32abf.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isWhitelisted(address user) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackIsWhitelisted(user common.Address) ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("isWhitelisted", user)
}

// UnpackIsWhitelisted is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x3af32abf.
//
// Solidity: function isWhitelisted(address user) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackIsWhitelisted(data []byte) (bool, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("isWhitelisted", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackLastRedeemBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7e7920df.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function lastRedeemBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackLastRedeemBlock() []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("lastRedeemBlock")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackLastRedeemBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7e7920df.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function lastRedeemBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackLastRedeemBlock() ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("lastRedeemBlock")
}

// UnpackLastRedeemBlock is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x7e7920df.
//
// Solidity: function lastRedeemBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackLastRedeemBlock(data []byte) (*big.Int, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("lastRedeemBlock", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemLimitPerBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x922ba4c5.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemLimitPerBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackRedeemLimitPerBlock() []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("redeemLimitPerBlock")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemLimitPerBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x922ba4c5.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemLimitPerBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackRedeemLimitPerBlock() ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("redeemLimitPerBlock")
}

// UnpackRedeemLimitPerBlock is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x922ba4c5.
//
// Solidity: function redeemLimitPerBlock() view returns(uint256)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackRedeemLimitPerBlock(data []byte) (*big.Int, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("redeemLimitPerBlock", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemOnchain is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1b903678.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemOnchain(address collateralAddress, uint256 usnAmount) returns()
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackRedeemOnchain(collateralAddress common.Address, usnAmount *big.Int) []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("redeemOnchain", collateralAddress, usnAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemOnchain is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1b903678.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemOnchain(address collateralAddress, uint256 usnAmount) returns()
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackRedeemOnchain(collateralAddress common.Address, usnAmount *big.Int) ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("redeemOnchain", collateralAddress, usnAmount)
}

// PackRedeemableCollaterals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9b308e3.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemableCollaterals(address collateral) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackRedeemableCollaterals(collateral common.Address) []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("redeemableCollaterals", collateral)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemableCollaterals is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9b308e3.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemableCollaterals(address collateral) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackRedeemableCollaterals(collateral common.Address) ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("redeemableCollaterals", collateral)
}

// UnpackRedeemableCollaterals is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc9b308e3.
//
// Solidity: function redeemableCollaterals(address collateral) view returns(bool)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackRedeemableCollaterals(data []byte) (bool, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("redeemableCollaterals", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackTreasury is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x61d027b3.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function treasury() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackTreasury() []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("treasury")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackTreasury is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x61d027b3.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function treasury() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackTreasury() ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("treasury")
}

// UnpackTreasury is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x61d027b3.
//
// Solidity: function treasury() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackTreasury(data []byte) (common.Address, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("treasury", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackUsnToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x955ff78f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function usnToken() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) PackUsnToken() []byte {
	enc, err := noonRedeemHandlerV2.abi.Pack("usnToken")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackUsnToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x955ff78f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function usnToken() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) TryPackUsnToken() ([]byte, error) {
	return noonRedeemHandlerV2.abi.Pack("usnToken")
}

// UnpackUsnToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x955ff78f.
//
// Solidity: function usnToken() view returns(address)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackUsnToken(data []byte) (common.Address, error) {
	out, err := noonRedeemHandlerV2.abi.Unpack("usnToken", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// NoonRedeemHandlerV2Redeemed represents a Redeemed event raised by the NoonRedeemHandlerV2 contract.
type NoonRedeemHandlerV2Redeemed struct {
	From             common.Address
	Collateral       common.Address
	UsnAmount        *big.Int
	CollateralAmount *big.Int
	Raw              *types.Log // Blockchain specific contextual infos
}

const NoonRedeemHandlerV2RedeemedEventName = "Redeemed"

// ContractEventName returns the user-defined event name.
func (NoonRedeemHandlerV2Redeemed) ContractEventName() string {
	return NoonRedeemHandlerV2RedeemedEventName
}

// UnpackRedeemedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Redeemed(address indexed from, address indexed collateral, uint256 usnAmount, uint256 collateralAmount)
func (noonRedeemHandlerV2 *NoonRedeemHandlerV2) UnpackRedeemedEvent(log *types.Log) (*NoonRedeemHandlerV2Redeemed, error) {
	event := "Redeemed"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != noonRedeemHandlerV2.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(NoonRedeemHandlerV2Redeemed)
	if len(log.Data) > 0 {
		if err := noonRedeemHandlerV2.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range noonRedeemHandlerV2.abi.Events[event].Inputs {
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
