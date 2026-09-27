// Package chain separates read RPC from transaction broadcast and nonce reads.
package chain

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-errors/errors"
)

type Client struct {
	*ethclient.Client
	write *ethclient.Client
}

func Dial(ctx context.Context, readURL, writeURL string, expected *big.Int) (*Client, error) {
	if expected == nil || expected.Sign() <= 0 {
		return nil, errors.New("positive expected chain ID required")
	}
	read, err := ethclient.DialContext(ctx, readURL)
	if err != nil {
		return nil, rpcFailure("connect read RPC", err)
	}
	got, err := read.ChainID(ctx)
	if err != nil {
		read.Close()
		return nil, rpcFailure("read RPC chain identity", err)
	}
	if got == nil || got.Cmp(expected) != 0 {
		read.Close()
		return nil, errors.New("read RPC chain identity mismatch")
	}
	write := read
	if writeURL != "" && writeURL != readURL {
		write, err = ethclient.DialContext(ctx, writeURL)
		if err != nil {
			read.Close()
			return nil, rpcFailure("connect write RPC", err)
		}
		got, err = write.ChainID(ctx)
		if err != nil {
			read.Close()
			write.Close()
			return nil, rpcFailure("write RPC chain identity", err)
		}
		if got == nil || got.Cmp(expected) != 0 {
			read.Close()
			write.Close()
			return nil, errors.New("write RPC chain identity mismatch")
		}
	}
	return &Client{Client: read, write: write}, nil
}
func (c *Client) Close() {
	c.Client.Close()
	if c.write != c.Client {
		c.write.Close()
	}
}
func (c *Client) NonceAt(ctx context.Context, a common.Address, b *big.Int) (uint64, error) {
	value, err := c.write.NonceAt(ctx, a, b)
	return value, rpcFailure("read mined nonce from write RPC", err)
}
func (c *Client) PendingNonceAt(ctx context.Context, a common.Address) (uint64, error) {
	value, err := c.write.PendingNonceAt(ctx, a)
	return value, rpcFailure("read pending nonce from write RPC", err)
}
func (c *Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return rpcFailure("submit transaction to write RPC", c.write.SendTransaction(ctx, tx))
}

func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	value, err := c.Client.CallContract(ctx, msg, block)
	return value, rpcFailure("call contract on read RPC", err)
}

func (c *Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	value, err := c.Client.HeaderByNumber(ctx, number)
	return value, rpcFailure("read header by number", err)
}

func (c *Client) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	value, err := c.Client.HeaderByHash(ctx, hash)
	return value, rpcFailure("read header by hash", err)
}

func (c *Client) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	value, err := c.Client.FilterLogs(ctx, query)
	return value, rpcFailure("read event logs", err)
}

func (c *Client) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	value, err := c.Client.EstimateGas(ctx, msg)
	return value, rpcFailure("estimate transaction gas", err)
}

func (c *Client) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	value, err := c.Client.TransactionReceipt(ctx, hash)
	return value, rpcFailure("read transaction receipt", err)
}

// RPC causes can contain authentication URLs, query parameters and response
// bodies. Keep them available for errors.Is/As, but never render them in logs.
type rpcError struct {
	operation string
	cause     error
}

func (e *rpcError) Error() string { return e.operation + " failed" }
func (e *rpcError) Unwrap() error { return e.cause }

func rpcFailure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &rpcError{operation: operation, cause: err}
}
