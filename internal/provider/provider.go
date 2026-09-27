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
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
)

// Reader pins protocol reads to the same block used for recipe discovery.
type Reader interface {
	CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error)
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
	Chain Reader
	HTTP  *http.Client
	Log   logr.Logger
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
