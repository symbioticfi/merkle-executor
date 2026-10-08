package protocolio

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
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

func manageEvent(t *testing.T, s provider.Snapshot, index uint, calls ...ManagedCall) types.Log {
	t.Helper()
	a := account.NewMerkleAccount()
	actions := make([]account.IMerkleAccountAction, len(calls))
	for i, call := range calls {
		actions[i] = account.IMerkleAccountAction{Target: call.Target, Value: new(big.Int), Data: call.Data}
	}
	event := a.GetABI().Events[account.MerkleAccountManageEventName]
	data, err := event.Inputs.NonIndexed().Pack(actions)
	if err != nil {
		t.Fatal(err)
	}
	return types.Log{Address: s.Account, Topics: []common.Hash{event.ID}, Data: data, TxHash: s.Execution.TxHash, BlockNumber: s.Execution.BlockNumber.Uint64(), BlockHash: s.Execution.BlockHash, Index: index}
}

func TestExecutedCallsRequiresExactOrderedSuccessfulActions(t *testing.T) {
	first := ManagedCall{Target: common.HexToAddress("0x100"), Data: []byte{1, 2, 3, 4}}
	second := ManagedCall{Target: common.HexToAddress("0x200"), Data: []byte{5, 6, 7, 8}}
	for _, mode := range []string{"valid", "split-events", "intervening-call", "wrong-account", "wrong-target", "wrong-data", "out-of-order", "duplicate-expected", "malformed", "missing", "nonzero-value"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := executionSnapshot()
			expected := []ManagedCall{first, second}
			s.Execution.Logs = []types.Log{manageEvent(t, s, 0, first, second)}
			switch mode {
			case "split-events":
				s.Execution.Logs = []types.Log{manageEvent(t, s, 0, first), manageEvent(t, s, 2, second)}
			case "intervening-call":
				s.Execution.Logs = []types.Log{manageEvent(t, s, 0, first, first, second)}
			case "wrong-account":
				s.Execution.Logs[0].Address = common.HexToAddress("0xbad")
			case "wrong-target":
				expected[0].Target = common.HexToAddress("0xbad")
			case "wrong-data":
				expected[0].Data = []byte{9}
			case "out-of-order":
				expected[0], expected[1] = expected[1], expected[0]
			case "duplicate-expected":
				expected = []ManagedCall{first, first, second}
			case "malformed":
				s.Execution.Logs[0].Data = []byte{1}
			case "missing":
				s.Execution.Logs = nil
			case "nonzero-value":
				a := account.NewMerkleAccount()
				event := a.GetABI().Events[account.MerkleAccountManageEventName]
				data, err := event.Inputs.NonIndexed().Pack([]account.IMerkleAccountAction{{Target: first.Target, Value: big.NewInt(1), Data: first.Data}, {Target: second.Target, Value: new(big.Int), Data: second.Data}})
				if err != nil {
					t.Fatal(err)
				}
				s.Execution.Logs[0].Data = data
			}
			err := ExecutedCalls(s, expected)
			valid := mode == "valid" || mode == "split-events" || mode == "intervening-call"
			if (err == nil) != valid {
				t.Fatalf("managed call correlation: %v", err)
			}
		})
	}
}
