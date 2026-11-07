// Package provider defines runtime data acquisition, separate from recipe execution.
package provider

import (
	"context"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// Neutral chain aliases retained during the runtime migration.
type Reader = chainio.Reader
type CodeOverrideReader = chainio.CodeOverrideReader
type LogReader = chainio.LogReader
type Execution = chainio.Execution

// ErrNotReady is normal idle work (no eligible rewards, budget or receipt yet).
// It is distinct from malformed data, failed validation and transport errors.
var ErrNotReady = errors.New("strategy is not ready")

type Snapshot = chainio.Snapshot

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

// ExclusiveTargetProvider requires sole ownership of managed calls to its
// settlement targets. Each key is a prepared action key. Hosts must validate
// these claims against the entire immutable execute call before submission and
// recovery, including providers whose progress was already persisted as done.
// This conservative boundary prevents receipt evidence being reused across
// providers or confused with static calls when the protocol has no action ID.
type ExclusiveTargetProvider interface {
	ExclusiveTargets() map[string]common.Address
}

// ExclusiveOutputTargetProvider reconstructs ownership from immutable persisted
// preparation data, including after restart and after its progress is done.
type ExclusiveOutputTargetProvider interface {
	ExclusiveOutputTargets(Output) (map[string]common.Address, error)
}
