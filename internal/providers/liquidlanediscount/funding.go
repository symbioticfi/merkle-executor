package liquidlanediscount

import (
	"context"
	"math/big"

	"github.com/go-errors/errors"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func decodeValue(word *big.Int) recipes.Value {
	raw := word.FillBytes(make([]byte, 32))
	return recipes.Value{Source: raw[4], Value: new(big.Int).SetBytes(raw[5:])}
}
func same(a, b recipes.Value) bool { return a.Source == b.Source && a.Value.Cmp(b.Value) == 0 }
func operands(step []byte) (recipes.Value, recipes.Value, error) {
	values, err := pairArgs.Unpack(step[1:])
	if err != nil {
		return recipes.Value{}, recipes.Value{}, err
	}
	return decodeValue(values[0].(*big.Int)), decodeValue(values[1].(*big.Int)), nil
}

// fundingLimit replays the unchanged authenticated funding prefix with a capped
// provisional payment. It recognizes only the generic fund pattern
// MIN(SUB(payment,cash),capacity); native venue reads/calls stay in the recipe.
func (p *Automatic) fundingLimit(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, request *request) (*big.Int, error) {
	inspection, err := recipes.Inspect(r)
	if err != nil {
		return nil, err
	}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	inputs := map[uint]string{uint(request.amount.Value.Uint64()): "1", uint(request.paymentInput.Value.Uint64()): maximum.String()}
	snap := recipes.Snapshot{BlockNumber: s.BlockNumber.Uint64(), BlockHash: s.BlockHash, Timestamp: s.Timestamp, Executor: s.Executor, Account: s.Account, Caller: s.Caller, RecipeHash: s.RecipeHash, Root: s.Root}
	resolve := func(values []recipes.Value) (map[uint]*big.Int, error) {
		return recipes.ResolveValuesWithRuntime(ctx, p.deps.Chain, snap, r, values, recipes.PrefixRuntime{Tree: t, Inputs: inputs})
	}
	firstEffect := request.action.Step - 4
	for _, slot := range inspection.Slots {
		if slot.StepIndex < firstEffect {
			firstEffect = slot.StepIndex
		}
	}
	// Explicit direct input ceilings must be known before funding starts. A cap
	// derived after a funding effect would need its own committed selection policy.
	request.amountLimit = new(big.Int).Set(maximum)
	for i := uint(0); i < request.action.Step; i++ {
		if r.Steps[i][0] != recipes.LessOrEqual {
			continue
		}
		left, right, err := operands(r.Steps[i])
		if err != nil {
			return nil, err
		}
		payment, amount := same(left, request.paymentInput), same(left, request.amount)
		if !payment && !amount {
			continue
		}
		var cap *big.Int
		switch right.Source {
		case recipes.Literal:
			cap = right.Value
		case recipes.Result:
			if right.Value.Uint64() >= uint64(firstEffect) {
				return nil, errors.New("LL input ceiling must precede funding effects")
			}
			values, err := resolve([]recipes.Value{right})
			if err != nil {
				return nil, err
			}
			cap = values[uint(right.Value.Uint64())]
		default:
			return nil, errors.New("LL input ceiling cannot depend on another external input")
		}
		if payment && cap.Cmp(maximum) < 0 {
			maximum.Set(cap)
			inputs[uint(request.paymentInput.Value.Uint64())] = maximum.String()
		}
		if amount && cap.Cmp(request.amountLimit) < 0 {
			request.amountLimit.Set(cap)
		}
	}
	if request.amountLimit.Sign() == 0 {
		return nil, provider.ErrNotReady
	}
	var budgetValues []recipes.Value
	for _, slot := range inspection.Slots {
		if slot.StepIndex >= request.action.Step-4 || slot.Operation != recipes.Manage {
			continue
		}
		if slot.Gate.Source != recipes.Result {
			return nil, errors.New("LL funding must use a committed capped source amount")
		}
		index := uint(slot.Gate.Value.Uint64())
		if index >= slot.StepIndex || r.Steps[index][0] != recipes.Min {
			return nil, errors.New("LL funding amount must be a committed MIN")
		}
		a, b, err := operands(r.Steps[index])
		if err != nil {
			return nil, err
		}
		var cash, capacity recipes.Value
		matched := false
		for _, pair := range [][2]recipes.Value{{a, b}, {b, a}} {
			deficit := pair[0]
			if deficit.Source != recipes.Result || deficit.Value.Uint64() >= uint64(index) {
				continue
			}
			step := r.Steps[deficit.Value.Uint64()]
			if step[0] != recipes.SaturatingSub {
				continue
			}
			payment, balance, err := operands(step)
			if err != nil {
				return nil, err
			}
			if same(payment, request.paymentInput) {
				cash, capacity, matched = balance, pair[1], true
				break
			}
		}
		if !matched || cash.Source != recipes.Result || recipes.ReadDependent(r, cash) != nil {
			return nil, errors.New("LL funding lacks committed payment shortfall and cash READ")
		}
		if capacity.Source == recipes.Input {
			return nil, errors.New("LL funding capacity cannot be another external input")
		}
		budgetValues = []recipes.Value{cash, capacity}
	}
	if len(budgetValues) == 0 {
		raw, err := protocolio.CallData(ctx, p.deps.Chain, s, request.payment, erc20.NewERC20().PackBalanceOf(s.Account))
		if err != nil || len(raw) != 32 {
			return nil, errors.New("LL funding cash requires canonical pinned balanceOf")
		}
		cash := new(big.Int).SetBytes(raw)
		if cash.Cmp(maximum) < 0 {
			maximum.Set(cash)
		}
		return maximum, nil
	}
	values, err := resolve(budgetValues)
	if err != nil {
		return nil, errors.Errorf("LL funding prefix: %w", err)
	}
	budget := new(big.Int)
	for _, value := range budgetValues {
		if value.Source == recipes.Literal {
			budget.Add(budget, value.Value)
		} else {
			budget.Add(budget, values[uint(value.Value.Uint64())])
		}
	}
	if budget.Cmp(maximum) > 0 {
		budget.Set(maximum)
	}
	return budget, nil
}
