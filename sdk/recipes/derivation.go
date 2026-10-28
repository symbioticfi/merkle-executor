package recipes

import (
	"bytes"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

// ManagedAction retains the authenticated template and its exact instruction.
type ManagedAction struct {
	Key      string
	Step     uint
	Slot     RuntimeSlot
	Leaf     Leaf
	Bindings []Binding
}

func CloneTree(t *Tree) *Tree {
	if t == nil {
		return nil
	}
	c := *t
	if t.ChainID != nil {
		c.ChainID = new(big.Int).Set(t.ChainID)
	}
	if t.Account != nil {
		a := *t.Account
		c.Account = &a
	}
	c.Leaves = make([]Leaf, len(t.Leaves))
	for i, leaf := range t.Leaves {
		c.Leaves[i] = leaf
		if leaf.Value != nil {
			c.Leaves[i].Value = new(big.Int).Set(leaf.Value)
		}
		c.Leaves[i].Data = bytes.Clone(leaf.Data)
		c.Leaves[i].DataMask = bytes.Clone(leaf.DataMask)
		c.Leaves[i].Proof = append([]common.Hash(nil), leaf.Proof...)
	}
	return &c
}

func ManagedActions(recipe Recipe, tree *Tree) ([]ManagedAction, error) {
	inspection, err := Inspect(recipe)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, errors.New("permission tree is required")
	}
	var actions []ManagedAction
	for _, slot := range inspection.Slots {
		if slot.Operation != Manage {
			continue
		}
		for i, hash := range slot.Leaves {
			leaf, err := tree.lookup(hash)
			if err != nil {
				return nil, err
			}
			if err = validateBindings(slot.Bindings[i], len(leaf.Data), func(Value) error { return nil }); err != nil {
				return nil, err
			}
			actions = append(actions, ManagedAction{Key: strconv.FormatUint(uint64(slot.StepIndex), 10) + ":" + strconv.Itoa(i), Step: slot.StepIndex, Slot: slot, Leaf: leaf, Bindings: slot.Bindings[i]})
		}
	}
	return actions, nil
}

// Pinned checks an identity field is fully committed and cannot be rebound.
func (a ManagedAction) Pinned(offset, length int) error {
	if offset < 0 || length < 0 || offset+length > len(a.Leaf.Data) || len(a.Leaf.DataMask) != len(a.Leaf.Data) {
		return errors.New("identity field exceeds calldata")
	}
	for _, mask := range a.Leaf.DataMask[offset : offset+length] {
		if mask != 0 {
			return errors.New("recipe leaves an identity field unconstrained")
		}
	}
	for _, b := range a.Bindings {
		if int(b.Offset) < offset+length && int(b.Offset)+32 > offset {
			return errors.New("recipe binds an identity field")
		}
	}
	return nil
}

// Field identifies a complete, non-overlapping word binding. Unbound numerical
// fields cannot acquire policy merely from a masked permission preimage.
func (a ManagedAction) Field(offset int) (Value, error) {
	for _, b := range a.Bindings {
		if int(b.Offset) == offset {
			return b.Value, nil
		}
		if int(b.Offset) < offset+32 && int(b.Offset)+32 > offset {
			return Value{}, errors.New("recipe binding partially overlaps a field")
		}
	}
	return Value{}, errors.New("recipe numerical field has no result binding")
}

type EconomicValue struct {
	Expression Value
	Upper      []Value
	Lower      []Value
	Input      *uint
	// Exact means the field itself is bound to this result. A provider must not
	// select a different signed value, even when it exceeds a minimum floor.
	Exact bool
}

// Budget requires the recipe to select an exact amount. Bounds
// validate that amount; they never choose it. Use MIN/MAX arithmetic in the
// recipe to express a capped amount, bound directly or through an input equality.
func Budget(recipe Recipe, value Value, before uint) (EconomicValue, error) {
	economic, err := ExactValue(recipe, value, before)
	if err != nil {
		return EconomicValue{}, err
	}
	if value.Source == Input {
		for i := uint(0); i < before && i < uint(len(recipe.Steps)); i++ {
			step := recipe.Steps[i]
			if step[0] != LessOrEqual {
				continue
			}
			args, e := binaryArgs.Unpack(step[1:])
			if e != nil {
				return EconomicValue{}, e
			}
			a, b := decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))
			var bounds *[]Value
			if sameValue(a, value) {
				bounds, a = &economic.Upper, b
			} else if sameValue(b, value) {
				bounds = &economic.Lower
			} else {
				continue
			}
			a, e = boundSource(recipe, a, i)
			if e != nil {
				return EconomicValue{}, e
			}
			*bounds = append(*bounds, a)
		}
	}
	if _, err = dependencies(recipe, EconomicExpressions(economic)); err != nil {
		return EconomicValue{}, err
	}
	return economic, nil
}

// EconomicExpressions includes every bound whose value must be proved before
// requesting external data.
func EconomicExpressions(value EconomicValue) []Value {
	values := []Value{value.Expression}
	values = append(values, value.Upper...)
	return append(values, value.Lower...)
}

// Floor combines the recipe's preceding lower bounds. Equality remains exact.
func Floor(recipe Recipe, value Value, before uint) ([]EconomicValue, error) {
	if value.Source == Literal || value.Source == Result {
		if _, err := dependencies(recipe, []Value{value}); err != nil {
			return nil, err
		}
		return []EconomicValue{{Expression: value, Exact: true}}, nil
	}
	if value.Source != Input || value.Value == nil || !value.Value.IsUint64() {
		return nil, errors.New("minimum proceeds must be a recipe value")
	}
	index := uint(value.Value.Uint64())
	var floors []EconomicValue
	for i := uint(0); i < before && i < uint(len(recipe.Steps)); i++ {
		step := recipe.Steps[i]
		if step[0] != LessOrEqual && step[0] != Equal {
			continue
		}
		args, err := binaryArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		a, b := decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))
		exact := step[0] == Equal
		if exact && sameValue(a, value) {
			a, b = b, a
		}
		if !sameValue(b, value) {
			continue
		}
		a, err = boundSource(recipe, a, i)
		if err != nil {
			return nil, err
		}
		_, err = dependencies(recipe, []Value{a})
		if err != nil {
			return nil, err
		}
		floors = append(floors, EconomicValue{Expression: a, Input: &index, Exact: exact})
	}
	if len(floors) == 0 {
		return nil, errors.New("minimum proceeds input lacks a recipe lower bound")
	}
	return floors, nil
}

func boundSource(recipe Recipe, value Value, before uint) (Value, error) {
	if value.Source != Input {
		return value, nil
	}
	return equalityAnchor(recipe, value, before)
}

func equalityAnchor(recipe Recipe, input Value, before uint) (Value, error) {
	var result Value
	for i := uint(0); i < before; i++ {
		if recipe.Steps[i][0] != Equal {
			continue
		}
		args, err := binaryArgs.Unpack(recipe.Steps[i][1:])
		if err != nil {
			return Value{}, err
		}
		a, b := decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))
		if sameValue(a, input) {
			a, b = b, a
		}
		if !sameValue(b, input) || (a.Source != Result && a.Source != Literal) {
			continue
		}
		if result.Value != nil && !sameValue(result, a) {
			return Value{}, errors.New("trade amount has ambiguous equality sources")
		}
		result = a
	}
	if result.Value == nil {
		return Value{}, errors.New("trade amount input lacks an exact recipe result equality")
	}
	return result, nil
}

// ExactValue resolves deterministic recipe values, including literal
// and constant-arithmetic results. Inputs require an exact preceding equality;
// a ceiling never selects a request ID, epoch or other identity index.
func ExactValue(recipe Recipe, value Value, before uint) (EconomicValue, error) {
	economic, err := exactSource(recipe, value, before)
	if err != nil {
		return EconomicValue{}, err
	}
	if _, err = dependencies(recipe, []Value{economic.Expression}); err != nil {
		return EconomicValue{}, err
	}
	return economic, nil
}
func exactSource(recipe Recipe, value Value, before uint) (EconomicValue, error) {
	expression := value
	var index *uint
	if value.Source == Input {
		if value.Value == nil || !value.Value.IsUint64() {
			return EconomicValue{}, errors.New("invalid exact input")
		}
		i := uint(value.Value.Uint64())
		index = &i
		var err error
		expression, err = equalityAnchor(recipe, value, before)
		if err != nil {
			return EconomicValue{}, err
		}
	}
	if expression.Source != Result && expression.Source != Literal {
		return EconomicValue{}, errors.New("recipe value is not deterministic")
	}
	if expression.Source == Result && (expression.Value == nil || !expression.Value.IsUint64() || expression.Value.Uint64() >= uint64(before)) {
		return EconomicValue{}, errors.New("exact result must precede use")
	}
	return EconomicValue{Expression: expression, Input: index, Exact: true}, nil
}

func sameValue(a, b Value) bool {
	return a.Source == b.Source && a.Value != nil && b.Value != nil && a.Value.Cmp(b.Value) == 0
}

func dependencies(recipe Recipe, values []Value) (map[uint]bool, error) {
	needed := map[uint]bool{}
	inputMemo := map[[2]uint]bool{}
	var visit func(Value, uint) error
	visit = func(v Value, before uint) error {
		if v.Value == nil {
			return errors.New("missing recipe operand")
		}
		if v.Source == Literal {
			return nil
		}
		if v.Source == Input {
			if !v.Value.IsUint64() {
				return errors.New("invalid recipe input index")
			}
			key := [2]uint{uint(v.Value.Uint64()), before}
			if inputMemo[key] {
				return nil
			}
			economic, err := exactSource(recipe, v, before)
			if err != nil {
				return errors.New("recipe expression depends on an input without a preceding deterministic result source")
			}
			if err := visit(economic.Expression, before); err != nil {
				return err
			}
			inputMemo[key] = true
			return nil
		}
		if v.Source != Result || !v.Value.IsUint64() || v.Value.Uint64() >= uint64(len(recipe.Steps)) || v.Value.Uint64() >= uint64(before) {
			return errors.New("recipe result is not an earlier numerical step")
		}
		index := uint(v.Value.Uint64())
		if needed[index] {
			return nil
		}
		step := recipe.Steps[index]
		if len(step) == 0 || step[0] > MulDiv {
			return errors.New("recipe result is not numeric")
		}
		needed[index] = true
		var operands []Value
		if step[0] == Read {
			args, err := readArgs.Unpack(step[1:])
			if err != nil {
				return err
			}
			query := args[0].(*big.Int)
			if !query.IsUint64() || query.Uint64() >= uint64(len(recipe.Queries)) {
				return errors.New("query index exceeds program")
			}
			args, err = queryArgs.Unpack(recipe.Queries[query.Uint64()])
			if err != nil {
				return err
			}
			for _, binding := range decodeBindings(args[2].([]*big.Int)) {
				operands = append(operands, binding.Value)
			}
		} else {
			args := binaryArgs
			if step[0] == MulDiv {
				args = mulDivArgs
			}
			decoded, err := args.Unpack(step[1:])
			if err != nil {
				return err
			}
			for _, raw := range decoded {
				operands = append(operands, decodeValue(raw.(*big.Int)))
			}
		}
		for _, operand := range operands {
			if err := visit(operand, index); err != nil {
				return err
			}
		}
		return nil
	}
	for _, value := range values {
		if err := visit(value, uint(len(recipe.Steps))); err != nil {
			return nil, err
		}
	}
	return needed, nil
}

func EconomicResult(v EconomicValue, results map[uint]*big.Int) (*big.Int, error) {
	resolve := func(value Value) (*big.Int, error) {
		if value.Value == nil {
			return nil, errors.New("missing economic expression")
		}
		if value.Source == Literal {
			return new(big.Int).Set(value.Value), nil
		}
		if value.Source != Result || !value.Value.IsUint64() {
			return nil, errors.New("economic expression must be deterministic numerical result")
		}
		x := results[uint(value.Value.Uint64())]
		if x == nil {
			return nil, errors.New("recipe result was not resolved")
		}
		return new(big.Int).Set(x), nil
	}
	x, err := resolve(v.Expression)
	if err != nil {
		return nil, err
	}
	for _, bound := range v.Upper {
		maximum, e := resolve(bound)
		if e != nil {
			return nil, e
		}
		if x.Cmp(maximum) > 0 {
			return nil, errors.New("exact recipe amount exceeds an upper bound")
		}
	}
	for _, bound := range v.Lower {
		minimum, e := resolve(bound)
		if e != nil {
			return nil, e
		}
		if x.Cmp(minimum) < 0 {
			return nil, errors.New("selected recipe amount violates a lower bound")
		}
	}
	return x, nil
}
