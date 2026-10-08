// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package recipeexecutor

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

// RecipeExecutorMetaData contains all meta data concerning the RecipeExecutor contract.
var RecipeExecutorMetaData = bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newAccount\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"account\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"caller\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"execute\",\"inputs\":[{\"name\":\"queries\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"},{\"name\":\"steps\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"},{\"name\":\"inputs\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"},{\"name\":\"runtime\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"multicall\",\"inputs\":[{\"name\":\"data\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recipeBlock\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint48\",\"internalType\":\"uint48\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recipeHash\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setCaller\",\"inputs\":[{\"name\":\"newCaller\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setRecipe\",\"inputs\":[{\"name\":\"newQueries\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"},{\"name\":\"newSteps\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetCaller\",\"inputs\":[{\"name\":\"caller\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetRecipe\",\"inputs\":[{\"name\":\"recipeHash\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"queries\",\"type\":\"bytes[]\",\"indexed\":false,\"internalType\":\"bytes[]\"},{\"name\":\"steps\",\"type\":\"bytes[]\",\"indexed\":false,\"internalType\":\"bytes[]\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ApprovalFailed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CheckFailed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"FailedCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InsufficientBalance\",\"inputs\":[{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"InvalidAction\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCaller\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidDataMaskLength\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidOperation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidQuery\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidRecipe\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidReference\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidRuntime\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]}]",
	ID:  "RecipeExecutor",
}

// RecipeExecutor is an auto generated Go binding around an Ethereum contract.
type RecipeExecutor struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *RecipeExecutor) GetABI() abi.ABI {
	return c.abi
}

// NewRecipeExecutor creates a new instance of RecipeExecutor.
func NewRecipeExecutor() *RecipeExecutor {
	parsed, err := RecipeExecutorMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &RecipeExecutor{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *RecipeExecutor) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackConstructor is the Go binding used to pack the parameters required for
// contract deployment.
//
// Solidity: constructor(address newOwner, address newAccount) returns()
func (recipeExecutor *RecipeExecutor) PackConstructor(newOwner common.Address, newAccount common.Address) []byte {
	enc, err := recipeExecutor.abi.Pack("", newOwner, newAccount)
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
func (recipeExecutor *RecipeExecutor) PackAccount() []byte {
	enc, err := recipeExecutor.abi.Pack("account")
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
func (recipeExecutor *RecipeExecutor) TryPackAccount() ([]byte, error) {
	return recipeExecutor.abi.Pack("account")
}

// UnpackAccount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5dab2420.
//
// Solidity: function account() view returns(address)
func (recipeExecutor *RecipeExecutor) UnpackAccount(data []byte) (common.Address, error) {
	out, err := recipeExecutor.abi.Unpack("account", data)
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
func (recipeExecutor *RecipeExecutor) PackCaller() []byte {
	enc, err := recipeExecutor.abi.Pack("caller")
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
func (recipeExecutor *RecipeExecutor) TryPackCaller() ([]byte, error) {
	return recipeExecutor.abi.Pack("caller")
}

// UnpackCaller is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xfc9c8d39.
//
// Solidity: function caller() view returns(address)
func (recipeExecutor *RecipeExecutor) UnpackCaller(data []byte) (common.Address, error) {
	out, err := recipeExecutor.abi.Unpack("caller", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc08a8760.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function execute(bytes[] queries, bytes[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipeExecutor *RecipeExecutor) PackExecute(queries [][]byte, steps [][]byte, inputs []*big.Int, runtime [][]byte) []byte {
	enc, err := recipeExecutor.abi.Pack("execute", queries, steps, inputs, runtime)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackExecute is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc08a8760.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function execute(bytes[] queries, bytes[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipeExecutor *RecipeExecutor) TryPackExecute(queries [][]byte, steps [][]byte, inputs []*big.Int, runtime [][]byte) ([]byte, error) {
	return recipeExecutor.abi.Pack("execute", queries, steps, inputs, runtime)
}

// UnpackExecute is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc08a8760.
//
// Solidity: function execute(bytes[] queries, bytes[] steps, uint256[] inputs, bytes[] runtime) returns(uint256[])
func (recipeExecutor *RecipeExecutor) UnpackExecute(data []byte) ([]*big.Int, error) {
	out, err := recipeExecutor.abi.Unpack("execute", data)
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
func (recipeExecutor *RecipeExecutor) PackMulticall(data [][]byte) []byte {
	enc, err := recipeExecutor.abi.Pack("multicall", data)
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
func (recipeExecutor *RecipeExecutor) TryPackMulticall(data [][]byte) ([]byte, error) {
	return recipeExecutor.abi.Pack("multicall", data)
}

// PackOwner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8da5cb5b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function owner() view returns(address)
func (recipeExecutor *RecipeExecutor) PackOwner() []byte {
	enc, err := recipeExecutor.abi.Pack("owner")
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
func (recipeExecutor *RecipeExecutor) TryPackOwner() ([]byte, error) {
	return recipeExecutor.abi.Pack("owner")
}

// UnpackOwner is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (recipeExecutor *RecipeExecutor) UnpackOwner(data []byte) (common.Address, error) {
	out, err := recipeExecutor.abi.Unpack("owner", data)
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
func (recipeExecutor *RecipeExecutor) PackRecipeBlock() []byte {
	enc, err := recipeExecutor.abi.Pack("recipeBlock")
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
func (recipeExecutor *RecipeExecutor) TryPackRecipeBlock() ([]byte, error) {
	return recipeExecutor.abi.Pack("recipeBlock")
}

// UnpackRecipeBlock is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x388081b5.
//
// Solidity: function recipeBlock() view returns(uint48)
func (recipeExecutor *RecipeExecutor) UnpackRecipeBlock(data []byte) (*big.Int, error) {
	out, err := recipeExecutor.abi.Unpack("recipeBlock", data)
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
func (recipeExecutor *RecipeExecutor) PackRecipeHash() []byte {
	enc, err := recipeExecutor.abi.Pack("recipeHash")
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
func (recipeExecutor *RecipeExecutor) TryPackRecipeHash() ([]byte, error) {
	return recipeExecutor.abi.Pack("recipeHash")
}

// UnpackRecipeHash is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0432b083.
//
// Solidity: function recipeHash() view returns(bytes32)
func (recipeExecutor *RecipeExecutor) UnpackRecipeHash(data []byte) ([32]byte, error) {
	out, err := recipeExecutor.abi.Unpack("recipeHash", data)
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
func (recipeExecutor *RecipeExecutor) PackRenounceOwnership() []byte {
	enc, err := recipeExecutor.abi.Pack("renounceOwnership")
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
func (recipeExecutor *RecipeExecutor) TryPackRenounceOwnership() ([]byte, error) {
	return recipeExecutor.abi.Pack("renounceOwnership")
}

// PackSetCaller is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xbeb92f55.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setCaller(address newCaller) returns()
func (recipeExecutor *RecipeExecutor) PackSetCaller(newCaller common.Address) []byte {
	enc, err := recipeExecutor.abi.Pack("setCaller", newCaller)
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
func (recipeExecutor *RecipeExecutor) TryPackSetCaller(newCaller common.Address) ([]byte, error) {
	return recipeExecutor.abi.Pack("setCaller", newCaller)
}

// PackSetRecipe is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9559aab5.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setRecipe(bytes[] newQueries, bytes[] newSteps) returns()
func (recipeExecutor *RecipeExecutor) PackSetRecipe(newQueries [][]byte, newSteps [][]byte) []byte {
	enc, err := recipeExecutor.abi.Pack("setRecipe", newQueries, newSteps)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetRecipe is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9559aab5.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setRecipe(bytes[] newQueries, bytes[] newSteps) returns()
func (recipeExecutor *RecipeExecutor) TryPackSetRecipe(newQueries [][]byte, newSteps [][]byte) ([]byte, error) {
	return recipeExecutor.abi.Pack("setRecipe", newQueries, newSteps)
}

// PackTransferOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2fde38b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (recipeExecutor *RecipeExecutor) PackTransferOwnership(newOwner common.Address) []byte {
	enc, err := recipeExecutor.abi.Pack("transferOwnership", newOwner)
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
func (recipeExecutor *RecipeExecutor) TryPackTransferOwnership(newOwner common.Address) ([]byte, error) {
	return recipeExecutor.abi.Pack("transferOwnership", newOwner)
}

// RecipeExecutorOwnershipTransferred represents a OwnershipTransferred event raised by the RecipeExecutor contract.
type RecipeExecutorOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const RecipeExecutorOwnershipTransferredEventName = "OwnershipTransferred"

// ContractEventName returns the user-defined event name.
func (RecipeExecutorOwnershipTransferred) ContractEventName() string {
	return RecipeExecutorOwnershipTransferredEventName
}

// UnpackOwnershipTransferredEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (recipeExecutor *RecipeExecutor) UnpackOwnershipTransferredEvent(log *types.Log) (*RecipeExecutorOwnershipTransferred, error) {
	event := "OwnershipTransferred"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipeExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipeExecutorOwnershipTransferred)
	if len(log.Data) > 0 {
		if err := recipeExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipeExecutor.abi.Events[event].Inputs {
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

// RecipeExecutorSetCaller represents a SetCaller event raised by the RecipeExecutor contract.
type RecipeExecutorSetCaller struct {
	Caller common.Address
	Raw    *types.Log // Blockchain specific contextual infos
}

const RecipeExecutorSetCallerEventName = "SetCaller"

// ContractEventName returns the user-defined event name.
func (RecipeExecutorSetCaller) ContractEventName() string {
	return RecipeExecutorSetCallerEventName
}

// UnpackSetCallerEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetCaller(address indexed caller)
func (recipeExecutor *RecipeExecutor) UnpackSetCallerEvent(log *types.Log) (*RecipeExecutorSetCaller, error) {
	event := "SetCaller"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipeExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipeExecutorSetCaller)
	if len(log.Data) > 0 {
		if err := recipeExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipeExecutor.abi.Events[event].Inputs {
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

// RecipeExecutorSetRecipe represents a SetRecipe event raised by the RecipeExecutor contract.
type RecipeExecutorSetRecipe struct {
	RecipeHash [32]byte
	Queries    [][]byte
	Steps      [][]byte
	Raw        *types.Log // Blockchain specific contextual infos
}

const RecipeExecutorSetRecipeEventName = "SetRecipe"

// ContractEventName returns the user-defined event name.
func (RecipeExecutorSetRecipe) ContractEventName() string {
	return RecipeExecutorSetRecipeEventName
}

// UnpackSetRecipeEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetRecipe(bytes32 indexed recipeHash, bytes[] queries, bytes[] steps)
func (recipeExecutor *RecipeExecutor) UnpackSetRecipeEvent(log *types.Log) (*RecipeExecutorSetRecipe, error) {
	event := "SetRecipe"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != recipeExecutor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(RecipeExecutorSetRecipe)
	if len(log.Data) > 0 {
		if err := recipeExecutor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range recipeExecutor.abi.Events[event].Inputs {
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
func (recipeExecutor *RecipeExecutor) UnpackError(raw []byte) (any, error) {
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["AddressEmptyCode"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackAddressEmptyCodeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["ApprovalFailed"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackApprovalFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["CheckFailed"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackCheckFailedError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["FailedCall"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackFailedCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InsufficientBalance"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInsufficientBalanceError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidAction"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidActionError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidCall"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidCaller"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidCallerError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidDataMaskLength"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidDataMaskLengthError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidOperation"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidOperationError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidQuery"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidQueryError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidRecipe"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidRecipeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidReference"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidReferenceError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["InvalidRuntime"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackInvalidRuntimeError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["OwnableInvalidOwner"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackOwnableInvalidOwnerError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["OwnableUnauthorizedAccount"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackOwnableUnauthorizedAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], recipeExecutor.abi.Errors["ReentrancyGuardReentrantCall"].ID.Bytes()[:4]) {
		return recipeExecutor.UnpackReentrancyGuardReentrantCallError(raw[4:])
	}
	return nil, errors.New("Unknown error")
}

// RecipeExecutorAddressEmptyCode represents a AddressEmptyCode error raised by the RecipeExecutor contract.
type RecipeExecutorAddressEmptyCode struct {
	Target common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AddressEmptyCode(address target)
func RecipeExecutorAddressEmptyCodeErrorID() common.Hash {
	return common.HexToHash("0x9996b315c842ff135b8fc4a08ad5df1c344efbc03d2687aecc0678050d2aac89")
}

// UnpackAddressEmptyCodeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AddressEmptyCode(address target)
func (recipeExecutor *RecipeExecutor) UnpackAddressEmptyCodeError(raw []byte) (*RecipeExecutorAddressEmptyCode, error) {
	out := new(RecipeExecutorAddressEmptyCode)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "AddressEmptyCode", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorApprovalFailed represents a ApprovalFailed error raised by the RecipeExecutor contract.
type RecipeExecutorApprovalFailed struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ApprovalFailed()
func RecipeExecutorApprovalFailedErrorID() common.Hash {
	return common.HexToHash("0x8164f8420f269e65f628de89f74bfefdaed46f99c50d22c6a87ba81966095c96")
}

// UnpackApprovalFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ApprovalFailed()
func (recipeExecutor *RecipeExecutor) UnpackApprovalFailedError(raw []byte) (*RecipeExecutorApprovalFailed, error) {
	out := new(RecipeExecutorApprovalFailed)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "ApprovalFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorCheckFailed represents a CheckFailed error raised by the RecipeExecutor contract.
type RecipeExecutorCheckFailed struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error CheckFailed()
func RecipeExecutorCheckFailedErrorID() common.Hash {
	return common.HexToHash("0xd7c1c92f5be804090974ee16e347e0dcf86078058f5840e4a58018e93693ce2b")
}

// UnpackCheckFailedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error CheckFailed()
func (recipeExecutor *RecipeExecutor) UnpackCheckFailedError(raw []byte) (*RecipeExecutorCheckFailed, error) {
	out := new(RecipeExecutorCheckFailed)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "CheckFailed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorFailedCall represents a FailedCall error raised by the RecipeExecutor contract.
type RecipeExecutorFailedCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error FailedCall()
func RecipeExecutorFailedCallErrorID() common.Hash {
	return common.HexToHash("0xd6bda27508c0fb6d8a39b4b122878dab26f731a7d4e4abe711dd3731899052a4")
}

// UnpackFailedCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error FailedCall()
func (recipeExecutor *RecipeExecutor) UnpackFailedCallError(raw []byte) (*RecipeExecutorFailedCall, error) {
	out := new(RecipeExecutorFailedCall)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "FailedCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInsufficientBalance represents a InsufficientBalance error raised by the RecipeExecutor contract.
type RecipeExecutorInsufficientBalance struct {
	Balance *big.Int
	Needed  *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func RecipeExecutorInsufficientBalanceErrorID() common.Hash {
	return common.HexToHash("0xcf4791818fba6e019216eb4864093b4947f674afada5d305e57d598b641dad1d")
}

// UnpackInsufficientBalanceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func (recipeExecutor *RecipeExecutor) UnpackInsufficientBalanceError(raw []byte) (*RecipeExecutorInsufficientBalance, error) {
	out := new(RecipeExecutorInsufficientBalance)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InsufficientBalance", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidAction represents a InvalidAction error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidAction struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidAction()
func RecipeExecutorInvalidActionErrorID() common.Hash {
	return common.HexToHash("0x4a7f394f1e5f6a837042ccf878bb1bb65701a39b8d0b01821669683cca399ae2")
}

// UnpackInvalidActionError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidAction()
func (recipeExecutor *RecipeExecutor) UnpackInvalidActionError(raw []byte) (*RecipeExecutorInvalidAction, error) {
	out := new(RecipeExecutorInvalidAction)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidAction", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidCall represents a InvalidCall error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCall()
func RecipeExecutorInvalidCallErrorID() common.Hash {
	return common.HexToHash("0xae962d4e06a5f2158f022ce0e8331adb6b53d60eb043c7902e2eaa3586c6a4fb")
}

// UnpackInvalidCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCall()
func (recipeExecutor *RecipeExecutor) UnpackInvalidCallError(raw []byte) (*RecipeExecutorInvalidCall, error) {
	out := new(RecipeExecutorInvalidCall)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidCaller represents a InvalidCaller error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidCaller struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCaller()
func RecipeExecutorInvalidCallerErrorID() common.Hash {
	return common.HexToHash("0x48f5c3ed50241a1b6c87d204a25d9d01339cd768de9a714ffbb53a5bb6ad572a")
}

// UnpackInvalidCallerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCaller()
func (recipeExecutor *RecipeExecutor) UnpackInvalidCallerError(raw []byte) (*RecipeExecutorInvalidCaller, error) {
	out := new(RecipeExecutorInvalidCaller)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidCaller", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidDataMaskLength represents a InvalidDataMaskLength error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidDataMaskLength struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidDataMaskLength()
func RecipeExecutorInvalidDataMaskLengthErrorID() common.Hash {
	return common.HexToHash("0x3609cb6625464291d6101e8be49be0729d37a57ea489cb9e041f9a5f86c64336")
}

// UnpackInvalidDataMaskLengthError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidDataMaskLength()
func (recipeExecutor *RecipeExecutor) UnpackInvalidDataMaskLengthError(raw []byte) (*RecipeExecutorInvalidDataMaskLength, error) {
	out := new(RecipeExecutorInvalidDataMaskLength)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidDataMaskLength", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidOperation represents a InvalidOperation error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidOperation struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidOperation()
func RecipeExecutorInvalidOperationErrorID() common.Hash {
	return common.HexToHash("0x398d4d32963991cf02978f96f1a4d81787345d957c0995bedfe70af66ce594fd")
}

// UnpackInvalidOperationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidOperation()
func (recipeExecutor *RecipeExecutor) UnpackInvalidOperationError(raw []byte) (*RecipeExecutorInvalidOperation, error) {
	out := new(RecipeExecutorInvalidOperation)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidOperation", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidQuery represents a InvalidQuery error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidQuery struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidQuery()
func RecipeExecutorInvalidQueryErrorID() common.Hash {
	return common.HexToHash("0x19e4cb1093499c0994f1c698c47ae2ed09e8ac37618f650c826923074fd6d526")
}

// UnpackInvalidQueryError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidQuery()
func (recipeExecutor *RecipeExecutor) UnpackInvalidQueryError(raw []byte) (*RecipeExecutorInvalidQuery, error) {
	out := new(RecipeExecutorInvalidQuery)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidQuery", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidRecipe represents a InvalidRecipe error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidRecipe struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidRecipe()
func RecipeExecutorInvalidRecipeErrorID() common.Hash {
	return common.HexToHash("0x7b776f4ceee4fd127ffa0880f638d4f034d331c1a2605609383fd7c68a818bc0")
}

// UnpackInvalidRecipeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidRecipe()
func (recipeExecutor *RecipeExecutor) UnpackInvalidRecipeError(raw []byte) (*RecipeExecutorInvalidRecipe, error) {
	out := new(RecipeExecutorInvalidRecipe)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidRecipe", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidReference represents a InvalidReference error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidReference struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidReference()
func RecipeExecutorInvalidReferenceErrorID() common.Hash {
	return common.HexToHash("0x5f41ff9294f1a1a14da42dbeef661318e70ed969230837ec353d3d643d6946e1")
}

// UnpackInvalidReferenceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidReference()
func (recipeExecutor *RecipeExecutor) UnpackInvalidReferenceError(raw []byte) (*RecipeExecutorInvalidReference, error) {
	out := new(RecipeExecutorInvalidReference)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidReference", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorInvalidRuntime represents a InvalidRuntime error raised by the RecipeExecutor contract.
type RecipeExecutorInvalidRuntime struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidRuntime()
func RecipeExecutorInvalidRuntimeErrorID() common.Hash {
	return common.HexToHash("0x02e22e6c69a39fc2d7cb34bccdcd940b08139115bb1d4d1dc335d8086d0dccde")
}

// UnpackInvalidRuntimeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidRuntime()
func (recipeExecutor *RecipeExecutor) UnpackInvalidRuntimeError(raw []byte) (*RecipeExecutorInvalidRuntime, error) {
	out := new(RecipeExecutorInvalidRuntime)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "InvalidRuntime", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorOwnableInvalidOwner represents a OwnableInvalidOwner error raised by the RecipeExecutor contract.
type RecipeExecutorOwnableInvalidOwner struct {
	Owner common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableInvalidOwner(address owner)
func RecipeExecutorOwnableInvalidOwnerErrorID() common.Hash {
	return common.HexToHash("0x1e4fbdf7f3ef8bcaa855599e3abf48b232380f183f08f6f813d9ffa5bd585188")
}

// UnpackOwnableInvalidOwnerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableInvalidOwner(address owner)
func (recipeExecutor *RecipeExecutor) UnpackOwnableInvalidOwnerError(raw []byte) (*RecipeExecutorOwnableInvalidOwner, error) {
	out := new(RecipeExecutorOwnableInvalidOwner)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "OwnableInvalidOwner", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorOwnableUnauthorizedAccount represents a OwnableUnauthorizedAccount error raised by the RecipeExecutor contract.
type RecipeExecutorOwnableUnauthorizedAccount struct {
	Account common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func RecipeExecutorOwnableUnauthorizedAccountErrorID() common.Hash {
	return common.HexToHash("0x118cdaa7a341953d1887a2245fd6665d741c67c8c50581daa59e1d03373fa188")
}

// UnpackOwnableUnauthorizedAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func (recipeExecutor *RecipeExecutor) UnpackOwnableUnauthorizedAccountError(raw []byte) (*RecipeExecutorOwnableUnauthorizedAccount, error) {
	out := new(RecipeExecutorOwnableUnauthorizedAccount)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "OwnableUnauthorizedAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// RecipeExecutorReentrancyGuardReentrantCall represents a ReentrancyGuardReentrantCall error raised by the RecipeExecutor contract.
type RecipeExecutorReentrancyGuardReentrantCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ReentrancyGuardReentrantCall()
func RecipeExecutorReentrancyGuardReentrantCallErrorID() common.Hash {
	return common.HexToHash("0x3ee5aeb571de7fc460830b4d0017439a1ca56fb0bc39062227ade4fe4a24c1ca")
}

// UnpackReentrancyGuardReentrantCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ReentrancyGuardReentrantCall()
func (recipeExecutor *RecipeExecutor) UnpackReentrancyGuardReentrantCallError(raw []byte) (*RecipeExecutorReentrancyGuardReentrantCall, error) {
	out := new(RecipeExecutorReentrancyGuardReentrantCall)
	if err := recipeExecutor.abi.UnpackIntoInterface(out, "ReentrancyGuardReentrantCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}
