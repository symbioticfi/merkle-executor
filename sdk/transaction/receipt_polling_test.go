package transaction

import (
	"context"
	"math"
	"math/big"
	"testing"
	"testing/synctest"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
)

type receiptPollingBackend struct {
	*practicalChain
	read func(context.Context, common.Hash) (*types.Receipt, error)
}

func (b *receiptPollingBackend) TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	return b.read(ctx, hash)
}

func receiptPollingManager(t *testing.T, count int) (*Manager, *receiptPollingBackend, []*types.Transaction) {
	t.Helper()
	chain := newPracticalChain()
	backend := &receiptPollingBackend{practicalChain: chain, read: chain.TransactionReceipt}
	manager, err := New(backend, practicalSigning(t, "1"), big.NewInt(1), Config{MaxFeeGwei: 100}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	manager.pending = &pendingNonce{}
	var transactions []*types.Transaction
	for i := range count {
		tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Gas: 21_000, Data: []byte{byte(i)}})
		transactions = append(transactions, tx)
		manager.pending.attempts = append(manager.pending.attempts, attempt{tx: tx, work: &work{finished: true}})
	}
	return manager, backend, transactions
}

func TestReceiptPollingContinuesPastAnOldHashFailure(t *testing.T) {
	for _, failure := range []string{"RPC error", "malformed receipt", "invalid proof"} {
		t.Run(failure, func(t *testing.T) {
			manager, backend, transactions := receiptPollingManager(t, 3)
			winner := transactions[1]
			manager.cfg.Confirmations = 1
			backend.mine(winner, types.ReceiptStatusSuccessful, 1)
			backend.read = func(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
				if hash != transactions[0].Hash() {
					return backend.practicalChain.TransactionReceipt(ctx, hash)
				}
				switch failure {
				case "RPC error":
					return nil, errors.New("old hash RPC unavailable")
				case "malformed receipt":
					return &types.Receipt{TxHash: hash}, nil
				default:
					return &types.Receipt{TxHash: hash, Status: types.ReceiptStatusSuccessful, BlockNumber: new(big.Int).SetUint64(math.MaxUint64), BlockHash: common.Hash{1}}, nil
				}
			}
			if !manager.observe(t.Context()) || manager.pending != nil || manager.nonceFloor != 1 {
				t.Fatal("older hash failure hid a canonical replacement receipt")
			}
		})
	}
}

func TestReceiptPollingRotatesAfterTheSweepBudgetExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, backend, transactions := receiptPollingManager(t, 4)
		var firstReads []common.Hash
		backend.read = func(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
			firstReads = append(firstReads, hash)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		for range 4 {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			manager.observe(ctx)
			cancel()
		}
		seen := map[common.Hash]bool{}
		for _, hash := range firstReads {
			seen[hash] = true
		}
		if len(seen) != len(transactions) {
			t.Fatalf("short receipt sweeps starved retained attempts: %v", firstReads)
		}
	})
}

func TestReceiptPollingBoundsEachUnavailableHash(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, backend, transactions := receiptPollingManager(t, 3)
		winner := transactions[1]
		backend.mine(winner, types.ReceiptStatusFailed, 0)
		var waited time.Duration
		backend.read = func(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
			if hash != transactions[0].Hash() {
				return backend.practicalChain.TransactionReceipt(ctx, hash)
			}
			started := time.Now()
			<-ctx.Done()
			waited = time.Since(started)
			return nil, ctx.Err()
		}
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		if !manager.observe(ctx) || manager.pending != nil || waited != 2*time.Second {
			t.Fatalf("unavailable old hash prevented canonical revert: waited=%s pending=%v", waited, manager.pending != nil)
		}
	})
}

func TestReceiptPollingPrioritizesNewSignaturesWithoutResettingRotation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, backend, transactions := receiptPollingManager(t, 3)
		var reads []common.Hash
		backend.read = func(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
			reads = append(reads, hash)
			<-ctx.Done()
			return nil, ethereum.NotFound
		}
		observe := func() {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			manager.observe(ctx)
		}
		observe()
		newest := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Gas: 21_000, Data: []byte{9}})
		manager.pending.attempts = append(manager.pending.attempts, attempt{tx: newest, work: &work{finished: true}})
		observe()
		observe()
		observe()
		if len(reads) != 4 || reads[0] != transactions[2].Hash() || reads[1] != transactions[0].Hash() || reads[2] != newest.Hash() || reads[3] != transactions[1].Hash() {
			t.Fatalf("new signatures reset or starved the ordinary cursor: %v", reads)
		}
	})
}
