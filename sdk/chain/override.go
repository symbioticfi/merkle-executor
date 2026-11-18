package chain

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// CallContractWithCode keeps caller and tx.origin intact for numerical recipe
// preparation. It deliberately has no Multicall or sender-changing fallback.
func (c *Client) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	if msg.To == nil {
		return nil, errors.New("code override requires a call target")
	}
	return c.CallContractWithCodeOverrides(ctx, msg, block, map[common.Address][]byte{*msg.To: code})
}

// CallContractWithCodeOverrides applies code only for the duration of a pinned eth_call.
func (c *Client) CallContractWithCodeOverrides(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code map[common.Address][]byte) ([]byte, error) {
	if msg.To == nil || *msg.To == (common.Address{}) || msg.From == (common.Address{}) || block == nil || block.Sign() < 0 || len(code) == 0 {
		return nil, errors.New("code override requires target, caller, numeric block and code")
	}
	overrides := make(map[common.Address]any, len(code))
	for address, runtime := range code {
		if address == (common.Address{}) || len(runtime) == 0 {
			return nil, errors.New("code override requires nonzero address and nonempty runtime")
		}
		overrides[address] = map[string]any{"code": hexutil.Bytes(runtime)}
	}
	var result hexutil.Bytes
	err := c.Client.Client().CallContext(ctx, &result, "eth_call", protocolio.CallArgument(msg), hexutil.EncodeBig(block), overrides)
	return result, rpcFailure("call recipe with code override", protocolio.CallFailure("eth_call", msg, block, err))
}
