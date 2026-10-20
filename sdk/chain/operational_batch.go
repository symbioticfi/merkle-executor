package chain

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// ReadAccountTelemetry returns a complete read-endpoint observation or no values.
func (c *Client) ReadAccountTelemetry(ctx context.Context, account common.Address) (*big.Int, uint64, uint64, error) {
	var balance hexutil.Big
	var mined, pending hexutil.Uint64
	calls := []rpc.BatchElem{
		{Method: "eth_getBalance", Args: []any{account, "latest"}, Result: &balance},
		{Method: "eth_getTransactionCount", Args: []any{account, "latest"}, Result: &mined},
		{Method: "eth_getTransactionCount", Args: []any{account, "pending"}, Result: &pending},
	}
	if err := c.batchRPC(ctx, calls); err != nil {
		return nil, 0, 0, err
	}
	for _, call := range calls {
		if call.Error != nil {
			return nil, 0, 0, call.Error
		}
	}
	return (*big.Int)(&balance), uint64(mined), uint64(pending), nil
}
func (c *Client) ReadNonces(ctx context.Context, account common.Address) (uint64, uint64, error) {
	var mined, pending hexutil.Uint64
	calls := []rpc.BatchElem{
		{Method: "eth_getTransactionCount", Args: []any{account, "latest"}, Result: &mined},
		{Method: "eth_getTransactionCount", Args: []any{account, "pending"}, Result: &pending},
	}
	if err := c.batchRPC(ctx, calls); err != nil {
		return 0, 0, err
	}
	for _, call := range calls {
		if call.Error != nil {
			return 0, 0, call.Error
		}
	}
	return uint64(mined), uint64(pending), nil
}

// ReadFeeContext retains independent errors so the engine can validate the head
// before consuming speculative history. It does not claim an atomic snapshot.
func (c *Client) ReadFeeContext(ctx context.Context, count uint64, percentiles []float64) (*types.Header, *ethereum.FeeHistory, error, error) {
	var header *types.Header
	var raw struct {
		OldestBlock  *hexutil.Big     `json:"oldestBlock"`
		Reward       [][]*hexutil.Big `json:"reward"`
		BaseFee      []*hexutil.Big   `json:"baseFeePerGas"`
		GasUsedRatio []float64        `json:"gasUsedRatio"`
	}
	calls := []rpc.BatchElem{
		{Method: "eth_getBlockByNumber", Args: []any{"latest", false}, Result: &header},
		{Method: "eth_feeHistory", Args: []any{hexutil.Uint64(count), "latest", percentiles}, Result: &raw},
	}
	if err := c.batchRPC(ctx, calls); err != nil {
		return nil, nil, err, err
	}
	history := &ethereum.FeeHistory{OldestBlock: (*big.Int)(raw.OldestBlock), GasUsedRatio: raw.GasUsedRatio}
	for _, fee := range raw.BaseFee {
		history.BaseFee = append(history.BaseFee, (*big.Int)(fee))
	}
	for _, row := range raw.Reward {
		rewards := make([]*big.Int, len(row))
		for i, reward := range row {
			rewards[i] = (*big.Int)(reward)
		}
		history.Reward = append(history.Reward, rewards)
	}
	return header, history, calls[0].Error, calls[1].Error
}
