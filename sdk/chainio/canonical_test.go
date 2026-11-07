package chainio

import (
	"context"
	ethereum "github.com/ethereum/go-ethereum"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type witnessReader struct {
	headers    map[uint64]*types.Header
	head       uint64
	receipt    *types.Receipt
	closeReorg bool
	reads      map[uint64]int
	hashReads  int
}

func (r *witnessReader) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if n == nil {
		return r.headers[r.head], nil
	}
	height := n.Uint64()
	r.reads[height]++
	h := r.headers[height]
	if r.closeReorg && r.reads[height] > 1 {
		c := types.CopyHeader(h)
		c.Extra = []byte{1}
		return c, nil
	}
	return h, nil
}
func (r *witnessReader) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	r.hashReads++
	for _, h := range r.headers {
		if h.Hash() == hash {
			return h, nil
		}
	}
	return nil, nil
}
func (r *witnessReader) TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error) {
	return r.receipt, nil
}
func witnessFixture() *witnessReader {
	r := &witnessReader{headers: map[uint64]*types.Header{}, head: 100, reads: map[uint64]int{}}
	for i := uint64(10); i <= 100; i++ {
		h := &types.Header{Number: new(big.Int).SetUint64(i), Time: i}
		if i > 10 {
			h.ParentHash = r.headers[i-1].Hash()
		}
		r.headers[i] = h
	}
	r.receipt = &types.Receipt{TxHash: common.HexToHash("0x12"), BlockNumber: big.NewInt(10), BlockHash: r.headers[10].Hash(), Status: 1}
	return r
}
func TestVerifyReceiptBoundedWitnessAndFreshClose(t *testing.T) {
	r := witnessFixture()
	got, e := VerifyReceipt(t.Context(), r, r.receipt, 2)
	if e != nil || got == nil {
		t.Fatalf("verify: %v", e)
	}
	if r.hashReads != 2 {
		t.Fatalf("walked %d parents", r.hashReads)
	}
	r = witnessFixture()
	r.closeReorg = true
	if got, e := VerifyReceipt(t.Context(), r, r.receipt, 2); e != nil || got != nil {
		t.Fatalf("accepted reorg: %v", e)
	}
}
func TestVerifyReceiptRejectsOrphanAndRemovedLog(t *testing.T) {
	for _, mode := range []string{"short", "orphan", "removed"} {
		t.Run(mode, func(t *testing.T) {
			r := witnessFixture()
			switch mode {
			case "short":
				r.head = 11
			case "orphan":
				r.receipt.BlockHash = common.HexToHash("0x99")
			case "removed":
				r.receipt.Logs = []*types.Log{{Removed: true}}
			}
			got, e := VerifyReceipt(t.Context(), r, r.receipt, 2)
			if got != nil {
				t.Fatal("accepted invalid receipt")
			}
			if mode == "removed" && e == nil {
				t.Fatal("accepted malformed receipt logs")
			}
		})
	}
}

func TestConfirmedSnapshotUsesBoundedSharedAncestry(t *testing.T) {
	r := witnessFixture()
	r.head = 12
	s, e := ConfirmedSnapshot(t.Context(), r, Snapshot{}, 2)
	if e != nil || s.BlockNumber.Cmp(big.NewInt(10)) != 0 || s.BlockHash != r.headers[10].Hash() {
		t.Fatalf("snapshot: %+v %v", s, e)
	}
	if e := VerifyConfirmedSnapshot(t.Context(), r, s, 2); e != nil {
		t.Fatal(e)
	}
	if r.hashReads != 4 {
		t.Fatalf("parent reads: %d", r.hashReads)
	}
}

func (r *witnessReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, nil
}
