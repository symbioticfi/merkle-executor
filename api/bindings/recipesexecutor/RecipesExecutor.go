// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package recipesexecutor

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

// IRecipesStep is an auto generated low-level Go binding around an user-defined struct.
type IRecipesStep struct {
	Operation uint8
	Data      []byte
}

// RecipesExecutorMetaData contains all meta data concerning the RecipesExecutor contract.
var RecipesExecutorMetaData = bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newAccount\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"account\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"caller\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"execute\",\"inputs\":[{\"name\":\"queries\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"},{\"name\":\"steps\",\"type\":\"tuple[]\",\"internalType\":\"structIRecipes.Step[]\",\"components\":[{\"name\":\"operation\",\"type\":\"uint8\",\"internalType\":\"enumIRecipes.Operation\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}]},{\"name\":\"inputs\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"},{\"name\":\"runtime\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"multicall\",\"inputs\":[{\"name\":\"data\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recipeBlock\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint48\",\"internalType\":\"uint48\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recipeHash\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setCaller\",\"inputs\":[{\"name\":\"newCaller\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setRecipe\",\"inputs\":[{\"name\":\"queries\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"},{\"name\":\"steps\",\"type\":\"tuple[]\",\"internalType\":\"structIRecipes.Step[]\",\"components\":[{\"name\":\"operation\",\"type\":\"uint8\",\"internalType\":\"enumIRecipes.Operation\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetCaller\",\"inputs\":[{\"name\":\"caller\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetRecipe\",\"inputs\":[{\"name\":\"recipeHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"queries\",\"type\":\"bytes[]\",\"indexed\":false,\"internalType\":\"bytes[]\"},{\"name\":\"steps\",\"type\":\"tuple[]\",\"indexed\":false,\"internalType\":\"structIRecipes.Step[]\",\"components\":[{\"name\":\"operation\",\"type\":\"uint8\",\"internalType\":\"enumIRecipes.Operation\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}]}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ApprovalFailed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CheckFailed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"FailedCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InsufficientBalance\",\"inputs\":[{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"InvalidAction\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCaller\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidDataMaskLength\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidOperation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidQuery\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidRecipe\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidReference\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidRuntime\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeCastOverflowedUintDowncast\",\"inputs\":[{\"name\":\"bits\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}]",
	ID:  "RecipesExecutor",
}

// RecipesExecutor is an auto generated Go binding around an Ethereum contract.
type RecipesExecutor struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *RecipesExecutor) GetABI() abi.ABI {
	return c.abi
}

// NewRecipesExecutor creates a new instance of RecipesExecutor.
func NewRecipesExecutor() *RecipesExecutor {
	parsed, err := RecipesExecutorMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &RecipesExecutor{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *RecipesExecutor) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackConstructor is the Go binding used to pack the parameters required for
// contract deployment.
//
// Solidity: constructor(address newOwner, address newAccount) returns()
func (recipesExecutor *RecipesExecutor) PackConstructor(newOwner common.Address, newAccount common.Address) []byte {
	enc, err := recipesExecutor.abi.Pack("", newOwner, newAccount)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackAccount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5dab2420.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function account() view returns(address)
func (recipesExecutor *RecipesExecutor) PackAccount() []byte {
	enc, err := recipesExecutor.abi.Pack("account")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAccount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5dab2420.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function account() view returns(address)
func (recipesExecutor *RecipesExecutor) TryPackAccount() ([]byte, error) {
	return recipesExecutor.abi.Pack("account")
}

// UnpackAccount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5dab2420.
//
// Solidity: function account() view returns(address)
func (recipesExecutor *RecipesExecutor) UnpackAccount(data []byte) (common.Address, error) {
	out, err := recipesExecutor.abi.Unpack("account", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackCaller is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc9c8d39.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function caller() view returns(address)
func (recipesExecutor *RecipesExecutor) PackCaller() []byte {
	enc, err := recipesExecutor.abi.Pack("caller")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCaller is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc9c8d39.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function caller() view returns(address)
func (recipesExecutor *RecipesExecutor) TryPackCaller() ([]byte, error) {
	return recipesExecutor.abi.Pack("caller")
}

// UnpackCaller is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xfc9c8d39.
//
// Solidity: function caller() view returns(address)
func (recipesExecutor *RecipesExecutor) UnpackCaller(data []byte) (common.Address, error) {
	out, err := recipesExecutor.abi.Unpack("caller", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x66036838.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function execute(bytes[] queries, (uint8,bytes)[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipesExecutor *RecipesExecutor) PackExecute(queries [][]byte, steps []IRecipesStep, inputs []*big.Int, runtime [][]byte) []byte {
	enc, err := recipesExecutor.abi.Pack("execute", queries, steps, inputs, runtime)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x66036838.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function execute(bytes[] queries, (uint8,bytes)[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipesExecutor *RecipesExecutor) TryPackExecute(queries [][]byte, steps []IRecipesStep, inputs []*big.Int, runtime [][]byte) ([]byte, error) {
	return recipesExecutor.abi.Pack("execute", queries, steps, inputs, runtime)
}

// UnpackExecute is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x66036838.
//
// Solidity: function execute(bytes[] queries, (uint8,bytes)[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipesExecutor *RecipesExecutor) UnpackExecute(data []byte) ([]*big.Int, error) {
	out, err := recipesExecutor.abi.Unpack("execute", data)
	if err != nil {
		return *new([]*big.Int), err
	}
	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)
	return out0, nil
}

// PackMulticall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xac9650d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function multicall(bytes[] data) returns()
func (recipesExecutor *RecipesExecutor) PackMulticall(data [][]byte) []byte {
	enc, err := recipesExecutor.abi.Pack("multicall", data)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMulticall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xac9650d8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function multicall(bytes[] data) returns()
func (recipesExecutor *RecipesExecutor) TryPackMulticall(data [][]byte) ([]byte, error) {
	return recipesExecutor.abi.Pack("multicall", data)
}

// PackOwner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8da5cb5b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function owner() view returns(address)
func (recipesExecutor *RecipesExecutor) PackOwner() []byte {
	enc, err := recipesExecutor.abi.Pack("owner")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackOwner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8da5cb5b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function owner() view returns(address)
func (recipesExecutor *RecipesExecutor) TryPackOwner() ([]byte, error) {
	return recipesExecutor.abi.Pack("owner")
}

// UnpackOwner is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (recipesExecutor *RecipesExecutor) UnpackOwner(data []byte) (common.Address, error) {
	out, err := recipesExecutor.abi.Unpack("owner", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRecipeBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x388081b5.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function recipeBlock() view returns(uint48)
func (recipesExecutor *RecipesExecutor) PackRecipeBlock() []byte {
	enc, err := recipesExecutor.abi.Pack("recipeBlock")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRecipeBlock is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x388081b5.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function recipeBlock() view returns(uint48)
func (recipesExecutor *RecipesExecutor) TryPackRecipeBlock() ([]byte, error) {
	return recipesExecutor.abi.Pack("recipeBlock")
}

// UnpackRecipeBlock is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x388081b5.
//
// Solidity: function recipeBlock() view returns(uint48)
func (recipesExecutor *RecipesExecutor) UnpackRecipeBlock(data []byte) (*big.Int, error) {
	out, err := recipesExecutor.abi.Unpack("recipeBlock", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRecipeHash is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0432b083.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function recipeHash() view returns(bytes32)
func (recipesExecutor *RecipesExecutor) PackRecipeHash() []byte {
	enc, err := recipesExecutor.abi.Pack("recipeHash")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRecipeHash is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0432b083.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function recipeHash() view returns(bytes32)
func (recipesExecutor *RecipesExecutor) TryPackRecipeHash() ([]byte, error) {
	return recipesExecutor.abi.Pack("recipeHash")
}

// UnpackRecipeHash is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0432b083.
//
// Solidity: function recipeHash() view returns(bytes32)
func (recipesExecutor *RecipesExecutor) UnpackRecipeHash(data []byte) ([32]byte, error) {
	out, err := recipesExecutor.abi.Unpack("recipeHash", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackRenounceOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x715018a6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function renounceOwnership() returns()
func (recipesExecutor *RecipesExecutor) PackRenounceOwnership() []byte {
	enc, err := recipesExecutor.abi.Pack("renounceOwnership")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRenounceOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x715018a6.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function renounceOwnership() returns()
func (recipesExecutor *RecipesExecutor) TryPackRenounceOwnership() ([]byte, error) {
	return recipesExecutor.abi.Pack("renounceOwnership")
}

// PackSetCaller is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbeb92f55.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setCaller(address newCaller) returns()
func (recipesExecutor *RecipesExecutor) PackSetCaller(newCaller common.Address) []byte {
	enc, err := recipesExecutor.abi.Pack("setCaller", newCaller)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetCaller is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbeb92f55.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setCaller(address newCaller) returns()
func (recipesExecutor *RecipesExecutor) TryPackSetCaller(newCaller common.Address) ([]byte, error) {
	return recipesExecutor.abi.Pack("setCaller", newCaller)
}

// PackSetRecipe is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf13f0417.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setRecipe(bytes[] queries, (uint8,bytes)[] steps) returns()
func (recipesExecutor *RecipesExecutor) PackSetRecipe(queries [][]byte, steps []IRecipesStep) []byte {
	enc, err := recipesExecutor.abi.Pack("setRecipe", queries, steps)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetRecipe is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf13f0417.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setRecipe(bytes[] queries, (uint8,bytes)[] steps) returns()
func (recipesExecutor *RecipesExecutor) TryPackSetRecipe(queries [][]byte, steps []IRecipesStep) ([]byte, error) {
	return recipesExecutor.abi.Pack("setRecipe", queries, steps)
}

// PackTransferOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2fde38b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (recipesExecutor *RecipesExecutor) PackTransferOwnership(newOwner common.Address) []byte {
	enc, err := recipesExecutor.abi.Pack("transferOwnership", newOwner)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackTransferOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2fde38b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (recipesExecutor *RecipesExecutor) TryPackTransferOwnership(newOwner common.Address) ([]byte, error) {
	return recipesExecutor.abi.Pack("transferOwnership", newOwner)
}

// RecipesExecutorOwnershipTransferred represents a OwnershipTransferred event raised by the RecipesExecutor contract.
type RecipesExecutorOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const RecipesExecutorOwnershipTransferredEventName = "OwnershipTransferred"

// ContractEventName returns the user-defined event name.
func (RecipesExecutorOwnershipTransferred) ContractEventName() string {
	return RecipesExecutorOwnershipTransferredEventName
}

// UnpackOwnershipTransferredEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (recipesExecutor *RecipesExecutor) UnpackOwnershipTransferredEvent(log *types.Log) (*RecipesExecutorOwnershipTransferred, error) {
	event := "OwnershipTransferred"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipesExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipesExecutorOwnershipTransferred)
	if len(log.Data) > 0 {
		if err := recipesExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipesExecutor.abi.Events[event].Inputs {
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

// RecipesExecutorSetCaller represents a SetCaller event raised by the RecipesExecutor contract.
type RecipesExecutorSetCaller struct {
	Caller common.Address
	Raw    *types.Log // Blockchain specific contextual infos
}

const RecipesExecutorSetCallerEventName = "SetCaller"

// ContractEventName returns the user-defined event name.
func (RecipesExecutorSetCaller) ContractEventName() string {
	return RecipesExecutorSetCallerEventName
}

// UnpackSetCallerEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetCaller(address indexed caller)
func (recipesExecutor *RecipesExecutor) UnpackSetCallerEvent(log *types.Log) (*RecipesExecutorSetCaller, error) {
	event := "SetCaller"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipesExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipesExecutorSetCaller)
	if len(log.Data) > 0 {
		if err := recipesExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipesExecutor.abi.Events[event].Inputs {
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

// RecipesExecutorSetRecipe represents a SetRecipe event raised by the RecipesExecutor contract.
type RecipesExecutorSetRecipe struct {
	RecipeHash [32]byte
	Queries    [][]byte
	Steps      []IRecipesStep
	Raw        *types.Log // Blockchain specific contextual infos
}

const RecipesExecutorSetRecipeEventName = "SetRecipe"

// ContractEventName returns the user-defined event name.
func (RecipesExecutorSetRecipe) ContractEventName() string {
	return RecipesExecutorSetRecipeEventName
}

// UnpackSetRecipeEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetRecipe(bytes32 indexed recipeHash, bytes[] queries, (uint8,bytes)[] steps)
func (recipesExecutor *RecipesExecutor) UnpackSetRecipeEvent(log *types.Log) (*RecipesExecutorSetRecipe, error) {
	event := "SetRecipe"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipesExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipesExecutorSetRecipe)
	if len(log.Data) > 0 {
		if err := recipesExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipesExecutor.abi.Events[event].Inputs {
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

// UnpackError attempts to decode the provided error data using user-defined
// error definitions.
func (recipesExecutor *RecipesExecutor) UnpackError(raw []byte) (any, error) {
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["AddressEmptyCode"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackAddressEmptyCodeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["ApprovalFailed"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackApprovalFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["CheckFailed"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackCheckFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["FailedCall"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackFailedCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InsufficientBalance"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInsufficientBalanceError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidAction"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidActionError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidCall"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidCaller"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidCallerError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidDataMaskLength"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidDataMaskLengthError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidOperation"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidOperationError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidQuery"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidQueryError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidRecipe"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidRecipeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidReference"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidReferenceError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["InvalidRuntime"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackInvalidRuntimeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["OwnableInvalidOwner"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackOwnableInvalidOwnerError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["OwnableUnauthorizedAccount"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackOwnableUnauthorizedAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["ReentrancyGuardReentrantCall"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackReentrancyGuardReentrantCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipesExecutor.abi.Errors["SafeCastOverflowedUintDowncast"].ID.Bytes()[:4]) {
		return recipesExecutor.UnpackSafeCastOverflowedUintDowncastError(raw[4:])
	}
	return nil, errors.New("Unknown error")
}

// RecipesExecutorAddressEmptyCode represents a AddressEmptyCode error raised by the RecipesExecutor contract.
type RecipesExecutorAddressEmptyCode struct {
	Target common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AddressEmptyCode(address target)
func RecipesExecutorAddressEmptyCodeErrorID() common.Hash {
	return common.HexToHash("0x9996b315c842ff135b8fc4a08ad5df1c344efbc03d2687aecc0678050d2aac89")
}

// UnpackAddressEmptyCodeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AddressEmptyCode(address target)
func (recipesExecutor *RecipesExecutor) UnpackAddressEmptyCodeError(raw []byte) (*RecipesExecutorAddressEmptyCode, error) {
	out := new(RecipesExecutorAddressEmptyCode)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "AddressEmptyCode", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorApprovalFailed represents a ApprovalFailed error raised by the RecipesExecutor contract.
type RecipesExecutorApprovalFailed struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ApprovalFailed()
func RecipesExecutorApprovalFailedErrorID() common.Hash {
	return common.HexToHash("0x8164f8420f269e65f628de89f74bfefdaed46f99c50d22c6a87ba81966095c96")
}

// UnpackApprovalFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ApprovalFailed()
func (recipesExecutor *RecipesExecutor) UnpackApprovalFailedError(raw []byte) (*RecipesExecutorApprovalFailed, error) {
	out := new(RecipesExecutorApprovalFailed)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "ApprovalFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorCheckFailed represents a CheckFailed error raised by the RecipesExecutor contract.
type RecipesExecutorCheckFailed struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error CheckFailed()
func RecipesExecutorCheckFailedErrorID() common.Hash {
	return common.HexToHash("0xd7c1c92f5be804090974ee16e347e0dcf86078058f5840e4a58018e93693ce2b")
}

// UnpackCheckFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error CheckFailed()
func (recipesExecutor *RecipesExecutor) UnpackCheckFailedError(raw []byte) (*RecipesExecutorCheckFailed, error) {
	out := new(RecipesExecutorCheckFailed)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "CheckFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorFailedCall represents a FailedCall error raised by the RecipesExecutor contract.
type RecipesExecutorFailedCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error FailedCall()
func RecipesExecutorFailedCallErrorID() common.Hash {
	return common.HexToHash("0xd6bda27508c0fb6d8a39b4b122878dab26f731a7d4e4abe711dd3731899052a4")
}

// UnpackFailedCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error FailedCall()
func (recipesExecutor *RecipesExecutor) UnpackFailedCallError(raw []byte) (*RecipesExecutorFailedCall, error) {
	out := new(RecipesExecutorFailedCall)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "FailedCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInsufficientBalance represents a InsufficientBalance error raised by the RecipesExecutor contract.
type RecipesExecutorInsufficientBalance struct {
	Balance *big.Int
	Needed  *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func RecipesExecutorInsufficientBalanceErrorID() common.Hash {
	return common.HexToHash("0xcf4791818fba6e019216eb4864093b4947f674afada5d305e57d598b641dad1d")
}

// UnpackInsufficientBalanceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func (recipesExecutor *RecipesExecutor) UnpackInsufficientBalanceError(raw []byte) (*RecipesExecutorInsufficientBalance, error) {
	out := new(RecipesExecutorInsufficientBalance)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InsufficientBalance", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidAction represents a InvalidAction error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidAction struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidAction()
func RecipesExecutorInvalidActionErrorID() common.Hash {
	return common.HexToHash("0x4a7f394f1e5f6a837042ccf878bb1bb65701a39b8d0b01821669683cca399ae2")
}

// UnpackInvalidActionError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidAction()
func (recipesExecutor *RecipesExecutor) UnpackInvalidActionError(raw []byte) (*RecipesExecutorInvalidAction, error) {
	out := new(RecipesExecutorInvalidAction)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidAction", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidCall represents a InvalidCall error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCall()
func RecipesExecutorInvalidCallErrorID() common.Hash {
	return common.HexToHash("0xae962d4e06a5f2158f022ce0e8331adb6b53d60eb043c7902e2eaa3586c6a4fb")
}

// UnpackInvalidCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCall()
func (recipesExecutor *RecipesExecutor) UnpackInvalidCallError(raw []byte) (*RecipesExecutorInvalidCall, error) {
	out := new(RecipesExecutorInvalidCall)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidCaller represents a InvalidCaller error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidCaller struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCaller()
func RecipesExecutorInvalidCallerErrorID() common.Hash {
	return common.HexToHash("0x48f5c3ed50241a1b6c87d204a25d9d01339cd768de9a714ffbb53a5bb6ad572a")
}

// UnpackInvalidCallerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCaller()
func (recipesExecutor *RecipesExecutor) UnpackInvalidCallerError(raw []byte) (*RecipesExecutorInvalidCaller, error) {
	out := new(RecipesExecutorInvalidCaller)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidCaller", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidDataMaskLength represents a InvalidDataMaskLength error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidDataMaskLength struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidDataMaskLength()
func RecipesExecutorInvalidDataMaskLengthErrorID() common.Hash {
	return common.HexToHash("0x3609cb6625464291d6101e8be49be0729d37a57ea489cb9e041f9a5f86c64336")
}

// UnpackInvalidDataMaskLengthError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidDataMaskLength()
func (recipesExecutor *RecipesExecutor) UnpackInvalidDataMaskLengthError(raw []byte) (*RecipesExecutorInvalidDataMaskLength, error) {
	out := new(RecipesExecutorInvalidDataMaskLength)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidDataMaskLength", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidOperation represents a InvalidOperation error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidOperation struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidOperation()
func RecipesExecutorInvalidOperationErrorID() common.Hash {
	return common.HexToHash("0x398d4d32963991cf02978f96f1a4d81787345d957c0995bedfe70af66ce594fd")
}

// UnpackInvalidOperationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidOperation()
func (recipesExecutor *RecipesExecutor) UnpackInvalidOperationError(raw []byte) (*RecipesExecutorInvalidOperation, error) {
	out := new(RecipesExecutorInvalidOperation)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidOperation", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidQuery represents a InvalidQuery error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidQuery struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidQuery()
func RecipesExecutorInvalidQueryErrorID() common.Hash {
	return common.HexToHash("0x19e4cb1093499c0994f1c698c47ae2ed09e8ac37618f650c826923074fd6d526")
}

// UnpackInvalidQueryError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidQuery()
func (recipesExecutor *RecipesExecutor) UnpackInvalidQueryError(raw []byte) (*RecipesExecutorInvalidQuery, error) {
	out := new(RecipesExecutorInvalidQuery)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidQuery", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidRecipe represents a InvalidRecipe error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidRecipe struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidRecipe()
func RecipesExecutorInvalidRecipeErrorID() common.Hash {
	return common.HexToHash("0x7b776f4ceee4fd127ffa0880f638d4f034d331c1a2605609383fd7c68a818bc0")
}

// UnpackInvalidRecipeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidRecipe()
func (recipesExecutor *RecipesExecutor) UnpackInvalidRecipeError(raw []byte) (*RecipesExecutorInvalidRecipe, error) {
	out := new(RecipesExecutorInvalidRecipe)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidRecipe", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidReference represents a InvalidReference error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidReference struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidReference()
func RecipesExecutorInvalidReferenceErrorID() common.Hash {
	return common.HexToHash("0x5f41ff9294f1a1a14da42dbeef661318e70ed969230837ec353d3d643d6946e1")
}

// UnpackInvalidReferenceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidReference()
func (recipesExecutor *RecipesExecutor) UnpackInvalidReferenceError(raw []byte) (*RecipesExecutorInvalidReference, error) {
	out := new(RecipesExecutorInvalidReference)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidReference", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorInvalidRuntime represents a InvalidRuntime error raised by the RecipesExecutor contract.
type RecipesExecutorInvalidRuntime struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidRuntime()
func RecipesExecutorInvalidRuntimeErrorID() common.Hash {
	return common.HexToHash("0x02e22e6c69a39fc2d7cb34bccdcd940b08139115bb1d4d1dc335d8086d0dccde")
}

// UnpackInvalidRuntimeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidRuntime()
func (recipesExecutor *RecipesExecutor) UnpackInvalidRuntimeError(raw []byte) (*RecipesExecutorInvalidRuntime, error) {
	out := new(RecipesExecutorInvalidRuntime)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "InvalidRuntime", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorOwnableInvalidOwner represents a OwnableInvalidOwner error raised by the RecipesExecutor contract.
type RecipesExecutorOwnableInvalidOwner struct {
	Owner common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableInvalidOwner(address owner)
func RecipesExecutorOwnableInvalidOwnerErrorID() common.Hash {
	return common.HexToHash("0x1e4fbdf7f3ef8bcaa855599e3abf48b232380f183f08f6f813d9ffa5bd585188")
}

// UnpackOwnableInvalidOwnerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableInvalidOwner(address owner)
func (recipesExecutor *RecipesExecutor) UnpackOwnableInvalidOwnerError(raw []byte) (*RecipesExecutorOwnableInvalidOwner, error) {
	out := new(RecipesExecutorOwnableInvalidOwner)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "OwnableInvalidOwner", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorOwnableUnauthorizedAccount represents a OwnableUnauthorizedAccount error raised by the RecipesExecutor contract.
type RecipesExecutorOwnableUnauthorizedAccount struct {
	Account common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func RecipesExecutorOwnableUnauthorizedAccountErrorID() common.Hash {
	return common.HexToHash("0x118cdaa7a341953d1887a2245fd6665d741c67c8c50581daa59e1d03373fa188")
}

// UnpackOwnableUnauthorizedAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func (recipesExecutor *RecipesExecutor) UnpackOwnableUnauthorizedAccountError(raw []byte) (*RecipesExecutorOwnableUnauthorizedAccount, error) {
	out := new(RecipesExecutorOwnableUnauthorizedAccount)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "OwnableUnauthorizedAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorReentrancyGuardReentrantCall represents a ReentrancyGuardReentrantCall error raised by the RecipesExecutor contract.
type RecipesExecutorReentrancyGuardReentrantCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ReentrancyGuardReentrantCall()
func RecipesExecutorReentrancyGuardReentrantCallErrorID() common.Hash {
	return common.HexToHash("0x3ee5aeb571de7fc460830b4d0017439a1ca56fb0bc39062227ade4fe4a24c1ca")
}

// UnpackReentrancyGuardReentrantCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ReentrancyGuardReentrantCall()
func (recipesExecutor *RecipesExecutor) UnpackReentrancyGuardReentrantCallError(raw []byte) (*RecipesExecutorReentrancyGuardReentrantCall, error) {
	out := new(RecipesExecutorReentrancyGuardReentrantCall)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "ReentrancyGuardReentrantCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipesExecutorSafeCastOverflowedUintDowncast represents a SafeCastOverflowedUintDowncast error raised by the RecipesExecutor contract.
type RecipesExecutorSafeCastOverflowedUintDowncast struct {
	Bits  uint8
	Value *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error SafeCastOverflowedUintDowncast(uint8 bits, uint256 value)
func RecipesExecutorSafeCastOverflowedUintDowncastErrorID() common.Hash {
	return common.HexToHash("0x6dfcc6503a32754ce7a89698e18201fc5294fd4aad43edefee786f88423b1a12")
}

// UnpackSafeCastOverflowedUintDowncastError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error SafeCastOverflowedUintDowncast(uint8 bits, uint256 value)
func (recipesExecutor *RecipesExecutor) UnpackSafeCastOverflowedUintDowncastError(raw []byte) (*RecipesExecutorSafeCastOverflowedUintDowncast, error) {
	out := new(RecipesExecutorSafeCastOverflowedUintDowncast)
	if err := recipesExecutor.abi.UnpackIntoInterface(out, "SafeCastOverflowedUintDowncast", raw); err != nil {
		return nil, err
	}
	return out, nil
}
