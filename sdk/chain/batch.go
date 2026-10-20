package chain

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// batchRPC shares the SDK's bounded parser with hosts supplying their own transport.
func (c *Client) batchRPC(ctx context.Context, batch []rpc.BatchElem) error {
	return protocolio.CallHTTPBatch(ctx, c.http, c.readURL, batch)
}

func (c *Client) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	reader := protocolio.RPCBatchReader{Call: c.batchRPC, Multicall: c.multicall, Codec: readAggregateCodec{}}
	return reader.BatchRead(ctx, requests)
}

type readAggregateCodec struct{}

func (readAggregateCodec) Encode(calls []protocolio.AggregateCall) ([]byte, error) {
	subcalls := make([]multicall.Multicall3Call3, len(calls))
	for i, call := range calls {
		subcalls[i] = multicall.Multicall3Call3{Target: call.Target, AllowFailure: true, CallData: call.Data}
	}
	return multicall.NewMulticall3().TryPackAggregate3(subcalls)
}
func (readAggregateCodec) Decode(raw []byte) ([]protocolio.AggregateResult, error) {
	decoded, err := multicall.NewMulticall3().UnpackAggregate3(raw)
	if err != nil {
		return nil, err
	}
	results := make([]protocolio.AggregateResult, len(decoded))
	for i, result := range decoded {
		results[i] = protocolio.AggregateResult{Success: result.Success, Data: result.ReturnData}
	}
	return results, nil
}
