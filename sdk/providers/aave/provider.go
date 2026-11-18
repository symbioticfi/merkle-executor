// Package aave prepares capacity inputs for authenticated Aave V3 borrowing.
package aave

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

//go:embed lens/AaveBorrowProbe.json
var artifactJSON []byte

type Provider struct{ reader provider.Reader }
type borrow struct {
	Pool, Asset  common.Address
	Floor        *big.Int
	Before, Step uint
}

func New(reader provider.Reader) (*Provider, error) {
	if reader == nil {
		return nil, errors.New("Aave capacity requires chain reader")
	}
	return &Provider{reader: reader}, nil
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
func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, _ map[uint]*big.Int) ([]provider.Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := derive(s, r, t)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return []provider.Output{{}}, nil
	}
	if s.BlockNumber == nil || !s.BlockNumber.IsUint64() {
		return nil, errors.New("Aave capacity requires pinned block")
	}
	var artifact struct{ Runtime hexutil.Bytes }
	if json.Unmarshal(artifactJSON, &artifact) != nil || len(artifact.Runtime) == 0 {
		return nil, errors.New("invalid Aave capacity artifact")
	}
	args, err := capacityArgs.Pack(b.Pool, b.Asset, s.Account, b.Floor)
	if err != nil {
		return nil, err
	}
	query := append(selector("maxBorrow(address,address,address,uint256)"), args...)
	raw, witness, err := recipes.ProbePrefix(ctx, p.reader, recipes.Snapshot{BlockNumber: s.BlockNumber.Uint64(), Executor: s.Executor, Account: s.Account, Caller: s.Caller, Sender: s.Sender}, r, b.Before, recipes.PrefixRuntime{Tree: t}, artifact.Runtime, query)
	if err != nil {
		return nil, err
	}
	values, err := uintArgs.Unpack(raw)
	if err != nil || len(values) != 1 {
		return nil, errors.New("invalid Aave capacity reply")
	}
	out := provider.Output{Inputs: map[uint]string{0: values[0].(*big.Int).String()}, Results: recipes.ResultsWitness(witness)}
	return []provider.Output{out}, nil
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
		b, amount, matched, err := nativeBorrow(s, r, action)
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
		operands, err := recipes.Operands(r, uint(i))
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
	operands, err := recipes.Operands(r, i)
	if err != nil {
		return false, err
	}
	a, err := capacityChain(r, operands[0], nodes)
	if err != nil {
		return false, err
	}
	b, err := capacityChain(r, operands[1], nodes)
	if err != nil {
		return false, err
	}
	if a || b {
		nodes[i] = true
	}
	return a || b, nil
}

// Native permissions pin the pool and every argument except the amount. The
// outer recipe must check this account's native health immediately after borrowing.
func nativeBorrow(s provider.Snapshot, r recipes.Recipe, a recipes.ManagedAction) (*borrow, recipes.Value, bool, error) {
	d := a.Leaf.Data
	if len(d) < 4 || !bytes.Equal(d[:4], selector("borrow(address,uint256,uint256,uint16,address)")) {
		return nil, recipes.Value{}, false, nil
	}
	fail := func() (*borrow, recipes.Value, bool, error) {
		return nil, recipes.Value{}, true, errors.New("Aave native borrow requires pinned identity and post-borrow health floor")
	}
	if a.Leaf.Target == (common.Address{}) || a.Leaf.Value == nil || a.Leaf.Value.Sign() != 0 || len(d) != 164 || len(a.Slot.Leaves) != 1 || len(a.Bindings) != 1 || a.Bindings[0].Offset != 36 || a.Pinned(0, 36) != nil || a.Pinned(68, len(d)-68) != nil {
		return fail()
	}
	args, err := borrowArgs.Unpack(d[4:])
	if err != nil || args[0].(common.Address) == (common.Address{}) || args[2].(*big.Int).Cmp(big.NewInt(2)) != 0 || args[3].(uint16) != 0 || args[4].(common.Address) != s.Account {
		return fail()
	}
	canonical, err := borrowArgs.Pack(args...)
	if err != nil || !bytes.Equal(canonical, d[4:]) {
		return fail()
	}
	account, _ := accountArgs.Pack(s.Account)
	expected := append(selector("getUserAccountData(address)"), account...)
	healthReads := map[uint]bool{}
	var floor *big.Int
	for i := a.Step + 1; i < uint(len(r.Steps)); i++ {
		step := r.Steps[i]
		if step[0] == recipes.Manage || step[0] == recipes.Call || step[0] == recipes.Approve {
			break
		}
		if step[0] == recipes.Read {
			read, e := uintArgs.Unpack(step[1:])
			if e != nil {
				return fail()
			}
			q, e := queryArgs.Unpack(r.Queries[read[0].(*big.Int).Uint64()])
			if e != nil {
				return fail()
			}
			if q[0].(common.Address) == a.Leaf.Target && bytes.Equal(q[1].([]byte), expected) && len(q[2].([]*big.Int)) == 0 && q[3].(*big.Int).Cmp(big.NewInt(160)) == 0 {
				healthReads[i] = true
			}
		}
		if step[0] == recipes.LessOrEqual {
			check, e := recipes.Operands(r, i)
			if e != nil {
				return fail()
			}
			left, right := check[0], check[1]
			if left.Source == recipes.Literal && left.Value.Cmp(big.NewInt(1_000_000_000_000_000_000)) >= 0 && right.Source == recipes.Result && healthReads[uint(right.Value.Uint64())] && (floor == nil || left.Value.Cmp(floor) > 0) {
				floor = left.Value
			}
		}
	}
	if floor == nil {
		return fail()
	}
	return &borrow{Pool: a.Leaf.Target, Asset: args[0].(common.Address), Floor: floor, Step: a.Step}, a.Bindings[0].Value, true, nil
}

func selector(s string) []byte { return crypto.Keccak256([]byte(s))[:4] }

var (
	queryArgs    = recipes.Arguments("address", "bytes", "uint256[]", "uint256")
	borrowArgs   = recipes.Arguments("address", "uint256", "uint256", "uint16", "address")
	accountArgs  = recipes.Arguments("address")
	capacityArgs = recipes.Arguments("address", "address", "address", "uint256")
	uintArgs     = recipes.Arguments("uint256")
)
