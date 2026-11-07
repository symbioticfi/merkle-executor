package runtime

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// Intent is immutable recipe calldata. Recipe execution never transfers native value.
type Intent struct {
	ID   string         `json:"id"`
	To   common.Address `json:"to"`
	Data hexutil.Bytes  `json:"data"`
}

// NotSubmitted proves no broadcast was attempted. Once broadcast is attempted,
// missing receipts, nonce consumption and cancellation retain sender ownership.
// Canonical reverts are valid execution evidence and return Reverted with nil Err.
type TransactionResult struct {
	NotSubmitted bool
	Reverted     bool
	Execution    *chainio.Execution
	Err          error
}

// Driver owns one sender operation until canonical evidence or process cancellation.
// Its context belongs to the runtime process, independently of any individual Tick.
type Driver interface {
	Execute(context.Context, Intent) TransactionResult
}
