package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

// operationEnvelope keeps protocol configuration/data without copying assignments.
type operationEnvelope struct {
	Version     int             `json:"version"`
	Policy      string          `json:"policy"`
	Config      json.RawMessage `json:"config"`
	Data        json.RawMessage `json:"data,omitempty"`
	Assignments common.Hash     `json:"assignments"`
}

func assignmentDigest(out Output) common.Hash {
	out.Data = nil
	raw, _ := json.Marshal(out)
	return common.Hash(sha256.Sum256(raw))
}
func EncodeOperation(policy string, cfg any, inner Output) (Output, error) {
	config, err := json.Marshal(cfg)
	if err != nil {
		return Output{}, err
	}
	data, err := json.Marshal(operationEnvelope{Version: 2, Policy: policy, Config: config, Data: inner.Data, Assignments: assignmentDigest(inner)})
	if err != nil {
		return Output{}, err
	}
	out := inner.Clone()
	out.Data = data
	return out, nil
}
func DecodeOperation(policy string, out Output, cfg any) (Output, error) {
	var envelope operationEnvelope
	if err := strictEnvelopeJSON(out.Data, &envelope); err != nil {
		return Output{}, err
	}
	if envelope.Version != 2 || envelope.Policy != policy || envelope.Assignments != assignmentDigest(out) {
		return Output{}, errors.New("persisted operation policy or assignments differ")
	}
	if err := strictEnvelopeJSON(envelope.Config, cfg); err != nil {
		return Output{}, err
	}
	inner := out.Clone()
	inner.Data = append([]byte(nil), envelope.Data...)
	return inner, nil
}

// OperationID depends only on the complete owned action keys, never registration order.
func OperationID(actions []string) string {
	keys := append([]string(nil), actions...)
	slices.Sort(keys)
	return strings.Join(keys, ",")
}
func OutputRequirements(out Output, targets map[string]common.Address) Requirements {
	requirements := Requirements{ExclusiveTargets: maps.Clone(targets)}
	for key := range out.Actions {
		requirements.Actions = append(requirements.Actions, key)
	}
	for index := range out.Inputs {
		requirements.Inputs = append(requirements.Inputs, index)
	}
	slices.Sort(requirements.Actions)
	slices.Sort(requirements.Inputs)
	return requirements
}

// ValidateRecord checks assignment and receipt claims before live protocol I/O.
// Prepared numerical witnesses are retained without rerunning their lens.
func ValidateRecord(record Record, integration, policy string, expected Requirements) error {
	if record.Integration != integration || record.Policy != policy || record.ID == "" || record.ID != OperationID(expected.Actions) {
		return errors.New("persisted operation identity or policy differs")
	}
	actions := append([]string(nil), record.Requirements.Actions...)
	slices.Sort(actions)
	wantActions := append([]string(nil), expected.Actions...)
	slices.Sort(wantActions)
	inputs := append([]uint(nil), record.Requirements.Inputs...)
	slices.Sort(inputs)
	wantInputs := append([]uint(nil), expected.Inputs...)
	slices.Sort(wantInputs)
	if !slices.Equal(actions, wantActions) || !slices.Equal(inputs, wantInputs) || !maps.Equal(record.Requirements.ExclusiveTargets, expected.ExclusiveTargets) || len(record.Output.SkipSteps) != 0 || len(record.Output.Actions) != len(actions) || len(record.Output.Inputs) != len(inputs) {
		return errors.New("persisted operation assignment or exclusive target scope differs")
	}
	for i, key := range actions {
		if key == "" || (i > 0 && key == actions[i-1]) {
			return errors.New("persisted operation action repeated")
		}
		if _, ok := record.Output.Actions[key]; !ok {
			return errors.New("persisted operation action missing")
		}
	}
	for i, index := range inputs {
		if i > 0 && index == inputs[i-1] {
			return errors.New("persisted operation input repeated")
		}
		if _, ok := record.Output.Inputs[index]; !ok {
			return errors.New("persisted operation input missing")
		}
	}
	for key, target := range expected.ExclusiveTargets {
		if !slices.Contains(actions, key) || target == (common.Address{}) {
			return errors.New("persisted operation exclusive claim invalid")
		}
	}
	for _, value := range record.Requirements.Values {
		if value.Value == nil || value.Value.Sign() < 0 || value.Value.BitLen() > 256 || value.Source > 2 {
			return errors.New("persisted operation numerical dependency invalid")
		}
	}
	return nil
}

// EqualOutput checks an operation cannot advance a different saved assignment/data.
func EqualOutput(a, b Output) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}

func strictEnvelopeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("malformed persisted operation data")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("multiple persisted operation values")
	}
	return nil
}
