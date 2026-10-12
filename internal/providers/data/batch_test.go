package data

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	sdkprovider "github.com/symbioticfi/merkle-executor/sdk/provider"
)

type rpcBatchReader struct {
	t                *testing.T
	batches, singles int
}

func (r *rpcBatchReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	r.singles++
	return common.LeftPadBytes([]byte{42}, 32), nil
}
func (r *rpcBatchReader) BatchRead(_ context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.batches++
	if len(requests) != 2 {
		r.t.Fatal("RPC provider did not batch all independent reads")
	}
	out := make([]protocolio.ReadResult, 2)
	for i, request := range requests {
		if request.MulticallSafe || request.Call.From != common.HexToAddress("0x01") || request.Block.Int64() != 77 || *request.Call.To != common.HexToAddress("0x01") {
			r.t.Fatal("arbitrary RPC call changed caller, target, block or execution isolation")
		}
		out[i].Data = common.LeftPadBytes([]byte{byte(42 + i)}, 32)
	}
	return out, nil
}
func TestRPCBatchesConfiguredReadsWithoutChangingCallerOrOutput(t *testing.T) {
	r := &rpcBatchReader{t: t}
	p, err := rpcFactory(node(t, "reads:\n  - {target: '{{account}}', data: '0x12345678', offset: 0, input: 0}\n  - {target: '{{account}}', data: '0x98765432', offset: 0, input: 1}\nactions:\n  '9:0':\n    data: '0x123456780000000000000000000000000000000000000000000000000000000000000000'\n    words: [{offset: 4, input: 1}]"), provider.Deps{Chain: r})
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Prepare(context.Background(), sdkprovider.Snapshot{Account: common.HexToAddress("0x01"), BlockNumber: big.NewInt(77)})
	if err != nil || r.batches != 1 || r.singles != 0 || out.Inputs[0] != "42" || out.Inputs[1] != "43" || out.Actions["9:0"][35] != 43 {
		t.Fatalf("batch/input/action mismatch: %+v %v batches%d singles%d", out, err, r.batches, r.singles)
	}
}
