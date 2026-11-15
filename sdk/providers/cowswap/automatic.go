package cowswap

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Automatic struct{ deps provider.Deps }
type recipeOrder struct {
	template     derived.SignTemplate
	cfg          Config
	budget       recipes.EconomicValue
	floors       []recipes.EconomicValue
	requirements keeper.RecipeRequirements
	expiry       []expiryBound
}

func NewAutomatic(deps provider.Deps) (keeper.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("automatic CoW requires chain reader")
	}
	return &Automatic{deps: deps}, nil
}

func (p *Automatic) derive(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (*recipeOrder, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var result *recipeOrder
	typeHash := crypto.Keccak256Hash([]byte("Order(address sellToken,address buyToken,address receiver,uint256 sellAmount,uint256 buyAmount,uint32 validTo,bytes32 appData,uint256 feeAmount,string kind,bool partiallyFillable,string sellTokenBalance,string buyTokenBalance)"))
	for _, action := range actions {
		sign, matched, err := derived.Sign(action, s.Account)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		looksLikeOrder := len(sign.Buffer) >= 32 && common.BytesToHash(sign.Buffer[:32]) == typeHash
		http, settlement, _, defaultsErr := ChainDefaults(s.ChainID)
		if defaultsErr == nil && sign.Domain == protocolDomain(s.ChainID, settlement) {
			looksLikeOrder = true
		}
		if !looksLikeOrder {
			continue
		}
		if defaultsErr != nil {
			return nil, defaultsErr
		}
		if result != nil {
			return nil, errors.New("multiple CoW authorizations require separate executors")
		}
		if len(sign.Buffer) != 416 || len(sign.Plan) != 1 || sign.Plan[0].Offset.Cmp(big.NewInt(0)) != 0 || sign.Plan[0].Length.Cmp(big.NewInt(416)) != 0 || sign.Plan[0].Destination.Sign() != 0 || sign.Domain != protocolDomain(s.ChainID, settlement) {
			return nil, errors.New("CoW recipe has incompatible sign domain or hash plan")
		}
		for _, index := range []int{0, 1, 2, 3, 8, 9, 10, 11, 12} {
			if err := sign.PinnedWord(index); err != nil {
				return nil, err
			}
		}
		sell, err := derived.WordAddress(sign.Buffer[32:64])
		if err != nil {
			return nil, err
		}
		buy, err := derived.WordAddress(sign.Buffer[64:96])
		if err != nil {
			return nil, err
		}
		receiver, err := derived.WordAddress(sign.Buffer[96:128])
		if err != nil || receiver != s.Account {
			return nil, errors.New("CoW recipe recipient must be authenticated account")
		}
		if sell == buy {
			return nil, errors.New("CoW recipe tokens must differ")
		}
		order := wire.CowOrder{SellToken: sell.Hex(), BuyToken: buy.Hex(), Receiver: receiver.Hex(), SellAmount: new(big.Int).SetBytes(sign.Buffer[128:160]).String(), BuyAmount: new(big.Int).SetBytes(sign.Buffer[160:192]).String(), ValidTo: int64(new(big.Int).SetBytes(sign.Buffer[192:224]).Uint64()), AppData: common.BytesToHash(sign.Buffer[224:256]).Hex(), FeeAmount: "0", Kind: "sell", PartiallyFillable: false, SellTokenBalance: "erc20", BuyTokenBalance: "erc20"}
		if new(big.Int).SetBytes(sign.Buffer[192:224]).BitLen() > 32 {
			return nil, errors.New("CoW template expiry exceeds uint32")
		}
		expected, err := EncodeOrder(order)
		if err != nil || !bytes.Equal(expected, sign.Buffer) {
			return nil, errors.New("CoW recipe must pin full-fill ERC20 sell order identity")
		}
		amount, err := sign.Field(4)
		if err != nil {
			return nil, err
		}
		budget, err := recipes.Budget(r, amount, action.Step)
		if err != nil {
			return nil, err
		}
		minimum, err := sign.Field(5)
		if err != nil {
			return nil, err
		}
		floors, err := recipes.Floor(r, minimum, action.Step)
		if err != nil {
			return nil, err
		}
		expiry, err := boundedExpiry(r, sign)
		if err != nil {
			return nil, err
		}
		economics := append([]recipes.EconomicValue{budget}, floors...)
		for _, bound := range expiry {
			economics = append(economics, recipes.EconomicValue{Expression: bound.clock, Exact: true})
		}
		for _, binding := range action.Bindings {
			if binding.Value.Source == recipes.Result {
				economics = append(economics, recipes.EconomicValue{Expression: binding.Value, Exact: true})
			}
		}
		requirements, err := derived.Requirements(sign, economics, map[int]bool{4: true, 5: true, 6: true, 7: true})
		if err != nil {
			return nil, err
		}
		result = &recipeOrder{template: sign, cfg: Config{HTTP: http, Settlement: settlement.Hex(), SellToken: sell.Hex(), BuyToken: buy.Hex(), Action: action.Key, ValiditySeconds: DefaultValiditySeconds, SellAmountInput: budget.Input}, budget: budget, floors: floors, requirements: requirements, expiry: expiry}
		if minimum.Source == recipes.Input {
			index := uint(minimum.Value.Uint64())
			result.cfg.BuyAmountInput = &index
		}
	}
	return result, nil
}

func protocolDomain(chainID *big.Int, settlement common.Address) common.Hash {
	return protocolio.Domain("Gnosis Protocol", "v2", chainID, settlement)
}

func (p *Automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	order, err := p.derive(s, r, t)
	if err != nil || order == nil {
		return keeper.RecipeRequirements{}, err
	}
	return order.requirements, nil
}
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) ([]provider.Output, error) {
	order, err := p.derive(s, r, t)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return []provider.Output{{}}, nil
	}
	amount, err := recipes.EconomicResult(order.budget, results)
	if err != nil {
		return nil, err
	}
	minimum := new(big.Int)
	for _, floor := range order.floors {
		value, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return nil, err
		}
		if value.Cmp(minimum) > 0 {
			minimum = value
		}
	}
	if amount.Sign() == 0 || minimum.Sign() == 0 {
		return nil, provider.ErrNotReady
	}
	validTo := new(big.Int).SetUint64(s.Timestamp + DefaultValiditySeconds)
	for _, bound := range order.expiry {
		clock, err := recipes.EconomicResult(recipes.EconomicValue{Expression: bound.clock}, results)
		if err != nil {
			return nil, err
		}
		limit := new(big.Int).Add(clock, new(big.Int).SetUint64(bound.duration))
		if limit.Cmp(validTo) < 0 {
			validTo = limit
		}
	}
	if !validTo.IsUint64() || validTo.Uint64() <= s.Timestamp {
		return nil, provider.ErrNotReady
	}
	order.cfg.ValiditySeconds = validTo.Uint64() - s.Timestamp
	order.cfg.SellAmount = amount.String()
	order.cfg.MinBuyAmount = minimum.String()
	runtime, err := New(order.cfg, p.deps)
	if err != nil {
		return nil, err
	}
	if order.template.PinnedWord(7) == nil {
		appData := common.BytesToHash(order.template.Buffer[224:256])
		runtime.pinnedAppData = &appData
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return nil, err
	}
	payload := out.Actions[order.cfg.Action]
	for _, floor := range order.floors {
		if !floor.Exact {
			continue
		}
		expected, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return nil, err
		}
		if new(big.Int).SetBytes(payload[order.template.Offset+160:order.template.Offset+192]).Cmp(expected) != 0 {
			return nil, errors.New("CoW dynamic minimum conflicts with exact recipe result binding")
		}
	}
	if err = derived.BindOutput(order.template, payload, results, &out); err != nil {
		return nil, err
	}
	return []provider.Output{out}, nil
}
func (p *Provider) confirmedTask(out provider.Output, saved prepared) provider.Task {
	authorization := bytes.Clone(out.Actions[p.cfg.Action])
	return provider.TaskFunc(func(ctx context.Context, s provider.Snapshot) (provider.Progress, error) {
		if err := protocolio.Execution(ctx, p.deps.Chain, s); err != nil {
			return provider.Progress{}, err
		}
		if s.Execution == nil {
			return provider.Progress{}, errors.New("CoW requires confirmed authorizing execution")
		}
		count := 0
		ab := account.NewMerkleAccount()
		for _, log := range s.Execution.Logs {
			if log.Address != s.Account || len(log.Topics) == 0 || log.Topics[0] != ab.GetABI().Events["Sign"].ID {
				continue
			}
			event, err := ab.UnpackSignEvent(&log)
			if err != nil {
				return provider.Progress{}, errors.New("CoW malformed confirmed Sign event")
			}
			if common.Hash(event.Digest) != saved.Digest {
				continue
			}
			data, err := ab.TryPackSign(event.DomainSeparator, event.Buffer, event.Plan)
			if err != nil || !bytes.Equal(data, authorization) {
				return provider.Progress{}, errors.New("CoW confirmed Sign differs from prepared order")
			}
			count++
		}
		if count != 1 {
			return provider.Progress{}, errors.New("CoW exact confirmed authorizing Sign event is required")
		}
		return p.advance(ctx, s, saved)
	})
}

// expiryBound retains the clock expression from a committed relative lifetime check.
type expiryBound struct {
	clock    recipes.Value
	duration uint64
}

func boundedExpiry(r recipes.Recipe, sign derived.SignTemplate) ([]expiryBound, error) {
	deadline, err := sign.Field(6)
	if err != nil {
		return nil, nil
	}
	decode := func(word []byte) recipes.Value {
		n := new(big.Int).SetBytes(word)
		source := uint8(new(big.Int).Rsh(new(big.Int).Set(n), 216).Uint64())
		n.And(n, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(1)))
		return recipes.Value{Source: source, Value: n}
	}
	var bounds []expiryBound
	for i, step := range r.Steps[:sign.Action.Step] {
		if len(step) != 65 || step[0] != recipes.LessOrEqual {
			continue
		}
		left, limit := decode(step[1:33]), decode(step[33:65])
		if left.Source != recipes.Result || !left.Value.IsUint64() || left.Value.Uint64() >= uint64(i) || limit.Source != recipes.Literal || !limit.Value.IsUint64() || limit.Value.Sign() == 0 {
			continue
		}
		sub := r.Steps[left.Value.Uint64()]
		if len(sub) != 65 || sub[0] != recipes.Sub {
			continue
		}
		a, clock := decode(sub[1:33]), decode(sub[33:65])
		if a.Source != deadline.Source || a.Value.Cmp(deadline.Value) != 0 {
			continue
		}
		exact, err := recipes.ExactValue(r, clock, uint(left.Value.Uint64()))
		if err != nil {
			return nil, errors.Errorf("CoW expiry clock: %w", err)
		}
		bounds = append(bounds, expiryBound{clock: exact.Expression, duration: limit.Value.Uint64()})
	}
	return bounds, nil
}
