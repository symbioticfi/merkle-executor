// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package liquidlanereactor

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

// IReactorOrder is an auto generated low-level Go binding around an user-defined struct.
type IReactorOrder struct {
	Request          IReactorRequest
	SwapperSignature []byte
	Swapper          common.Address
	Filler           common.Address
	Outputs          []IReactorOutput
}

// IReactorOutput is an auto generated low-level Go binding around an user-defined struct.
type IReactorOutput struct {
	Token     common.Address
	Amount    *big.Int
	Recipient common.Address
}

// IReactorRequest is an auto generated low-level Go binding around an user-defined struct.
type IReactorRequest struct {
	TokenIn  common.Address
	AmountIn *big.Int
	Outputs  []IReactorOutput
	Deadline *big.Int
	Nonce    *big.Int
	Protocol common.Address
}

// LiquidLaneReactorMetaData contains all meta data concerning the LiquidLaneReactor contract.
var LiquidLaneReactorMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"eip712Domain\",\"inputs\":[],\"outputs\":[{\"name\":\"fields\",\"type\":\"bytes1\",\"internalType\":\"bytes1\"},{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"version\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"chainId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"verifyingContract\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"salt\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"extensions\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isUsedNonce\",\"inputs\":[{\"name\":\"swapper\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"used\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"Fill\",\"inputs\":[{\"name\":\"order\",\"type\":\"tuple\",\"indexed\":false,\"internalType\":\"structIReactor.Order\",\"components\":[{\"name\":\"request\",\"type\":\"tuple\",\"internalType\":\"structIReactor.Request\",\"components\":[{\"name\":\"tokenIn\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"outputs\",\"type\":\"tuple[]\",\"internalType\":\"structIReactor.Output[]\",\"components\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"name\":\"deadline\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"protocol\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"name\":\"swapperSignature\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"swapper\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"filler\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"outputs\",\"type\":\"tuple[]\",\"internalType\":\"structIReactor.Output[]\",\"components\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"}]}]}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"InvalidateNonce\",\"inputs\":[{\"name\":\"swapper\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false}]",
	ID:  "LiquidLaneReactor",
}

// LiquidLaneReactor is an auto generated Go binding around an Ethereum contract.
type LiquidLaneReactor struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *LiquidLaneReactor) GetABI() abi.ABI {
	return c.abi
}

// NewLiquidLaneReactor creates a new instance of LiquidLaneReactor.
func NewLiquidLaneReactor() *LiquidLaneReactor {
	parsed, err := LiquidLaneReactorMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &LiquidLaneReactor{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *LiquidLaneReactor) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackEip712Domain is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x84b0196e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function eip712Domain() view returns(bytes1 fields, string name, string version, uint256 chainId, address verifyingContract, bytes32 salt, uint256[] extensions)
func (liquidLaneReactor *LiquidLaneReactor) PackEip712Domain() []byte {
	enc, err := liquidLaneReactor.abi.Pack("eip712Domain")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackEip712Domain is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x84b0196e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function eip712Domain() view returns(bytes1 fields, string name, string version, uint256 chainId, address verifyingContract, bytes32 salt, uint256[] extensions)
func (liquidLaneReactor *LiquidLaneReactor) TryPackEip712Domain() ([]byte, error) {
	return liquidLaneReactor.abi.Pack("eip712Domain")
}

// Eip712DomainOutput serves as a container for the return parameters of contract
// method Eip712Domain.
type Eip712DomainOutput struct {
	Fields            [1]byte
	Name              string
	Version           string
	ChainId           *big.Int
	VerifyingContract common.Address
	Salt              [32]byte
	Extensions        []*big.Int
}

// UnpackEip712Domain is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x84b0196e.
//
// Solidity: function eip712Domain() view returns(bytes1 fields, string name, string version, uint256 chainId, address verifyingContract, bytes32 salt, uint256[] extensions)
func (liquidLaneReactor *LiquidLaneReactor) UnpackEip712Domain(data []byte) (Eip712DomainOutput, error) {
	out, err := liquidLaneReactor.abi.Unpack("eip712Domain", data)
	outstruct := new(Eip712DomainOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Fields = *abi.ConvertType(out[0], new([1]byte)).(*[1]byte)
	outstruct.Name = *abi.ConvertType(out[1], new(string)).(*string)
	outstruct.Version = *abi.ConvertType(out[2], new(string)).(*string)
	outstruct.ChainId = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	outstruct.VerifyingContract = *abi.ConvertType(out[4], new(common.Address)).(*common.Address)
	outstruct.Salt = *abi.ConvertType(out[5], new([32]byte)).(*[32]byte)
	outstruct.Extensions = *abi.ConvertType(out[6], new([]*big.Int)).(*[]*big.Int)
	return *outstruct, nil
}

// PackIsUsedNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0ee60fa7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isUsedNonce(address swapper, uint256 nonce) view returns(bool used)
func (liquidLaneReactor *LiquidLaneReactor) PackIsUsedNonce(swapper common.Address, nonce *big.Int) []byte {
	enc, err := liquidLaneReactor.abi.Pack("isUsedNonce", swapper, nonce)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsUsedNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0ee60fa7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isUsedNonce(address swapper, uint256 nonce) view returns(bool used)
func (liquidLaneReactor *LiquidLaneReactor) TryPackIsUsedNonce(swapper common.Address, nonce *big.Int) ([]byte, error) {
	return liquidLaneReactor.abi.Pack("isUsedNonce", swapper, nonce)
}

// UnpackIsUsedNonce is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0ee60fa7.
//
// Solidity: function isUsedNonce(address swapper, uint256 nonce) view returns(bool used)
func (liquidLaneReactor *LiquidLaneReactor) UnpackIsUsedNonce(data []byte) (bool, error) {
	out, err := liquidLaneReactor.abi.Unpack("isUsedNonce", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// LiquidLaneReactorFill represents a Fill event raised by the LiquidLaneReactor contract.
type LiquidLaneReactorFill struct {
	Order IReactorOrder
	Raw   *types.Log // Blockchain specific contextual infos
}

const LiquidLaneReactorFillEventName = "Fill"

// ContractEventName returns the user-defined event name.
func (LiquidLaneReactorFill) ContractEventName() string {
	return LiquidLaneReactorFillEventName
}

// UnpackFillEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Fill(((address,uint256,(address,uint256,address)[],uint256,uint256,address),bytes,address,address,(address,uint256,address)[]) order)
func (liquidLaneReactor *LiquidLaneReactor) UnpackFillEvent(log *types.Log) (*LiquidLaneReactorFill, error) {
	event := "Fill"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidLaneReactor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidLaneReactorFill)
	if len(log.Data) > 0 {
		if err := liquidLaneReactor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidLaneReactor.abi.Events[event].Inputs {
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

// LiquidLaneReactorInvalidateNonce represents a InvalidateNonce event raised by the LiquidLaneReactor contract.
type LiquidLaneReactorInvalidateNonce struct {
	Swapper common.Address
	Nonce   *big.Int
	Raw     *types.Log // Blockchain specific contextual infos
}

const LiquidLaneReactorInvalidateNonceEventName = "InvalidateNonce"

// ContractEventName returns the user-defined event name.
func (LiquidLaneReactorInvalidateNonce) ContractEventName() string {
	return LiquidLaneReactorInvalidateNonceEventName
}

// UnpackInvalidateNonceEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event InvalidateNonce(address indexed swapper, uint256 nonce)
func (liquidLaneReactor *LiquidLaneReactor) UnpackInvalidateNonceEvent(log *types.Log) (*LiquidLaneReactorInvalidateNonce, error) {
	event := "InvalidateNonce"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidLaneReactor.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidLaneReactorInvalidateNonce)
	if len(log.Data) > 0 {
		if err := liquidLaneReactor.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidLaneReactor.abi.Events[event].Inputs {
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
