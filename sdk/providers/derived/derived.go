// Package derived validates authenticated runtime fields.
package derived

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

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

// BindOutput proves every committed binding leaves the selected signed bytes
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

// Trade is the recipe-selected amount and minimum of one signed protocol request.
type Trade struct {
	Template     SignTemplate
	Budget       recipes.EconomicValue
	Floors       []recipes.EconomicValue
	Requirements keeper.RecipeRequirements
}

// NewTrade binds the template's amount and minimum words to their recipe sources.
// Extra values and result bindings are proved with them before external requests.
func NewTrade(r recipes.Recipe, sign SignTemplate, amountWord, minimumWord int, mutable map[int]bool, extra ...recipes.EconomicValue) (Trade, recipes.Value, error) {
	amount, err := sign.Field(amountWord)
	if err != nil {
		return Trade{}, recipes.Value{}, err
	}
	budget, err := recipes.Budget(r, amount, sign.Action.Step)
	if err != nil {
		return Trade{}, recipes.Value{}, err
	}
	minimum, err := sign.Field(minimumWord)
	if err != nil {
		return Trade{}, recipes.Value{}, err
	}
	floors, err := recipes.Floor(r, minimum, sign.Action.Step)
	if err != nil {
		return Trade{}, recipes.Value{}, err
	}
	economics := append(append([]recipes.EconomicValue{budget}, floors...), extra...)
	for _, binding := range sign.Action.Bindings {
		if binding.Value.Source == recipes.Result {
			economics = append(economics, recipes.EconomicValue{Expression: binding.Value, Exact: true})
		}
	}
	requirements, err := Requirements(sign, economics, mutable)
	if err != nil {
		return Trade{}, recipes.Value{}, err
	}
	return Trade{Template: sign, Budget: budget, Floors: floors, Requirements: requirements}, minimum, nil
}

// Amounts resolves the exact amount and the strongest floor; either zero is idle.
func (t Trade) Amounts(results map[uint]*big.Int) (*big.Int, *big.Int, error) {
	amount, err := recipes.EconomicResult(t.Budget, results)
	if err != nil {
		return nil, nil, err
	}
	minimum := new(big.Int)
	for _, floor := range t.Floors {
		value, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return nil, nil, err
		}
		if value.Cmp(minimum) > 0 {
			minimum = value
		}
	}
	if amount.Sign() == 0 || minimum.Sign() == 0 {
		return nil, nil, provider.ErrNotReady
	}
	return amount, minimum, nil
}

// Bind requires exact floors at the selected minimum word, then binds the payload.
func (t Trade) Bind(payload []byte, minimumWord int, results map[uint]*big.Int, out *provider.Output) error {
	at := t.Template.Offset + 32*minimumWord
	for _, floor := range t.Floors {
		if !floor.Exact {
			continue
		}
		expected, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return err
		}
		if new(big.Int).SetBytes(payload[at:at+32]).Cmp(expected) != 0 {
			return errors.New("selected minimum conflicts with exact recipe result binding")
		}
	}
	return BindOutput(t.Template, payload, results, out)
}

// AuthorizingSign requires exactly one confirmed account Sign event for digest,
// carrying the exact prepared authorization payload.
func AuthorizingSign(e *provider.Execution, owner common.Address, digest common.Hash, authorization []byte) error {
	a := account.NewMerkleAccount()
	id := a.GetABI().Events["Sign"].ID
	count := 0
	for _, log := range e.Logs {
		if log.Address != owner || len(log.Topics) == 0 || log.Topics[0] != id {
			continue
		}
		event, err := a.UnpackSignEvent(&log)
		if err != nil {
			return errors.New("malformed confirmed Sign event")
		}
		if common.Hash(event.Digest) != digest {
			continue
		}
		payload, err := a.TryPackSign(event.DomainSeparator, event.Buffer, event.Plan)
		if err != nil || !bytes.Equal(payload, authorization) {
			return errors.New("confirmed Sign differs from prepared authorization")
		}
		count++
	}
	if count != 1 {
		return errors.New("exact confirmed authorizing Sign event is required")
	}
	return nil
}
