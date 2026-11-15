// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package merkldistributor

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

// MerklDistributorMetaData contains all meta data concerning the MerklDistributor contract.
var MerklDistributorMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"claim\",\"inputs\":[{\"name\":\"users\",\"type\":\"address[]\"},{\"name\":\"tokens\",\"type\":\"address[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\"},{\"name\":\"proofs\",\"type\":\"bytes32[][]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getMerkleRoot\",\"inputs\":[],\"outputs\":[{\"name\":\"root\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimed\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\"},{\"name\":\"token\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint208\"},{\"name\":\"timestamp\",\"type\":\"uint48\"},{\"name\":\"merkleRoot\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"claimRecipient\",\"inputs\":[{\"name\":\"user\",\"type\":\"address\"},{\"name\":\"token\",\"type\":\"address\"}],\"outputs\":[{\"name\":\"recipient\",\"type\":\"address\"}],\"stateMutability\":\"view\"}]",
	ID:  "MerklDistributor",
}

// MerklDistributor is an auto generated Go binding around an Ethereum contract.
type MerklDistributor struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *MerklDistributor) GetABI() abi.ABI {
	return c.abi
}

// NewMerklDistributor creates a new instance of MerklDistributor.
func NewMerklDistributor() *MerklDistributor {
	parsed, err := MerklDistributorMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &MerklDistributor{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *MerklDistributor) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackClaim is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x71ee95c0.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claim(address[] users, address[] tokens, uint256[] amounts, bytes32[][] proofs) returns()
func (merklDistributor *MerklDistributor) PackClaim(users []common.Address, tokens []common.Address, amounts []*big.Int, proofs [][][32]byte) []byte {
	enc, err := merklDistributor.abi.Pack("claim", users, tokens, amounts, proofs)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaim is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x71ee95c0.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claim(address[] users, address[] tokens, uint256[] amounts, bytes32[][] proofs) returns()
func (merklDistributor *MerklDistributor) TryPackClaim(users []common.Address, tokens []common.Address, amounts []*big.Int, proofs [][][32]byte) ([]byte, error) {
	return merklDistributor.abi.Pack("claim", users, tokens, amounts, proofs)
}

// PackClaimRecipient is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x4d272d95.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimRecipient(address user, address token) view returns(address recipient)
func (merklDistributor *MerklDistributor) PackClaimRecipient(user common.Address, token common.Address) []byte {
	enc, err := merklDistributor.abi.Pack("claimRecipient", user, token)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimRecipient is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x4d272d95.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimRecipient(address user, address token) view returns(address recipient)
func (merklDistributor *MerklDistributor) TryPackClaimRecipient(user common.Address, token common.Address) ([]byte, error) {
	return merklDistributor.abi.Pack("claimRecipient", user, token)
}

// UnpackClaimRecipient is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x4d272d95.
//
// Solidity: function claimRecipient(address user, address token) view returns(address recipient)
func (merklDistributor *MerklDistributor) UnpackClaimRecipient(data []byte) (common.Address, error) {
	out, err := merklDistributor.abi.Unpack("claimRecipient", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackClaimed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0c9cbf0e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function claimed(address user, address token) view returns(uint208 amount, uint48 timestamp, bytes32 merkleRoot)
func (merklDistributor *MerklDistributor) PackClaimed(user common.Address, token common.Address) []byte {
	enc, err := merklDistributor.abi.Pack("claimed", user, token)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackClaimed is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x0c9cbf0e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function claimed(address user, address token) view returns(uint208 amount, uint48 timestamp, bytes32 merkleRoot)
func (merklDistributor *MerklDistributor) TryPackClaimed(user common.Address, token common.Address) ([]byte, error) {
	return merklDistributor.abi.Pack("claimed", user, token)
}

// ClaimedOutput serves as a container for the return parameters of contract
// method Claimed.
type ClaimedOutput struct {
	Amount     *big.Int
	Timestamp  *big.Int
	MerkleRoot [32]byte
}

// UnpackClaimed is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x0c9cbf0e.
//
// Solidity: function claimed(address user, address token) view returns(uint208 amount, uint48 timestamp, bytes32 merkleRoot)
func (merklDistributor *MerklDistributor) UnpackClaimed(data []byte) (ClaimedOutput, error) {
	out, err := merklDistributor.abi.Unpack("claimed", data)
	outstruct := new(ClaimedOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Amount = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.Timestamp = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.MerkleRoot = *abi.ConvertType(out[2], new([32]byte)).(*[32]byte)
	return *outstruct, nil
}

// PackGetMerkleRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x49590657.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getMerkleRoot() view returns(bytes32 root)
func (merklDistributor *MerklDistributor) PackGetMerkleRoot() []byte {
	enc, err := merklDistributor.abi.Pack("getMerkleRoot")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetMerkleRoot is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x49590657.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getMerkleRoot() view returns(bytes32 root)
func (merklDistributor *MerklDistributor) TryPackGetMerkleRoot() ([]byte, error) {
	return merklDistributor.abi.Pack("getMerkleRoot")
}

// UnpackGetMerkleRoot is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x49590657.
//
// Solidity: function getMerkleRoot() view returns(bytes32 root)
func (merklDistributor *MerklDistributor) UnpackGetMerkleRoot(data []byte) ([32]byte, error) {
	out, err := merklDistributor.abi.Unpack("getMerkleRoot", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}
