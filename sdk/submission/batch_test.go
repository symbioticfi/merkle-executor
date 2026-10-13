package submission

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type batchBackend struct {
	*testTransport
	groups     [][]protocolio.ReadRequest
	below      bool
	firstError bool
}

func (b *batchBackend) BatchRead(ctx context.Context, req []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	b.groups = append(b.groups, req)
	out := make([]protocolio.ReadResult, len(req))
	for i, q := range req {
		switch q.Kind {
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = b.HeaderByNumber(ctx, q.Block)
			if b.below && i == 1 {
				out[i].Header = nil
				out[i].Err = errors.New("unused witness error")
			}
		case protocolio.ReadReceipt:
			out[i].Receipt, out[i].Err = b.TransactionReceipt(ctx, q.Hash)
			if b.firstError && i == 0 {
				out[i].Err = errors.New("earlier attempt error")
			}
		}
	}
	return out, nil
}
func TestConfirmationBatchIgnoresUnusedWitnessBelowDepth(t *testing.T) {
	transport, _ := fixture(t)
	backend := &batchBackend{testTransport: transport, below: true}
	receipt := &types.Receipt{BlockNumber: big.NewInt(10)}
	got, err := ConfirmReceipt(t.Context(), backend, receipt, 2)
	if err != nil || got != nil || len(backend.groups) != 1 || len(backend.groups[0]) != 2 || backend.groups[0][1].Block.Int64() != 12 {
		t.Fatalf("opening batch/precedence: %v %v %v", got, err, backend.groups)
	}
}
func TestWaitBatchesAttemptsAndLaterReceiptWins(t *testing.T) {
	transport, store := fixture(t)
	m := manager(t, transport, store)
	req := Request{ID: "batch-owned", To: common.HexToAddress("0x1234"), Data: []byte{1}}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	pending, _ := m.Send(ctx, req)
	cancel()
	if len(pending.Attempts) != 2 {
		t.Fatal("fixture missing signed variants")
	}
	transport.receipt = &types.Receipt{TxHash: pending.Attempts[1], BlockNumber: big.NewInt(10), BlockHash: transport.header.Hash(), Status: types.ReceiptStatusSuccessful}
	backend := &batchBackend{testTransport: transport, firstError: true}
	recovered, err := New(backend, crypto.PubkeyToAddress(transport.key.PublicKey), big.NewInt(1), store, Config{PollInterval: time.Millisecond}, transport, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	final, err := recovered.wait(t.Context())
	if err != nil || !final.Confirmed || len(backend.groups) != 2 || len(backend.groups[0]) != 2 || backend.groups[0][0].Hash != pending.Attempts[0] || backend.groups[0][1].Hash != pending.Attempts[1] {
		t.Fatalf("sweep lost later result: %+v %v groups=%v", final, err, backend.groups)
	}
	if transport.sends != 2 || transport.owner != req.ID {
		t.Fatal("ownership or broadcast changed")
	}
}
