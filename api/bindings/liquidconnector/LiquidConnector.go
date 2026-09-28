// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package liquidconnector

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

// ILiquidLaneConnectorDiscount is an auto generated low-level Go binding around an user-defined struct.
type ILiquidLaneConnectorDiscount struct {
	TokenToRedeem common.Address
	Discount      *big.Int
	Signer        common.Address
	Protocol      common.Address
	Nonce         *big.Int
	Deadline      *big.Int
}

// ILiquidLaneConnectorDiscountSwap is an auto generated low-level Go binding around an user-defined struct.
type ILiquidLaneConnectorDiscountSwap struct {
	Discount         ILiquidLaneConnectorDiscount
	SignerSignature  []byte
	ProtocolDeadline *big.Int
}

// ILiquidLaneConnectorSwap is an auto generated low-level Go binding around an user-defined struct.
type ILiquidLaneConnectorSwap struct {
	Recipient common.Address
	TokenIn   common.Address
	AmountIn  *big.Int
	AmountOut *big.Int
}

// LiquidConnectorMetaData contains all meta data concerning the LiquidConnector contract.
var LiquidConnectorMetaData = bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"newFactory\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newMerkleAccountFactory\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newOracleRegistry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"FACTORY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MERKLE_ACCOUNT_FACTORY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"ORACLE_REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"account\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"addTokenToRedeem\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"assetLimits\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"maxAssets\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"duration\",\"type\":\"uint48\",\"internalType\":\"uint48\"},{\"name\":\"periodStart\",\"type\":\"uint48\",\"internalType\":\"uint48\"},{\"name\":\"spent\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"eip712Domain\",\"inputs\":[],\"outputs\":[{\"name\":\"fields\",\"type\":\"bytes1\",\"internalType\":\"bytes1\"},{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"version\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"chainId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"verifyingContract\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"salt\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"extensions\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAmountOut\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"initialize\",\"inputs\":[{\"name\":\"initialVersion\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"owner_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"invalidateNonce\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"isUsedNonce\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"invalidated\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"migrate\",\"inputs\":[{\"name\":\"newVersion\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"minDiscount\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"discount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"oracles\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"oracle\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"outputAsset\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"remainingAssets\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"removeTokenToRedeem\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMaxAssetsPerDuration\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newMaxAssets\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"newDuration\",\"type\":\"uint48\",\"internalType\":\"uint48\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMinDiscount\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newMinDiscount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSigner\",\"inputs\":[{\"name\":\"newSigner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"signer\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"swap\",\"inputs\":[{\"name\":\"newSwap\",\"type\":\"tuple\",\"internalType\":\"structILiquidLaneConnector.Swap\",\"components\":[{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenIn\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"swap\",\"inputs\":[{\"name\":\"discountSwap\",\"type\":\"tuple\",\"internalType\":\"structILiquidLaneConnector.DiscountSwap\",\"components\":[{\"name\":\"discount\",\"type\":\"tuple\",\"internalType\":\"structILiquidLaneConnector.Discount\",\"components\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"discount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"signer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"protocol\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"deadline\",\"type\":\"uint48\",\"internalType\":\"uint48\"}]},{\"name\":\"signerSignature\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"protocolDeadline\",\"type\":\"uint48\",\"internalType\":\"uint48\"}]},{\"name\":\"protocolSignature\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"version\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"AddTokenToRedeem\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"oracle\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"DoSwap\",\"inputs\":[{\"name\":\"newSwap\",\"type\":\"tuple\",\"indexed\":false,\"internalType\":\"structILiquidLaneConnector.Swap\",\"components\":[{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenIn\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"amountIn\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amountOut\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"EIP712DomainChanged\",\"inputs\":[],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Initialized\",\"inputs\":[{\"name\":\"version\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"InvalidateNonce\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"nonce\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RemoveTokenToRedeem\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetMaxAssetsPerDuration\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"maxAssets\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"duration\",\"type\":\"uint48\",\"indexed\":false,\"internalType\":\"uint48\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetMinDiscount\",\"inputs\":[{\"name\":\"tokenToRedeem\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"minDiscount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SetSigner\",\"inputs\":[{\"name\":\"signer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AlreadyInitialized\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AlreadyUsedNonce\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AssetLimitExceeded\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ExpiredSwap\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidAccount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCaller\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidDiscount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidDuration\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidInitialization\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidOracle\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidShortString\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidSignature\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidSigner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidSwapRate\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidTokenToRedeem\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotFactory\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotInitializing\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"StringTooLong\",\"inputs\":[{\"name\":\"str\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"type\":\"error\",\"name\":\"UnconfiguredAsset\",\"inputs\":[]}]",
	ID:  "LiquidConnector",
}

// LiquidConnector is an auto generated Go binding around an Ethereum contract.
type LiquidConnector struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *LiquidConnector) GetABI() abi.ABI {
	return c.abi
}

// NewLiquidConnector creates a new instance of LiquidConnector.
func NewLiquidConnector() *LiquidConnector {
	parsed, err := LiquidConnectorMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &LiquidConnector{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *LiquidConnector) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackConstructor is the Go binding used to pack the parameters required for
// contract deployment.
//
// Solidity: constructor(address newFactory, address newMerkleAccountFactory, address newOracleRegistry) returns()
func (liquidConnector *LiquidConnector) PackConstructor(newFactory common.Address, newMerkleAccountFactory common.Address, newOracleRegistry common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("", newFactory, newMerkleAccountFactory, newOracleRegistry)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2dd31000.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) PackFACTORY() []byte {
	enc, err := liquidConnector.abi.Pack("FACTORY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2dd31000.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) TryPackFACTORY() ([]byte, error) {
	return liquidConnector.abi.Pack("FACTORY")
}

// UnpackFACTORY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2dd31000.
//
// Solidity: function FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) UnpackFACTORY(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("FACTORY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackMERKLEACCOUNTFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x631506fe.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function MERKLE_ACCOUNT_FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) PackMERKLEACCOUNTFACTORY() []byte {
	enc, err := liquidConnector.abi.Pack("MERKLE_ACCOUNT_FACTORY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMERKLEACCOUNTFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x631506fe.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function MERKLE_ACCOUNT_FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) TryPackMERKLEACCOUNTFACTORY() ([]byte, error) {
	return liquidConnector.abi.Pack("MERKLE_ACCOUNT_FACTORY")
}

// UnpackMERKLEACCOUNTFACTORY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x631506fe.
//
// Solidity: function MERKLE_ACCOUNT_FACTORY() view returns(address)
func (liquidConnector *LiquidConnector) UnpackMERKLEACCOUNTFACTORY(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("MERKLE_ACCOUNT_FACTORY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackORACLEREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x78b122d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function ORACLE_REGISTRY() view returns(address)
func (liquidConnector *LiquidConnector) PackORACLEREGISTRY() []byte {
	enc, err := liquidConnector.abi.Pack("ORACLE_REGISTRY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackORACLEREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x78b122d8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function ORACLE_REGISTRY() view returns(address)
func (liquidConnector *LiquidConnector) TryPackORACLEREGISTRY() ([]byte, error) {
	return liquidConnector.abi.Pack("ORACLE_REGISTRY")
}

// UnpackORACLEREGISTRY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x78b122d8.
//
// Solidity: function ORACLE_REGISTRY() view returns(address)
func (liquidConnector *LiquidConnector) UnpackORACLEREGISTRY(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("ORACLE_REGISTRY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackAccount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5dab2420.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function account() view returns(address)
func (liquidConnector *LiquidConnector) PackAccount() []byte {
	enc, err := liquidConnector.abi.Pack("account")
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
func (liquidConnector *LiquidConnector) TryPackAccount() ([]byte, error) {
	return liquidConnector.abi.Pack("account")
}

// UnpackAccount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5dab2420.
//
// Solidity: function account() view returns(address)
func (liquidConnector *LiquidConnector) UnpackAccount(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("account", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackAddTokenToRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9b3c58d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function addTokenToRedeem(address tokenToRedeem) returns()
func (liquidConnector *LiquidConnector) PackAddTokenToRedeem(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("addTokenToRedeem", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAddTokenToRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9b3c58d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function addTokenToRedeem(address tokenToRedeem) returns()
func (liquidConnector *LiquidConnector) TryPackAddTokenToRedeem(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("addTokenToRedeem", tokenToRedeem)
}

// PackAssetLimits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x560ee9dd.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function assetLimits(address tokenToRedeem) view returns(uint256 maxAssets, uint48 duration, uint48 periodStart, uint256 spent)
func (liquidConnector *LiquidConnector) PackAssetLimits(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("assetLimits", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAssetLimits is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x560ee9dd.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function assetLimits(address tokenToRedeem) view returns(uint256 maxAssets, uint48 duration, uint48 periodStart, uint256 spent)
func (liquidConnector *LiquidConnector) TryPackAssetLimits(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("assetLimits", tokenToRedeem)
}

// AssetLimitsOutput serves as a container for the return parameters of contract
// method AssetLimits.
type AssetLimitsOutput struct {
	MaxAssets   *big.Int
	Duration    *big.Int
	PeriodStart *big.Int
	Spent       *big.Int
}

// UnpackAssetLimits is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x560ee9dd.
//
// Solidity: function assetLimits(address tokenToRedeem) view returns(uint256 maxAssets, uint48 duration, uint48 periodStart, uint256 spent)
func (liquidConnector *LiquidConnector) UnpackAssetLimits(data []byte) (AssetLimitsOutput, error) {
	out, err := liquidConnector.abi.Unpack("assetLimits", data)
	outstruct := new(AssetLimitsOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.MaxAssets = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Duration = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.PeriodStart = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.Spent = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackEip712Domain is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x84b0196e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function eip712Domain() view returns(bytes1 fields, string name, string version, uint256 chainId, address verifyingContract, bytes32 salt, uint256[] extensions)
func (liquidConnector *LiquidConnector) PackEip712Domain() []byte {
	enc, err := liquidConnector.abi.Pack("eip712Domain")
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
func (liquidConnector *LiquidConnector) TryPackEip712Domain() ([]byte, error) {
	return liquidConnector.abi.Pack("eip712Domain")
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
func (liquidConnector *LiquidConnector) UnpackEip712Domain(data []byte) (Eip712DomainOutput, error) {
	out, err := liquidConnector.abi.Unpack("eip712Domain", data)
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

// PackGetAmountOut is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xca706bcf.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getAmountOut(address tokenToRedeem, uint256 amountIn) view returns(uint256)
func (liquidConnector *LiquidConnector) PackGetAmountOut(tokenToRedeem common.Address, amountIn *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("getAmountOut", tokenToRedeem, amountIn)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetAmountOut is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xca706bcf.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getAmountOut(address tokenToRedeem, uint256 amountIn) view returns(uint256)
func (liquidConnector *LiquidConnector) TryPackGetAmountOut(tokenToRedeem common.Address, amountIn *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("getAmountOut", tokenToRedeem, amountIn)
}

// UnpackGetAmountOut is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xca706bcf.
//
// Solidity: function getAmountOut(address tokenToRedeem, uint256 amountIn) view returns(uint256)
func (liquidConnector *LiquidConnector) UnpackGetAmountOut(data []byte) (*big.Int, error) {
	out, err := liquidConnector.abi.Unpack("getAmountOut", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackInitialize is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x57ec83cc.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function initialize(uint64 initialVersion, address owner_, bytes data) returns()
func (liquidConnector *LiquidConnector) PackInitialize(initialVersion uint64, owner common.Address, data []byte) []byte {
	enc, err := liquidConnector.abi.Pack("initialize", initialVersion, owner, data)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackInitialize is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x57ec83cc.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function initialize(uint64 initialVersion, address owner_, bytes data) returns()
func (liquidConnector *LiquidConnector) TryPackInitialize(initialVersion uint64, owner common.Address, data []byte) ([]byte, error) {
	return liquidConnector.abi.Pack("initialize", initialVersion, owner, data)
}

// PackInvalidateNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0d3762b5.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function invalidateNonce(address tokenToRedeem, uint256 nonce) returns()
func (liquidConnector *LiquidConnector) PackInvalidateNonce(tokenToRedeem common.Address, nonce *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("invalidateNonce", tokenToRedeem, nonce)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackInvalidateNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0d3762b5.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function invalidateNonce(address tokenToRedeem, uint256 nonce) returns()
func (liquidConnector *LiquidConnector) TryPackInvalidateNonce(tokenToRedeem common.Address, nonce *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("invalidateNonce", tokenToRedeem, nonce)
}

// PackIsUsedNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0ee60fa7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isUsedNonce(address tokenToRedeem, uint256 nonce) view returns(bool invalidated)
func (liquidConnector *LiquidConnector) PackIsUsedNonce(tokenToRedeem common.Address, nonce *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("isUsedNonce", tokenToRedeem, nonce)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsUsedNonce is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0ee60fa7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isUsedNonce(address tokenToRedeem, uint256 nonce) view returns(bool invalidated)
func (liquidConnector *LiquidConnector) TryPackIsUsedNonce(tokenToRedeem common.Address, nonce *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("isUsedNonce", tokenToRedeem, nonce)
}

// UnpackIsUsedNonce is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0ee60fa7.
//
// Solidity: function isUsedNonce(address tokenToRedeem, uint256 nonce) view returns(bool invalidated)
func (liquidConnector *LiquidConnector) UnpackIsUsedNonce(data []byte) (bool, error) {
	out, err := liquidConnector.abi.Unpack("isUsedNonce", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackMigrate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2abe3048.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function migrate(uint64 newVersion, bytes data) returns()
func (liquidConnector *LiquidConnector) PackMigrate(newVersion uint64, data []byte) []byte {
	enc, err := liquidConnector.abi.Pack("migrate", newVersion, data)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMigrate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2abe3048.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function migrate(uint64 newVersion, bytes data) returns()
func (liquidConnector *LiquidConnector) TryPackMigrate(newVersion uint64, data []byte) ([]byte, error) {
	return liquidConnector.abi.Pack("migrate", newVersion, data)
}

// PackMinDiscount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd9104c14.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function minDiscount(address tokenToRedeem) view returns(uint256 discount)
func (liquidConnector *LiquidConnector) PackMinDiscount(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("minDiscount", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMinDiscount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd9104c14.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function minDiscount(address tokenToRedeem) view returns(uint256 discount)
func (liquidConnector *LiquidConnector) TryPackMinDiscount(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("minDiscount", tokenToRedeem)
}

// UnpackMinDiscount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd9104c14.
//
// Solidity: function minDiscount(address tokenToRedeem) view returns(uint256 discount)
func (liquidConnector *LiquidConnector) UnpackMinDiscount(data []byte) (*big.Int, error) {
	out, err := liquidConnector.abi.Unpack("minDiscount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackOracles is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaddd5099.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function oracles(address tokenToRedeem) view returns(address oracle)
func (liquidConnector *LiquidConnector) PackOracles(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("oracles", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackOracles is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaddd5099.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function oracles(address tokenToRedeem) view returns(address oracle)
func (liquidConnector *LiquidConnector) TryPackOracles(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("oracles", tokenToRedeem)
}

// UnpackOracles is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xaddd5099.
//
// Solidity: function oracles(address tokenToRedeem) view returns(address oracle)
func (liquidConnector *LiquidConnector) UnpackOracles(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("oracles", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackOutputAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69cb2a8b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function outputAsset() view returns(address)
func (liquidConnector *LiquidConnector) PackOutputAsset() []byte {
	enc, err := liquidConnector.abi.Pack("outputAsset")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackOutputAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x69cb2a8b.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function outputAsset() view returns(address)
func (liquidConnector *LiquidConnector) TryPackOutputAsset() ([]byte, error) {
	return liquidConnector.abi.Pack("outputAsset")
}

// UnpackOutputAsset is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x69cb2a8b.
//
// Solidity: function outputAsset() view returns(address)
func (liquidConnector *LiquidConnector) UnpackOutputAsset(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("outputAsset", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackOwner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8da5cb5b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function owner() view returns(address)
func (liquidConnector *LiquidConnector) PackOwner() []byte {
	enc, err := liquidConnector.abi.Pack("owner")
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
func (liquidConnector *LiquidConnector) TryPackOwner() ([]byte, error) {
	return liquidConnector.abi.Pack("owner")
}

// UnpackOwner is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (liquidConnector *LiquidConnector) UnpackOwner(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("owner", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRemainingAssets is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd91bbc69.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function remainingAssets(address tokenToRedeem) view returns(uint256)
func (liquidConnector *LiquidConnector) PackRemainingAssets(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("remainingAssets", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRemainingAssets is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd91bbc69.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function remainingAssets(address tokenToRedeem) view returns(uint256)
func (liquidConnector *LiquidConnector) TryPackRemainingAssets(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("remainingAssets", tokenToRedeem)
}

// UnpackRemainingAssets is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xd91bbc69.
//
// Solidity: function remainingAssets(address tokenToRedeem) view returns(uint256)
func (liquidConnector *LiquidConnector) UnpackRemainingAssets(data []byte) (*big.Int, error) {
	out, err := liquidConnector.abi.Unpack("remainingAssets", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRemoveTokenToRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe250fef7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function removeTokenToRedeem(address tokenToRedeem) returns()
func (liquidConnector *LiquidConnector) PackRemoveTokenToRedeem(tokenToRedeem common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("removeTokenToRedeem", tokenToRedeem)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRemoveTokenToRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe250fef7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function removeTokenToRedeem(address tokenToRedeem) returns()
func (liquidConnector *LiquidConnector) TryPackRemoveTokenToRedeem(tokenToRedeem common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("removeTokenToRedeem", tokenToRedeem)
}

// PackRenounceOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x715018a6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function renounceOwnership() returns()
func (liquidConnector *LiquidConnector) PackRenounceOwnership() []byte {
	enc, err := liquidConnector.abi.Pack("renounceOwnership")
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
func (liquidConnector *LiquidConnector) TryPackRenounceOwnership() ([]byte, error) {
	return liquidConnector.abi.Pack("renounceOwnership")
}

// PackSetMaxAssetsPerDuration is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x940e98c9.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setMaxAssetsPerDuration(address tokenToRedeem, uint256 newMaxAssets, uint48 newDuration) returns()
func (liquidConnector *LiquidConnector) PackSetMaxAssetsPerDuration(tokenToRedeem common.Address, newMaxAssets *big.Int, newDuration *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("setMaxAssetsPerDuration", tokenToRedeem, newMaxAssets, newDuration)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetMaxAssetsPerDuration is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x940e98c9.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setMaxAssetsPerDuration(address tokenToRedeem, uint256 newMaxAssets, uint48 newDuration) returns()
func (liquidConnector *LiquidConnector) TryPackSetMaxAssetsPerDuration(tokenToRedeem common.Address, newMaxAssets *big.Int, newDuration *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("setMaxAssetsPerDuration", tokenToRedeem, newMaxAssets, newDuration)
}

// PackSetMinDiscount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0517ef9a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setMinDiscount(address tokenToRedeem, uint256 newMinDiscount) returns()
func (liquidConnector *LiquidConnector) PackSetMinDiscount(tokenToRedeem common.Address, newMinDiscount *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("setMinDiscount", tokenToRedeem, newMinDiscount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetMinDiscount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0517ef9a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setMinDiscount(address tokenToRedeem, uint256 newMinDiscount) returns()
func (liquidConnector *LiquidConnector) TryPackSetMinDiscount(tokenToRedeem common.Address, newMinDiscount *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("setMinDiscount", tokenToRedeem, newMinDiscount)
}

// PackSetSigner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6c19e783.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function setSigner(address newSigner) returns()
func (liquidConnector *LiquidConnector) PackSetSigner(newSigner common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("setSigner", newSigner)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSetSigner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6c19e783.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function setSigner(address newSigner) returns()
func (liquidConnector *LiquidConnector) TryPackSetSigner(newSigner common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("setSigner", newSigner)
}

// PackSigner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x238ac933.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function signer() view returns(address)
func (liquidConnector *LiquidConnector) PackSigner() []byte {
	enc, err := liquidConnector.abi.Pack("signer")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSigner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x238ac933.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function signer() view returns(address)
func (liquidConnector *LiquidConnector) TryPackSigner() ([]byte, error) {
	return liquidConnector.abi.Pack("signer")
}

// UnpackSigner is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x238ac933.
//
// Solidity: function signer() view returns(address)
func (liquidConnector *LiquidConnector) UnpackSigner(data []byte) (common.Address, error) {
	out, err := liquidConnector.abi.Unpack("signer", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackSwap is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x53f549d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function swap((address,address,uint256,uint256) newSwap) returns()
func (liquidConnector *LiquidConnector) PackSwap(newSwap ILiquidLaneConnectorSwap) []byte {
	enc, err := liquidConnector.abi.Pack("swap", newSwap)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSwap is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x53f549d8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function swap((address,address,uint256,uint256) newSwap) returns()
func (liquidConnector *LiquidConnector) TryPackSwap(newSwap ILiquidLaneConnectorSwap) ([]byte, error) {
	return liquidConnector.abi.Pack("swap", newSwap)
}

// PackSwap0 is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8fa5c671.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function swap(((address,uint256,address,address,uint256,uint48),bytes,uint48) discountSwap, bytes protocolSignature, address recipient, uint256 amountIn) returns(uint256 amountOut)
func (liquidConnector *LiquidConnector) PackSwap0(discountSwap ILiquidLaneConnectorDiscountSwap, protocolSignature []byte, recipient common.Address, amountIn *big.Int) []byte {
	enc, err := liquidConnector.abi.Pack("swap0", discountSwap, protocolSignature, recipient, amountIn)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSwap0 is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8fa5c671.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function swap(((address,uint256,address,address,uint256,uint48),bytes,uint48) discountSwap, bytes protocolSignature, address recipient, uint256 amountIn) returns(uint256 amountOut)
func (liquidConnector *LiquidConnector) TryPackSwap0(discountSwap ILiquidLaneConnectorDiscountSwap, protocolSignature []byte, recipient common.Address, amountIn *big.Int) ([]byte, error) {
	return liquidConnector.abi.Pack("swap0", discountSwap, protocolSignature, recipient, amountIn)
}

// UnpackSwap0 is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8fa5c671.
//
// Solidity: function swap(((address,uint256,address,address,uint256,uint48),bytes,uint48) discountSwap, bytes protocolSignature, address recipient, uint256 amountIn) returns(uint256 amountOut)
func (liquidConnector *LiquidConnector) UnpackSwap0(data []byte) (*big.Int, error) {
	out, err := liquidConnector.abi.Unpack("swap0", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackTransferOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2fde38b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (liquidConnector *LiquidConnector) PackTransferOwnership(newOwner common.Address) []byte {
	enc, err := liquidConnector.abi.Pack("transferOwnership", newOwner)
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
func (liquidConnector *LiquidConnector) TryPackTransferOwnership(newOwner common.Address) ([]byte, error) {
	return liquidConnector.abi.Pack("transferOwnership", newOwner)
}

// PackVersion is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x54fd4d50.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function version() view returns(uint64)
func (liquidConnector *LiquidConnector) PackVersion() []byte {
	enc, err := liquidConnector.abi.Pack("version")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackVersion is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x54fd4d50.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function version() view returns(uint64)
func (liquidConnector *LiquidConnector) TryPackVersion() ([]byte, error) {
	return liquidConnector.abi.Pack("version")
}

// UnpackVersion is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x54fd4d50.
//
// Solidity: function version() view returns(uint64)
func (liquidConnector *LiquidConnector) UnpackVersion(data []byte) (uint64, error) {
	out, err := liquidConnector.abi.Unpack("version", data)
	if err != nil {
		return *new(uint64), err
	}
	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)
	return out0, nil
}

// LiquidConnectorAddTokenToRedeem represents a AddTokenToRedeem event raised by the LiquidConnector contract.
type LiquidConnectorAddTokenToRedeem struct {
	TokenToRedeem common.Address
	Oracle        common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorAddTokenToRedeemEventName = "AddTokenToRedeem"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorAddTokenToRedeem) ContractEventName() string {
	return LiquidConnectorAddTokenToRedeemEventName
}

// UnpackAddTokenToRedeemEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event AddTokenToRedeem(address indexed tokenToRedeem, address indexed oracle)
func (liquidConnector *LiquidConnector) UnpackAddTokenToRedeemEvent(log *types.Log) (*LiquidConnectorAddTokenToRedeem, error) {
	event := "AddTokenToRedeem"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorAddTokenToRedeem)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorDoSwap represents a DoSwap event raised by the LiquidConnector contract.
type LiquidConnectorDoSwap struct {
	NewSwap ILiquidLaneConnectorSwap
	Raw     *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorDoSwapEventName = "DoSwap"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorDoSwap) ContractEventName() string {
	return LiquidConnectorDoSwapEventName
}

// UnpackDoSwapEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event DoSwap((address,address,uint256,uint256) newSwap)
func (liquidConnector *LiquidConnector) UnpackDoSwapEvent(log *types.Log) (*LiquidConnectorDoSwap, error) {
	event := "DoSwap"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorDoSwap)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorEIP712DomainChanged represents a EIP712DomainChanged event raised by the LiquidConnector contract.
type LiquidConnectorEIP712DomainChanged struct {
	Raw *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorEIP712DomainChangedEventName = "EIP712DomainChanged"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorEIP712DomainChanged) ContractEventName() string {
	return LiquidConnectorEIP712DomainChangedEventName
}

// UnpackEIP712DomainChangedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event EIP712DomainChanged()
func (liquidConnector *LiquidConnector) UnpackEIP712DomainChangedEvent(log *types.Log) (*LiquidConnectorEIP712DomainChanged, error) {
	event := "EIP712DomainChanged"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorEIP712DomainChanged)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorInitialized represents a Initialized event raised by the LiquidConnector contract.
type LiquidConnectorInitialized struct {
	Version uint64
	Raw     *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorInitializedEventName = "Initialized"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorInitialized) ContractEventName() string {
	return LiquidConnectorInitializedEventName
}

// UnpackInitializedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Initialized(uint64 version)
func (liquidConnector *LiquidConnector) UnpackInitializedEvent(log *types.Log) (*LiquidConnectorInitialized, error) {
	event := "Initialized"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorInitialized)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorInvalidateNonce represents a InvalidateNonce event raised by the LiquidConnector contract.
type LiquidConnectorInvalidateNonce struct {
	TokenToRedeem common.Address
	Nonce         *big.Int
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorInvalidateNonceEventName = "InvalidateNonce"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorInvalidateNonce) ContractEventName() string {
	return LiquidConnectorInvalidateNonceEventName
}

// UnpackInvalidateNonceEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event InvalidateNonce(address indexed tokenToRedeem, uint256 nonce)
func (liquidConnector *LiquidConnector) UnpackInvalidateNonceEvent(log *types.Log) (*LiquidConnectorInvalidateNonce, error) {
	event := "InvalidateNonce"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorInvalidateNonce)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorOwnershipTransferred represents a OwnershipTransferred event raised by the LiquidConnector contract.
type LiquidConnectorOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorOwnershipTransferredEventName = "OwnershipTransferred"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorOwnershipTransferred) ContractEventName() string {
	return LiquidConnectorOwnershipTransferredEventName
}

// UnpackOwnershipTransferredEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (liquidConnector *LiquidConnector) UnpackOwnershipTransferredEvent(log *types.Log) (*LiquidConnectorOwnershipTransferred, error) {
	event := "OwnershipTransferred"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorOwnershipTransferred)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorRemoveTokenToRedeem represents a RemoveTokenToRedeem event raised by the LiquidConnector contract.
type LiquidConnectorRemoveTokenToRedeem struct {
	TokenToRedeem common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorRemoveTokenToRedeemEventName = "RemoveTokenToRedeem"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorRemoveTokenToRedeem) ContractEventName() string {
	return LiquidConnectorRemoveTokenToRedeemEventName
}

// UnpackRemoveTokenToRedeemEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RemoveTokenToRedeem(address indexed tokenToRedeem)
func (liquidConnector *LiquidConnector) UnpackRemoveTokenToRedeemEvent(log *types.Log) (*LiquidConnectorRemoveTokenToRedeem, error) {
	event := "RemoveTokenToRedeem"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorRemoveTokenToRedeem)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorSetMaxAssetsPerDuration represents a SetMaxAssetsPerDuration event raised by the LiquidConnector contract.
type LiquidConnectorSetMaxAssetsPerDuration struct {
	TokenToRedeem common.Address
	MaxAssets     *big.Int
	Duration      *big.Int
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorSetMaxAssetsPerDurationEventName = "SetMaxAssetsPerDuration"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorSetMaxAssetsPerDuration) ContractEventName() string {
	return LiquidConnectorSetMaxAssetsPerDurationEventName
}

// UnpackSetMaxAssetsPerDurationEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetMaxAssetsPerDuration(address indexed tokenToRedeem, uint256 maxAssets, uint48 duration)
func (liquidConnector *LiquidConnector) UnpackSetMaxAssetsPerDurationEvent(log *types.Log) (*LiquidConnectorSetMaxAssetsPerDuration, error) {
	event := "SetMaxAssetsPerDuration"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorSetMaxAssetsPerDuration)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorSetMinDiscount represents a SetMinDiscount event raised by the LiquidConnector contract.
type LiquidConnectorSetMinDiscount struct {
	TokenToRedeem common.Address
	MinDiscount   *big.Int
	Raw           *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorSetMinDiscountEventName = "SetMinDiscount"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorSetMinDiscount) ContractEventName() string {
	return LiquidConnectorSetMinDiscountEventName
}

// UnpackSetMinDiscountEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetMinDiscount(address indexed tokenToRedeem, uint256 minDiscount)
func (liquidConnector *LiquidConnector) UnpackSetMinDiscountEvent(log *types.Log) (*LiquidConnectorSetMinDiscount, error) {
	event := "SetMinDiscount"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorSetMinDiscount)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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

// LiquidConnectorSetSigner represents a SetSigner event raised by the LiquidConnector contract.
type LiquidConnectorSetSigner struct {
	Signer common.Address
	Raw    *types.Log // Blockchain specific contextual infos
}

const LiquidConnectorSetSignerEventName = "SetSigner"

// ContractEventName returns the user-defined event name.
func (LiquidConnectorSetSigner) ContractEventName() string {
	return LiquidConnectorSetSignerEventName
}

// UnpackSetSignerEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event SetSigner(address indexed signer)
func (liquidConnector *LiquidConnector) UnpackSetSignerEvent(log *types.Log) (*LiquidConnectorSetSigner, error) {
	event := "SetSigner"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != liquidConnector.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(LiquidConnectorSetSigner)
	if len(log.Data) > 0 {
		if err := liquidConnector.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range liquidConnector.abi.Events[event].Inputs {
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
func (liquidConnector *LiquidConnector) UnpackError(raw []byte) (any, error) {
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["AlreadyInitialized"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackAlreadyInitializedError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["AlreadyUsedNonce"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackAlreadyUsedNonceError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["AssetLimitExceeded"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackAssetLimitExceededError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["ExpiredSwap"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackExpiredSwapError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidAccount"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidCaller"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidCallerError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidDiscount"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidDiscountError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidDuration"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidDurationError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidInitialization"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidInitializationError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidOracle"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidOracleError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidShortString"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidShortStringError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidSignature"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidSignatureError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidSigner"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidSignerError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidSwapRate"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidSwapRateError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["InvalidTokenToRedeem"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackInvalidTokenToRedeemError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["NotFactory"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackNotFactoryError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["NotInitializing"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackNotInitializingError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["OwnableInvalidOwner"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackOwnableInvalidOwnerError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["OwnableUnauthorizedAccount"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackOwnableUnauthorizedAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["ReentrancyGuardReentrantCall"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackReentrancyGuardReentrantCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["SafeERC20FailedOperation"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackSafeERC20FailedOperationError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["StringTooLong"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackStringTooLongError(raw[4:])
	}
	if bytes.Equal(raw[:4], liquidConnector.abi.Errors["UnconfiguredAsset"].ID.Bytes()[:4]) {
		return liquidConnector.UnpackUnconfiguredAssetError(raw[4:])
	}
	return nil, errors.New("Unknown error")
}

// LiquidConnectorAlreadyInitialized represents a AlreadyInitialized error raised by the LiquidConnector contract.
type LiquidConnectorAlreadyInitialized struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AlreadyInitialized()
func LiquidConnectorAlreadyInitializedErrorID() common.Hash {
	return common.HexToHash("0x0dc149f07762891dbcea3fe72770f3d63a1863fc54b2f084e8c59ec476996927")
}

// UnpackAlreadyInitializedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AlreadyInitialized()
func (liquidConnector *LiquidConnector) UnpackAlreadyInitializedError(raw []byte) (*LiquidConnectorAlreadyInitialized, error) {
	out := new(LiquidConnectorAlreadyInitialized)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "AlreadyInitialized", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorAlreadyUsedNonce represents a AlreadyUsedNonce error raised by the LiquidConnector contract.
type LiquidConnectorAlreadyUsedNonce struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AlreadyUsedNonce()
func LiquidConnectorAlreadyUsedNonceErrorID() common.Hash {
	return common.HexToHash("0x5247cb318fe1d375b7264beff576b70703d01bb4ac69fa1e1530f58741bcaf65")
}

// UnpackAlreadyUsedNonceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AlreadyUsedNonce()
func (liquidConnector *LiquidConnector) UnpackAlreadyUsedNonceError(raw []byte) (*LiquidConnectorAlreadyUsedNonce, error) {
	out := new(LiquidConnectorAlreadyUsedNonce)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "AlreadyUsedNonce", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorAssetLimitExceeded represents a AssetLimitExceeded error raised by the LiquidConnector contract.
type LiquidConnectorAssetLimitExceeded struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AssetLimitExceeded()
func LiquidConnectorAssetLimitExceededErrorID() common.Hash {
	return common.HexToHash("0x60b61aa9fddeb67a340693aaa708501fda9326e5142914e68238c77925c8c334")
}

// UnpackAssetLimitExceededError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AssetLimitExceeded()
func (liquidConnector *LiquidConnector) UnpackAssetLimitExceededError(raw []byte) (*LiquidConnectorAssetLimitExceeded, error) {
	out := new(LiquidConnectorAssetLimitExceeded)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "AssetLimitExceeded", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorExpiredSwap represents a ExpiredSwap error raised by the LiquidConnector contract.
type LiquidConnectorExpiredSwap struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ExpiredSwap()
func LiquidConnectorExpiredSwapErrorID() common.Hash {
	return common.HexToHash("0x1c74180ce4ab063a7fe469e2a3b2a873fe947c3b7649f1a95af498681712959d")
}

// UnpackExpiredSwapError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ExpiredSwap()
func (liquidConnector *LiquidConnector) UnpackExpiredSwapError(raw []byte) (*LiquidConnectorExpiredSwap, error) {
	out := new(LiquidConnectorExpiredSwap)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "ExpiredSwap", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidAccount represents a InvalidAccount error raised by the LiquidConnector contract.
type LiquidConnectorInvalidAccount struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidAccount()
func LiquidConnectorInvalidAccountErrorID() common.Hash {
	return common.HexToHash("0x6d187b28dc0ac7ec3747dcca312a7f01229ba51941237588cf813e48090f2a50")
}

// UnpackInvalidAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidAccount()
func (liquidConnector *LiquidConnector) UnpackInvalidAccountError(raw []byte) (*LiquidConnectorInvalidAccount, error) {
	out := new(LiquidConnectorInvalidAccount)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidCaller represents a InvalidCaller error raised by the LiquidConnector contract.
type LiquidConnectorInvalidCaller struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCaller()
func LiquidConnectorInvalidCallerErrorID() common.Hash {
	return common.HexToHash("0x48f5c3ed50241a1b6c87d204a25d9d01339cd768de9a714ffbb53a5bb6ad572a")
}

// UnpackInvalidCallerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCaller()
func (liquidConnector *LiquidConnector) UnpackInvalidCallerError(raw []byte) (*LiquidConnectorInvalidCaller, error) {
	out := new(LiquidConnectorInvalidCaller)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidCaller", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidDiscount represents a InvalidDiscount error raised by the LiquidConnector contract.
type LiquidConnectorInvalidDiscount struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidDiscount()
func LiquidConnectorInvalidDiscountErrorID() common.Hash {
	return common.HexToHash("0x997ea3601b175042a225bcd305c8e0434449bb795e78e4cb3f93a718c1a88caf")
}

// UnpackInvalidDiscountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidDiscount()
func (liquidConnector *LiquidConnector) UnpackInvalidDiscountError(raw []byte) (*LiquidConnectorInvalidDiscount, error) {
	out := new(LiquidConnectorInvalidDiscount)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidDiscount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidDuration represents a InvalidDuration error raised by the LiquidConnector contract.
type LiquidConnectorInvalidDuration struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidDuration()
func LiquidConnectorInvalidDurationErrorID() common.Hash {
	return common.HexToHash("0x76166401494e5a511541446d9197f2558d8ed87f455106fcd343b5ffbb4fc7bb")
}

// UnpackInvalidDurationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidDuration()
func (liquidConnector *LiquidConnector) UnpackInvalidDurationError(raw []byte) (*LiquidConnectorInvalidDuration, error) {
	out := new(LiquidConnectorInvalidDuration)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidDuration", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidInitialization represents a InvalidInitialization error raised by the LiquidConnector contract.
type LiquidConnectorInvalidInitialization struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidInitialization()
func LiquidConnectorInvalidInitializationErrorID() common.Hash {
	return common.HexToHash("0xf92ee8a957075833165f68c320933b1a1294aafc84ee6e0dd3fb178008f9aaf5")
}

// UnpackInvalidInitializationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidInitialization()
func (liquidConnector *LiquidConnector) UnpackInvalidInitializationError(raw []byte) (*LiquidConnectorInvalidInitialization, error) {
	out := new(LiquidConnectorInvalidInitialization)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidInitialization", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidOracle represents a InvalidOracle error raised by the LiquidConnector contract.
type LiquidConnectorInvalidOracle struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidOracle()
func LiquidConnectorInvalidOracleErrorID() common.Hash {
	return common.HexToHash("0x9589a27d464cce309224596a505cbfd22e5fde1f0f420cecf8a6b6c1d65791b6")
}

// UnpackInvalidOracleError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidOracle()
func (liquidConnector *LiquidConnector) UnpackInvalidOracleError(raw []byte) (*LiquidConnectorInvalidOracle, error) {
	out := new(LiquidConnectorInvalidOracle)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidOracle", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidShortString represents a InvalidShortString error raised by the LiquidConnector contract.
type LiquidConnectorInvalidShortString struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidShortString()
func LiquidConnectorInvalidShortStringErrorID() common.Hash {
	return common.HexToHash("0xb3512b0c6163e5f0bafab72bb631b9d58cd7a731b082f910338aa21c83d5c274")
}

// UnpackInvalidShortStringError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidShortString()
func (liquidConnector *LiquidConnector) UnpackInvalidShortStringError(raw []byte) (*LiquidConnectorInvalidShortString, error) {
	out := new(LiquidConnectorInvalidShortString)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidShortString", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidSignature represents a InvalidSignature error raised by the LiquidConnector contract.
type LiquidConnectorInvalidSignature struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidSignature()
func LiquidConnectorInvalidSignatureErrorID() common.Hash {
	return common.HexToHash("0x8baa579fce362245063d36f11747a89dd489c54795634fc673cc0e0db51fedc5")
}

// UnpackInvalidSignatureError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidSignature()
func (liquidConnector *LiquidConnector) UnpackInvalidSignatureError(raw []byte) (*LiquidConnectorInvalidSignature, error) {
	out := new(LiquidConnectorInvalidSignature)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidSignature", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidSigner represents a InvalidSigner error raised by the LiquidConnector contract.
type LiquidConnectorInvalidSigner struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidSigner()
func LiquidConnectorInvalidSignerErrorID() common.Hash {
	return common.HexToHash("0x815e1d64efb74fbe314c20a2b8a2335d18bce12a19165e447fa36bcb35959528")
}

// UnpackInvalidSignerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidSigner()
func (liquidConnector *LiquidConnector) UnpackInvalidSignerError(raw []byte) (*LiquidConnectorInvalidSigner, error) {
	out := new(LiquidConnectorInvalidSigner)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidSigner", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidSwapRate represents a InvalidSwapRate error raised by the LiquidConnector contract.
type LiquidConnectorInvalidSwapRate struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidSwapRate()
func LiquidConnectorInvalidSwapRateErrorID() common.Hash {
	return common.HexToHash("0x285217814127ad6ebb3698d6e77d442a1bfc2ce040377b7a098f217e85a0789e")
}

// UnpackInvalidSwapRateError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidSwapRate()
func (liquidConnector *LiquidConnector) UnpackInvalidSwapRateError(raw []byte) (*LiquidConnectorInvalidSwapRate, error) {
	out := new(LiquidConnectorInvalidSwapRate)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidSwapRate", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorInvalidTokenToRedeem represents a InvalidTokenToRedeem error raised by the LiquidConnector contract.
type LiquidConnectorInvalidTokenToRedeem struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidTokenToRedeem()
func LiquidConnectorInvalidTokenToRedeemErrorID() common.Hash {
	return common.HexToHash("0x997c4c8864a0026f8a0f15ff73d73b124e940d19db898a179d4983158f41cdf5")
}

// UnpackInvalidTokenToRedeemError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidTokenToRedeem()
func (liquidConnector *LiquidConnector) UnpackInvalidTokenToRedeemError(raw []byte) (*LiquidConnectorInvalidTokenToRedeem, error) {
	out := new(LiquidConnectorInvalidTokenToRedeem)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "InvalidTokenToRedeem", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorNotFactory represents a NotFactory error raised by the LiquidConnector contract.
type LiquidConnectorNotFactory struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error NotFactory()
func LiquidConnectorNotFactoryErrorID() common.Hash {
	return common.HexToHash("0x32cc723614e775fc4a8386492bc9a860c12fe98d5f5f28ec17e265818645b229")
}

// UnpackNotFactoryError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error NotFactory()
func (liquidConnector *LiquidConnector) UnpackNotFactoryError(raw []byte) (*LiquidConnectorNotFactory, error) {
	out := new(LiquidConnectorNotFactory)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "NotFactory", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorNotInitializing represents a NotInitializing error raised by the LiquidConnector contract.
type LiquidConnectorNotInitializing struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error NotInitializing()
func LiquidConnectorNotInitializingErrorID() common.Hash {
	return common.HexToHash("0xd7e6bcf8597daa127dc9f0048d2f08d5ef140a2cb659feabd700beff1f7a8302")
}

// UnpackNotInitializingError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error NotInitializing()
func (liquidConnector *LiquidConnector) UnpackNotInitializingError(raw []byte) (*LiquidConnectorNotInitializing, error) {
	out := new(LiquidConnectorNotInitializing)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "NotInitializing", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorOwnableInvalidOwner represents a OwnableInvalidOwner error raised by the LiquidConnector contract.
type LiquidConnectorOwnableInvalidOwner struct {
	Owner common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableInvalidOwner(address owner)
func LiquidConnectorOwnableInvalidOwnerErrorID() common.Hash {
	return common.HexToHash("0x1e4fbdf7f3ef8bcaa855599e3abf48b232380f183f08f6f813d9ffa5bd585188")
}

// UnpackOwnableInvalidOwnerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableInvalidOwner(address owner)
func (liquidConnector *LiquidConnector) UnpackOwnableInvalidOwnerError(raw []byte) (*LiquidConnectorOwnableInvalidOwner, error) {
	out := new(LiquidConnectorOwnableInvalidOwner)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "OwnableInvalidOwner", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorOwnableUnauthorizedAccount represents a OwnableUnauthorizedAccount error raised by the LiquidConnector contract.
type LiquidConnectorOwnableUnauthorizedAccount struct {
	Account common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func LiquidConnectorOwnableUnauthorizedAccountErrorID() common.Hash {
	return common.HexToHash("0x118cdaa7a341953d1887a2245fd6665d741c67c8c50581daa59e1d03373fa188")
}

// UnpackOwnableUnauthorizedAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func (liquidConnector *LiquidConnector) UnpackOwnableUnauthorizedAccountError(raw []byte) (*LiquidConnectorOwnableUnauthorizedAccount, error) {
	out := new(LiquidConnectorOwnableUnauthorizedAccount)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "OwnableUnauthorizedAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorReentrancyGuardReentrantCall represents a ReentrancyGuardReentrantCall error raised by the LiquidConnector contract.
type LiquidConnectorReentrancyGuardReentrantCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ReentrancyGuardReentrantCall()
func LiquidConnectorReentrancyGuardReentrantCallErrorID() common.Hash {
	return common.HexToHash("0x3ee5aeb571de7fc460830b4d0017439a1ca56fb0bc39062227ade4fe4a24c1ca")
}

// UnpackReentrancyGuardReentrantCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ReentrancyGuardReentrantCall()
func (liquidConnector *LiquidConnector) UnpackReentrancyGuardReentrantCallError(raw []byte) (*LiquidConnectorReentrancyGuardReentrantCall, error) {
	out := new(LiquidConnectorReentrancyGuardReentrantCall)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "ReentrancyGuardReentrantCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorSafeERC20FailedOperation represents a SafeERC20FailedOperation error raised by the LiquidConnector contract.
type LiquidConnectorSafeERC20FailedOperation struct {
	Token common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error SafeERC20FailedOperation(address token)
func LiquidConnectorSafeERC20FailedOperationErrorID() common.Hash {
	return common.HexToHash("0x5274afe73c98b4749fc91ffae6b7b574e7842cb2144a159e9377a5f20b32edf9")
}

// UnpackSafeERC20FailedOperationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error SafeERC20FailedOperation(address token)
func (liquidConnector *LiquidConnector) UnpackSafeERC20FailedOperationError(raw []byte) (*LiquidConnectorSafeERC20FailedOperation, error) {
	out := new(LiquidConnectorSafeERC20FailedOperation)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "SafeERC20FailedOperation", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorStringTooLong represents a StringTooLong error raised by the LiquidConnector contract.
type LiquidConnectorStringTooLong struct {
	Str string
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error StringTooLong(string str)
func LiquidConnectorStringTooLongErrorID() common.Hash {
	return common.HexToHash("0x305a27a93f8e33b7392df0a0f91d6fc63847395853c45991eec52dbf24d72381")
}

// UnpackStringTooLongError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error StringTooLong(string str)
func (liquidConnector *LiquidConnector) UnpackStringTooLongError(raw []byte) (*LiquidConnectorStringTooLong, error) {
	out := new(LiquidConnectorStringTooLong)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "StringTooLong", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// LiquidConnectorUnconfiguredAsset represents a UnconfiguredAsset error raised by the LiquidConnector contract.
type LiquidConnectorUnconfiguredAsset struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error UnconfiguredAsset()
func LiquidConnectorUnconfiguredAssetErrorID() common.Hash {
	return common.HexToHash("0x552a0531c16f1a1cb5775b34916021b261addb8704ee0960c2f52e5afe6ef591")
}

// UnpackUnconfiguredAssetError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error UnconfiguredAsset()
func (liquidConnector *LiquidConnector) UnpackUnconfiguredAssetError(raw []byte) (*LiquidConnectorUnconfiguredAsset, error) {
	out := new(LiquidConnectorUnconfiguredAsset)
	if err := liquidConnector.abi.UnpackIntoInterface(out, "UnconfiguredAsset", raw); err != nil {
		return nil, err
	}
	return out, nil
}
