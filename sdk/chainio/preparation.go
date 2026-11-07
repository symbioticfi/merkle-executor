package chainio

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"math/big"
)

// PreparationPin is provisional configured identity at a selected block. It has
// no recipe commitment or permission root and grants no authority. ChainID is a
// canonical positive decimal string so the entire pin is immutable by value.
type PreparationPin struct {
	ChainID                   string
	BlockNumber               uint64
	BlockHash                 common.Hash
	Timestamp                 uint64
	Executor, Account, Caller common.Address
}

func (p PreparationPin) Validate() error {
	id, ok := new(big.Int).SetString(p.ChainID, 10)
	if !ok || id.Sign() <= 0 || id.String() != p.ChainID || p.BlockHash == (common.Hash{}) || p.Executor == (common.Address{}) || p.Account == (common.Address{}) || p.Caller == (common.Address{}) {
		return errors.New("invalid provisional preparation identity")
	}
	return nil
}
func (p PreparationPin) Matches(s Snapshot) bool {
	return s.ChainID != nil && s.ChainID.String() == p.ChainID && s.BlockNumber != nil && s.BlockNumber.IsUint64() && s.BlockNumber.Uint64() == p.BlockNumber && s.BlockHash == p.BlockHash && s.Timestamp == p.Timestamp && s.Executor == p.Executor && s.Account == p.Account && s.Caller == p.Caller
}

// ReadAccessor lazily consumes only the assigned preparation results.
type ReadAccessor func(int) ReadResult
