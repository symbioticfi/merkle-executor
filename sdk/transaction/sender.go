package transaction

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type ExecutionRequest struct {
	To   common.Address
	Data []byte
}
type ExecutionResult struct {
	Hash      common.Hash
	Execution *provider.Execution
	Reverted  bool
}

// Sender retains the keeper's request ticket; signing and observation belong to Manager.
type Sender struct {
	Deadline func() time.Time
	mu       sync.Mutex
	manager  *Manager
	name     string
	active   *work
}

func (m *Manager) Sender(name string) *Sender { return &Sender{manager: m, name: name} }
func (s *Sender) Busy() bool                  { s.mu.Lock(); defer s.mu.Unlock(); return s.active != nil }
func (s *Sender) Submit(ctx context.Context, req ExecutionRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return ErrBusy
	}
	if s.manager == nil {
		return errors.New("transaction manager required")
	}
	request := Request{To: req.To, Data: req.Data, Label: "execute_recipe", Solver: s.name}
	if s.Deadline != nil {
		request.Deadline = s.Deadline()
	}
	w, accepted := s.manager.admit(ctx, request, true)
	if !accepted {
		return ctx.Err()
	}
	s.active = w
	<-w.ready
	if !w.signed {
		result := <-w.results
		s.active = nil
		return result.Err
	}
	return ctx.Err()
}
func (s *Sender) Poll(ctx context.Context) (*ExecutionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case result := <-s.active.results:
		if result.Uncertain() && !errors.Is(result.Err, ErrNonceConsumed) {
			s.active.results <- result
			return nil, result.Err
		}
		s.active = nil
		if result.Err != nil {
			return nil, result.Err
		}
		execution, err := executionFromReceipt(result.Receipt, result.Hash)
		if err != nil {
			return nil, err
		}
		return &ExecutionResult{Hash: result.Hash, Execution: execution, Reverted: result.Reverted()}, nil
	default:
	}
	select {
	case err := <-s.active.errors:
		return nil, err
	default:
		return nil, nil
	}
}

func executionFromReceipt(receipt *types.Receipt, hash common.Hash) (*provider.Execution, error) {
	if receipt == nil || receipt.TxHash != hash || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockHash == (common.Hash{}) || receipt.Status > 1 {
		return nil, errors.New("invalid transaction receipt")
	}
	execution := &provider.Execution{TxHash: receipt.TxHash, BlockNumber: receipt.BlockNumber, BlockHash: receipt.BlockHash, Logs: make([]types.Log, len(receipt.Logs))}
	for i, log := range receipt.Logs {
		if log == nil || log.TxIndex != receipt.TransactionIndex {
			return nil, errors.New("invalid transaction receipt log")
		}
		execution.Logs[i] = *log
	}
	if err := execution.Validate(); err != nil {
		return nil, err
	}
	return execution.Clone(), nil
}
