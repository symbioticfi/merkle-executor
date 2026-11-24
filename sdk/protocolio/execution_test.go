package protocolio

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestExecutionRequiresReceiptAtOrBeforePinnedSnapshot(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "future"} {
		t.Run(mode, func(t *testing.T) {
			s := provider.Snapshot{ChainID: big.NewInt(1), Account: common.HexToAddress("0x20"), BlockNumber: big.NewInt(11), BlockHash: common.HexToHash("0x11"), Timestamp: 110, Execution: &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), BlockHash: common.HexToHash("0x10")}}
			switch mode {
			case "missing":
				s.Execution = nil
			case "future":
				s.Execution.BlockNumber = big.NewInt(12)
			}
			if err := Execution(s); (err == nil) != (mode == "valid") {
				t.Fatalf("execution validation: %v", err)
			}
		})
	}
}
