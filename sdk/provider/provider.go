// Package provider defines runtime data acquisition, separate from recipe execution.
package provider

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

// ErrNotReady is normal idle work (no eligible rewards, budget or receipt yet).
// It is distinct from malformed data, failed validation and transport errors.
var ErrNotReady = errors.New("strategy is not ready")

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

// ValidateSnapshot fences a compiled operation to its authenticated attempt.
// A fresh program must compile new operations rather than reuse old route data.
func (p Preparation) ValidateSnapshot(expected chainio.Snapshot) error {
	actual := p.Program.Snapshot
	if expected.ChainID == nil || actual.ChainID == nil || expected.BlockNumber == nil || actual.BlockNumber == nil || expected.ChainID.Cmp(actual.ChainID) != 0 || expected.BlockNumber.Cmp(actual.BlockNumber) != 0 || expected.BlockHash != actual.BlockHash || expected.Timestamp != actual.Timestamp || expected.Executor != actual.Executor || expected.Account != actual.Account || expected.Caller != actual.Caller || expected.RecipeHash != actual.RecipeHash || expected.Root != actual.Root {
		return errors.New("preparation identity differs from compiled program")
	}
	return nil
}
