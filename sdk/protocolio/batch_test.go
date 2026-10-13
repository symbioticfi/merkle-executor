package protocolio

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

type singleBatchFixture struct {
	calls int
	fail  error
}

func (f *singleBatchFixture) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	f.calls++
	if block.Int64() != 12 || msg.From != common.HexToAddress("0x01") {
		panic("call context changed")
	}
	if msg.Data[0] == 2 {
		return nil, f.fail
	}
	return []byte{msg.Data[0]}, ctx.Err()
}

type capableBatchFixture struct {
	singleBatchFixture
	groups  []int
	failure error
	missing bool
}

func (f *capableBatchFixture) BatchRead(_ context.Context, requests []ReadRequest) ([]ReadResult, error) {
	f.groups = append(f.groups, len(requests))
	if f.failure != nil {
		return nil, f.failure
	}
	results := make([]ReadResult, len(requests))
	for i, r := range requests {
		results[i].Data = r.Call.Data
		if r.Call.Data[0] == 2 {
			results[i].Err = f.fail
		}
	}
	if f.missing {
		return results[:len(results)-1], nil
	}
	return results, nil
}
func batchRequests(n int) []ReadRequest {
	out := make([]ReadRequest, n)
	for i := range out {
		out[i] = ReadRequest{Kind: ReadCall, Call: ethereum.CallMsg{From: common.HexToAddress("0x01"), Data: []byte{byte(i%250 + 1)}}, Block: big.NewInt(12)}
	}
	return out
}
func TestReadGroupDefersLegacySinglesUntilConsumed(t *testing.T) {
	sentinel := errors.New("member unavailable")
	reader := &singleBatchFixture{fail: sentinel}
	group, err := ReadGroup(context.Background(), reader, batchRequests(2)...)
	if err != nil || reader.calls != 0 {
		t.Fatalf("legacy group eagerly read: %v calls=%d", err, reader.calls)
	}
	if got := group.At(0); got.Err != nil || got.Data[0] != 1 || reader.calls != 1 {
		t.Fatalf("first read: %+v", got)
	}
	if got := group.At(1); !errors.Is(got.Err, sentinel) || reader.calls != 2 {
		t.Fatalf("member error lost: %+v", got)
	}
	_ = group.At(0)
	if reader.calls != 2 {
		t.Fatal("same group member was read twice")
	}
}
func TestReadGroupBoundsRequestsAndNeverRetriesFailedBatch(t *testing.T) {
	sentinel := errors.New("batch unavailable")
	for _, mode := range []string{"members", "transport", "missing"} {
		t.Run(mode, func(t *testing.T) {
			reader := &capableBatchFixture{singleBatchFixture: singleBatchFixture{fail: sentinel}}
			reader.missing = mode == "missing"
			if mode == "transport" {
				reader.failure = sentinel
			}
			group, err := ReadGroup(context.Background(), reader, batchRequests(258)...)
			if mode != "members" {
				if err == nil {
					t.Fatal("invalid batch accepted")
				}
				if reader.calls != 0 {
					t.Fatal("failed batch retried as singles")
				}
				return
			}
			if err != nil || len(reader.groups) != 2 || reader.groups[0] != 256 || reader.groups[1] != 2 {
				t.Fatalf("bounded batch lost: %v %+v", err, reader.groups)
			}
			if group.At(0).Err != nil || !errors.Is(group.At(1).Err, sentinel) || group.At(257).Data[0] != 8 || reader.calls != 0 {
				t.Fatal("ordered per-member results changed")
			}
		})
	}
}
