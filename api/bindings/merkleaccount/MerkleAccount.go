// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package merkleaccount

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

// IMerkleAccountAction is an auto generated low-level Go binding around an user-defined struct.
type IMerkleAccountAction struct {
	Target   common.Address
	Value    *big.Int
	Data     []byte
	DataMask []byte
	Proof    [][32]byte
}

// IMerkleAccountHash is an auto generated low-level Go binding around an user-defined struct.
type IMerkleAccountHash struct {
	Offset      *big.Int
	Length      *big.Int
	Destination *big.Int
}

// MerkleAccountMetaData contains all meta data concerning the MerkleAccount contract.
var MerkleAccountMetaData = bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"newFactory\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newMerkleAdapterFactory\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newRootRegistry\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"newCapabilitiesRegistry\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"fallback\",\"stateMutability\":\"payable\"},{\"type\":\"receive\",\"stateMutability\":\"payable\"},{\"type\":\"function\",\"name\":\"CAPABILITIES_REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DEFAULT_ADMIN_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"FACTORY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"MERKLE_ADAPTER_FACTORY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"ROOT_REGISTRY\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"acceptRoot\",\"inputs\":[{\"name\":\"root\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"adapter\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"addCapability\",\"inputs\":[{\"name\":\"capability\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"capabilityBySelector\",\"inputs\":[{\"name\":\"selector\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"capability\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"capabilitySelectors\",\"inputs\":[{\"name\":\"capability\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"selectors\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getCapabilities\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"address[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleAdmin\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleMember\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"index\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleMemberCount\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRoleMembers\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"address[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"grantRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hasRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"initialize\",\"inputs\":[{\"name\":\"initialVersion\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"owner_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"isValidSignature\",\"inputs\":[{\"name\":\"digest\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"manage\",\"inputs\":[{\"name\":\"actions\",\"type\":\"tuple[]\",\"internalType\":\"structIMerkleAccount.Action[]\",\"components\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint96\",\"internalType\":\"uint96\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"dataMask\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"proof\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"}]}],\"outputs\":[{\"name\":\"results\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"merkleRoot\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"migrate\",\"inputs\":[{\"name\":\"newVersion\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"multicall\",\"inputs\":[{\"name\":\"data\",\"type\":\"bytes[]\",\"internalType\":\"bytes[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"onERC721Received\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"removeCapability\",\"inputs\":[{\"name\":\"capability\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"callerConfirmation\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeSignature\",\"inputs\":[{\"name\":\"digest\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"sign\",\"inputs\":[{\"name\":\"domainSeparator\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"buffer\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"plan\",\"type\":\"tuple[]\",\"internalType\":\"structIMerkleAccount.Hash[]\",\"components\":[{\"name\":\"offset\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"length\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"destination\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"signatures\",\"inputs\":[{\"name\":\"digest\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"approved\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"version\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"AcceptRoot\",\"inputs\":[{\"name\":\"merkleRoot\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"AddCapability\",\"inputs\":[{\"name\":\"capability\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Initialized\",\"inputs\":[{\"name\":\"version\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Manage\",\"inputs\":[{\"name\":\"actions\",\"type\":\"tuple[]\",\"indexed\":false,\"internalType\":\"structIMerkleAccount.Action[]\",\"components\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint96\",\"internalType\":\"uint96\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"dataMask\",\"type\":\"bytes\",\"internalType\":\"bytes\"},{\"name\":\"proof\",\"type\":\"bytes32[]\",\"internalType\":\"bytes32[]\"}]}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RemoveCapability\",\"inputs\":[{\"name\":\"capability\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RevokeSignature\",\"inputs\":[{\"name\":\"digest\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleAdminChanged\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"previousAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"newAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleGranted\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleRevoked\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Sign\",\"inputs\":[{\"name\":\"digest\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"domainSeparator\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"buffer\",\"type\":\"bytes\",\"indexed\":false,\"internalType\":\"bytes\"},{\"name\":\"plan\",\"type\":\"tuple[]\",\"indexed\":false,\"internalType\":\"structIMerkleAccount.Hash[]\",\"components\":[{\"name\":\"offset\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"length\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"destination\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AccessControlBadConfirmation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AccessControlUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"neededRole\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AddressEmptyCode\",\"inputs\":[{\"name\":\"target\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"AlreadyInitialized\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CapabilityAlreadyAdded\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CapabilityNotAdded\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CapabilityNotWhitelisted\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"FailedCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InsufficientBalance\",\"inputs\":[{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"needed\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"InvalidAdapter\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidCaller\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidDataMaskLength\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidHashPlan\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidInitialization\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidMerkleProof\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidRoot\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotFactory\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotInitializing\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"SelectorAlreadyUsed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"UnknownSelector\",\"inputs\":[]}]",
	ID:  "MerkleAccount",
}

// MerkleAccount is an auto generated Go binding around an Ethereum contract.
type MerkleAccount struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *MerkleAccount) GetABI() abi.ABI {
	return c.abi
}

// NewMerkleAccount creates a new instance of MerkleAccount.
func NewMerkleAccount() *MerkleAccount {
	parsed, err := MerkleAccountMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &MerkleAccount{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *MerkleAccount) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackConstructor is the Go binding used to pack the parameters required for
// contract deployment.
//
// Solidity: constructor(address newFactory, address newMerkleAdapterFactory, address newRootRegistry, address newCapabilitiesRegistry) returns()
func (merkleAccount *MerkleAccount) PackConstructor(newFactory common.Address, newMerkleAdapterFactory common.Address, newRootRegistry common.Address, newCapabilitiesRegistry common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("", newFactory, newMerkleAdapterFactory, newRootRegistry, newCapabilitiesRegistry)
	if err != nil {
		panic(err)
	}
	return enc
}

// PackCAPABILITIESREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5e4dcb53.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function CAPABILITIES_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) PackCAPABILITIESREGISTRY() []byte {
	enc, err := merkleAccount.abi.Pack("CAPABILITIES_REGISTRY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCAPABILITIESREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5e4dcb53.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function CAPABILITIES_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) TryPackCAPABILITIESREGISTRY() ([]byte, error) {
	return merkleAccount.abi.Pack("CAPABILITIES_REGISTRY")
}

// UnpackCAPABILITIESREGISTRY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5e4dcb53.
//
// Solidity: function CAPABILITIES_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) UnpackCAPABILITIESREGISTRY(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("CAPABILITIES_REGISTRY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackDEFAULTADMINROLE is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa217fddf.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (merkleAccount *MerkleAccount) PackDEFAULTADMINROLE() []byte {
	enc, err := merkleAccount.abi.Pack("DEFAULT_ADMIN_ROLE")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDEFAULTADMINROLE is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa217fddf.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (merkleAccount *MerkleAccount) TryPackDEFAULTADMINROLE() ([]byte, error) {
	return merkleAccount.abi.Pack("DEFAULT_ADMIN_ROLE")
}

// UnpackDEFAULTADMINROLE is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (merkleAccount *MerkleAccount) UnpackDEFAULTADMINROLE(data []byte) ([32]byte, error) {
	out, err := merkleAccount.abi.Unpack("DEFAULT_ADMIN_ROLE", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2dd31000.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function FACTORY() view returns(address)
func (merkleAccount *MerkleAccount) PackFACTORY() []byte {
	enc, err := merkleAccount.abi.Pack("FACTORY")
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
func (merkleAccount *MerkleAccount) TryPackFACTORY() ([]byte, error) {
	return merkleAccount.abi.Pack("FACTORY")
}

// UnpackFACTORY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2dd31000.
//
// Solidity: function FACTORY() view returns(address)
func (merkleAccount *MerkleAccount) UnpackFACTORY(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("FACTORY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackMERKLEADAPTERFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x403f17a5.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function MERKLE_ADAPTER_FACTORY() view returns(address)
func (merkleAccount *MerkleAccount) PackMERKLEADAPTERFACTORY() []byte {
	enc, err := merkleAccount.abi.Pack("MERKLE_ADAPTER_FACTORY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMERKLEADAPTERFACTORY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x403f17a5.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function MERKLE_ADAPTER_FACTORY() view returns(address)
func (merkleAccount *MerkleAccount) TryPackMERKLEADAPTERFACTORY() ([]byte, error) {
	return merkleAccount.abi.Pack("MERKLE_ADAPTER_FACTORY")
}

// UnpackMERKLEADAPTERFACTORY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x403f17a5.
//
// Solidity: function MERKLE_ADAPTER_FACTORY() view returns(address)
func (merkleAccount *MerkleAccount) UnpackMERKLEADAPTERFACTORY(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("MERKLE_ADAPTER_FACTORY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackROOTREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc92cc49a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function ROOT_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) PackROOTREGISTRY() []byte {
	enc, err := merkleAccount.abi.Pack("ROOT_REGISTRY")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackROOTREGISTRY is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc92cc49a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function ROOT_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) TryPackROOTREGISTRY() ([]byte, error) {
	return merkleAccount.abi.Pack("ROOT_REGISTRY")
}

// UnpackROOTREGISTRY is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xc92cc49a.
//
// Solidity: function ROOT_REGISTRY() view returns(address)
func (merkleAccount *MerkleAccount) UnpackROOTREGISTRY(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("ROOT_REGISTRY", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackAcceptRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7c234289.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function acceptRoot(bytes32 root) returns()
func (merkleAccount *MerkleAccount) PackAcceptRoot(root [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("acceptRoot", root)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAcceptRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7c234289.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function acceptRoot(bytes32 root) returns()
func (merkleAccount *MerkleAccount) TryPackAcceptRoot(root [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("acceptRoot", root)
}

// PackAdapter is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x03eadcfc.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function adapter() view returns(address)
func (merkleAccount *MerkleAccount) PackAdapter() []byte {
	enc, err := merkleAccount.abi.Pack("adapter")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAdapter is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x03eadcfc.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function adapter() view returns(address)
func (merkleAccount *MerkleAccount) TryPackAdapter() ([]byte, error) {
	return merkleAccount.abi.Pack("adapter")
}

// UnpackAdapter is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x03eadcfc.
//
// Solidity: function adapter() view returns(address)
func (merkleAccount *MerkleAccount) UnpackAdapter(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("adapter", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackAddCapability is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7714e835.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function addCapability(address capability) returns()
func (merkleAccount *MerkleAccount) PackAddCapability(capability common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("addCapability", capability)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAddCapability is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7714e835.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function addCapability(address capability) returns()
func (merkleAccount *MerkleAccount) TryPackAddCapability(capability common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("addCapability", capability)
}

// PackCapabilityBySelector is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5c1c0f58.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function capabilityBySelector(bytes4 selector) view returns(address capability)
func (merkleAccount *MerkleAccount) PackCapabilityBySelector(selector [4]byte) []byte {
	enc, err := merkleAccount.abi.Pack("capabilityBySelector", selector)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCapabilityBySelector is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5c1c0f58.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function capabilityBySelector(bytes4 selector) view returns(address capability)
func (merkleAccount *MerkleAccount) TryPackCapabilityBySelector(selector [4]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("capabilityBySelector", selector)
}

// UnpackCapabilityBySelector is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5c1c0f58.
//
// Solidity: function capabilityBySelector(bytes4 selector) view returns(address capability)
func (merkleAccount *MerkleAccount) UnpackCapabilityBySelector(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("capabilityBySelector", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackCapabilitySelectors is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x37f02013.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function capabilitySelectors(address capability, uint256 ) view returns(bytes4 selectors)
func (merkleAccount *MerkleAccount) PackCapabilitySelectors(capability common.Address, arg1 *big.Int) []byte {
	enc, err := merkleAccount.abi.Pack("capabilitySelectors", capability, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackCapabilitySelectors is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x37f02013.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function capabilitySelectors(address capability, uint256 ) view returns(bytes4 selectors)
func (merkleAccount *MerkleAccount) TryPackCapabilitySelectors(capability common.Address, arg1 *big.Int) ([]byte, error) {
	return merkleAccount.abi.Pack("capabilitySelectors", capability, arg1)
}

// UnpackCapabilitySelectors is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x37f02013.
//
// Solidity: function capabilitySelectors(address capability, uint256 ) view returns(bytes4 selectors)
func (merkleAccount *MerkleAccount) UnpackCapabilitySelectors(data []byte) ([4]byte, error) {
	out, err := merkleAccount.abi.Unpack("capabilitySelectors", data)
	if err != nil {
		return *new([4]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([4]byte)).(*[4]byte)
	return out0, nil
}

// PackGetCapabilities is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xddbe4f82.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getCapabilities() view returns(address[])
func (merkleAccount *MerkleAccount) PackGetCapabilities() []byte {
	enc, err := merkleAccount.abi.Pack("getCapabilities")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetCapabilities is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xddbe4f82.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getCapabilities() view returns(address[])
func (merkleAccount *MerkleAccount) TryPackGetCapabilities() ([]byte, error) {
	return merkleAccount.abi.Pack("getCapabilities")
}

// UnpackGetCapabilities is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xddbe4f82.
//
// Solidity: function getCapabilities() view returns(address[])
func (merkleAccount *MerkleAccount) UnpackGetCapabilities(data []byte) ([]common.Address, error) {
	out, err := merkleAccount.abi.Unpack("getCapabilities", data)
	if err != nil {
		return *new([]common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	return out0, nil
}

// PackGetRoleAdmin is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x248a9ca3.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (merkleAccount *MerkleAccount) PackGetRoleAdmin(role [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("getRoleAdmin", role)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRoleAdmin is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x248a9ca3.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (merkleAccount *MerkleAccount) TryPackGetRoleAdmin(role [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("getRoleAdmin", role)
}

// UnpackGetRoleAdmin is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (merkleAccount *MerkleAccount) UnpackGetRoleAdmin(data []byte) ([32]byte, error) {
	out, err := merkleAccount.abi.Unpack("getRoleAdmin", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackGetRoleMember is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9010d07c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRoleMember(bytes32 role, uint256 index) view returns(address)
func (merkleAccount *MerkleAccount) PackGetRoleMember(role [32]byte, index *big.Int) []byte {
	enc, err := merkleAccount.abi.Pack("getRoleMember", role, index)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRoleMember is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x9010d07c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRoleMember(bytes32 role, uint256 index) view returns(address)
func (merkleAccount *MerkleAccount) TryPackGetRoleMember(role [32]byte, index *big.Int) ([]byte, error) {
	return merkleAccount.abi.Pack("getRoleMember", role, index)
}

// UnpackGetRoleMember is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x9010d07c.
//
// Solidity: function getRoleMember(bytes32 role, uint256 index) view returns(address)
func (merkleAccount *MerkleAccount) UnpackGetRoleMember(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("getRoleMember", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackGetRoleMemberCount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xca15c873.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRoleMemberCount(bytes32 role) view returns(uint256)
func (merkleAccount *MerkleAccount) PackGetRoleMemberCount(role [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("getRoleMemberCount", role)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRoleMemberCount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xca15c873.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRoleMemberCount(bytes32 role) view returns(uint256)
func (merkleAccount *MerkleAccount) TryPackGetRoleMemberCount(role [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("getRoleMemberCount", role)
}

// UnpackGetRoleMemberCount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xca15c873.
//
// Solidity: function getRoleMemberCount(bytes32 role) view returns(uint256)
func (merkleAccount *MerkleAccount) UnpackGetRoleMemberCount(data []byte) (*big.Int, error) {
	out, err := merkleAccount.abi.Unpack("getRoleMemberCount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackGetRoleMembers is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa3246ad3.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRoleMembers(bytes32 role) view returns(address[])
func (merkleAccount *MerkleAccount) PackGetRoleMembers(role [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("getRoleMembers", role)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRoleMembers is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xa3246ad3.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRoleMembers(bytes32 role) view returns(address[])
func (merkleAccount *MerkleAccount) TryPackGetRoleMembers(role [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("getRoleMembers", role)
}

// UnpackGetRoleMembers is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xa3246ad3.
//
// Solidity: function getRoleMembers(bytes32 role) view returns(address[])
func (merkleAccount *MerkleAccount) UnpackGetRoleMembers(data []byte) ([]common.Address, error) {
	out, err := merkleAccount.abi.Unpack("getRoleMembers", data)
	if err != nil {
		return *new([]common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	return out0, nil
}

// PackGrantRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2f2ff15d.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (merkleAccount *MerkleAccount) PackGrantRole(role [32]byte, account common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("grantRole", role, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGrantRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2f2ff15d.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (merkleAccount *MerkleAccount) TryPackGrantRole(role [32]byte, account common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("grantRole", role, account)
}

// PackHasRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x91d14854.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (merkleAccount *MerkleAccount) PackHasRole(role [32]byte, account common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("hasRole", role, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackHasRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x91d14854.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (merkleAccount *MerkleAccount) TryPackHasRole(role [32]byte, account common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("hasRole", role, account)
}

// UnpackHasRole is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (merkleAccount *MerkleAccount) UnpackHasRole(data []byte) (bool, error) {
	out, err := merkleAccount.abi.Unpack("hasRole", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackInitialize is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x57ec83cc.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function initialize(uint64 initialVersion, address owner_, bytes data) returns()
func (merkleAccount *MerkleAccount) PackInitialize(initialVersion uint64, owner common.Address, data []byte) []byte {
	enc, err := merkleAccount.abi.Pack("initialize", initialVersion, owner, data)
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
func (merkleAccount *MerkleAccount) TryPackInitialize(initialVersion uint64, owner common.Address, data []byte) ([]byte, error) {
	return merkleAccount.abi.Pack("initialize", initialVersion, owner, data)
}

// PackIsValidSignature is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1626ba7e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function isValidSignature(bytes32 digest, bytes ) view returns(bytes4)
func (merkleAccount *MerkleAccount) PackIsValidSignature(digest [32]byte, arg1 []byte) []byte {
	enc, err := merkleAccount.abi.Pack("isValidSignature", digest, arg1)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackIsValidSignature is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1626ba7e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function isValidSignature(bytes32 digest, bytes ) view returns(bytes4)
func (merkleAccount *MerkleAccount) TryPackIsValidSignature(digest [32]byte, arg1 []byte) ([]byte, error) {
	return merkleAccount.abi.Pack("isValidSignature", digest, arg1)
}

// UnpackIsValidSignature is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x1626ba7e.
//
// Solidity: function isValidSignature(bytes32 digest, bytes ) view returns(bytes4)
func (merkleAccount *MerkleAccount) UnpackIsValidSignature(data []byte) ([4]byte, error) {
	out, err := merkleAccount.abi.Unpack("isValidSignature", data)
	if err != nil {
		return *new([4]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([4]byte)).(*[4]byte)
	return out0, nil
}

// PackManage is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xda21f7df.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function manage((address,uint96,bytes,bytes,bytes32[])[] actions) returns(bytes[] results)
func (merkleAccount *MerkleAccount) PackManage(actions []IMerkleAccountAction) []byte {
	enc, err := merkleAccount.abi.Pack("manage", actions)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackManage is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xda21f7df.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function manage((address,uint96,bytes,bytes,bytes32[])[] actions) returns(bytes[] results)
func (merkleAccount *MerkleAccount) TryPackManage(actions []IMerkleAccountAction) ([]byte, error) {
	return merkleAccount.abi.Pack("manage", actions)
}

// UnpackManage is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xda21f7df.
//
// Solidity: function manage((address,uint96,bytes,bytes,bytes32[])[] actions) returns(bytes[] results)
func (merkleAccount *MerkleAccount) UnpackManage(data []byte) ([][]byte, error) {
	out, err := merkleAccount.abi.Unpack("manage", data)
	if err != nil {
		return *new([][]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([][]byte)).(*[][]byte)
	return out0, nil
}

// PackMerkleRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2eb4a7ab.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function merkleRoot() view returns(bytes32)
func (merkleAccount *MerkleAccount) PackMerkleRoot() []byte {
	enc, err := merkleAccount.abi.Pack("merkleRoot")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMerkleRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2eb4a7ab.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function merkleRoot() view returns(bytes32)
func (merkleAccount *MerkleAccount) TryPackMerkleRoot() ([]byte, error) {
	return merkleAccount.abi.Pack("merkleRoot")
}

// UnpackMerkleRoot is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2eb4a7ab.
//
// Solidity: function merkleRoot() view returns(bytes32)
func (merkleAccount *MerkleAccount) UnpackMerkleRoot(data []byte) ([32]byte, error) {
	out, err := merkleAccount.abi.Unpack("merkleRoot", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackMigrate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2abe3048.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function migrate(uint64 newVersion, bytes data) returns()
func (merkleAccount *MerkleAccount) PackMigrate(newVersion uint64, data []byte) []byte {
	enc, err := merkleAccount.abi.Pack("migrate", newVersion, data)
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
func (merkleAccount *MerkleAccount) TryPackMigrate(newVersion uint64, data []byte) ([]byte, error) {
	return merkleAccount.abi.Pack("migrate", newVersion, data)
}

// PackMulticall is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xac9650d8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function multicall(bytes[] data) returns()
func (merkleAccount *MerkleAccount) PackMulticall(data [][]byte) []byte {
	enc, err := merkleAccount.abi.Pack("multicall", data)
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
func (merkleAccount *MerkleAccount) TryPackMulticall(data [][]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("multicall", data)
}

// PackOnERC721Received is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x150b7a02.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function onERC721Received(address , address , uint256 , bytes ) returns(bytes4)
func (merkleAccount *MerkleAccount) PackOnERC721Received(arg0 common.Address, arg1 common.Address, arg2 *big.Int, arg3 []byte) []byte {
	enc, err := merkleAccount.abi.Pack("onERC721Received", arg0, arg1, arg2, arg3)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackOnERC721Received is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x150b7a02.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function onERC721Received(address , address , uint256 , bytes ) returns(bytes4)
func (merkleAccount *MerkleAccount) TryPackOnERC721Received(arg0 common.Address, arg1 common.Address, arg2 *big.Int, arg3 []byte) ([]byte, error) {
	return merkleAccount.abi.Pack("onERC721Received", arg0, arg1, arg2, arg3)
}

// UnpackOnERC721Received is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x150b7a02.
//
// Solidity: function onERC721Received(address , address , uint256 , bytes ) returns(bytes4)
func (merkleAccount *MerkleAccount) UnpackOnERC721Received(data []byte) ([4]byte, error) {
	out, err := merkleAccount.abi.Unpack("onERC721Received", data)
	if err != nil {
		return *new([4]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([4]byte)).(*[4]byte)
	return out0, nil
}

// PackOwner is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x8da5cb5b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function owner() view returns(address)
func (merkleAccount *MerkleAccount) PackOwner() []byte {
	enc, err := merkleAccount.abi.Pack("owner")
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
func (merkleAccount *MerkleAccount) TryPackOwner() ([]byte, error) {
	return merkleAccount.abi.Pack("owner")
}

// UnpackOwner is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (merkleAccount *MerkleAccount) UnpackOwner(data []byte) (common.Address, error) {
	out, err := merkleAccount.abi.Unpack("owner", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRemoveCapability is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x134f905a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function removeCapability(address capability) returns()
func (merkleAccount *MerkleAccount) PackRemoveCapability(capability common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("removeCapability", capability)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRemoveCapability is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x134f905a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function removeCapability(address capability) returns()
func (merkleAccount *MerkleAccount) TryPackRemoveCapability(capability common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("removeCapability", capability)
}

// PackRenounceOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x715018a6.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function renounceOwnership() returns()
func (merkleAccount *MerkleAccount) PackRenounceOwnership() []byte {
	enc, err := merkleAccount.abi.Pack("renounceOwnership")
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
func (merkleAccount *MerkleAccount) TryPackRenounceOwnership() ([]byte, error) {
	return merkleAccount.abi.Pack("renounceOwnership")
}

// PackRenounceRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x36568abe.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (merkleAccount *MerkleAccount) PackRenounceRole(role [32]byte, callerConfirmation common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("renounceRole", role, callerConfirmation)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRenounceRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x36568abe.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (merkleAccount *MerkleAccount) TryPackRenounceRole(role [32]byte, callerConfirmation common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("renounceRole", role, callerConfirmation)
}

// PackRevokeRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd547741f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (merkleAccount *MerkleAccount) PackRevokeRole(role [32]byte, account common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("revokeRole", role, account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRevokeRole is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xd547741f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (merkleAccount *MerkleAccount) TryPackRevokeRole(role [32]byte, account common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("revokeRole", role, account)
}

// PackRevokeSignature is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9fd8ee8.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function revokeSignature(bytes32 digest) returns()
func (merkleAccount *MerkleAccount) PackRevokeSignature(digest [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("revokeSignature", digest)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRevokeSignature is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xc9fd8ee8.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function revokeSignature(bytes32 digest) returns()
func (merkleAccount *MerkleAccount) TryPackRevokeSignature(digest [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("revokeSignature", digest)
}

// PackSign is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe285d46e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function sign(bytes32 domainSeparator, bytes buffer, (uint256,uint256,uint256)[] plan) returns()
func (merkleAccount *MerkleAccount) PackSign(domainSeparator [32]byte, buffer []byte, plan []IMerkleAccountHash) []byte {
	enc, err := merkleAccount.abi.Pack("sign", domainSeparator, buffer, plan)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSign is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe285d46e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function sign(bytes32 domainSeparator, bytes buffer, (uint256,uint256,uint256)[] plan) returns()
func (merkleAccount *MerkleAccount) TryPackSign(domainSeparator [32]byte, buffer []byte, plan []IMerkleAccountHash) ([]byte, error) {
	return merkleAccount.abi.Pack("sign", domainSeparator, buffer, plan)
}

// PackSignatures is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2422224e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function signatures(bytes32 digest) view returns(bool approved)
func (merkleAccount *MerkleAccount) PackSignatures(digest [32]byte) []byte {
	enc, err := merkleAccount.abi.Pack("signatures", digest)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSignatures is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2422224e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function signatures(bytes32 digest) view returns(bool approved)
func (merkleAccount *MerkleAccount) TryPackSignatures(digest [32]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("signatures", digest)
}

// UnpackSignatures is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2422224e.
//
// Solidity: function signatures(bytes32 digest) view returns(bool approved)
func (merkleAccount *MerkleAccount) UnpackSignatures(data []byte) (bool, error) {
	out, err := merkleAccount.abi.Unpack("signatures", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackSupportsInterface is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x01ffc9a7.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (merkleAccount *MerkleAccount) PackSupportsInterface(interfaceId [4]byte) []byte {
	enc, err := merkleAccount.abi.Pack("supportsInterface", interfaceId)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackSupportsInterface is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x01ffc9a7.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (merkleAccount *MerkleAccount) TryPackSupportsInterface(interfaceId [4]byte) ([]byte, error) {
	return merkleAccount.abi.Pack("supportsInterface", interfaceId)
}

// UnpackSupportsInterface is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (merkleAccount *MerkleAccount) UnpackSupportsInterface(data []byte) (bool, error) {
	out, err := merkleAccount.abi.Unpack("supportsInterface", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackTransferOwnership is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf2fde38b.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (merkleAccount *MerkleAccount) PackTransferOwnership(newOwner common.Address) []byte {
	enc, err := merkleAccount.abi.Pack("transferOwnership", newOwner)
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
func (merkleAccount *MerkleAccount) TryPackTransferOwnership(newOwner common.Address) ([]byte, error) {
	return merkleAccount.abi.Pack("transferOwnership", newOwner)
}

// PackVersion is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x54fd4d50.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function version() view returns(uint64)
func (merkleAccount *MerkleAccount) PackVersion() []byte {
	enc, err := merkleAccount.abi.Pack("version")
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
func (merkleAccount *MerkleAccount) TryPackVersion() ([]byte, error) {
	return merkleAccount.abi.Pack("version")
}

// UnpackVersion is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x54fd4d50.
//
// Solidity: function version() view returns(uint64)
func (merkleAccount *MerkleAccount) UnpackVersion(data []byte) (uint64, error) {
	out, err := merkleAccount.abi.Unpack("version", data)
	if err != nil {
		return *new(uint64), err
	}
	out0 := *abi.ConvertType(out[0], new(uint64)).(*uint64)
	return out0, nil
}

// MerkleAccountAcceptRoot represents a AcceptRoot event raised by the MerkleAccount contract.
type MerkleAccountAcceptRoot struct {
	MerkleRoot [32]byte
	Raw        *types.Log // Blockchain specific contextual infos
}

const MerkleAccountAcceptRootEventName = "AcceptRoot"

// ContractEventName returns the user-defined event name.
func (MerkleAccountAcceptRoot) ContractEventName() string {
	return MerkleAccountAcceptRootEventName
}

// UnpackAcceptRootEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event AcceptRoot(bytes32 indexed merkleRoot)
func (merkleAccount *MerkleAccount) UnpackAcceptRootEvent(log *types.Log) (*MerkleAccountAcceptRoot, error) {
	event := "AcceptRoot"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountAcceptRoot)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountAddCapability represents a AddCapability event raised by the MerkleAccount contract.
type MerkleAccountAddCapability struct {
	Capability common.Address
	Raw        *types.Log // Blockchain specific contextual infos
}

const MerkleAccountAddCapabilityEventName = "AddCapability"

// ContractEventName returns the user-defined event name.
func (MerkleAccountAddCapability) ContractEventName() string {
	return MerkleAccountAddCapabilityEventName
}

// UnpackAddCapabilityEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event AddCapability(address indexed capability)
func (merkleAccount *MerkleAccount) UnpackAddCapabilityEvent(log *types.Log) (*MerkleAccountAddCapability, error) {
	event := "AddCapability"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountAddCapability)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountInitialized represents a Initialized event raised by the MerkleAccount contract.
type MerkleAccountInitialized struct {
	Version uint64
	Raw     *types.Log // Blockchain specific contextual infos
}

const MerkleAccountInitializedEventName = "Initialized"

// ContractEventName returns the user-defined event name.
func (MerkleAccountInitialized) ContractEventName() string {
	return MerkleAccountInitializedEventName
}

// UnpackInitializedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Initialized(uint64 version)
func (merkleAccount *MerkleAccount) UnpackInitializedEvent(log *types.Log) (*MerkleAccountInitialized, error) {
	event := "Initialized"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountInitialized)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountManage represents a Manage event raised by the MerkleAccount contract.
type MerkleAccountManage struct {
	Actions []IMerkleAccountAction
	Raw     *types.Log // Blockchain specific contextual infos
}

const MerkleAccountManageEventName = "Manage"

// ContractEventName returns the user-defined event name.
func (MerkleAccountManage) ContractEventName() string {
	return MerkleAccountManageEventName
}

// UnpackManageEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Manage((address,uint96,bytes,bytes,bytes32[])[] actions)
func (merkleAccount *MerkleAccount) UnpackManageEvent(log *types.Log) (*MerkleAccountManage, error) {
	event := "Manage"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountManage)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountOwnershipTransferred represents a OwnershipTransferred event raised by the MerkleAccount contract.
type MerkleAccountOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           *types.Log // Blockchain specific contextual infos
}

const MerkleAccountOwnershipTransferredEventName = "OwnershipTransferred"

// ContractEventName returns the user-defined event name.
func (MerkleAccountOwnershipTransferred) ContractEventName() string {
	return MerkleAccountOwnershipTransferredEventName
}

// UnpackOwnershipTransferredEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (merkleAccount *MerkleAccount) UnpackOwnershipTransferredEvent(log *types.Log) (*MerkleAccountOwnershipTransferred, error) {
	event := "OwnershipTransferred"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountOwnershipTransferred)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountRemoveCapability represents a RemoveCapability event raised by the MerkleAccount contract.
type MerkleAccountRemoveCapability struct {
	Capability common.Address
	Raw        *types.Log // Blockchain specific contextual infos
}

const MerkleAccountRemoveCapabilityEventName = "RemoveCapability"

// ContractEventName returns the user-defined event name.
func (MerkleAccountRemoveCapability) ContractEventName() string {
	return MerkleAccountRemoveCapabilityEventName
}

// UnpackRemoveCapabilityEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RemoveCapability(address indexed capability)
func (merkleAccount *MerkleAccount) UnpackRemoveCapabilityEvent(log *types.Log) (*MerkleAccountRemoveCapability, error) {
	event := "RemoveCapability"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountRemoveCapability)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountRevokeSignature represents a RevokeSignature event raised by the MerkleAccount contract.
type MerkleAccountRevokeSignature struct {
	Digest [32]byte
	Raw    *types.Log // Blockchain specific contextual infos
}

const MerkleAccountRevokeSignatureEventName = "RevokeSignature"

// ContractEventName returns the user-defined event name.
func (MerkleAccountRevokeSignature) ContractEventName() string {
	return MerkleAccountRevokeSignatureEventName
}

// UnpackRevokeSignatureEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RevokeSignature(bytes32 indexed digest)
func (merkleAccount *MerkleAccount) UnpackRevokeSignatureEvent(log *types.Log) (*MerkleAccountRevokeSignature, error) {
	event := "RevokeSignature"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountRevokeSignature)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountRoleAdminChanged represents a RoleAdminChanged event raised by the MerkleAccount contract.
type MerkleAccountRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               *types.Log // Blockchain specific contextual infos
}

const MerkleAccountRoleAdminChangedEventName = "RoleAdminChanged"

// ContractEventName returns the user-defined event name.
func (MerkleAccountRoleAdminChanged) ContractEventName() string {
	return MerkleAccountRoleAdminChangedEventName
}

// UnpackRoleAdminChangedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (merkleAccount *MerkleAccount) UnpackRoleAdminChangedEvent(log *types.Log) (*MerkleAccountRoleAdminChanged, error) {
	event := "RoleAdminChanged"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountRoleAdminChanged)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountRoleGranted represents a RoleGranted event raised by the MerkleAccount contract.
type MerkleAccountRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     *types.Log // Blockchain specific contextual infos
}

const MerkleAccountRoleGrantedEventName = "RoleGranted"

// ContractEventName returns the user-defined event name.
func (MerkleAccountRoleGranted) ContractEventName() string {
	return MerkleAccountRoleGrantedEventName
}

// UnpackRoleGrantedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (merkleAccount *MerkleAccount) UnpackRoleGrantedEvent(log *types.Log) (*MerkleAccountRoleGranted, error) {
	event := "RoleGranted"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountRoleGranted)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountRoleRevoked represents a RoleRevoked event raised by the MerkleAccount contract.
type MerkleAccountRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     *types.Log // Blockchain specific contextual infos
}

const MerkleAccountRoleRevokedEventName = "RoleRevoked"

// ContractEventName returns the user-defined event name.
func (MerkleAccountRoleRevoked) ContractEventName() string {
	return MerkleAccountRoleRevokedEventName
}

// UnpackRoleRevokedEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (merkleAccount *MerkleAccount) UnpackRoleRevokedEvent(log *types.Log) (*MerkleAccountRoleRevoked, error) {
	event := "RoleRevoked"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountRoleRevoked)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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

// MerkleAccountSign represents a Sign event raised by the MerkleAccount contract.
type MerkleAccountSign struct {
	Digest          [32]byte
	DomainSeparator [32]byte
	Buffer          []byte
	Plan            []IMerkleAccountHash
	Raw             *types.Log // Blockchain specific contextual infos
}

const MerkleAccountSignEventName = "Sign"

// ContractEventName returns the user-defined event name.
func (MerkleAccountSign) ContractEventName() string {
	return MerkleAccountSignEventName
}

// UnpackSignEvent is the Go binding that unpacks the event data emitted
// by contract.
//
// Solidity: event Sign(bytes32 indexed digest, bytes32 indexed domainSeparator, bytes buffer, (uint256,uint256,uint256)[] plan)
func (merkleAccount *MerkleAccount) UnpackSignEvent(log *types.Log) (*MerkleAccountSign, error) {
	event := "Sign"
	if len(log.Topics) == 0 {
		return nil, bind.ErrNoEventSignature
	}
	if log.Topics[0] != merkleAccount.abi.Events[event].ID {
		return nil, bind.ErrEventSignatureMismatch
	}
	out := new(MerkleAccountSign)
	if len(log.Data) > 0 {
		if err := merkleAccount.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return nil, err
		}
	}
	var indexed abi.Arguments
	for _, arg := range merkleAccount.abi.Events[event].Inputs {
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
func (merkleAccount *MerkleAccount) UnpackError(raw []byte) (any, error) {
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["AccessControlBadConfirmation"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackAccessControlBadConfirmationError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["AccessControlUnauthorizedAccount"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackAccessControlUnauthorizedAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["AddressEmptyCode"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackAddressEmptyCodeError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["AlreadyInitialized"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackAlreadyInitializedError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["CapabilityAlreadyAdded"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackCapabilityAlreadyAddedError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["CapabilityNotAdded"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackCapabilityNotAddedError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["CapabilityNotWhitelisted"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackCapabilityNotWhitelistedError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["FailedCall"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackFailedCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InsufficientBalance"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInsufficientBalanceError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidAdapter"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidAdapterError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidCaller"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidCallerError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidDataMaskLength"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidDataMaskLengthError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidHashPlan"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidHashPlanError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidInitialization"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidInitializationError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidMerkleProof"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidMerkleProofError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["InvalidRoot"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackInvalidRootError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["NotFactory"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackNotFactoryError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["NotInitializing"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackNotInitializingError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["OwnableInvalidOwner"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackOwnableInvalidOwnerError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["OwnableUnauthorizedAccount"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackOwnableUnauthorizedAccountError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["ReentrancyGuardReentrantCall"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackReentrancyGuardReentrantCallError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["SafeERC20FailedOperation"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackSafeERC20FailedOperationError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["SelectorAlreadyUsed"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackSelectorAlreadyUsedError(raw[4:])
	}
	if bytes.Equal(raw[:4], merkleAccount.abi.Errors["UnknownSelector"].ID.Bytes()[:4]) {
		return merkleAccount.UnpackUnknownSelectorError(raw[4:])
	}
	return nil, errors.New("Unknown error")
}

// MerkleAccountAccessControlBadConfirmation represents a AccessControlBadConfirmation error raised by the MerkleAccount contract.
type MerkleAccountAccessControlBadConfirmation struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AccessControlBadConfirmation()
func MerkleAccountAccessControlBadConfirmationErrorID() common.Hash {
	return common.HexToHash("0x6697b23232a647058342c0724fe7c415cab25915b54e5dbc03f233173d37b41c")
}

// UnpackAccessControlBadConfirmationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AccessControlBadConfirmation()
func (merkleAccount *MerkleAccount) UnpackAccessControlBadConfirmationError(raw []byte) (*MerkleAccountAccessControlBadConfirmation, error) {
	out := new(MerkleAccountAccessControlBadConfirmation)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "AccessControlBadConfirmation", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountAccessControlUnauthorizedAccount represents a AccessControlUnauthorizedAccount error raised by the MerkleAccount contract.
type MerkleAccountAccessControlUnauthorizedAccount struct {
	Account    common.Address
	NeededRole [32]byte
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AccessControlUnauthorizedAccount(address account, bytes32 neededRole)
func MerkleAccountAccessControlUnauthorizedAccountErrorID() common.Hash {
	return common.HexToHash("0xe2517d3fbfae6f8515ef5ff1ccedc3933ab0cbbda0b492c06eb54ad10ef03b3e")
}

// UnpackAccessControlUnauthorizedAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AccessControlUnauthorizedAccount(address account, bytes32 neededRole)
func (merkleAccount *MerkleAccount) UnpackAccessControlUnauthorizedAccountError(raw []byte) (*MerkleAccountAccessControlUnauthorizedAccount, error) {
	out := new(MerkleAccountAccessControlUnauthorizedAccount)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "AccessControlUnauthorizedAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountAddressEmptyCode represents a AddressEmptyCode error raised by the MerkleAccount contract.
type MerkleAccountAddressEmptyCode struct {
	Target common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AddressEmptyCode(address target)
func MerkleAccountAddressEmptyCodeErrorID() common.Hash {
	return common.HexToHash("0x9996b315c842ff135b8fc4a08ad5df1c344efbc03d2687aecc0678050d2aac89")
}

// UnpackAddressEmptyCodeError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AddressEmptyCode(address target)
func (merkleAccount *MerkleAccount) UnpackAddressEmptyCodeError(raw []byte) (*MerkleAccountAddressEmptyCode, error) {
	out := new(MerkleAccountAddressEmptyCode)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "AddressEmptyCode", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountAlreadyInitialized represents a AlreadyInitialized error raised by the MerkleAccount contract.
type MerkleAccountAlreadyInitialized struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error AlreadyInitialized()
func MerkleAccountAlreadyInitializedErrorID() common.Hash {
	return common.HexToHash("0x0dc149f07762891dbcea3fe72770f3d63a1863fc54b2f084e8c59ec476996927")
}

// UnpackAlreadyInitializedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error AlreadyInitialized()
func (merkleAccount *MerkleAccount) UnpackAlreadyInitializedError(raw []byte) (*MerkleAccountAlreadyInitialized, error) {
	out := new(MerkleAccountAlreadyInitialized)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "AlreadyInitialized", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountCapabilityAlreadyAdded represents a CapabilityAlreadyAdded error raised by the MerkleAccount contract.
type MerkleAccountCapabilityAlreadyAdded struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error CapabilityAlreadyAdded()
func MerkleAccountCapabilityAlreadyAddedErrorID() common.Hash {
	return common.HexToHash("0x94c3b03bbad06560a20d4f0ec15f01d1f6fb8894915d1dc7ee62e3f01d2b9668")
}

// UnpackCapabilityAlreadyAddedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error CapabilityAlreadyAdded()
func (merkleAccount *MerkleAccount) UnpackCapabilityAlreadyAddedError(raw []byte) (*MerkleAccountCapabilityAlreadyAdded, error) {
	out := new(MerkleAccountCapabilityAlreadyAdded)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "CapabilityAlreadyAdded", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountCapabilityNotAdded represents a CapabilityNotAdded error raised by the MerkleAccount contract.
type MerkleAccountCapabilityNotAdded struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error CapabilityNotAdded()
func MerkleAccountCapabilityNotAddedErrorID() common.Hash {
	return common.HexToHash("0x2565b9cf3c74ac126c984f2226c5b08c5b6d05a8e327bad6800f0eeab5cbbb32")
}

// UnpackCapabilityNotAddedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error CapabilityNotAdded()
func (merkleAccount *MerkleAccount) UnpackCapabilityNotAddedError(raw []byte) (*MerkleAccountCapabilityNotAdded, error) {
	out := new(MerkleAccountCapabilityNotAdded)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "CapabilityNotAdded", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountCapabilityNotWhitelisted represents a CapabilityNotWhitelisted error raised by the MerkleAccount contract.
type MerkleAccountCapabilityNotWhitelisted struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error CapabilityNotWhitelisted()
func MerkleAccountCapabilityNotWhitelistedErrorID() common.Hash {
	return common.HexToHash("0x7452f540932bb3fa1551e4f01424e9305a3f45666d869162c766b4fcd32c3a64")
}

// UnpackCapabilityNotWhitelistedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error CapabilityNotWhitelisted()
func (merkleAccount *MerkleAccount) UnpackCapabilityNotWhitelistedError(raw []byte) (*MerkleAccountCapabilityNotWhitelisted, error) {
	out := new(MerkleAccountCapabilityNotWhitelisted)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "CapabilityNotWhitelisted", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountFailedCall represents a FailedCall error raised by the MerkleAccount contract.
type MerkleAccountFailedCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error FailedCall()
func MerkleAccountFailedCallErrorID() common.Hash {
	return common.HexToHash("0xd6bda27508c0fb6d8a39b4b122878dab26f731a7d4e4abe711dd3731899052a4")
}

// UnpackFailedCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error FailedCall()
func (merkleAccount *MerkleAccount) UnpackFailedCallError(raw []byte) (*MerkleAccountFailedCall, error) {
	out := new(MerkleAccountFailedCall)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "FailedCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInsufficientBalance represents a InsufficientBalance error raised by the MerkleAccount contract.
type MerkleAccountInsufficientBalance struct {
	Balance *big.Int
	Needed  *big.Int
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func MerkleAccountInsufficientBalanceErrorID() common.Hash {
	return common.HexToHash("0xcf4791818fba6e019216eb4864093b4947f674afada5d305e57d598b641dad1d")
}

// UnpackInsufficientBalanceError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InsufficientBalance(uint256 balance, uint256 needed)
func (merkleAccount *MerkleAccount) UnpackInsufficientBalanceError(raw []byte) (*MerkleAccountInsufficientBalance, error) {
	out := new(MerkleAccountInsufficientBalance)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InsufficientBalance", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidAdapter represents a InvalidAdapter error raised by the MerkleAccount contract.
type MerkleAccountInvalidAdapter struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidAdapter()
func MerkleAccountInvalidAdapterErrorID() common.Hash {
	return common.HexToHash("0xfbf66df16c4e0cc635846a65caa4e7a61b59d2693cbd9bb842a271cee3fb9ae1")
}

// UnpackInvalidAdapterError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidAdapter()
func (merkleAccount *MerkleAccount) UnpackInvalidAdapterError(raw []byte) (*MerkleAccountInvalidAdapter, error) {
	out := new(MerkleAccountInvalidAdapter)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidAdapter", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidCaller represents a InvalidCaller error raised by the MerkleAccount contract.
type MerkleAccountInvalidCaller struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidCaller()
func MerkleAccountInvalidCallerErrorID() common.Hash {
	return common.HexToHash("0x48f5c3ed50241a1b6c87d204a25d9d01339cd768de9a714ffbb53a5bb6ad572a")
}

// UnpackInvalidCallerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidCaller()
func (merkleAccount *MerkleAccount) UnpackInvalidCallerError(raw []byte) (*MerkleAccountInvalidCaller, error) {
	out := new(MerkleAccountInvalidCaller)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidCaller", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidDataMaskLength represents a InvalidDataMaskLength error raised by the MerkleAccount contract.
type MerkleAccountInvalidDataMaskLength struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidDataMaskLength()
func MerkleAccountInvalidDataMaskLengthErrorID() common.Hash {
	return common.HexToHash("0x3609cb6625464291d6101e8be49be0729d37a57ea489cb9e041f9a5f86c64336")
}

// UnpackInvalidDataMaskLengthError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidDataMaskLength()
func (merkleAccount *MerkleAccount) UnpackInvalidDataMaskLengthError(raw []byte) (*MerkleAccountInvalidDataMaskLength, error) {
	out := new(MerkleAccountInvalidDataMaskLength)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidDataMaskLength", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidHashPlan represents a InvalidHashPlan error raised by the MerkleAccount contract.
type MerkleAccountInvalidHashPlan struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidHashPlan()
func MerkleAccountInvalidHashPlanErrorID() common.Hash {
	return common.HexToHash("0xc6a7fb040b51e80f45aae987bb4bfa1a1f8ba7794e73608af0c4888acd3e68e3")
}

// UnpackInvalidHashPlanError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidHashPlan()
func (merkleAccount *MerkleAccount) UnpackInvalidHashPlanError(raw []byte) (*MerkleAccountInvalidHashPlan, error) {
	out := new(MerkleAccountInvalidHashPlan)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidHashPlan", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidInitialization represents a InvalidInitialization error raised by the MerkleAccount contract.
type MerkleAccountInvalidInitialization struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidInitialization()
func MerkleAccountInvalidInitializationErrorID() common.Hash {
	return common.HexToHash("0xf92ee8a957075833165f68c320933b1a1294aafc84ee6e0dd3fb178008f9aaf5")
}

// UnpackInvalidInitializationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidInitialization()
func (merkleAccount *MerkleAccount) UnpackInvalidInitializationError(raw []byte) (*MerkleAccountInvalidInitialization, error) {
	out := new(MerkleAccountInvalidInitialization)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidInitialization", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidMerkleProof represents a InvalidMerkleProof error raised by the MerkleAccount contract.
type MerkleAccountInvalidMerkleProof struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidMerkleProof()
func MerkleAccountInvalidMerkleProofErrorID() common.Hash {
	return common.HexToHash("0xb05e92facfa0fd6ba9338977017107202232768a12b21141a91a36a56212ad1e")
}

// UnpackInvalidMerkleProofError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidMerkleProof()
func (merkleAccount *MerkleAccount) UnpackInvalidMerkleProofError(raw []byte) (*MerkleAccountInvalidMerkleProof, error) {
	out := new(MerkleAccountInvalidMerkleProof)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidMerkleProof", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountInvalidRoot represents a InvalidRoot error raised by the MerkleAccount contract.
type MerkleAccountInvalidRoot struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error InvalidRoot()
func MerkleAccountInvalidRootErrorID() common.Hash {
	return common.HexToHash("0x504570e3dbb7b86bfb4ee628f3b5ae306dd12bad6ed9b4dc6a090b78236f968b")
}

// UnpackInvalidRootError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error InvalidRoot()
func (merkleAccount *MerkleAccount) UnpackInvalidRootError(raw []byte) (*MerkleAccountInvalidRoot, error) {
	out := new(MerkleAccountInvalidRoot)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "InvalidRoot", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountNotFactory represents a NotFactory error raised by the MerkleAccount contract.
type MerkleAccountNotFactory struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error NotFactory()
func MerkleAccountNotFactoryErrorID() common.Hash {
	return common.HexToHash("0x32cc723614e775fc4a8386492bc9a860c12fe98d5f5f28ec17e265818645b229")
}

// UnpackNotFactoryError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error NotFactory()
func (merkleAccount *MerkleAccount) UnpackNotFactoryError(raw []byte) (*MerkleAccountNotFactory, error) {
	out := new(MerkleAccountNotFactory)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "NotFactory", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountNotInitializing represents a NotInitializing error raised by the MerkleAccount contract.
type MerkleAccountNotInitializing struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error NotInitializing()
func MerkleAccountNotInitializingErrorID() common.Hash {
	return common.HexToHash("0xd7e6bcf8597daa127dc9f0048d2f08d5ef140a2cb659feabd700beff1f7a8302")
}

// UnpackNotInitializingError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error NotInitializing()
func (merkleAccount *MerkleAccount) UnpackNotInitializingError(raw []byte) (*MerkleAccountNotInitializing, error) {
	out := new(MerkleAccountNotInitializing)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "NotInitializing", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountOwnableInvalidOwner represents a OwnableInvalidOwner error raised by the MerkleAccount contract.
type MerkleAccountOwnableInvalidOwner struct {
	Owner common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableInvalidOwner(address owner)
func MerkleAccountOwnableInvalidOwnerErrorID() common.Hash {
	return common.HexToHash("0x1e4fbdf7f3ef8bcaa855599e3abf48b232380f183f08f6f813d9ffa5bd585188")
}

// UnpackOwnableInvalidOwnerError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableInvalidOwner(address owner)
func (merkleAccount *MerkleAccount) UnpackOwnableInvalidOwnerError(raw []byte) (*MerkleAccountOwnableInvalidOwner, error) {
	out := new(MerkleAccountOwnableInvalidOwner)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "OwnableInvalidOwner", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountOwnableUnauthorizedAccount represents a OwnableUnauthorizedAccount error raised by the MerkleAccount contract.
type MerkleAccountOwnableUnauthorizedAccount struct {
	Account common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func MerkleAccountOwnableUnauthorizedAccountErrorID() common.Hash {
	return common.HexToHash("0x118cdaa7a341953d1887a2245fd6665d741c67c8c50581daa59e1d03373fa188")
}

// UnpackOwnableUnauthorizedAccountError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error OwnableUnauthorizedAccount(address account)
func (merkleAccount *MerkleAccount) UnpackOwnableUnauthorizedAccountError(raw []byte) (*MerkleAccountOwnableUnauthorizedAccount, error) {
	out := new(MerkleAccountOwnableUnauthorizedAccount)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "OwnableUnauthorizedAccount", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountReentrancyGuardReentrantCall represents a ReentrancyGuardReentrantCall error raised by the MerkleAccount contract.
type MerkleAccountReentrancyGuardReentrantCall struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error ReentrancyGuardReentrantCall()
func MerkleAccountReentrancyGuardReentrantCallErrorID() common.Hash {
	return common.HexToHash("0x3ee5aeb571de7fc460830b4d0017439a1ca56fb0bc39062227ade4fe4a24c1ca")
}

// UnpackReentrancyGuardReentrantCallError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error ReentrancyGuardReentrantCall()
func (merkleAccount *MerkleAccount) UnpackReentrancyGuardReentrantCallError(raw []byte) (*MerkleAccountReentrancyGuardReentrantCall, error) {
	out := new(MerkleAccountReentrancyGuardReentrantCall)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "ReentrancyGuardReentrantCall", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountSafeERC20FailedOperation represents a SafeERC20FailedOperation error raised by the MerkleAccount contract.
type MerkleAccountSafeERC20FailedOperation struct {
	Token common.Address
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error SafeERC20FailedOperation(address token)
func MerkleAccountSafeERC20FailedOperationErrorID() common.Hash {
	return common.HexToHash("0x5274afe73c98b4749fc91ffae6b7b574e7842cb2144a159e9377a5f20b32edf9")
}

// UnpackSafeERC20FailedOperationError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error SafeERC20FailedOperation(address token)
func (merkleAccount *MerkleAccount) UnpackSafeERC20FailedOperationError(raw []byte) (*MerkleAccountSafeERC20FailedOperation, error) {
	out := new(MerkleAccountSafeERC20FailedOperation)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "SafeERC20FailedOperation", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountSelectorAlreadyUsed represents a SelectorAlreadyUsed error raised by the MerkleAccount contract.
type MerkleAccountSelectorAlreadyUsed struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error SelectorAlreadyUsed()
func MerkleAccountSelectorAlreadyUsedErrorID() common.Hash {
	return common.HexToHash("0xe1f3531626a75db37ebf353d3b9af390b108e647b026f08012b718ced55196fb")
}

// UnpackSelectorAlreadyUsedError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error SelectorAlreadyUsed()
func (merkleAccount *MerkleAccount) UnpackSelectorAlreadyUsedError(raw []byte) (*MerkleAccountSelectorAlreadyUsed, error) {
	out := new(MerkleAccountSelectorAlreadyUsed)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "SelectorAlreadyUsed", raw); err != nil {
		return nil, err
	}
	return out, nil
}

// MerkleAccountUnknownSelector represents a UnknownSelector error raised by the MerkleAccount contract.
type MerkleAccountUnknownSelector struct {
}

// ErrorID returns the hash of canonical representation of the error's signature.
//
// Solidity: error UnknownSelector()
func MerkleAccountUnknownSelectorErrorID() common.Hash {
	return common.HexToHash("0xdc66f67306b994c24bd23f0ee4b6101ed96febd809232982e31305bdb7bbea01")
}

// UnpackUnknownSelectorError is the Go binding used to decode the provided
// error data into the corresponding Go error struct.
//
// Solidity: error UnknownSelector()
func (merkleAccount *MerkleAccount) UnpackUnknownSelectorError(raw []byte) (*MerkleAccountUnknownSelector, error) {
	out := new(MerkleAccountUnknownSelector)
	if err := merkleAccount.abi.UnpackIntoInterface(out, "UnknownSelector", raw); err != nil {
		return nil, err
	}
	return out, nil
}
