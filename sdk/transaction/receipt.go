package transaction

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// confirmReceipt proves inclusion under one canonical descendant, then rechecks
// both receipt and numbered witness. Nil means not yet confirmed or a reorg.
func confirmReceipt(ctx context.Context, chain Backend, receipt *types.Receipt, depth uint64) (*types.Receipt, error) {
	witnessNumber := new(big.Int).Add(receipt.BlockNumber, new(big.Int).SetUint64(depth))
	if !witnessNumber.IsUint64() {
		return nil, errors.New("confirmation height overflows")
	}
	reads, err := protocolio.ReadGroup(ctx, chain, protocolio.ReadRequest{Kind: protocolio.ReadHeader}, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: witnessNumber})
	if err != nil {
		return nil, failure("read confirmation headers", err)
	}
	headResult := reads.At(0)
	head, err := headResult.Header, headResult.Err
	if err != nil {
		return nil, failure("read confirmation head", err)
	}
	if head == nil || head.Number == nil || !head.Number.IsUint64() {
		return nil, errors.New("invalid confirmation head")
	}
	if head.Number.Cmp(witnessNumber) < 0 {
		return nil, nil
	}
	witnessResult := reads.At(1)
	witness, err := witnessResult.Header, witnessResult.Err
	if errors.Is(err, ethereum.NotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, failure("read confirmation witness", err)
	}
	if witness == nil || witness.Number == nil || witness.Number.Cmp(witnessNumber) != 0 {
		return nil, errors.New("invalid confirmation witness")
	}
	ancestor := witness
	for ancestor.Number.Cmp(receipt.BlockNumber) > 0 {
		parent, err := chain.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, failure("read confirmation ancestry", err)
		}
		if parent == nil || parent.Number == nil || parent.Hash() != ancestor.ParentHash || new(big.Int).Add(parent.Number, big.NewInt(1)).Cmp(ancestor.Number) != 0 {
			return nil, errors.New("inconsistent receipt ancestry")
		}
		ancestor = parent
	}
	if ancestor.Hash() != receipt.BlockHash {
		return nil, nil
	}
	fresh, err := chain.TransactionReceipt(ctx, receipt.TxHash)
	if errors.Is(err, ethereum.NotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, failure("recheck transaction receipt", err)
	}
	if _, err := executionFromReceipt(fresh, receipt.TxHash); err != nil {
		return nil, err
	}
	if fresh.BlockHash != receipt.BlockHash || fresh.BlockNumber.Cmp(receipt.BlockNumber) != 0 || fresh.Status != receipt.Status {
		return nil, nil
	}
	stable, err := chain.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return nil, failure("recheck confirmation witness", err)
	}
	if stable == nil || stable.Hash() != witness.Hash() {
		return nil, nil
	}
	return fresh, nil
}
