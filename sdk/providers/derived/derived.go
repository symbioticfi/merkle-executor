// Package derived implements protocol-neutral authenticated runtime envelopes.
package derived

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Envelope struct {
	Version int             `json:"version"`
	Policy  string          `json:"policy"`
	Config  json.RawMessage `json:"config,omitempty"`
	Output  provider.Output `json:"output"`
	Matched bool            `json:"matched"`
}

func Encode(policy string, cfg any, inner provider.Output, matched bool) (provider.Output, error) {
	var raw json.RawMessage
	var err error
	if matched {
		raw, err = json.Marshal(cfg)
		if err != nil {
			return provider.Output{}, err
		}
	}
	data, err := json.Marshal(Envelope{Version: 1, Policy: policy, Config: raw, Output: inner, Matched: matched})
	if err != nil {
		return provider.Output{}, err
	}
	outer := inner
	outer.Data = data
	return outer, nil
}

func Decode(policy string, out provider.Output, cfg any) (provider.Output, bool, error) {
	var envelope Envelope
	if err := StrictJSON(out.Data, &envelope); err != nil {
		return provider.Output{}, false, err
	}
	if envelope.Version != 1 || envelope.Policy != policy {
		return provider.Output{}, false, errors.New("persisted derived provider version or policy mismatch")
	}
	inner := out
	inner.Data = envelope.Output.Data
	actual, _ := json.Marshal(inner)
	expected, _ := json.Marshal(envelope.Output)
	if !bytes.Equal(actual, expected) {
		return provider.Output{}, false, errors.New("persisted derived output differs from its immutable envelope")
	}
	if !envelope.Matched {
		if len(envelope.Config) != 0 || len(inner.Inputs) != 0 || len(inner.Actions) != 0 || len(inner.SkipSteps) != 0 || len(inner.Data) != 0 || len(inner.Results) != 0 {
			return provider.Output{}, false, errors.New("invalid unmatched provider envelope")
		}
		return inner, false, nil
	}
	if err := StrictJSON(envelope.Config, cfg); err != nil {
		return provider.Output{}, false, err
	}
	return inner, true, nil
}

func StrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("malformed persisted derived provider data")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("multiple persisted derived provider values")
	}
	return nil
}

type SignTemplate struct {
	Action recipes.ManagedAction
	Domain common.Hash
	Buffer []byte
	Plan   []account.IMerkleAccountHash
	Offset int
}

// Sign decodes only canonical sign layouts. All ABI framing and hash-plan bytes
// must be pinned; dynamic replacement is restricted to buffer data itself.
func Sign(action recipes.ManagedAction, owner common.Address) (SignTemplate, bool, error) {
	method := account.NewMerkleAccount().GetABI().Methods["sign"]
	data := action.Leaf.Data
	if action.Leaf.Target != owner || len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
		return SignTemplate{}, false, nil
	}
	if err := action.Pinned(0, 4); err != nil {
		return SignTemplate{}, true, err
	}
	values, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return SignTemplate{}, true, errors.New("invalid sign permission template")
	}
	if len(values) != 3 {
		return SignTemplate{}, true, errors.New("invalid sign permission arguments")
	}
	domain := common.Hash(values[0].([32]byte))
	buffer := values[1].([]byte)
	plan := *abi.ConvertType(values[2], new([]account.IMerkleAccountHash)).(*[]account.IMerkleAccountHash)
	encoded, err := account.NewMerkleAccount().TryPackSign(domain, buffer, plan)
	if err != nil || !bytes.Equal(encoded, data) {
		return SignTemplate{}, true, errors.New("noncanonical sign template is unsupported")
	}
	const bufferOffset = 132
	if err := action.Pinned(0, bufferOffset); err != nil {
		return SignTemplate{}, true, err
	}
	if err := action.Pinned(bufferOffset+len(buffer), len(data)-bufferOffset-len(buffer)); err != nil {
		return SignTemplate{}, true, err
	}
	return SignTemplate{Action: action, Domain: domain, Buffer: buffer, Plan: plan, Offset: bufferOffset}, true, nil
}

func (s SignTemplate) PinnedWord(index int) error { return s.Action.Pinned(s.Offset+32*index, 32) }
func (s SignTemplate) Field(index int) (recipes.Value, error) {
	return s.Action.Field(s.Offset + 32*index)
}
func WordAddress(word []byte) (common.Address, error) {
	if len(word) != 32 || !bytes.Equal(word[:12], make([]byte, 12)) {
		return common.Address{}, errors.New("noncanonical address in recipe identity")
	}
	a := common.BytesToAddress(word[12:])
	if a == (common.Address{}) {
		return a, errors.New("zero recipe identity address")
	}
	return a, nil
}

// Requirements owns only full-word input bindings inside an authenticated
// protocol buffer. Result bindings declare numerical dependencies and are later
// compared to the final payload before original-code full simulation.
func Requirements(template SignTemplate, economic []recipes.EconomicValue, mutableWords map[int]bool) (keeper.RecipeRequirements, error) {
	requirements := keeper.RecipeRequirements{Actions: []string{template.Action.Key}}
	for _, value := range economic {
		requirements.Values = append(requirements.Values, recipes.EconomicExpressions(value)...)
	}
	seen := map[uint]bool{}
	for _, binding := range template.Action.Bindings {
		relative := int(binding.Offset) - template.Offset
		if relative < 0 || relative%32 != 0 || !mutableWords[relative/32] {
			return requirements, errors.New("sign binding targets an unsupported or identity field")
		}
		switch binding.Value.Source {
		case recipes.Input:
			index := uint(binding.Value.Value.Uint64())
			if seen[index] {
				return requirements, errors.New("one input cannot identify multiple signed protocol fields")
			}
			requirements.Inputs = append(requirements.Inputs, index)
			seen[index] = true
		case recipes.Result:
			// Every result binding must already declare a deterministic expression;
			// only financial amount and floor expressions require READ ancestry.
			found := false
			for _, value := range economic {
				if value.Expression.Source == recipes.Result && value.Expression.Value.Cmp(binding.Value.Value) == 0 {
					found = true
				}
			}
			if !found {
				return requirements, errors.New("sign result binding has no supported derived source")
			}
		case recipes.Literal:
		default:
			return requirements, errors.New("invalid sign binding source")
		}
	}
	return requirements, nil
}

// BindOutput proves every committed binding leaves the persisted signed bytes
// exactly unchanged. Quoted external fields may fill their declared inputs;
// literals/results must equal the selected payload word.
func BindOutput(template SignTemplate, payload []byte, results map[uint]*big.Int, out *provider.Output) error {
	if len(payload) != len(template.Action.Leaf.Data) {
		return errors.New("derived payload changes template length")
	}
	if out.Inputs == nil {
		out.Inputs = map[uint]string{}
	}
	for _, binding := range template.Action.Bindings {
		at := int(binding.Offset)
		word := new(big.Int).SetBytes(payload[at : at+32])
		switch binding.Value.Source {
		case recipes.Input:
			index := uint(binding.Value.Value.Uint64())
			if old, ok := out.Inputs[index]; ok && old != word.String() {
				return errors.New("recipe input has conflicting signed field meanings")
			}
			out.Inputs[index] = word.String()
		case recipes.Result:
			actual := results[uint(binding.Value.Value.Uint64())]
			if actual == nil || actual.Cmp(word) != 0 {
				return errors.New("recipe result binding differs from selected signed value")
			}
		case recipes.Literal:
			if binding.Value.Value.Cmp(word) != 0 {
				return errors.New("recipe literal binding differs from selected signed value")
			}
		default:
			return errors.New("unsupported signed field binding")
		}
	}
	for i, b := range payload {
		if (b^template.Action.Leaf.Data[i]) & ^template.Action.Leaf.DataMask[i] != 0 {
			return errors.New("derived signed payload changes pinned permission bits")
		}
	}
	return nil
}

func Empty(policy string) (provider.Output, error) {
	return Encode(policy, nil, provider.Output{}, false)
}
