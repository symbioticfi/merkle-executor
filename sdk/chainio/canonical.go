package chainio

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"math/big"
)

// ConfirmReceipt proves inclusion and depth against one canonical descendant,
// walking its exact parent hashes and then rechecking that witness by number.
// Nil means the receipt is not currently canonical or sufficiently confirmed.
func VerifyReceipt(ctx context.Context, chain ReceiptReader, receipt *types.Receipt, depth uint64) (*types.Receipt, error) {
	if receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || depth > 1024 || receipt.Status > 1 {
		return nil, errors.New("invalid receipt confirmation request")
	}
	if _, err := ReceiptExecution(receipt); err != nil {
		return nil, err
	}
	witnessNumber := new(big.Int).Add(receipt.BlockNumber, new(big.Int).SetUint64(depth))
	reads, err := ReadGroup(ctx, chain, ReadRequest{Kind: ReadHeader}, ReadRequest{Kind: ReadHeader, Block: witnessNumber})
	if err != nil {
		return nil, err
	}
	headResult := reads.At(0)
	head, err := headResult.Header, headResult.Err
	if err != nil {
		return nil, err
	}
	if head == nil || head.Number == nil || !head.Number.IsUint64() {
		return nil, errors.New("invalid confirmation head")
	}
	if head.Number.Cmp(witnessNumber) < 0 {
		return nil, nil
	}
	witnessResult := reads.At(1)
	witness, err := witnessResult.Header, witnessResult.Err
	if err != nil {
		return nil, err
	}
	if witness == nil || witness.Number == nil || witness.Number.Cmp(witnessNumber) != 0 {
		return nil, errors.New("invalid confirmation witness")
	}
	ancestor, err := confirmationAncestor(ctx, chain, witness, receipt.BlockNumber)
	if err != nil {
		return nil, err
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
	if _, err := ReceiptExecution(fresh); err != nil {
		return nil, err
	}
	if fresh.BlockHash != receipt.BlockHash || fresh.BlockNumber.Cmp(receipt.BlockNumber) != 0 || fresh.Status != receipt.Status {
		return nil, nil
	}
	if err := closeWitness(ctx, chain, witness); err != nil {
		if errors.Is(err, errWitnessReorganized) {
			return nil, nil
		}
		return nil, err
	}
	return fresh, nil
}

// ReceiptReader exposes exactly the evidence needed for a bounded canonical proof.
type ReceiptReader interface {
	CanonicalReader
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

// ReceiptExecution validates and clones the canonical transaction identity/logs.
// A reverted execution is valid evidence and must retain nonce ownership until persisted.
func ReceiptExecution(receipt *types.Receipt) (*Execution, error) {
	if receipt == nil || receipt.Status > 1 {
		return nil, errors.New("invalid receipt execution")
	}
	e := &Execution{TxHash: receipt.TxHash, BlockNumber: receipt.BlockNumber, BlockHash: receipt.BlockHash}
	for _, log := range receipt.Logs {
		if log == nil {
			return nil, errors.New("nil receipt log")
		}
		e.Logs = append(e.Logs, *log)
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e.Clone(), nil
}

// ConfirmReceipt returns confirmed execution evidence; nil is not yet confirmed.
func ConfirmReceipt(ctx context.Context, r ReceiptReader, receipt *types.Receipt, depth uint64) (*Execution, error) {
	verified, err := VerifyReceipt(ctx, r, receipt, depth)
	if err != nil || verified == nil {
		return nil, err
	}
	return ReceiptExecution(verified)
}

// ValidateSnapshot checks the neutral snapshot identity before chain/external I/O.
func ValidateSnapshot(s Snapshot) error {
	if s.ChainID == nil || s.ChainID.Sign() <= 0 || s.ChainID.BitLen() > 256 || s.BlockNumber == nil || s.BlockNumber.Sign() < 0 || s.Account == (common.Address{}) {
		return errors.New("provider requires a pinned chain/account snapshot")
	}
	return nil
}

// Canonical is a fresh closing observation after a covered external/read phase.
func Canonical(ctx context.Context, r Reader, s Snapshot) error {
	if err := ValidateSnapshot(s); err != nil {
		return err
	}
	reader, ok := r.(HeaderReader)
	if !ok {
		return errors.New("protocol external phase requires canonical header reader")
	}
	if s.BlockHash == (common.Hash{}) {
		return errors.New("protocol snapshot block hash is required")
	}
	h, err := reader.HeaderByNumber(ctx, s.BlockNumber)
	if err != nil || h == nil || h.Number == nil || h.Number.Cmp(s.BlockNumber) != 0 || h.Hash() != s.BlockHash || h.Time != s.Timestamp {
		return errors.New("protocol pinned snapshot is no longer canonical")
	}
	return nil
}

// VerifyExecution closes the confirmed receipt inclusion and its pinned snapshot.
func VerifyExecution(ctx context.Context, r Reader, s Snapshot) error {
	if err := ValidateSnapshot(s); err != nil {
		return err
	}
	if err := s.Execution.Validate(); err != nil {
		return err
	}
	if s.Execution.BlockNumber.Cmp(s.BlockNumber) > 0 {
		return errors.New("execution receipt is later than the confirmed snapshot")
	}
	reader, ok := r.(HeaderReader)
	if !ok {
		return errors.New("execution requires a canonical header reader")
	}
	h, err := reader.HeaderByNumber(ctx, s.Execution.BlockNumber)
	if err != nil || h == nil || h.Number == nil || h.Number.Cmp(s.Execution.BlockNumber) != 0 || h.Hash() != s.Execution.BlockHash {
		return errors.New("execution receipt block is no longer canonical")
	}
	return Canonical(ctx, r, s)
}

// SnapshotReader preserves call/header capabilities without depending on provider.
type SnapshotReader interface {
	Reader
	CanonicalReader
}

func confirmationHeader(h *types.Header, number *big.Int) error {
	if h == nil || h.Number == nil || !h.Number.IsUint64() {
		return errors.New("invalid confirmation header")
	}
	if number != nil && h.Number.Cmp(number) != 0 {
		return errors.New("RPC returned another confirmation block")
	}
	return nil
}
func confirmationAncestor(ctx context.Context, r CanonicalReader, witness *types.Header, number *big.Int) (*types.Header, error) {
	if err := confirmationHeader(witness, nil); err != nil {
		return nil, err
	}
	if number == nil || number.Sign() < 0 || number.Cmp(witness.Number) > 0 {
		return nil, errors.New("invalid confirmation ancestor number")
	}
	ancestor := witness
	for ancestor.Number.Cmp(number) > 0 {
		parent, err := r.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, err
		}
		expected := new(big.Int).Sub(ancestor.Number, big.NewInt(1))
		if err := confirmationHeader(parent, expected); err != nil {
			return nil, err
		}
		if parent.Hash() != ancestor.ParentHash {
			return nil, errors.New("inconsistent confirmation ancestry")
		}
		ancestor = parent
	}
	return ancestor, nil
}

var errWitnessReorganized = errors.New("confirmation block was reorganized")

func closeWitness(ctx context.Context, r CanonicalReader, witness *types.Header) error {
	stable, err := r.HeaderByNumber(ctx, witness.Number)
	if err != nil {
		return err
	}
	if err := confirmationHeader(stable, witness.Number); err != nil {
		return err
	}
	if stable.Hash() != witness.Hash() {
		return errWitnessReorganized
	}
	return nil
}

// ConfirmedSnapshot selects one captured head-depth ancestor and closes its witness
// after checking the authorizing execution. It never chases an advancing head.
func ConfirmedSnapshot(ctx context.Context, r SnapshotReader, original Snapshot, depth uint64) (Snapshot, error) {
	if depth > 1024 {
		return original, errors.New("confirmation depth exceeds bound")
	}
	witness, err := r.HeaderByNumber(ctx, nil)
	if err != nil {
		return original, err
	}
	if err := confirmationHeader(witness, nil); err != nil {
		return original, err
	}
	number := new(big.Int).Sub(witness.Number, new(big.Int).SetUint64(depth))
	if number.Sign() < 0 {
		return original, errors.New("insufficient blocks for confirmations")
	}
	ancestor, err := confirmationAncestor(ctx, r, witness, number)
	if err != nil {
		return original, err
	}
	original.BlockNumber = new(big.Int).Set(ancestor.Number)
	original.BlockHash = ancestor.Hash()
	original.Timestamp = ancestor.Time
	if original.Execution != nil {
		if err := VerifyExecution(ctx, r, original); err != nil {
			return original, err
		}
	}
	if err := closeWitness(ctx, r, witness); err != nil {
		return original, err
	}
	return original, nil
}

// VerifyConfirmedSnapshot freshly closes depth and identity after provider reads.
func VerifyConfirmedSnapshot(ctx context.Context, r SnapshotReader, s Snapshot, depth uint64) error {
	if depth > 1024 || s.BlockNumber == nil || !s.BlockNumber.IsUint64() || s.BlockHash == (common.Hash{}) {
		return errors.New("invalid confirmed snapshot")
	}
	number := new(big.Int).Add(s.BlockNumber, new(big.Int).SetUint64(depth))
	witness, err := r.HeaderByNumber(ctx, number)
	if err != nil {
		return err
	}
	if err := confirmationHeader(witness, number); err != nil {
		return err
	}
	ancestor, err := confirmationAncestor(ctx, r, witness, s.BlockNumber)
	if err != nil {
		return err
	}
	if ancestor.Hash() != s.BlockHash || ancestor.Time != s.Timestamp {
		return errors.New("confirmed snapshot was reorganized")
	}
	if s.Execution != nil {
		if err := VerifyExecution(ctx, r, s); err != nil {
			return err
		}
	}
	return closeWitness(ctx, r, witness)
}
