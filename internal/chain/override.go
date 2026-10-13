package chain

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
)

// CallContractWithCode keeps caller and tx.origin intact for numerical recipe
// preparation. It deliberately has no Multicall or sender-changing fallback.
func (c *Client) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	if msg.To == nil || *msg.To == (common.Address{}) || msg.From == (common.Address{}) || block == nil || block.Sign() < 0 || len(code) == 0 {
		return nil, errors.New("code override requires target, caller, numeric block and code")
	}
	args := map[string]any{"from": msg.From, "to": msg.To, "input": hexutil.Bytes(msg.Data)}
	if msg.Gas != 0 {
		args["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.Value != nil {
		args["value"] = (*hexutil.Big)(msg.Value)
	}
	overrides := map[common.Address]any{*msg.To: map[string]any{"code": hexutil.Bytes(code)}}
	var result hexutil.Bytes
	err := c.Client.Client().CallContext(ctx, &result, "eth_call", args, hexutil.EncodeBig(block), overrides)
	return result, rpcFailure("call recipe with code override", err)
}
