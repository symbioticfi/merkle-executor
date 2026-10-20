package txengine

import (
	"context"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type operationalBackend struct {
	*mockBackend
	telemetry, fees, confirmations int
	historyErr                     error
}

func (b *operationalBackend) ReadAccountTelemetry(context.Context, common.Address) (*big.Int, uint64, uint64, error) {
	b.telemetry++
	return big.NewInt(7), 21, 22, nil
}
func (b *operationalBackend) ReadFeeContext(ctx context.Context, count uint64, percentiles []float64) (*types.Header, *ethereum.FeeHistory, error, error) {
	b.fees++
	h, he := b.HeaderByNumber(ctx, nil)
	f, fe := b.FeeHistory(ctx, count, nil, percentiles)
	if b.historyErr != nil {
		fe = b.historyErr
	}
	return h, f, he, fe
}
func (b *operationalBackend) BatchRead(ctx context.Context, req []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	b.confirmations++
	out := make([]protocolio.ReadResult, len(req))
	for i, q := range req {
		switch q.Kind {
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = b.HeaderByNumber(ctx, q.Block)
		case protocolio.ReadReceipt:
			out[i].Receipt, out[i].Err = b.TransactionReceipt(ctx, q.Hash)
		}
	}
	return out, nil
}
func TestEnginePrefersIndependentOperationalBatches(t *testing.T) {
	b := &operationalBackend{mockBackend: newMockBackend()}
	m := New(b, mustSigner(t), big.NewInt(1), Config{}, logr.Discard())
	reading, err := m.readAccountTelemetry(t.Context())
	if err != nil || b.telemetry != 1 || reading.balance.Int64() != 7 || reading.latestNonce != 21 || reading.pendingNonce != 22 {
		t.Fatalf("telemetry capability unused: %+v %v calls%d", reading, err, b.telemetry)
	}
	_, _, err = m.readFeeSnapshot(t.Context())
	if err != nil || b.fees != 1 {
		t.Fatalf("fee capability unused: %v calls%d", err, b.fees)
	}
	b.historyErr = errors.New("history unavailable")
	_, _, err = m.readFeeSnapshot(t.Context())
	if !errors.Is(err, errFreshFeesUnavailable) || b.fees != 2 {
		t.Fatalf("fee error %v", err)
	}
}

func TestConfirmationOpeningBatchKeepsFinalWitnessFence(t *testing.T) {
	tx := types.NewTx(&types.DynamicFeeTx{Nonce: 7, To: ptr(common.Address{1})})
	base := newWitnessBackend(tx, 2)
	base.onNumber = func(_ *big.Int, calls int, header *types.Header) (*types.Header, error) {
		if calls == 2 {
			header.Extra = []byte("reorg at closing witness fence")
		}
		return header, nil
	}
	b := &witnessBatchBackend{base}
	m := New(b, mustSigner(t), big.NewInt(1), Config{Confirmations: 2, PollInterval: time.Millisecond}, logr.Discard())
	_, err := m.waitForConfirmations(t.Context(), tx.Hash(), b.receipts[tx.Hash()], 2)
	if !errors.Is(err, errReceiptReorged) || b.batchReads != 1 || b.numberedReads != 2 || b.receiptReads != 2 || b.parentReads != 2 {
		t.Fatalf("batch or final witness fence: err %v groups %d numbered %d receipts %d ancestry %d", err, b.batchReads, b.numberedReads, b.receiptReads, b.parentReads)
	}
}
