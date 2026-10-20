// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package cowsettlement

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

// CowSettlementMetaData contains all meta data concerning the CowSettlement contract.
var CowSettlementMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"domainSeparator\",\"inputs\":[],\"outputs\":[{\"name\":\"domain\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"filledAmount\",\"inputs\":[{\"name\":\"orderUid\",\"type\":\"bytes\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "CowSettlement",
}

// CowSettlement is an auto generated Go binding around an Ethereum contract.
type CowSettlement struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *CowSettlement) GetABI() abi.ABI {
	return c.abi
}

// NewCowSettlement creates a new instance of CowSettlement.
func NewCowSettlement() *CowSettlement {
	parsed, err := CowSettlementMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &CowSettlement{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *CowSettlement) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackDomainSeparator is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf698da25.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function domainSeparator() view returns(bytes32 domain)
func (cowSettlement *CowSettlement) PackDomainSeparator() []byte {
	enc, err := cowSettlement.abi.Pack("domainSeparator")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackDomainSeparator is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xf698da25.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function domainSeparator() view returns(bytes32 domain)
func (cowSettlement *CowSettlement) TryPackDomainSeparator() ([]byte, error) {
	return cowSettlement.abi.Pack("domainSeparator")
}

// UnpackDomainSeparator is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xf698da25.
//
// Solidity: function domainSeparator() view returns(bytes32 domain)
func (cowSettlement *CowSettlement) UnpackDomainSeparator(data []byte) ([32]byte, error) {
	out, err := cowSettlement.abi.Unpack("domainSeparator", data)
	if err != nil {
		return *new([32]byte), err
	}
	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	return out0, nil
}

// PackFilledAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2479fb6e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function filledAmount(bytes orderUid) view returns(uint256 amount)
func (cowSettlement *CowSettlement) PackFilledAmount(orderUid []byte) []byte {
	enc, err := cowSettlement.abi.Pack("filledAmount", orderUid)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackFilledAmount is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2479fb6e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function filledAmount(bytes orderUid) view returns(uint256 amount)
func (cowSettlement *CowSettlement) TryPackFilledAmount(orderUid []byte) ([]byte, error) {
	return cowSettlement.abi.Pack("filledAmount", orderUid)
}

// UnpackFilledAmount is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2479fb6e.
//
// Solidity: function filledAmount(bytes orderUid) view returns(uint256 amount)
func (cowSettlement *CowSettlement) UnpackFilledAmount(data []byte) (*big.Int, error) {
	out, err := cowSettlement.abi.Unpack("filledAmount", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}
