// Package aave prepares capacity inputs for authenticated Aave V3 borrowing.
package aave

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const policy = "aave-v3/native-capacity-v1"

//go:embed lens/AaveBorrowProbe.json
var artifactJSON []byte

type Provider struct{ reader provider.Reader }
type borrow struct {
	Pool, Asset  common.Address
	Floor        *big.Int
	Before, Step uint
}
type saved struct {
	Input uint `json:"input"`
}

func New(reader provider.Reader) (*Provider, error) {
	if reader == nil {
		return nil, errors.New("Aave capacity requires chain reader")
	}
	return &Provider{reader: reader}, nil
}
func (*Provider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("Aave capacity requires authenticated recipe")
}
func (p *Provider) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	b, err := derive(s, r, t)
	if err != nil {
		return keeper.RecipeRequirements{}, err
	}
	if b == nil {
		return keeper.RecipeRequirements{}, nil
	}
	return keeper.RecipeRequirements{Inputs: []uint{0}}, nil
}
func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, _ map[uint]*big.Int) (provider.Output, error) {
	if err := ctx.Err(); err != nil {
		return provider.Output{}, err
	}
	b, err := derive(s, r, t)
	if err != nil {
		return provider.Output{}, err
	}
	if b == nil {
		return derived.Empty(policy)
	}
	if s.BlockNumber == nil || !s.BlockNumber.IsUint64() {
		return provider.Output{}, errors.New("Aave capacity requires pinned block")
	}
	var artifact struct{ Runtime hexutil.Bytes }
	if json.Unmarshal(artifactJSON, &artifact) != nil || len(artifact.Runtime) == 0 {
		return provider.Output{}, errors.New("invalid Aave capacity artifact")
	}
	args, err := capacityArgs.Pack(b.Pool, b.Asset, s.Account, b.Floor)
	if err != nil {
		return provider.Output{}, err
	}
	query := append(selector("maxBorrow(address,address,address,uint256)"), args...)
	raw, witness, err := recipes.ProbePrefix(ctx, p.reader, recipes.Snapshot{BlockNumber: s.BlockNumber.Uint64(), Executor: s.Executor, Account: s.Account, Caller: s.Caller}, r, b.Before, recipes.PrefixRuntime{Tree: t}, artifact.Runtime, query)
	if err != nil {
		return provider.Output{}, err
	}
	values, err := uintArgs.Unpack(raw)
	if err != nil || len(values) != 1 {
		return provider.Output{}, errors.New("invalid Aave capacity reply")
	}
	out := provider.Output{Inputs: map[uint]string{0: values[0].(*big.Int).String()}, Results: recipes.ResultsWitness(witness)}
	return derived.Encode(policy, saved{Input: 0}, out, true)
}
func (p *Provider) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if err := ctx.Err(); err != nil {
		return provider.Progress{}, err
	}
	var cfg saved
	inner, matched, err := derived.Decode(policy, out, &cfg)
	if err != nil {
		return provider.Progress{}, err
	}
	if matched && (cfg.Input != 0 || len(inner.Inputs) != 1 || inner.Inputs[0] == "" || len(inner.Actions) != 0 || len(inner.SkipSteps) != 0) {
		return provider.Progress{}, errors.New("persisted Aave capacity output differs")
	}
	if err := s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	return provider.Progress{Done: true}, nil
}

func derive(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (*borrow, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var found *borrow
	var allowed map[uint]bool
	for _, action := range actions {
		candidate := false
		for _, binding := range action.Bindings {
			has, e := capacityChain(r, binding.Value, map[uint]bool{})
			if e != nil {
				return nil, e
			}
			candidate = candidate || has
		}
		if !candidate {
			continue
		}
		b, amount, matched, err := guardedBorrow(s, action)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		if amount.Source != recipes.Result {
			return nil, errors.New("Aave borrow amount must be a capacity-capped result")
		}
		nodes := map[uint]bool{}
		has, err := capacityChain(r, amount, nodes)
		if err != nil {
			return nil, err
		}
		if !has {
			continue
		}
		if found != nil {
			return nil, errors.New("multiple Aave capacity groups are unsupported")
		}
		found, allowed = b, nodes
	}
	if found == nil {
		return nil, nil
	}
	// Only the single direct Min operand may consume the capacity input. All
	// other Inputs retain the generic exact-source requirement.
	before := uint(len(r.Steps))
	uses := 0
	for i, step := range r.Steps {
		operands, err := stepOperands(r, uint(i))
		if err != nil {
			return nil, err
		}
		for _, v := range operands {
			if v.Source == recipes.Input && v.Value.Sign() == 0 {
				if step[0] != recipes.Min || !allowed[uint(i)] {
					return nil, errors.New("Aave capacity input must only feed its borrow Min chain")
				}
				uses++
				if uint(i) < before {
					before = uint(i)
				}
			}
		}
	}
	if uses != 1 {
		return nil, errors.New("Aave capacity input must have one unambiguous Min use")
	}
	found.Before = before
	for i := before; i < found.Step; i++ {
		if op := r.Steps[i][0]; op == recipes.Manage || op == recipes.Approve || op == recipes.Call {
			return nil, errors.New("Aave capacity does not support effects between sizing and borrow")
		}
	}
	if err := recipes.ValidateProbePrefix(r, before, recipes.PrefixRuntime{Tree: t}); err != nil {
		return nil, err
	}
	return found, nil
}

func capacityChain(r recipes.Recipe, v recipes.Value, nodes map[uint]bool) (bool, error) {
	if v.Source == recipes.Input {
		if v.Value.Sign() != 0 {
			return false, nil
		}
		return true, nil
	}
	if v.Source != recipes.Result {
		return false, nil
	}
	i := uint(v.Value.Uint64())
	if i >= uint(len(r.Steps)) {
		return false, errors.New("invalid Aave amount result")
	}
	if r.Steps[i][0] != recipes.Min {
		return false, nil
	}
	args, err := binaryArgs.Unpack(r.Steps[i][1:])
	if err != nil {
		return false, err
	}
	a, err := capacityChain(r, operand(args[0].(*big.Int)), nodes)
	if err != nil {
		return false, err
	}
	b, err := capacityChain(r, operand(args[1].(*big.Int)), nodes)
	if err != nil {
		return false, err
	}
	if a || b {
		nodes[i] = true
	}
	return a || b, nil
}

func guardedBorrow(s provider.Snapshot, a recipes.ManagedAction) (*borrow, recipes.Value, bool, error) {
	d := a.Leaf.Data
	if a.Leaf.Target != s.Account || len(d) < 4 || !bytes.Equal(d[:4], selector("execute(bytes[],bytes[],uint256[])")) {
		return nil, recipes.Value{}, false, nil
	}
	fail := func() (*borrow, recipes.Value, bool, error) {
		return nil, recipes.Value{}, true, errors.New("Aave guarded borrow has unsupported or unpinned shape")
	}
	decoded, err := executeArgs.Unpack(d[4:])
	if err != nil {
		return fail()
	}
	queries, steps, inputs := decoded[0].([][]byte), decoded[1].([][]byte), decoded[2].([]*big.Int)
	// Other account capabilities belong to their existing providers.
	if len(steps) == 0 || len(steps[0]) == 0 || steps[0][0] != recipes.Call {
		return nil, recipes.Value{}, false, nil
	}
	call, err := callArgs.Unpack(steps[0][1:])
	if err != nil {
		return fail()
	}
	data := call[1].([]byte)
	if len(data) < 4 || !bytes.Equal(data[:4], selector("borrow(address,uint256,uint256,uint16,address)")) {
		return nil, recipes.Value{}, false, nil
	}
	canonical, err := executeArgs.Pack(decoded...)
	if err != nil || !bytes.Equal(canonical, d[4:]) {
		return fail()
	}

	if len(inputs) != 0 || len(queries) != 1 || len(steps) != 3 || len(call[2].([]*big.Int)) != 0 || len(data) != 164 || len(steps[1]) == 0 || len(steps[2]) == 0 || steps[1][0] != recipes.Read || steps[2][0] != recipes.LessOrEqual || a.Leaf.Value == nil || a.Leaf.Value.Sign() != 0 {
		return fail()
	}
	args, err := borrowArgs.Unpack(data[4:])
	if err != nil {
		return fail()
	}
	if args[0].(common.Address) == (common.Address{}) || args[2].(*big.Int).Cmp(big.NewInt(2)) != 0 || args[3].(uint16) != 0 || args[4].(common.Address) != s.Account {
		return fail()
	}
	q, err := queryArgs.Unpack(queries[0])
	if err != nil {
		return fail()
	}
	health := q[1].([]byte)
	pool := call[0].(common.Address)
	if pool == (common.Address{}) || q[0].(common.Address) == (common.Address{}) || len(q[2].([]*big.Int)) != 0 || q[3].(*big.Int).Sign() != 0 || len(health) != 68 || !bytes.Equal(health[:4], selector("getHealthFactor(address,address)")) {
		return fail()
	}
	expected, _ := healthArgs.Pack(pool, s.Account)
	if !bytes.Equal(health[4:], expected) {
		return fail()
	}
	read, err := uintArgs.Unpack(steps[1][1:])
	if err != nil || read[0].(*big.Int).Sign() != 0 {
		return fail()
	}
	check, err := binaryArgs.Unpack(steps[2][1:])
	if err != nil {
		return fail()
	}
	floor, result := operand(check[0].(*big.Int)), operand(check[1].(*big.Int))
	if floor.Source != recipes.Literal || floor.Value.Cmp(big.NewInt(1_000_000_000_000_000_000)) < 0 || result.Source != recipes.Result || result.Value.Cmp(big.NewInt(1)) != 0 {
		return fail()
	}
	if bytes.Count(d, data) != 1 {
		return fail()
	}
	offset := bytes.Index(d, data) + 36
	if len(a.Slot.Leaves) != 1 || len(a.Bindings) != 1 || int(a.Bindings[0].Offset) != offset || a.Pinned(0, offset) != nil || a.Pinned(offset+32, len(d)-offset-32) != nil {
		return fail()
	}
	return &borrow{Pool: pool, Asset: args[0].(common.Address), Floor: floor.Value, Step: a.Step}, a.Bindings[0].Value, true, nil
}

func stepOperands(r recipes.Recipe, index uint) ([]recipes.Value, error) {
	step := r.Steps[index]
	var raw []*big.Int
	switch step[0] {
	case recipes.Read:
		q, err := uintArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		args, err := queryArgs.Unpack(r.Queries[q[0].(*big.Int).Uint64()])
		if err != nil {
			return nil, err
		}
		raw = args[2].([]*big.Int)
	case recipes.Call:
		args, err := callArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		raw = args[2].([]*big.Int)
	case recipes.Manage:
		args, err := manageArgs.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		raw = append(raw, args[0].(*big.Int))
		for _, bindings := range args[2].([][]*big.Int) {
			raw = append(raw, bindings...)
		}
	case recipes.Approve:
		raw = []*big.Int{new(big.Int).SetBytes(step[1:33])}
	default:
		args := binaryArgs
		if step[0] == recipes.MulDiv {
			args = mulDivArgs
		}
		values, err := args.Unpack(step[1:])
		if err != nil {
			return nil, err
		}
		for _, v := range values {
			raw = append(raw, v.(*big.Int))
		}
	}
	values := make([]recipes.Value, len(raw))
	for i, v := range raw {
		values[i] = operand(v)
	}
	return values, nil
}
func operand(v *big.Int) recipes.Value {
	var word [32]byte
	v.FillBytes(word[:])
	return recipes.Value{Source: word[4], Value: new(big.Int).SetBytes(word[5:])}
}
func selector(s string) []byte { return crypto.Keccak256([]byte(s))[:4] }
func argTypes(names ...string) abi.Arguments {
	args := abi.Arguments{}
	for _, name := range names {
		t, err := abi.NewType(name, "", nil)
		if err != nil {
			panic(err)
		}
		args = append(args, abi.Argument{Type: t})
	}
	return args
}

var (
	executeArgs  = argTypes("bytes[]", "bytes[]", "uint256[]")
	callArgs     = argTypes("address", "bytes", "uint256[]")
	queryArgs    = argTypes("address", "bytes", "uint256[]", "uint256")
	borrowArgs   = argTypes("address", "uint256", "uint256", "uint16", "address")
	healthArgs   = argTypes("address", "address")
	capacityArgs = argTypes("address", "address", "address", "uint256")
	uintArgs     = argTypes("uint256")
	binaryArgs   = argTypes("uint256", "uint256")
	mulDivArgs   = argTypes("uint256", "uint256", "uint256")
	manageArgs   = argTypes("uint256", "bytes32[]", "uint256[][]")
)
