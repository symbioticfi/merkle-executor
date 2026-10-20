package recipes

import (
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
)

// PrefixRuntime contains only authenticated outputs already prepared before the
// shared lens. AvailableInputs permits pure preflight before external fetches;
// Resolve requires each such input to actually exist. BlockedActions prevents
// executing unresolved quote templates as if their placeholder bytes were data.
type PrefixRuntime struct {
	Tree            *Tree
	Inputs          map[uint]string
	Actions         map[string]hexutil.Bytes
	SkipSteps       []uint
	AvailableInputs map[uint]bool
	BlockedActions  map[string]bool
}

type prefixProgram struct {
	needed  map[uint]bool
	anchors map[uint]Value
	runtime [][]byte
	size    uint
}

// ValidatePrefix performs source/dependency validation without network I/O.
func ValidatePrefix(recipe Recipe, expressions []Value, runtime PrefixRuntime) error {
	_, err := buildPrefix(recipe, expressions, runtime)
	return err
}

func buildPrefix(recipe Recipe, expressions []Value, runtime PrefixRuntime) (prefixProgram, error) {
	p := prefixProgram{anchors: map[uint]Value{}}
	needed, _, err := dependencies(recipe, expressions)
	if err != nil {
		return p, err
	}
	p.needed = needed
	inspection, err := Inspect(recipe)
	if err != nil {
		return p, err
	}
	if len(recipe.Steps) > 4096 || len(recipe.Queries) > 4096 {
		return p, errors.New("recipe prefix exceeds resource bounds")
	}
	if len(inspection.Inputs) > 0 {
		p.size = inspection.Inputs[len(inspection.Inputs)-1] + 1
	}
	if len(needed) == 0 {
		return p, nil
	}
	end := uint(0)
	for index := range needed {
		if index+1 > end {
			end = index + 1
		}
	}
	add := func(v Value, before uint) error {
		if v.Source == Input {
			index := uint(v.Value.Uint64())
			if _, ok := runtime.Inputs[index]; ok || runtime.AvailableInputs[index] {
				return nil
			}
			economic, e := ExactValue(recipe, v, uint(len(recipe.Steps)))
			if e != nil {
				return errors.Errorf("unsupported prefix: input %d has no known authorized value", index)
			}
			for _, expression := range EconomicExpressions(economic) {
				if expression.Source == Result && expression.Value.Uint64() >= uint64(before) {
					return errors.New("unsupported prefix: input source is a future result")
				}
				dependencies, _, e := dependencies(recipe, []Value{expression})
				if e != nil {
					return e
				}
				for i := range dependencies {
					p.needed[i] = true
				}
			}
			p.anchors[index] = economic.Expression
			return nil
		}
		dependencies, _, e := dependencies(recipe, []Value{v})
		if e != nil {
			return e
		}
		for i := range dependencies {
			if i >= before {
				return errors.New("prefix reference is a future result")
			}
			p.needed[i] = true
		}
		return nil
	}
	// Every effect executes in order, including its exact gate and binding values.
	// Comparisons before those reads execute as well, and cannot use dummy inputs.
	for i := uint(0); i < end; i++ {
		op := recipe.Steps[i][0]
		var operands []Value
		switch op {
		case Equal, LessOrEqual:
			args, e := binaryArgs.Unpack(recipe.Steps[i][1:])
			if e != nil {
				return p, e
			}
			operands = []Value{decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))}
		case Call:
			args, e := callArgs.Unpack(recipe.Steps[i][1:])
			if e != nil {
				return p, e
			}
			for _, b := range decodeBindings(args[2].([]*big.Int)) {
				operands = append(operands, b.Value)
			}
		}
		for _, operand := range operands {
			if err = add(operand, i); err != nil {
				return p, err
			}
		}
	}
	p.runtime = make([][]byte, len(inspection.Slots))
	used := map[string]bool{}
	skipped := map[uint]bool{}
	for _, i := range runtime.SkipSteps {
		skipped[i] = true
	}
	for _, slot := range inspection.Slots {
		if slot.StepIndex >= end {
			continue
		}
		if runtime.Tree == nil {
			return p, errors.New("stateful prefix requires authenticated permission tree")
		}
		if skipped[slot.StepIndex] {
			p.runtime[slot.Index] = []byte{}
			if err = add(slot.Gate, slot.StepIndex); err != nil {
				return p, err
			}
			continue
		}
		if slot.Operation == Manage {
			for i := range slot.Leaves {
				key := strconv.FormatUint(uint64(slot.StepIndex), 10) + ":" + strconv.Itoa(i)
				if runtime.BlockedActions[key] {
					return p, errors.New("unsupported prefix: earlier action depends on an unresolved provider result")
				}
				for _, b := range slot.Bindings[i] {
					if err = add(b.Value, slot.StepIndex); err != nil {
						return p, err
					}
				}
			}
			if err = add(slot.Gate, slot.StepIndex); err != nil {
				return p, err
			}
			p.runtime[slot.Index], err = assembleManage(slot, runtime.Tree, runtime.Actions, used)
		} else {
			if err = add(slot.Amount, slot.StepIndex); err != nil {
				return p, err
			}
			p.runtime[slot.Index], err = assembleApproval(slot, runtime.Tree)
		}
		if err != nil {
			return p, err
		}
	}
	// Numeric dependencies may contain anchored Inputs inside arithmetic/queries.
	visited := map[uint]bool{}
	for len(visited) < len(p.needed) {
		index := uint(0)
		found := false
		for candidate := uint(0); candidate < uint(len(recipe.Steps)); candidate++ {
			if p.needed[candidate] && !visited[candidate] {
				index = candidate
				found = true
				break
			}
		}
		if !found {
			return p, errors.New("invalid numeric dependency closure")
		}
		visited[index] = true
		step := recipe.Steps[index]
		var operands []Value
		if step[0] == Read {
			args, e := readArgs.Unpack(step[1:])
			if e != nil {
				return p, e
			}
			q, e := queryArgs.Unpack(recipe.Queries[args[0].(*big.Int).Uint64()])
			if e != nil {
				return p, e
			}
			for _, b := range decodeBindings(q[2].([]*big.Int)) {
				operands = append(operands, b.Value)
			}
		} else {
			args := binaryArgs
			if step[0] == MulDiv {
				args = mulDivArgs
			}
			decoded, e := args.Unpack(step[1:])
			if e != nil {
				return p, e
			}
			for _, v := range decoded {
				operands = append(operands, decodeValue(v.(*big.Int)))
			}
		}
		for _, v := range operands {
			if v.Source == Input {
				if err = add(v, index); err != nil {
					return p, err
				}
			}
		}
	}
	return p, nil
}

func prefixInputs(p prefixProgram, inputs map[uint]string) ([]*big.Int, []bool, [][]byte, error) {
	values := make([]*big.Int, p.size)
	known := make([]bool, p.size)
	anchors := make([][]byte, p.size)
	for i := range values {
		values[i] = new(big.Int)
		anchors[i] = []byte{}
	}
	for index, text := range inputs {
		if index >= p.size {
			return nil, nil, nil, errors.New("prefix input exceeds recipe")
		}
		value, err := parse.Uint256(text)
		if err != nil {
			return nil, nil, nil, err
		}
		values[index] = value
		known[index] = true
	}
	for index, expression := range p.anchors {
		raw, err := lensAnchorArgs.Pack(encodeValue(expression))
		if err != nil {
			return nil, nil, nil, err
		}
		anchors[index] = raw
	}
	return values, known, anchors, nil
}

func encodeValue(value Value) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(value.Source)), 216), value.Value)
}
