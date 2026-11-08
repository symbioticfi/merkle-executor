package keeper

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

func TestConfirmedSnapshotRejectsFutureAndOrphanedExecution(t *testing.T) {
	for _, mode := range []string{"future", "orphaned", "reorg-after-advance"} {
		t.Run(mode, func(t *testing.T) {
			c := newConfirmationChain()
			p := &Planner{Chain: c, Confirmations: 2}
			execution := &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), BlockHash: c.canonical[10].Hash()}
			original := provider.Snapshot{ChainID: big.NewInt(1), Account: common.HexToAddress("0x20"), Execution: execution}
			switch mode {
			case "future":
				execution.BlockNumber = big.NewInt(11)
				execution.BlockHash = c.canonical[11].Hash()
			case "orphaned":
				execution.BlockHash = common.HexToHash("0xbad")
			}
			snapshot, err := p.ConfirmedSnapshot(context.Background(), original)
			if mode != "reorg-after-advance" {
				if err == nil {
					t.Fatal("invalid receipt allowed into Advance snapshot")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			c.canonical[10] = types.CopyHeader(c.canonical[10])
			c.canonical[10].Extra = []byte("reorganized receipt")
			if err := p.VerifyConfirmedSnapshot(context.Background(), snapshot); err == nil {
				t.Fatal("orphaned receipt could advance RAM settlement")
			}
		})
	}
}

type confirmationChain struct {
	canonical map[uint64]*types.Header
	byHash    map[common.Hash]*types.Header
	head      *types.Header
	onNumber  func(*big.Int)
}

func newConfirmationChain() *confirmationChain {
	c := &confirmationChain{canonical: map[uint64]*types.Header{}, byHash: map[common.Hash]*types.Header{}}
	for i := uint64(10); i <= 12; i++ {
		h := &types.Header{Number: new(big.Int).SetUint64(i), Time: i * 10}
		if c.head != nil {
			h.ParentHash = c.head.Hash()
		}
		c.canonical[i] = h
		c.byHash[h.Hash()] = h
		c.head = h
	}
	return c
}
func (c *confirmationChain) HeaderByNumber(_ context.Context, n *big.Int) (*types.Header, error) {
	if c.onNumber != nil {
		c.onNumber(n)
	}
	h := c.head
	if n != nil {
		h = c.canonical[n.Uint64()]
	}
	if h == nil {
		return nil, nil
	}
	return types.CopyHeader(h), nil
}
func (c *confirmationChain) HeaderByHash(_ context.Context, hash common.Hash) (*types.Header, error) {
	h := c.byHash[hash]
	if h == nil {
		return nil, ethereum.NotFound
	}
	return types.CopyHeader(h), nil
}
func (c *confirmationChain) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, errors.New("unexpected contract call")
}
func (c *confirmationChain) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	return nil, errors.New("unexpected log query")
}

func TestConfirmedSnapshotRequiresExactDescendantDepth(t *testing.T) {
	c := newConfirmationChain()
	p := &Planner{Chain: c, Confirmations: 2}
	s, err := p.ConfirmedSnapshot(context.Background(), provider.Snapshot{Account: common.HexToAddress("0x20")})
	if err != nil || s.BlockNumber.Cmp(big.NewInt(10)) != 0 || s.BlockHash != c.canonical[10].Hash() || s.Timestamp != 100 {
		t.Fatalf("confirmation snapshot: %+v %v", s, err)
	}
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	c.canonical[12] = types.CopyHeader(c.canonical[12])
	c.canonical[12].Extra = []byte("fork")
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err != nil {
		t.Fatal("same confirmed ancestor should survive tip-only reorg", err)
	}
	c.canonical[12].ParentHash = common.HexToHash("0xbad")
	if err := p.VerifyConfirmedSnapshot(context.Background(), s); err == nil {
		t.Fatal("unrelated confirmation witness accepted")
	}
}

func TestConfirmedSnapshotRejectsHeadReorgAndMalformedHeaders(t *testing.T) {
	for _, mode := range []string{"nil", "wrong-number", "fork-during-read", "wrong-parent"} {
		t.Run(mode, func(t *testing.T) {
			c := newConfirmationChain()
			switch mode {
			case "nil":
				c.head = nil
			case "wrong-number":
				c.canonical[12] = &types.Header{Number: big.NewInt(11)}
			case "fork-during-read":
				c.onNumber = func(n *big.Int) {
					if n != nil {
						h := types.CopyHeader(c.canonical[12])
						h.Extra = []byte("new canonical witness")
						c.canonical[12] = h
					}
				}
			case "wrong-parent":
				c.head.ParentHash = common.HexToHash("0xbad")
			}
			p := &Planner{Chain: c, Confirmations: 2}
			if _, err := p.ConfirmedSnapshot(context.Background(), provider.Snapshot{}); err == nil {
				t.Fatal("invalid confirmation snapshot accepted")
			}
		})
	}
}
