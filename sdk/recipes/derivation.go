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

func CloneRecipe(r Recipe) Recipe { return cloneRecipe(r) }
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

// Budget requires the recipe to select an exact read-derived amount. Bounds
// validate that amount; they never choose it. Use MIN/MAX arithmetic in the
// recipe to express a capped amount, bound directly or through an input equality.
func Budget(recipe Recipe, value Value, before uint) (EconomicValue, error) {
	economic, err := ExactValue(recipe, value, before)
	if err != nil {
		return EconomicValue{}, err
	}
	if err = ReadDependent(recipe, economic.Expression); err != nil {
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
	if _, _, err = dependencies(recipe, EconomicExpressions(economic)); err != nil {
		return EconomicValue{}, err
	}
	return economic, nil
}

// EconomicExpressions includes every bound whose value must be proved before
// requesting external data. Nonfinancial deterministic results need no READ.
func EconomicExpressions(value EconomicValue) []Value {
	values := []Value{value.Expression}
	values = append(values, value.Upper...)
	return append(values, value.Lower...)
}

// Floor combines all preceding lower bounds, including literal caps, provided
// at least one lower bound depends on a recipe read. Equality remains exact.
func Floor(recipe Recipe, value Value, before uint) ([]EconomicValue, error) {
	if value.Source == Result {
		if err := ReadDependent(recipe, value); err != nil {
			return nil, err
		}
		return []EconomicValue{{Expression: value, Exact: true}}, nil
	}
	if value.Source != Input || value.Value == nil || !value.Value.IsUint64() {
		return nil, errors.New("minimum proceeds must depend on recipe results")
	}
	index := uint(value.Value.Uint64())
	var floors []EconomicValue
	read := false
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
		_, hasRead, err := dependencies(recipe, []Value{a})
		if err != nil {
			return nil, err
		}
		read = read || hasRead
		floors = append(floors, EconomicValue{Expression: a, Input: &index, Exact: exact})
	}
	if len(floors) == 0 || !read {
		return nil, errors.New("minimum proceeds input lacks a recipe read-dependent lower bound")
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

// ExactValue resolves deterministic nonfinancial values, including literal
// and constant-arithmetic results. Inputs require an exact preceding equality;
// a ceiling never selects a request ID, epoch or other identity index.
func ExactValue(recipe Recipe, value Value, before uint) (EconomicValue, error) {
	economic, err := exactSource(recipe, value, before)
	if err != nil {
		return EconomicValue{}, err
	}
	if _, _, err = dependencies(recipe, []Value{economic.Expression}); err != nil {
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
		return EconomicValue{}, errors.New("nonfinancial value is not deterministic")
	}
	if expression.Source == Result && (expression.Value == nil || !expression.Value.IsUint64() || expression.Value.Uint64() >= uint64(before)) {
		return EconomicValue{}, errors.New("exact result must precede use")
	}
	return EconomicValue{Expression: expression, Input: index, Exact: true}, nil
}

func sameValue(a, b Value) bool {
	return a.Source == b.Source && a.Value != nil && b.Value != nil && a.Value.Cmp(b.Value) == 0
}

// ReadDependent also rejects unknown inputs in the expression's read bindings.
func ReadDependent(recipe Recipe, value Value) error {
	_, read, err := dependencies(recipe, []Value{value})
	if err != nil {
		return err
	}
	if !read {
		return errors.New("economic expression has no recipe read dependency")
	}
	return nil
}

func dependencies(recipe Recipe, values []Value) (map[uint]bool, bool, error) {
	needed := map[uint]bool{}
	memo := map[uint]bool{}
	inputMemo := map[[2]uint]bool{}
	var visit func(Value, uint) (bool, error)
	visit = func(v Value, before uint) (bool, error) {
		if v.Value == nil {
			return false, errors.New("missing recipe operand")
		}
		if v.Source == Literal {
			return false, nil
		}
		if v.Source == Input {
			if !v.Value.IsUint64() {
				return false, errors.New("invalid recipe input index")
			}
			key := [2]uint{uint(v.Value.Uint64()), before}
			if read, ok := inputMemo[key]; ok {
				return read, nil
			}
			economic, err := exactSource(recipe, v, before)
			if err != nil {
				return false, errors.New("recipe expression depends on an input without a preceding deterministic result source")
			}
			read, err := visit(economic.Expression, before)
			if err != nil {
				return false, err
			}
			inputMemo[key] = read
			return read, nil
		}
		if v.Source != Result || !v.Value.IsUint64() || v.Value.Uint64() >= uint64(len(recipe.Steps)) || v.Value.Uint64() >= uint64(before) {
			return false, errors.New("recipe result is not an earlier numerical step")
		}
		index := uint(v.Value.Uint64())
		if read, ok := memo[index]; ok {
			return read, nil
		}
		step := recipe.Steps[index]
		if len(step) == 0 || step[0] > MulDiv {
			return false, errors.New("recipe result is not numeric")
		}
		needed[index] = true
		var operands []Value
		read := step[0] == Read
		if read {
			args, err := readArgs.Unpack(step[1:])
			if err != nil {
				return false, err
			}
			query := args[0].(*big.Int)
			if !query.IsUint64() || query.Uint64() >= uint64(len(recipe.Queries)) {
				return false, errors.New("query index exceeds program")
			}
			args, err = queryArgs.Unpack(recipe.Queries[query.Uint64()])
			if err != nil {
				return false, err
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
				return false, err
			}
			for _, raw := range decoded {
				operands = append(operands, decodeValue(raw.(*big.Int)))
			}
		}
		for _, operand := range operands {
			hasRead, err := visit(operand, index)
			if err != nil {
				return false, err
			}
			read = read || hasRead
		}
		memo[index] = read
		return read, nil
	}
	read := false
	for _, value := range values {
		hasRead, err := visit(value, uint(len(recipe.Steps)))
		if err != nil {
			return nil, false, err
		}
		read = read || hasRead
	}
	return needed, read, nil
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

// ValidateMutableActions rejects artifact preimages that are not committed by
// masks or explicit numerical bindings. Protocol providers validate their owned
// action's mutable wire fields in RecipeRequirements before external I/O.
func ValidateMutableActions(recipe Recipe, tree *Tree, owned map[string]bool) error {
	actions, err := ManagedActions(recipe, tree)
	if err != nil {
		return err
	}
	for _, action := range actions {
		if owned[action.Key] {
			continue
		}
		for at, mask := range action.Leaf.DataMask {
			if mask == 0 {
				continue
			}
			covered := false
			for _, binding := range action.Bindings {
				if at >= int(binding.Offset) && at < int(binding.Offset)+32 {
					covered = true
					break
				}
			}
			if !covered {
				return errors.New("unowned managed action has mutable bytes without committed bindings")
			}
		}
	}
	return nil
}
