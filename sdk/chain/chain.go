// Package chain separates read RPC from transaction broadcast and nonce reads.
package chain

import (
	"context"
	"math/big"
	"net/http"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type Client struct {
	*ethclient.Client
	write     *ethclient.Client
	http      *http.Client
	readURL   string
	multicall common.Address
}

func Dial(ctx context.Context, readURL, writeURL string, expected *big.Int, aggregate ...common.Address) (*Client, error) {
	if len(aggregate) > 1 || len(aggregate) == 1 && aggregate[0] == (common.Address{}) {
		return nil, errors.New("one nonzero Multicall3 address required when configured")
	}
	var multicall common.Address
	if len(aggregate) == 1 {
		multicall = aggregate[0]
	}
	if expected == nil || expected.Sign() <= 0 {
		return nil, errors.New("positive expected chain ID required")
	}

	httpClient := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	connected := false
	defer func() {
		if !connected {
			httpClient.CloseIdleConnections()
		}
	}()
	read, err := dialHTTP(ctx, readURL, httpClient, expected, "read")
	if err != nil {
		return nil, err
	}
	write := read
	if writeURL != "" && writeURL != readURL {
		if write, err = dialHTTP(ctx, writeURL, httpClient, expected, "write"); err != nil {
			read.Close()
			return nil, err
		}
	}
	connected = true
	return &Client{Client: read, write: write, http: httpClient, readURL: readURL, multicall: multicall}, nil
}

// dialHTTP connects one endpoint and requires the expected chain identity.
func dialHTTP(ctx context.Context, endpoint string, client *http.Client, expected *big.Int, role string) (*ethclient.Client, error) {
	connection, err := rpc.DialOptions(ctx, endpoint, rpc.WithHTTPClient(client))
	if err != nil {
		return nil, rpcFailure("connect "+role+" RPC", err)
	}
	c := ethclient.NewClient(connection)
	got, err := c.ChainID(ctx)
	if err != nil {
		c.Close()
		return nil, rpcFailure(role+" RPC eth_chainId", err)
	}
	if got == nil || got.Cmp(expected) != 0 {
		c.Close()
		return nil, errors.New(role + " RPC chain identity mismatch")
	}
	return c, nil
}
func (c *Client) Close() {
	if c.http != nil {
		defer c.http.CloseIdleConnections()
	}
	c.Client.Close()
	if c.write != c.Client {
		c.write.Close()
	}
}
func (c *Client) NonceAt(ctx context.Context, a common.Address, b *big.Int) (uint64, error) {
	value, err := c.write.NonceAt(ctx, a, b)
	return value, rpcFailure("read mined nonce from write RPC eth_getTransactionCount", err)
}
func (c *Client) PendingNonceAt(ctx context.Context, a common.Address) (uint64, error) {
	value, err := c.write.PendingNonceAt(ctx, a)
	return value, rpcFailure("read pending nonce from write RPC eth_getTransactionCount", err)
}
func (c *Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return rpcFailure("submit transaction to write RPC eth_sendRawTransaction", c.write.SendTransaction(ctx, tx))
}

func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	value, err := c.Client.CallContract(ctx, msg, block)
	return value, rpcFailure("call contract on read RPC", protocolio.CallFailure("eth_call", msg, block, err))
}

func (c *Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	value, err := c.Client.HeaderByNumber(ctx, number)
	return value, rpcFailure("read RPC eth_getBlockByNumber", err)
}

func (c *Client) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	value, err := c.Client.HeaderByHash(ctx, hash)
	return value, rpcFailure("read RPC eth_getBlockByHash", err)
}

func (c *Client) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	value, err := c.Client.FilterLogs(ctx, query)
	return value, rpcFailure("read RPC eth_getLogs", err)
}

func (c *Client) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	value, err := c.Client.EstimateGas(ctx, msg)
	return value, rpcFailure("estimate transaction gas", protocolio.CallFailure("eth_estimateGas", msg, nil, err))
}

func (c *Client) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	value, err := c.Client.TransactionReceipt(ctx, hash)
	return value, rpcFailure("read RPC eth_getTransactionReceipt", err)
}

func rpcFailure(operation string, err error) error {
	return protocolio.Failure(operation, err)
}

// FeeHistory uses the read endpoint; sender nonce admission remains on write.
func (c *Client) FeeHistory(ctx context.Context, count uint64, last *big.Int, percentiles []float64) (*ethereum.FeeHistory, error) {
	history, err := c.Client.FeeHistory(ctx, count, last, percentiles)
	return history, rpcFailure("read RPC eth_feeHistory", err)
}

// BalanceAt reads account funds with the same redaction boundary as other I/O.
func (c *Client) BalanceAt(ctx context.Context, account common.Address, block *big.Int) (*big.Int, error) {
	balance, err := c.Client.BalanceAt(ctx, account, block)
	return balance, rpcFailure("read RPC eth_getBalance", err)
}

// TransactionSenderBalanceAt reads funds from the same endpoint as nonce
// admission and submission. A failed sending-endpoint read never falls back.
func (c *Client) TransactionSenderBalanceAt(ctx context.Context, account common.Address, block *big.Int) (*big.Int, error) {
	balance, err := c.write.BalanceAt(ctx, account, block)
	return balance, rpcFailure("write RPC eth_getBalance", err)
}
