// Code generated via abigen V2 - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package openedenexpress

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

// OpenEdenExpressMetaData contains all meta data concerning the OpenEdenExpress contract.
var OpenEdenExpressMetaData = bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"getRedeemQueueInfo\",\"inputs\":[{\"name\":\"index\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"shareAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"redeemAssetAmt\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"feeAssetAmt\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"requestTimestamp\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"id\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRedeemQueueLength\",\"inputs\":[],\"outputs\":[{\"name\":\"length\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"maxStalePeriod\",\"inputs\":[],\"outputs\":[{\"name\":\"period\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pendingRedeemInfo\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"previewRedeem\",\"inputs\":[{\"name\":\"tokenAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"feeAmt\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"redeemAssetAmt\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"netRedeemAssetAmt\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"priceOracle\",\"inputs\":[],\"outputs\":[{\"name\":\"oracle\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemAsset\",\"inputs\":[],\"outputs\":[{\"name\":\"asset\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemInfo\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemMinimum\",\"inputs\":[],\"outputs\":[{\"name\":\"minimum\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"requestRedeem\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"kycManager\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"pausedRedeem\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"redeemFeeRate\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"token\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"}]",
	ID:  "OpenEdenExpress",
}

// OpenEdenExpress is an auto generated Go binding around an Ethereum contract.
type OpenEdenExpress struct {
	abi abi.ABI
}

// GetABI returns the ABI associated with this contract binding.
func (c *OpenEdenExpress) GetABI() abi.ABI {
	return c.abi
}

// NewOpenEdenExpress creates a new instance of OpenEdenExpress.
func NewOpenEdenExpress() *OpenEdenExpress {
	parsed, err := OpenEdenExpressMetaData.ParseABI()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	return &OpenEdenExpress{abi: *parsed}
}

// Instance creates a wrapper for a deployed contract instance at the given address.
// Use this to create the instance object passed to abigen v2 library functions Call, Transact, etc.
func (c *OpenEdenExpress) Instance(backend bind.ContractBackend, addr common.Address) *bind.BoundContract {
	return bind.NewBoundContract(addr, c.abi, backend, backend, backend)
}

// PackGetRedeemQueueInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe3a1f58a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRedeemQueueInfo(uint256 index) view returns(address sender, address receiver, uint256 tokenAmount, uint256 shareAmount, uint256 redeemAssetAmt, uint256 feeAssetAmt, uint256 requestTimestamp, bytes32 id)
func (openEdenExpress *OpenEdenExpress) PackGetRedeemQueueInfo(index *big.Int) []byte {
	enc, err := openEdenExpress.abi.Pack("getRedeemQueueInfo", index)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRedeemQueueInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xe3a1f58a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRedeemQueueInfo(uint256 index) view returns(address sender, address receiver, uint256 tokenAmount, uint256 shareAmount, uint256 redeemAssetAmt, uint256 feeAssetAmt, uint256 requestTimestamp, bytes32 id)
func (openEdenExpress *OpenEdenExpress) TryPackGetRedeemQueueInfo(index *big.Int) ([]byte, error) {
	return openEdenExpress.abi.Pack("getRedeemQueueInfo", index)
}

// GetRedeemQueueInfoOutput serves as a container for the return parameters of contract
// method GetRedeemQueueInfo.
type GetRedeemQueueInfoOutput struct {
	Sender           common.Address
	Receiver         common.Address
	TokenAmount      *big.Int
	ShareAmount      *big.Int
	RedeemAssetAmt   *big.Int
	FeeAssetAmt      *big.Int
	RequestTimestamp *big.Int
	Id               [32]byte
}

// UnpackGetRedeemQueueInfo is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xe3a1f58a.
//
// Solidity: function getRedeemQueueInfo(uint256 index) view returns(address sender, address receiver, uint256 tokenAmount, uint256 shareAmount, uint256 redeemAssetAmt, uint256 feeAssetAmt, uint256 requestTimestamp, bytes32 id)
func (openEdenExpress *OpenEdenExpress) UnpackGetRedeemQueueInfo(data []byte) (GetRedeemQueueInfoOutput, error) {
	out, err := openEdenExpress.abi.Unpack("getRedeemQueueInfo", data)
	outstruct := new(GetRedeemQueueInfoOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.Sender = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Receiver = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.TokenAmount = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	outstruct.ShareAmount = abi.ConvertType(out[3], new(big.Int)).(*big.Int)
	outstruct.RedeemAssetAmt = abi.ConvertType(out[4], new(big.Int)).(*big.Int)
	outstruct.FeeAssetAmt = abi.ConvertType(out[5], new(big.Int)).(*big.Int)
	outstruct.RequestTimestamp = abi.ConvertType(out[6], new(big.Int)).(*big.Int)
	outstruct.Id = *abi.ConvertType(out[7], new([32]byte)).(*[32]byte)
	return *outstruct, nil
}

// PackGetRedeemQueueLength is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1b45a827.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function getRedeemQueueLength() view returns(uint256 length)
func (openEdenExpress *OpenEdenExpress) PackGetRedeemQueueLength() []byte {
	enc, err := openEdenExpress.abi.Pack("getRedeemQueueLength")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackGetRedeemQueueLength is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x1b45a827.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function getRedeemQueueLength() view returns(uint256 length)
func (openEdenExpress *OpenEdenExpress) TryPackGetRedeemQueueLength() ([]byte, error) {
	return openEdenExpress.abi.Pack("getRedeemQueueLength")
}

// UnpackGetRedeemQueueLength is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x1b45a827.
//
// Solidity: function getRedeemQueueLength() view returns(uint256 length)
func (openEdenExpress *OpenEdenExpress) UnpackGetRedeemQueueLength(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("getRedeemQueueLength", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackKycManager is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7afea44f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function kycManager() view returns(address)
func (openEdenExpress *OpenEdenExpress) PackKycManager() []byte {
	enc, err := openEdenExpress.abi.Pack("kycManager")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackKycManager is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7afea44f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function kycManager() view returns(address)
func (openEdenExpress *OpenEdenExpress) TryPackKycManager() ([]byte, error) {
	return openEdenExpress.abi.Pack("kycManager")
}

// UnpackKycManager is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x7afea44f.
//
// Solidity: function kycManager() view returns(address)
func (openEdenExpress *OpenEdenExpress) UnpackKycManager(data []byte) (common.Address, error) {
	out, err := openEdenExpress.abi.Unpack("kycManager", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackMaxStalePeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7361e5ff.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function maxStalePeriod() view returns(uint256 period)
func (openEdenExpress *OpenEdenExpress) PackMaxStalePeriod() []byte {
	enc, err := openEdenExpress.abi.Pack("maxStalePeriod")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackMaxStalePeriod is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x7361e5ff.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function maxStalePeriod() view returns(uint256 period)
func (openEdenExpress *OpenEdenExpress) TryPackMaxStalePeriod() ([]byte, error) {
	return openEdenExpress.abi.Pack("maxStalePeriod")
}

// UnpackMaxStalePeriod is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x7361e5ff.
//
// Solidity: function maxStalePeriod() view returns(uint256 period)
func (openEdenExpress *OpenEdenExpress) UnpackMaxStalePeriod(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("maxStalePeriod", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackPausedRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e207b3f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function pausedRedeem() view returns(bool)
func (openEdenExpress *OpenEdenExpress) PackPausedRedeem() []byte {
	enc, err := openEdenExpress.abi.Pack("pausedRedeem")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPausedRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x6e207b3f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function pausedRedeem() view returns(bool)
func (openEdenExpress *OpenEdenExpress) TryPackPausedRedeem() ([]byte, error) {
	return openEdenExpress.abi.Pack("pausedRedeem")
}

// UnpackPausedRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x6e207b3f.
//
// Solidity: function pausedRedeem() view returns(bool)
func (openEdenExpress *OpenEdenExpress) UnpackPausedRedeem(data []byte) (bool, error) {
	out, err := openEdenExpress.abi.Unpack("pausedRedeem", data)
	if err != nil {
		return *new(bool), err
	}
	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)
	return out0, nil
}

// PackPendingRedeemInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x106d3e5c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function pendingRedeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) PackPendingRedeemInfo(account common.Address) []byte {
	enc, err := openEdenExpress.abi.Pack("pendingRedeemInfo", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPendingRedeemInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x106d3e5c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function pendingRedeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) TryPackPendingRedeemInfo(account common.Address) ([]byte, error) {
	return openEdenExpress.abi.Pack("pendingRedeemInfo", account)
}

// UnpackPendingRedeemInfo is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x106d3e5c.
//
// Solidity: function pendingRedeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) UnpackPendingRedeemInfo(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("pendingRedeemInfo", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackPreviewRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x4cdad506.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function previewRedeem(uint256 tokenAmount) view returns(uint256 feeAmt, uint256 redeemAssetAmt, uint256 netRedeemAssetAmt)
func (openEdenExpress *OpenEdenExpress) PackPreviewRedeem(tokenAmount *big.Int) []byte {
	enc, err := openEdenExpress.abi.Pack("previewRedeem", tokenAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPreviewRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x4cdad506.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function previewRedeem(uint256 tokenAmount) view returns(uint256 feeAmt, uint256 redeemAssetAmt, uint256 netRedeemAssetAmt)
func (openEdenExpress *OpenEdenExpress) TryPackPreviewRedeem(tokenAmount *big.Int) ([]byte, error) {
	return openEdenExpress.abi.Pack("previewRedeem", tokenAmount)
}

// PreviewRedeemOutput serves as a container for the return parameters of contract
// method PreviewRedeem.
type PreviewRedeemOutput struct {
	FeeAmt            *big.Int
	RedeemAssetAmt    *big.Int
	NetRedeemAssetAmt *big.Int
}

// UnpackPreviewRedeem is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x4cdad506.
//
// Solidity: function previewRedeem(uint256 tokenAmount) view returns(uint256 feeAmt, uint256 redeemAssetAmt, uint256 netRedeemAssetAmt)
func (openEdenExpress *OpenEdenExpress) UnpackPreviewRedeem(data []byte) (PreviewRedeemOutput, error) {
	out, err := openEdenExpress.abi.Unpack("previewRedeem", data)
	outstruct := new(PreviewRedeemOutput)
	if err != nil {
		return *outstruct, err
	}
	outstruct.FeeAmt = abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	outstruct.RedeemAssetAmt = abi.ConvertType(out[1], new(big.Int)).(*big.Int)
	outstruct.NetRedeemAssetAmt = abi.ConvertType(out[2], new(big.Int)).(*big.Int)
	return *outstruct, nil
}

// PackPriceOracle is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2630c12f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function priceOracle() view returns(address oracle)
func (openEdenExpress *OpenEdenExpress) PackPriceOracle() []byte {
	enc, err := openEdenExpress.abi.Pack("priceOracle")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackPriceOracle is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x2630c12f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function priceOracle() view returns(address oracle)
func (openEdenExpress *OpenEdenExpress) TryPackPriceOracle() ([]byte, error) {
	return openEdenExpress.abi.Pack("priceOracle")
}

// UnpackPriceOracle is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x2630c12f.
//
// Solidity: function priceOracle() view returns(address oracle)
func (openEdenExpress *OpenEdenExpress) UnpackPriceOracle(data []byte) (common.Address, error) {
	out, err := openEdenExpress.abi.Unpack("priceOracle", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRedeemAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x537f90ac.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemAsset() view returns(address asset)
func (openEdenExpress *OpenEdenExpress) PackRedeemAsset() []byte {
	enc, err := openEdenExpress.abi.Pack("redeemAsset")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemAsset is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x537f90ac.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemAsset() view returns(address asset)
func (openEdenExpress *OpenEdenExpress) TryPackRedeemAsset() ([]byte, error) {
	return openEdenExpress.abi.Pack("redeemAsset")
}

// UnpackRedeemAsset is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x537f90ac.
//
// Solidity: function redeemAsset() view returns(address asset)
func (openEdenExpress *OpenEdenExpress) UnpackRedeemAsset(data []byte) (common.Address, error) {
	out, err := openEdenExpress.abi.Unpack("redeemAsset", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}

// PackRedeemFeeRate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5872e6fa.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemFeeRate() view returns(uint256)
func (openEdenExpress *OpenEdenExpress) PackRedeemFeeRate() []byte {
	enc, err := openEdenExpress.abi.Pack("redeemFeeRate")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemFeeRate is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x5872e6fa.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemFeeRate() view returns(uint256)
func (openEdenExpress *OpenEdenExpress) TryPackRedeemFeeRate() ([]byte, error) {
	return openEdenExpress.abi.Pack("redeemFeeRate")
}

// UnpackRedeemFeeRate is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x5872e6fa.
//
// Solidity: function redeemFeeRate() view returns(uint256)
func (openEdenExpress *OpenEdenExpress) UnpackRedeemFeeRate(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("redeemFeeRate", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x697367da.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) PackRedeemInfo(account common.Address) []byte {
	enc, err := openEdenExpress.abi.Pack("redeemInfo", account)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemInfo is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x697367da.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) TryPackRedeemInfo(account common.Address) ([]byte, error) {
	return openEdenExpress.abi.Pack("redeemInfo", account)
}

// UnpackRedeemInfo is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0x697367da.
//
// Solidity: function redeemInfo(address account) view returns(uint256 amount)
func (openEdenExpress *OpenEdenExpress) UnpackRedeemInfo(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("redeemInfo", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRedeemMinimum is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaf1dbe7c.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function redeemMinimum() view returns(uint256 minimum)
func (openEdenExpress *OpenEdenExpress) PackRedeemMinimum() []byte {
	enc, err := openEdenExpress.abi.Pack("redeemMinimum")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRedeemMinimum is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xaf1dbe7c.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function redeemMinimum() view returns(uint256 minimum)
func (openEdenExpress *OpenEdenExpress) TryPackRedeemMinimum() ([]byte, error) {
	return openEdenExpress.abi.Pack("redeemMinimum")
}

// UnpackRedeemMinimum is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xaf1dbe7c.
//
// Solidity: function redeemMinimum() view returns(uint256 minimum)
func (openEdenExpress *OpenEdenExpress) UnpackRedeemMinimum(data []byte) (*big.Int, error) {
	out, err := openEdenExpress.abi.Unpack("redeemMinimum", data)
	if err != nil {
		return new(big.Int), err
	}
	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	return out0, nil
}

// PackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x65b89f9f.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function requestRedeem(address to, uint256 tokenAmount) returns()
func (openEdenExpress *OpenEdenExpress) PackRequestRedeem(to common.Address, tokenAmount *big.Int) []byte {
	enc, err := openEdenExpress.abi.Pack("requestRedeem", to, tokenAmount)
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackRequestRedeem is the Go binding used to pack the parameters required for calling
// the contract method with ID 0x65b89f9f.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function requestRedeem(address to, uint256 tokenAmount) returns()
func (openEdenExpress *OpenEdenExpress) TryPackRequestRedeem(to common.Address, tokenAmount *big.Int) ([]byte, error) {
	return openEdenExpress.abi.Pack("requestRedeem", to, tokenAmount)
}

// PackToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc0c546a.  This method will panic if any
// invalid/nil inputs are passed.
//
// Solidity: function token() view returns(address)
func (openEdenExpress *OpenEdenExpress) PackToken() []byte {
	enc, err := openEdenExpress.abi.Pack("token")
	if err != nil {
		panic(err)
	}
	return enc
}

// TryPackToken is the Go binding used to pack the parameters required for calling
// the contract method with ID 0xfc0c546a.  This method will return an error
// if any inputs are invalid/nil.
//
// Solidity: function token() view returns(address)
func (openEdenExpress *OpenEdenExpress) TryPackToken() ([]byte, error) {
	return openEdenExpress.abi.Pack("token")
}

// UnpackToken is the Go binding that unpacks the parameters returned
// from invoking the contract method with ID 0xfc0c546a.
//
// Solidity: function token() view returns(address)
func (openEdenExpress *OpenEdenExpress) UnpackToken(data []byte) (common.Address, error) {
	out, err := openEdenExpress.abi.Unpack("token", data)
	if err != nil {
		return *new(common.Address), err
	}
	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return out0, nil
}
