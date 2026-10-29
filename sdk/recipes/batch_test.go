package recipes

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type batchingRecipeReader struct {
	*fakeReader
	groups                     [][]protocolio.ReadRequest
	singleCalls, singleHeaders int
	reorgFence                 bool
}

func (r *batchingRecipeReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.singleCalls++
	return r.fakeReader.CallContract(ctx, msg, block)
}
func (r *batchingRecipeReader) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	r.singleHeaders++
	h, e := r.fakeReader.HeaderByNumber(ctx, block)
	if r.reorgFence && h != nil {
		copy := *h
		copy.Extra = []byte{1}
		return &copy, e
	}
	return h, e
}
func (r *batchingRecipeReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.groups = append(r.groups, requests)
	results := make([]protocolio.ReadResult, len(requests))
	for i, req := range requests {
		switch req.Kind {
		case protocolio.ReadCall:
			if !req.MulticallSafe || req.Call.From != (common.Address{}) {
				r.t.Fatal("verified metadata getter lost aggregate eligibility")
			}
			if *req.Call.To != r.account && *req.Call.To != r.executor {
				results[i].Err = errors.New("unknown speculative account")
				continue
			}
			results[i].Data, results[i].Err = r.fakeReader.CallContract(ctx, req.Call, req.Block)
		case protocolio.ReadHeader:
			results[i].Header, results[i].Err = r.fakeReader.HeaderByNumber(ctx, req.Block)
		case protocolio.ReadLogs:
			results[i].Logs, results[i].Err = r.FilterLogs(ctx, req.Filter)
		default:
			r.t.Fatal("unexpected discovery request")
		}
	}
	return results, nil
}
func TestDiscoveryBatchesExpectedAccountMetadataAndEventEvidence(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, Recipe{})}
	snapshot, _, err := DiscoverForAccount(context.Background(), reader, reader.executor, reader.account, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 2 || len(reader.groups[0]) != 5 || len(reader.groups[1]) != 2 || reader.singleCalls != 0 || reader.singleHeaders != 2 {
		t.Fatalf("discovery groups=%v calls=%d headers=%d", reader.groups, reader.singleCalls, reader.singleHeaders)
	}
	reader.groups = nil
	reader.singleHeaders = 0
	if err := ValidateSnapshot(context.Background(), reader, snapshot, reader.caller); err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 2 || len(reader.groups[0]) != 2 || len(reader.groups[1]) != 5 || reader.singleHeaders != 1 {
		t.Fatal("validation did not batch opening evidence or lost final fence")
	}
	reader.reorgFence = true
	if err := ValidateSnapshot(context.Background(), reader, snapshot, reader.caller); err == nil {
		t.Fatal("batched reads hid reorg at final fence")
	}
}
func TestDiscoveryStillRejectsAccountMismatchBeforeRoot(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, Recipe{})}
	if _, _, err := DiscoverForAccount(context.Background(), reader, reader.executor, common.HexToAddress("0x09"), nil); err == nil {
		t.Fatal("unexpected executor account accepted")
	}
}
func TestRoleBatchDeduplicatesLeavesAndPreservesFinalFence(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, Recipe{})}
	snap := Snapshot{BlockNumber: 10, BlockHash: reader.headers[10].Hash(), Account: reader.account, Executor: reader.executor}
	a, b := common.HexToHash("0x01"), common.HexToHash("0x02")
	if err := VerifyRoles(context.Background(), reader, snap, []common.Hash{a, b, a}); err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 1 || len(reader.groups[0]) != 2 || reader.roleCalls != 2 || reader.singleCalls != 0 || reader.singleHeaders != 1 {
		t.Fatal("roles not deduplicated and batched")
	}
	reader.allowed = false
	if err := VerifyRoles(context.Background(), reader, snap, []common.Hash{a}); err == nil {
		t.Fatal("missing role accepted")
	}
	reader.groups = nil
	if err := VerifyRoles(context.Background(), reader, snap, nil); err != nil || len(reader.groups) != 0 {
		t.Fatal("empty role set emitted a contract batch")
	}
}
