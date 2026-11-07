package runtime

import (
	"context"
	"math/big"

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

// Transaction is the complete durable signer ownership record. Attempts contains
// signed wire bytes in signing order, including rejected or ambiguous submissions.
// Terminal is empty, "confirmed", or "reverted"; only canonical receipts prove it.
type Transaction struct {
	Intent    Intent             `json:"intent"`
	Sender    common.Address     `json:"sender"`
	ChainID   *big.Int           `json:"chainId"`
	Nonce     uint64             `json:"nonce"`
	Admitted  bool               `json:"admitted"`
	Attempts  []hexutil.Bytes    `json:"attempts,omitempty"`
	Terminal  string             `json:"terminal,omitempty"`
	Execution *chainio.Execution `json:"execution,omitempty"`
}

// NotSubmitted proves that no signed attempt exists. Pending retains ownership;
// missing receipts, foreign nonce consumption and cancellation never prove failure.
type TransactionResult struct {
	Pending      bool
	NotSubmitted bool
	Reverted     bool
	Execution    *chainio.Execution
	Err          error
}

type PersistTransaction func(Transaction) error

// Driver fences every other host job using this signer until Release. Execute and
// Resume receive process-owned contexts. Signed recovery observes all saved hashes
// without signing, broadcasting or replaying saved calldata. Persistence accepts
// signed bytes and canonical evidence before any broadcast or reservation release.
type Driver interface {
	Reserve(context.Context, string) error
	Release(context.Context, string) error
	Execute(context.Context, Intent, PersistTransaction) TransactionResult
	Resume(context.Context, Transaction, PersistTransaction) TransactionResult
}
