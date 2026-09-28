// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package securitizetopology

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

// SecuritizeTopologyMetaData contains all meta data concerning the SecuritizeTopology contract.
var SecuritizeTopologyMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"liquidityToken\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"availableLiquidity\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\"}]",
	ID:  "SecuritizeTopology",
}

// SecuritizeTopology is an auto generated Go binding around an Ethereum contract.
type SecuritizeTopology struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *SecuritizeTopology) GetABI() abi.ABI {
	return c.abi
}

// NewSecuritizeTopology creates a new instance of SecuritizeTopology.
func NewSecuritizeTopology() *SecuritizeTopology {
	parsed, err := SecuritizeTopologyMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &SecuritizeTopology{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *SecuritizeTopology) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackAvailableLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x74375359.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeTopology *SecuritizeTopology) PackAvailableLiquidity() []byte {
	enc, err := securitizeTopology.abi.Pack("availableLiquidity")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackAvailableLiquidity is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x74375359.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeTopology *SecuritizeTopology) TryPackAvailableLiquidity() ([]byte, error) {
	return securitizeTopology.abi.Pack("availableLiquidity")
}

// UnpackAvailableLiquidity is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x74375359.
//
// Solidity: function availableLiquidity() view returns(uint256)
func (securitizeTopology *SecuritizeTopology) UnpackAvailableLiquidity(data []byte) (*big.Int, error) {
	out, err := securitizeTopology.abi.Unpack("availableLiquidity", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackLiquidityToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x43cd8f7e.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function liquidityToken() view returns(address)
func (securitizeTopology *SecuritizeTopology) PackLiquidityToken() []byte {
	enc, err := securitizeTopology.abi.Pack("liquidityToken")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackLiquidityToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x43cd8f7e.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function liquidityToken() view returns(address)
func (securitizeTopology *SecuritizeTopology) TryPackLiquidityToken() ([]byte, error) {
	return securitizeTopology.abi.Pack("liquidityToken")
}

// UnpackLiquidityToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x43cd8f7e.
//
// Solidity: function liquidityToken() view returns(address)
func (securitizeTopology *SecuritizeTopology) UnpackLiquidityToken(data []byte) (common.Address, error) {
	out, err := securitizeTopology.abi.Unpack("liquidityToken", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}
