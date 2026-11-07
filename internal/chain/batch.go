package chain

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// batchRPC delegates the bounded wire format to the neutral SDK transport.
func (c *Client) batchRPC(ctx context.Context, batch []rpc.BatchElem) error {
	return chainio.CallHTTPBatch(ctx, c.http, c.readURL, batch)
}

func (c *Client) BatchRead(ctx context.Context, requests []chainio.ReadRequest) ([]chainio.ReadResult, error) {
	reader := chainio.RPCBatchReader{Call: c.batchRPC, Multicall: c.multicall, Codec: readAggregateCodec{}}
	return reader.BatchRead(ctx, requests)
}

type readAggregateCodec struct{}

func (readAggregateCodec) Encode(calls []chainio.AggregateCall) ([]byte, error) {
	subcalls := make([]multicall.Multicall3Call3, len(calls))
	for i, call := range calls {
		subcalls[i] = multicall.Multicall3Call3{Target: call.Target, AllowFailure: true, CallData: call.Data}
	}
	return multicall.NewMulticall3().TryPackAggregate3(subcalls)
}
func (readAggregateCodec) Decode(raw []byte) ([]chainio.AggregateResult, error) {
	decoded, err := multicall.NewMulticall3().UnpackAggregate3(raw)
	if err != nil {
		return nil, err
	}
	results := make([]chainio.AggregateResult, len(decoded))
	for i, result := range decoded {
		results[i] = chainio.AggregateResult{Success: result.Success, Data: result.ReturnData}
	}
	return results, nil
}
