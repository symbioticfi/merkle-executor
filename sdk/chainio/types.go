// Package chainio contains neutral immutable chain identities and bounded reads.
package chainio

import (
	"context"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"math/big"
)

// Reader pins protocol reads to the same block used for recipe discovery.
type Reader interface {
	CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error)
}

// CodeOverrideReader executes a read with replacement code at the call target.
// The original sender, target, storage and block are preserved. Hosts must fail
// when their RPC does not implement state overrides; changing the call sender or
// evaluating the queries through Multicall is not equivalent.
type CodeOverrideReader interface {
	CallContractWithCode(context.Context, ethereum.CallMsg, *big.Int, []byte) ([]byte, error)
}

// LogReader discovers issuer-owned requests at the pinned snapshot. Providers
// that need event discovery require this interface in addition to Reader.
type LogReader interface {
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
}

// Execution identifies the confirmed recipe transaction. It is persisted before
// releasing nonce ownership and is available only while reconciling that execution.
type Execution struct {
	TxHash      common.Hash `json:"txHash"`
	BlockNumber *big.Int    `json:"blockNumber"`
	BlockHash   common.Hash `json:"blockHash"`
	Logs        []types.Log `json:"logs"`
}

func (e *Execution) Validate() error {
	if e == nil || e.TxHash == (common.Hash{}) || e.BlockNumber == nil || !e.BlockNumber.IsUint64() || e.BlockHash == (common.Hash{}) {
		return errors.New("confirmed execution identity is required")
	}
	for i, log := range e.Logs {
		if log.Removed || log.TxHash != e.TxHash || log.BlockHash != e.BlockHash || log.BlockNumber != e.BlockNumber.Uint64() || (i > 0 && log.Index <= e.Logs[i-1].Index) {
			return errors.New("execution log identity mismatch")
		}
	}
	return nil
}

// Clone keeps returned provider data from changing the durable transaction record.
func (e *Execution) Clone() *Execution {
	if e == nil {
		return nil
	}
	copy := *e
	if e.BlockNumber != nil {
		copy.BlockNumber = new(big.Int).Set(e.BlockNumber)
	}
	copy.Logs = make([]types.Log, len(e.Logs))
	for i, log := range e.Logs {
		copy.Logs[i] = log
		// Geth's JSON decoder requires an array, including for anonymous logs.
		copy.Logs[i].Topics = append([]common.Hash{}, log.Topics...)
		copy.Logs[i].Data = append([]byte{}, log.Data...)
	}
	return &copy
}

type Snapshot struct {
	ChainID     *big.Int       `json:"chainId"`
	BlockNumber *big.Int       `json:"blockNumber"`
	BlockHash   common.Hash    `json:"blockHash"`
	Timestamp   uint64         `json:"timestamp"`
	Executor    common.Address `json:"executor"`
	Account     common.Address `json:"account"`
	Caller      common.Address `json:"caller"`
	RecipeHash  common.Hash    `json:"recipeHash"`
	Root        common.Hash    `json:"root"`
	Execution   *Execution     `json:"execution,omitempty"`
}

type HeaderReader interface {
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
}
type CanonicalReader interface {
	HeaderReader
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
}
