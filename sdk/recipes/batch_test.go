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
	simulations                int
	reorgFence                 bool
}

func (r *batchingRecipeReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.singleCalls++
	return r.fakeReader.CallContract(ctx, msg, block)
}
func (r *batchingRecipeReader) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	r.singleHeaders++
	return r.fakeReader.HeaderByNumber(ctx, block)
}
func (r *batchingRecipeReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.groups = append(r.groups, requests)
	results := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		switch request.Kind {
		case protocolio.ReadCall:
			if isExecutionCall(request.Call.Data) {
				r.simulations++
				if request.MulticallSafe || request.Call.From != r.caller {
					r.t.Fatal("full original simulation lost sender-sensitive semantics")
				}
			} else if !request.MulticallSafe || request.Call.From != (common.Address{}) {
				r.t.Fatal("verified metadata getter lost aggregate eligibility")
			}
			if *request.Call.To != r.account && *request.Call.To != r.executor {
				results[i].Err = errors.New("unknown speculative account")
				continue
			}
			results[i].Data, results[i].Err = r.fakeReader.CallContract(ctx, request.Call, request.Block)
		case protocolio.ReadHeader:
			results[i].Header, results[i].Err = r.fakeReader.HeaderByNumber(ctx, request.Block)
			if r.reorgFence && r.simulations != 0 && results[i].Header != nil {
				header := types.CopyHeader(results[i].Header)
				header.Extra = []byte("closing reorg")
				results[i].Header = header
			}
		case protocolio.ReadLogs:
			results[i].Logs, results[i].Err = r.FilterLogs(ctx, request.Filter)
		default:
			r.t.Fatal("unexpected discovery request")
		}
	}
	return results, nil
}

func TestDiscoveryBatchesExpectedAccountMetadataAndEventEvidence(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, admissionRecipe(t))}
	prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 3 || len(reader.groups[0]) != 5 || len(reader.groups[1]) != 2 || len(reader.groups[2]) != 1 || reader.singleCalls != 0 || reader.singleHeaders != 1 {
		t.Fatalf("discovery groups=%v calls=%d headers=%d", reader.groups, reader.singleCalls, reader.singleHeaders)
	}
	reader.groups = nil
	reader.singleHeaders = 0
	call := assembledCall(t, prepared[0])
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 2 || len(reader.groups[0]) != 7 || len(reader.groups[1]) != 1 || reader.singleHeaders != 1 || reader.simulations != 1 {
		t.Fatal("admission did not batch metadata, opening pin and exact simulation before final fence")
	}
	reader.reorgFence, reader.simulations = true, 0
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err == nil {
		t.Fatal("batched reads hid reorg at final fence")
	}
}

func TestDiscoveryStillRejectsAccountMismatchBeforeRoot(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, admissionRecipe(t))}
	if _, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{common.HexToAddress("0x09")}, reader.caller, PreparationOptions{}); err == nil {
		t.Fatal("unexpected executor account accepted")
	}
}

func TestAdmissionDeduplicatesRolesAndPreservesFinalFence(t *testing.T) {
	reader := &batchingRecipeReader{fakeReader: fakeChain(t, admissionRecipe(t))}
	prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	call := assembledCall(t, prepared[0])
	reader.groups, reader.singleHeaders = nil, 0
	a, b := common.HexToHash("0x01"), common.HexToHash("0x02")
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{a, b, a}}, []map[uint]string{nil}, reader.caller); err != nil {
		t.Fatal(err)
	}
	if len(reader.groups) != 2 || len(reader.groups[0]) != 9 || len(reader.groups[1]) != 1 || reader.roleCalls != 2 || reader.singleCalls != 0 || reader.singleHeaders != 1 {
		t.Fatal("roles not deduplicated and batched with final closing fence")
	}
	reader.allowed = false
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{a}}, []map[uint]string{nil}, reader.caller); err == nil {
		t.Fatal("missing role accepted")
	}
	checked := reader.roleCalls
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err != nil || reader.roleCalls != checked {
		t.Fatal("empty role set emitted a role read")
	}
	reader.reorgFence, reader.simulations, reader.allowed = true, 0, true
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{a}}, []map[uint]string{nil}, reader.caller); err == nil {
		t.Fatal("role admission skipped final canonical fence")
	}
}
