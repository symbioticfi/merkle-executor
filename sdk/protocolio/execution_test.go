package protocolio

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type executionReader struct {
	headers map[uint64]*types.Header
}

func (*executionReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, errors.New("unexpected contract read")
}

func (r *executionReader) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if header := r.headers[number.Uint64()]; header != nil {
		return types.CopyHeader(header), nil
	}
	return nil, ethereum.NotFound
}

func executionSnapshot() (provider.Snapshot, *executionReader) {
	receipt := &types.Header{Number: big.NewInt(10), Time: 100}
	pinned := &types.Header{Number: big.NewInt(11), Time: 110, ParentHash: receipt.Hash()}
	return provider.Snapshot{ChainID: big.NewInt(1), Account: common.HexToAddress("0x20"), BlockNumber: big.NewInt(11), BlockHash: pinned.Hash(), Timestamp: 110, Execution: &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), BlockHash: receipt.Hash()}}, &executionReader{headers: map[uint64]*types.Header{10: receipt, 11: pinned}}
}

func TestExecutionRequiresCanonicalReceiptBeforePinnedSnapshot(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "future", "orphaned-receipt", "orphaned-snapshot"} {
		t.Run(mode, func(t *testing.T) {
			s, reader := executionSnapshot()
			switch mode {
			case "missing":
				s.Execution = nil
			case "future":
				s.Execution.BlockNumber = big.NewInt(12)
			case "orphaned-receipt":
				reader.headers[10].Extra = []byte("fork")
			case "orphaned-snapshot":
				reader.headers[11].Extra = []byte("fork")
			}
			err := Execution(context.Background(), reader, s)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("execution validation: %v", err)
			}
		})
	}
}
