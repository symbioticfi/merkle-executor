package protocolio

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

// MaxReadBatch bounds one backend request. ReadGroup chunks larger independent groups.
const MaxReadBatch = 256

type ReadKind uint8

const (
	ReadCall ReadKind = iota + 1
	ReadHeader
	ReadLogs
	ReadReceipt
)

// ReadRequest describes an independent read. MulticallSafe is an explicit promise
// that a getter is caller-independent, read-only and unaffected by shared transient
// state. A backend may aggregate compatible safe calls; other calls retain Call's
// exact execution context in isolated eth_call requests. Block nil means latest.
type ReadRequest struct {
	Kind          ReadKind
	Call          ethereum.CallMsg
	Block         *big.Int
	Filter        ethereum.FilterQuery
	Hash          common.Hash
	MulticallSafe bool
}
type ReadResult struct {
	Data    []byte
	Header  *types.Header
	Logs    []types.Log
	Receipt *types.Receipt
	Err     error
}

// BatchReader is optional. It must return exactly one ordered result per request;
// member errors stay attached to their member, independently of transport failure.
// Implementations must not retry an actual failed batch as individual reads.
type BatchReader interface {
	BatchRead(context.Context, []ReadRequest) ([]ReadResult, error)
}

// ReadResults is scoped to one sequential read stage, not a cross-block cache.
// At lazily reads legacy backends, preserving early-return behavior and existing
// interfaces. Real batches execute once and defer only consumption of member errors.
// It is intended for use by one goroutine.
type ReadResults struct {
	ctx      context.Context
	reader   any
	requests []ReadRequest
	results  []ReadResult
	loaded   []bool
}

func ReadGroup(ctx context.Context, reader any, requests ...ReadRequest) (*ReadResults, error) {
	group := &ReadResults{ctx: ctx, reader: reader, requests: requests, results: make([]ReadResult, len(requests)), loaded: make([]bool, len(requests))}
	for _, request := range requests {
		if request.Kind < ReadCall || request.Kind > ReadReceipt {
			return nil, errors.New("invalid read request kind")
		}
	}
	if backend, ok := reader.(BatchReader); ok {
		for start := 0; start < len(requests); start += MaxReadBatch {
			end := min(start+MaxReadBatch, len(requests))
			results, err := backend.BatchRead(ctx, requests[start:end])
			if err != nil {
				return nil, err
			}
			if len(results) != end-start {
				return nil, errors.New("read batch result count mismatch")
			}
			copy(group.results[start:end], results)
			for i := start; i < end; i++ {
				group.loaded[i] = true
			}
		}
	}
	return group, nil
}
func (g *ReadResults) At(index int) ReadResult {
	if index < 0 || index >= len(g.requests) {
		return ReadResult{Err: errors.New("read result index outside group")}
	}
	if !g.loaded[index] {
		g.results[index] = readSingle(g.ctx, g.reader, g.requests[index])
		g.loaded[index] = true
	}
	return g.results[index]
}
func readSingle(ctx context.Context, reader any, request ReadRequest) (out ReadResult) {
	if err := ctx.Err(); err != nil {
		out.Err = err
		return
	}
	switch request.Kind {
	case ReadCall:
		backend, ok := reader.(interface {
			CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
		})
		if !ok {
			out.Err = errors.New("contract reader required")
			return
		}
		out.Data, out.Err = backend.CallContract(ctx, request.Call, request.Block)
	case ReadHeader:
		backend, ok := reader.(interface {
			HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
		})
		if !ok {
			out.Err = errors.New("header reader required")
			return
		}
		out.Header, out.Err = backend.HeaderByNumber(ctx, request.Block)
	case ReadLogs:
		backend, ok := reader.(interface {
			FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
		})
		if !ok {
			out.Err = errors.New("log reader required")
			return
		}
		out.Logs, out.Err = backend.FilterLogs(ctx, request.Filter)
	case ReadReceipt:
		backend, ok := reader.(interface {
			TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
		})
		if !ok {
			out.Err = errors.New("receipt reader required")
			return
		}
		out.Receipt, out.Err = backend.TransactionReceipt(ctx, request.Hash)
	}
	return
}

func ContractRead(target common.Address, data []byte, block *big.Int, safe bool) ReadRequest {
	return ReadRequest{Kind: ReadCall, Call: ethereum.CallMsg{To: &target, Data: data}, Block: block, MulticallSafe: safe}
}

// CallData preserves the protocol read error boundary for batched member failures.
func (r ReadResult) CallData() ([]byte, error) {
	if r.Err != nil {
		return nil, errors.New("protocol chain read failed")
	}
	return r.Data, nil
}
