package submission

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"math/big"
)

// ConfirmReceipt proves inclusion and depth against one canonical descendant,
// walking its exact parent hashes and then rechecking that witness by number.
// Nil means the receipt is not currently canonical or sufficiently confirmed.
func ConfirmReceipt(ctx context.Context, chain Backend, receipt *types.Receipt, depth uint64) (*types.Receipt, error) {
	if receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || depth > 1024 {
		return nil, errors.New("invalid receipt confirmation request")
	}
	witnessNumber := new(big.Int).Add(receipt.BlockNumber, new(big.Int).SetUint64(depth))
	head, err := chain.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	if head == nil || head.Number == nil || !head.Number.IsUint64() {
		return nil, errors.New("invalid confirmation head")
	}
	if head.Number.Cmp(witnessNumber) < 0 {
		return nil, nil
	}
	witness, err := chain.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return nil, err
	}
	if witness == nil || witness.Number == nil || witness.Number.Cmp(witnessNumber) != 0 {
		return nil, errors.New("invalid confirmation witness")
	}
	ancestor := witness
	for ancestor.Number.Cmp(receipt.BlockNumber) > 0 {
		parent, err := chain.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, err
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
	if err != nil {
		return nil, err
	}
	if fresh == nil || fresh.TxHash != receipt.TxHash || fresh.BlockNumber == nil || !fresh.BlockNumber.IsUint64() || fresh.Status > 1 {
		return nil, errors.New("invalid rechecked receipt")
	}
	if fresh.BlockHash != receipt.BlockHash || fresh.BlockNumber.Cmp(receipt.BlockNumber) != 0 || fresh.Status != receipt.Status {
		return nil, nil
	}
	stable, err := chain.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return nil, err
	}
	if stable == nil || stable.Hash() != witness.Hash() {
		return nil, nil
	}
	return fresh, nil
}
