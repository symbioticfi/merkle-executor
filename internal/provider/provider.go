// Package provider defines runtime data acquisition, separate from recipe execution.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
)

// Reader pins protocol reads to the same block used for recipe discovery.
type Reader interface {
	CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error)
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
		copy.Logs[i].Data = bytes.Clone(log.Data)
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

// Output is persisted before sending. Action keys are "stepIndex:actionIndex";
// values replace calldata only, never the target, mask, proof or native value.
// Inputs are decimal uint256 strings. Duplicate assignments are rejected.
type Output struct {
	Inputs    map[uint]string          `json:"inputs,omitempty"`
	Actions   map[string]hexutil.Bytes `json:"actions,omitempty"`
	SkipSteps []uint                   `json:"skipSteps,omitempty"`
	Data      json.RawMessage          `json:"data,omitempty"`
}

// Progress separates confirmed recipe execution from external settlement.
type Progress struct {
	Done      bool            `json:"done"`
	Outcome   string          `json:"outcome,omitempty"`
	State     json.RawMessage `json:"state,omitempty"`
	Reference string          `json:"reference,omitempty"`
}

// Provider prepares bounded inputs and optionally reconciles an asynchronous
// operation after its authorization recipe confirms. Advance must be restart-safe:
// persist deterministic identity in Output and reconcile before retrying a POST.
type Provider interface {
	Prepare(ctx context.Context, snapshot Snapshot) (Output, error)
	Advance(ctx context.Context, snapshot Snapshot, output Output, state json.RawMessage) (Progress, error)
}

type Deps struct {
	Chain     Reader
	HTTP      *http.Client
	Log       logr.Logger
	ConfigDir string
}

type Factory func(raw yaml.Node, deps Deps) (Provider, error)

// Registry is built during startup, then read by the single keeper loop.
type Registry struct{ factories map[string]Factory }

func NewRegistry() *Registry { return &Registry{factories: make(map[string]Factory)} }

func (r *Registry) Register(name string, factory Factory) error {
	if name == "" || factory == nil {
		return errors.New("provider name and factory required")
	}
	if _, exists := r.factories[name]; exists {
		return errors.Errorf("duplicate provider %q", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *Registry) New(name string, raw yaml.Node, deps Deps) (Provider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, errors.Errorf("unknown provider %q", name)
	}
	return factory(raw, deps)
}
