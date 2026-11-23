package cowswap

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Automatic struct{ deps provider.Deps }
type recipeOrder struct {
	trade  derived.Trade
	cfg    Config
	expiry []expiryBound
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
		expiry, err := boundedExpiry(r, sign)
		if err != nil {
			return nil, err
		}
		var clocks []recipes.EconomicValue
		for _, bound := range expiry {
			clocks = append(clocks, recipes.EconomicValue{Expression: bound.clock, Exact: true})
		}
		trade, minimum, err := derived.NewTrade(r, sign, 4, 5, map[int]bool{4: true, 5: true, 6: true, 7: true}, clocks...)
		if err != nil {
			return nil, err
		}
		result = &recipeOrder{trade: trade, cfg: Config{HTTP: http, Settlement: settlement.Hex(), SellToken: sell.Hex(), BuyToken: buy.Hex(), Action: action.Key, ValiditySeconds: DefaultValiditySeconds, SellAmountInput: trade.Budget.Input}, expiry: expiry}
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
	return order.trade.Requirements, nil
}
func (p *Automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) ([]provider.Output, error) {
	order, err := p.derive(s, r, t)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return []provider.Output{{}}, nil
	}
	amount, minimum, err := order.trade.Amounts(results)
	if err != nil {
		return nil, err
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
	if order.trade.Template.PinnedWord(7) == nil {
		appData := common.BytesToHash(order.trade.Template.Buffer[224:256])
		runtime.pinnedAppData = &appData
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return nil, err
	}
	if err = order.trade.Bind(out.Actions[order.cfg.Action], 5, results, &out); err != nil {
		return nil, err
	}
	return []provider.Output{out}, nil
}
func (p *Provider) confirmedTask(out provider.Output, saved prepared) provider.Task {
	authorization := bytes.Clone(out.Actions[p.cfg.Action])
	return provider.TaskFunc(func(ctx context.Context, s provider.Snapshot) (provider.Progress, error) {
		if err := protocolio.Execution(s); err != nil {
			return provider.Progress{}, err
		}
		if err := derived.AuthorizingSign(s.Execution, s.Account, saved.Digest, authorization); err != nil {
			return provider.Progress{}, err
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
	var bounds []expiryBound
	for i, step := range r.Steps[:sign.Action.Step] {
		if len(step) != 65 || step[0] != recipes.LessOrEqual {
			continue
		}
		check, _ := recipes.Operands(r, uint(i))
		left, limit := check[0], check[1]
		if left.Source != recipes.Result || !left.Value.IsUint64() || left.Value.Uint64() >= uint64(i) || limit.Source != recipes.Literal || !limit.Value.IsUint64() || limit.Value.Sign() == 0 {
			continue
		}
		sub := r.Steps[left.Value.Uint64()]
		if len(sub) != 65 || sub[0] != recipes.Sub {
			continue
		}
		difference, _ := recipes.Operands(r, uint(left.Value.Uint64()))
		a, clock := difference[0], difference[1]
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
