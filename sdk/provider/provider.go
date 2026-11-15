// Package provider defines runtime data acquisition, separate from recipe execution.
package provider

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

// Deps supplies caller-owned transports for runtime data acquisition.
type Deps struct {
	Chain Reader
	HTTP  *http.Client
	// LookupSecret resolves configured credential references without environment access in the SDK.
	LookupSecret func(string) string
}

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

// Execution identifies a canonically confirmed recipe transaction.
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

// Clone isolates receipt evidence from mutable provider data.
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

// ErrNotReady is normal idle work (no eligible rewards, budget or receipt yet).
// It is distinct from malformed data, failed validation and transport errors.
var ErrNotReady = errors.New("strategy is not ready")

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

// Output fills one execution. Action keys are "stepIndex:actionIndex";
// values replace calldata only, never the target, mask, proof or native value.
// Inputs are decimal uint256 strings. Duplicate assignments are rejected.
type Output struct {
	// Results binds replayed prefix reads to the full original execution.
	Results   map[uint]string          `json:"results,omitempty"`
	Inputs    map[uint]string          `json:"inputs,omitempty"`
	Actions   map[string]hexutil.Bytes `json:"actions,omitempty"`
	SkipSteps []uint                   `json:"skipSteps,omitempty"`
	Data      json.RawMessage          `json:"data,omitempty"`
	// Task retains optional external settlement work for this process only.
	Task Task `json:"-"`
}

// Clone isolates authorized runtime fields while retaining the same in-memory task.
func (o Output) Clone() Output {
	copy := o
	if o.Results != nil {
		copy.Results = make(map[uint]string, len(o.Results))
		for key, value := range o.Results {
			copy.Results[key] = value
		}
	}
	if o.Inputs != nil {
		copy.Inputs = make(map[uint]string, len(o.Inputs))
		for key, value := range o.Inputs {
			copy.Inputs[key] = value
		}
	}
	if o.Actions != nil {
		copy.Actions = make(map[string]hexutil.Bytes, len(o.Actions))
		for key, value := range o.Actions {
			copy.Actions[key] = append(hexutil.Bytes(nil), value...)
		}
	}
	copy.SkipSteps = append([]uint(nil), o.SkipSteps...)
	copy.Data = append(json.RawMessage(nil), o.Data...)
	return copy
}

// Task follows external settlement after the recipe transaction confirms.
// It owns its state in memory. Hosts call it serially after confirmation.
type Task interface {
	Advance(context.Context, Snapshot) (Progress, error)
}

// TaskFunc adapts in-memory settlement work to Task.
type TaskFunc func(context.Context, Snapshot) (Progress, error)

func (f TaskFunc) Advance(ctx context.Context, s Snapshot) (Progress, error) {
	return f(ctx, s)
}

// Progress separates confirmed recipe execution from external settlement.
type Progress struct {
	Done      bool   `json:"done"`
	Outcome   string `json:"outcome,omitempty"`
	Reference string `json:"reference,omitempty"`
}

// ExclusiveOutputTargetProvider declares receipt attribution for prepared data.
type ExclusiveOutputTargetProvider interface {
	ExclusiveOutputTargets(Output) (map[string]common.Address, error)
}
