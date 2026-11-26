// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package iris

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

// Struct0 is an auto generated low-level Go binding around an user-defined struct.
type Struct0 struct {
	Borrower        common.Address
	Solver          common.Address
	CollateralToken common.Address
	DebtToken       common.Address
	VenueBitmap     *big.Int
	Maturity        uint32
	OverduePeriod   uint32
	FixedRate       uint16
	OverdueRate     uint16
	BondLltv        uint16
	Fee             uint16
}

// Struct1 is an auto generated low-level Go binding around an user-defined struct.
type Struct1 struct {
	Collateral      *big.Int
	Debt            *big.Int
	Bond            *big.Int
	BondRequirement *big.Int
	CollateralIndex *big.Int
	DebtIndex       *big.Int
	FixedLeg        *big.Int
	FloatingLeg     *big.Int
	Surplus         *big.Int
	LastUpdate      uint32
	VenueId         uint8
	Data            []byte
}

// IrisMetaData contains all meta data concerning the Iris contract.
var IrisMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"getLoan\",\"stateMutability\":\"view\",\"inputs\":[{\"type\":\"address\",\"name\":\"pod\"}],\"outputs\":[{\"type\":\"tuple\",\"name\":\"\",\"components\":[{\"type\":\"address\",\"name\":\"borrower\"},{\"type\":\"address\",\"name\":\"solver\"},{\"type\":\"address\",\"name\":\"collateralToken\"},{\"type\":\"address\",\"name\":\"debtToken\"},{\"type\":\"uint128\",\"name\":\"venueBitmap\"},{\"type\":\"uint32\",\"name\":\"maturity\"},{\"type\":\"uint32\",\"name\":\"overduePeriod\"},{\"type\":\"uint16\",\"name\":\"fixedRate\"},{\"type\":\"uint16\",\"name\":\"overdueRate\"},{\"type\":\"uint16\",\"name\":\"bondLltv\"},{\"type\":\"uint16\",\"name\":\"fee\"}]}]},{\"type\":\"function\",\"name\":\"getPosition\",\"stateMutability\":\"view\",\"inputs\":[{\"type\":\"address\",\"name\":\"pod\"}],\"outputs\":[{\"type\":\"tuple\",\"name\":\"\",\"components\":[{\"type\":\"uint128\",\"name\":\"collateral\"},{\"type\":\"uint128\",\"name\":\"debt\"},{\"type\":\"uint128\",\"name\":\"bond\"},{\"type\":\"uint128\",\"name\":\"bondRequirement\"},{\"type\":\"uint128\",\"name\":\"collateralIndex\"},{\"type\":\"uint128\",\"name\":\"debtIndex\"},{\"type\":\"uint128\",\"name\":\"fixedLeg\"},{\"type\":\"uint128\",\"name\":\"floatingLeg\"},{\"type\":\"uint128\",\"name\":\"surplus\"},{\"type\":\"uint32\",\"name\":\"lastUpdate\"},{\"type\":\"uint8\",\"name\":\"venueId\"},{\"type\":\"bytes\",\"name\":\"data\"}]}]}]",
	ID:  "Iris",
}

// Iris is an auto generated Go binding around an Ethereum contract.
type Iris struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *Iris) GetABI() abi.ABI {
	return c.abi
}

// NewIris creates a new instance of Iris.
func NewIris() *Iris {
	parsed, err := IrisMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &Iris{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *Iris) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackGetLoan is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x33481fc9.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getLoan(address pod) view returns((address,address,address,address,uint128,uint32,uint32,uint16,uint16,uint16,uint16))
func (iris *Iris) PackGetLoan(pod common.Address) []byte {
	enc, err := iris.abi.Pack("getLoan", pod)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetLoan is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x33481fc9.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getLoan(address pod) view returns((address,address,address,address,uint128,uint32,uint32,uint16,uint16,uint16,uint16))
func (iris *Iris) TryPackGetLoan(pod common.Address) ([]byte, error) {
	return iris.abi.Pack("getLoan", pod)
}

// UnpackGetLoan is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x33481fc9.
//
// Solidity: function getLoan(address pod) view returns((address,address,address,address,uint128,uint32,uint32,uint16,uint16,uint16,uint16))
func (iris *Iris) UnpackGetLoan(data []byte) (Struct0, error) {
	out, err := iris.abi.Unpack("getLoan", data)
	if err != nil {
		return *new(Struct0), err
	}
	out0 := *abi.ConvertType(out[0], new(Struct0)).(*Struct0)
	return out0, nil
}

// PackGetPosition is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x16c19739.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getPosition(address pod) view returns((uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint32,uint8,bytes))
func (iris *Iris) PackGetPosition(pod common.Address) []byte {
	enc, err := iris.abi.Pack("getPosition", pod)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetPosition is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x16c19739.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getPosition(address pod) view returns((uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint32,uint8,bytes))
func (iris *Iris) TryPackGetPosition(pod common.Address) ([]byte, error) {
	return iris.abi.Pack("getPosition", pod)
}

// UnpackGetPosition is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x16c19739.
//
// Solidity: function getPosition(address pod) view returns((uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint128,uint32,uint8,bytes))
func (iris *Iris) UnpackGetPosition(data []byte) (Struct1, error) {
	out, err := iris.abi.Unpack("getPosition", data)
	if err != nil {
		return *new(Struct1), err
	}
	out0 := *abi.ConvertType(out[0], new(Struct1)).(*Struct1)
	return out0, nil
}
