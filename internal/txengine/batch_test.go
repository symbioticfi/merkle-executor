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

func TestConfirmationOpeningBatchKeepsFinalHeadFence(t *testing.T) {
	b := &operationalBackend{mockBackend: newMockBackend()}
	b.head = 102
	b.latestHeads = []uint64{102, 100, 102, 102}
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Nonce: 7, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: ptr(common.HexToAddress("0xabc"))})
	receipt := successfulReceipt(tx, 100)
	b.receipts[tx.Hash()] = receipt
	m := New(b, mustSigner(t), big.NewInt(1), Config{Confirmations: 2, PollInterval: time.Millisecond}, logr.Discard())
	got, err := m.waitForConfirmations(t.Context(), tx.Hash(), receipt, 2)
	if err != nil || got != receipt || b.confirmations != 2 || len(b.latestHeads) != 0 || b.headerHashReads != 4 {
		t.Fatalf("batch or final fence: %v groups%d heads%v ancestry%d", err, b.confirmations, b.latestHeads, b.headerHashReads)
	}
}
