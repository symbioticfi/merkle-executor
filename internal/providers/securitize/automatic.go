package securitize

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	legacy "github.com/symbioticfi/merkle-executor/api/bindings/securitizelegacyofframp"
	modern "github.com/symbioticfi/merkle-executor/api/bindings/securitizeofframp"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const CatalogueSHA256 = "4eacabc956d8863fd2373e4f910315d84fc9ab0ed4b5be2969901ee31dbb9669"
const PolicyVersion = "securitize-recipes-v1-" + CatalogueSHA256

//go:embed catalogue/securitize.yaml
var embeddedCatalogue []byte

type automatic struct {
	deps   registry.Deps
	assets []Asset
}
type selectedRoute struct {
	Config   Config
	Decimals uint8
	Action   recipes.ManagedAction
	Amount   recipes.EconomicValue
	Floors   []recipes.EconomicValue
}
type durableRoute struct {
	Config   Config          `json:"config"`
	Decimals uint8           `json:"decimals"`
	Output   provider.Output `json:"output"`
}
type durableRoutes struct {
	Account common.Address `json:"account"`
	Routes  []durableRoute `json:"routes"`
}

func NewAutomatic(deps registry.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("Securitize chain reader required")
	}
	hash := sha256.Sum256(embeddedCatalogue)
	if hex.EncodeToString(hash[:]) != CatalogueSHA256 {
		return nil, errors.New("Securitize catalogue digest changed")
	}
	catalogue, err := decodeCatalog(embeddedCatalogue)
	if err != nil {
		return nil, err
	}
	return &automatic{deps: deps, assets: catalogue.Assets}, nil
}
func (p *automatic) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("Securitize requires authenticated recipe")
}
func template(action recipes.ManagedAction, expected []byte, offsets ...int) error {
	if action.Leaf.Value == nil || action.Leaf.Value.Sign() != 0 || len(action.Leaf.Data) != len(expected) {
		return errors.New("Securitize template has incorrect native value or calldata shape")
	}
	allowed := map[int]bool{}
	for _, at := range offsets {
		allowed[at] = true
	}
	for _, binding := range action.Bindings {
		if !allowed[int(binding.Offset)] {
			return errors.New("Securitize binding changes identity")
		}
	}
	for i := 0; i < len(expected); {
		if allowed[i] {
			i += 32
			continue
		}
		start := i
		for i < len(expected) && !allowed[i] {
			i++
		}
		if err := action.Pinned(start, i-start); err != nil {
			return err
		}
		if !bytes.Equal(action.Leaf.Data[start:i], expected[start:i]) {
			return errors.New("Securitize identity differs from known issuer route")
		}
	}
	return nil
}
func match(action recipes.ManagedAction, target common.Address, data []byte) bool {
	return action.Leaf.Target == target && len(action.Leaf.Data) >= 4 && bytes.Equal(action.Leaf.Data[:4], data[:4])
}
func (p *automatic) selectRoutes(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) ([]selectedRoute, error) {
	if s.ChainID == nil || !s.ChainID.IsUint64() || s.ChainID.Sign() <= 0 || s.Account == (common.Address{}) {
		return nil, errors.New("Securitize authenticated snapshot invalid")
	}
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var selected []selectedRoute
	targets := map[common.Address]bool{}
	for _, action := range actions {
		var matches []selectedRoute
		for _, asset := range p.assets {
			if asset.ChainID != s.ChainID.Uint64() {
				continue
			}
			token := common.HexToAddress(asset.Token)
			routes := append([]Route{}, asset.Offramps...)
			if asset.RedemptionWallet != "" {
				routes = append(routes, Route{Mode: "wallet", Target: asset.RedemptionWallet})
			}
			if asset.Unavailable != "" {
				continue
			}
			for _, route := range routes {
				cfg := Config{ChainID: asset.ChainID, Mode: route.Mode, Token: token.Hex(), Target: route.Target, PaymentToken: route.PaymentToken, Action: action.Key}
				target := common.HexToAddress(route.Target)
				destination := target
				var data []byte
				offsets := []int{4}
				amountOffset := 4
				switch route.Mode {
				case "wallet":
					destination = token
					data = erc20.NewERC20().PackTransfer(target, new(big.Int))
					offsets = []int{36}
					amountOffset = 36
				case "modern":
					data = modern.NewSecuritizeOffRamp().PackRedeem(new(big.Int), new(big.Int))
					offsets = []int{4, 36}
				case "legacy":
					data = legacy.NewSecuritizeLegacyOffRamp().PackRedeem(new(big.Int))
				}
				if !match(action, destination, data) {
					continue
				}
				// Wallet addresses distinguish ordinary transfers from redemption requests.
				if route.Mode == "wallet" && len(action.Leaf.Data) == 68 && action.Pinned(4, 32) == nil && !bytes.Equal(action.Leaf.Data[4:36], data[4:36]) {
					continue
				}
				if err = template(action, data, offsets...); err != nil {
					return nil, err
				}
				value, err := action.Field(amountOffset)
				if err != nil {
					return nil, err
				}
				amount, err := recipes.Budget(r, value, action.Step)
				if err != nil {
					return nil, err
				}
				selectedRoute := selectedRoute{Config: cfg, Decimals: asset.Decimals, Action: action, Amount: amount}
				if route.Mode == "modern" {
					value, err := action.Field(36)
					if err != nil {
						return nil, err
					}
					selectedRoute.Floors, err = recipes.Floor(r, value, action.Step)
					if err != nil {
						return nil, err
					}
				}
				if route.Mode == "legacy" {
					selectedRoute.Floors, err = legacyFloors(r, action, common.HexToAddress(route.PaymentToken), s.Account)
					if err != nil {
						return nil, err
					}
				}
				matches = append(matches, selectedRoute)
			}
		}
		if len(matches) > 1 {
			return nil, errors.New("Securitize action has ambiguous issuer routes")
		}
		if len(matches) == 0 {
			if p.knownIdentity(action.Leaf.Target) && len(action.Leaf.Data) >= 4 && (bytes.Equal(action.Leaf.Data[:4], modern.NewSecuritizeOffRamp().GetABI().Methods["redeem"].ID) || bytes.Equal(action.Leaf.Data[:4], legacy.NewSecuritizeLegacyOffRamp().GetABI().Methods["redeem"].ID)) {
				return nil, errors.New("Securitize redemption selector has no supported catalogue route on this chain")
			}
			continue
		}
		if targets[action.Leaf.Target] {
			return nil, errors.New("Securitize receipt target repeated")
		}
		targets[action.Leaf.Target] = true
		selected = append(selected, matches[0])
	}
	return selected, nil
}
func (p *automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	routes, err := p.selectRoutes(s, r, t)
	if err != nil {
		return keeper.RecipeRequirements{}, err
	}
	requirements := keeper.RecipeRequirements{ExclusiveTargets: map[string]common.Address{}}
	seen := map[uint]bool{}
	for _, route := range routes {
		requirements.ExclusiveTargets[route.Action.Key] = route.Action.Leaf.Target
		requirements.Actions = append(requirements.Actions, route.Action.Key)
		requirements.Values = append(requirements.Values, recipes.EconomicExpressions(route.Amount)...)
		for _, floor := range route.Floors {
			requirements.Values = append(requirements.Values, recipes.EconomicExpressions(floor)...)
		}
		for _, binding := range route.Action.Bindings {
			if binding.Value.Source == recipes.Input {
				index := uint(binding.Value.Value.Uint64())
				if seen[index] {
					return requirements, errors.New("Securitize fields share input ownership")
				}
				seen[index] = true
				requirements.Inputs = append(requirements.Inputs, index)
			}
		}
		if route.Config.Mode == "legacy" {
			var index *uint
			for _, floor := range route.Floors {
				if floor.Input != nil {
					if index != nil && *index != *floor.Input {
						return requirements, errors.New("legacy Securitize has multiple guard input identities")
					}
					index = floor.Input
				}
			}
			if index != nil {
				if seen[*index] {
					return requirements, errors.New("legacy Securitize guard shares input ownership")
				}
				seen[*index] = true
				requirements.Inputs = append(requirements.Inputs, *index)
			}
		}
	}
	return requirements, nil
}
func (p *automatic) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) (provider.Output, error) {
	routes, err := p.selectRoutes(s, r, t)
	if err != nil {
		return provider.Output{}, err
	}
	if len(routes) == 0 {
		return derived.Empty(PolicyVersion)
	}
	bundle := durableRoutes{Account: s.Account}
	merged := provider.Output{Inputs: map[uint]string{}, Actions: map[string]hexutil.Bytes{}}
	for _, route := range routes {
		cfg := route.Config
		amount, err := recipes.EconomicResult(route.Amount, results)
		if err != nil {
			return provider.Output{}, err
		}
		cfg.Amount = amount.String()
		cfg.AmountInput = route.Amount.Input
		minimum := new(big.Int)
		var exact *big.Int
		for _, floor := range route.Floors {
			value, err := recipes.EconomicResult(floor, results)
			if err != nil {
				return provider.Output{}, err
			}
			if value.Cmp(minimum) > 0 {
				minimum.Set(value)
			}
			if floor.Exact {
				if exact != nil && exact.Cmp(value) != 0 {
					return provider.Output{}, errors.New("Securitize has conflicting exact proceeds")
				}
				exact = value
			}
		}
		if exact != nil && exact.Cmp(minimum) != 0 {
			return provider.Output{}, errors.New("Securitize exact proceeds are below another recipe floor")
		}
		if cfg.Mode != "wallet" {
			cfg.MinOutput = minimum.String()
			for _, floor := range route.Floors {
				if floor.Input != nil {
					if cfg.Mode == "legacy" {
						if cfg.GuardMinimumInput != nil && *cfg.GuardMinimumInput != *floor.Input {
							return provider.Output{}, errors.New("legacy Securitize has multiple guard input identities")
						}
						cfg.GuardMinimumInput = floor.Input
					} else {
						cfg.MinOutputInput = floor.Input
					}
				}
			}
		}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return provider.Output{}, err
		}
		decimals := route.Decimals
		runtime.decimals = &decimals
		out, err := runtime.Prepare(ctx, s)
		if err != nil {
			return provider.Output{}, err
		}
		if err = derived.BindOutput(derived.SignTemplate{Action: route.Action}, out.Actions[cfg.Action], results, &out); err != nil {
			return provider.Output{}, err
		}
		for i, v := range out.Inputs {
			if _, ok := merged.Inputs[i]; ok {
				return provider.Output{}, errors.New("Securitize duplicated input")
			}
			merged.Inputs[i] = v
		}
		for key, data := range out.Actions {
			if _, ok := merged.Actions[key]; ok {
				return provider.Output{}, errors.New("Securitize duplicated action")
			}
			merged.Actions[key] = data
		}
		bundle.Routes = append(bundle.Routes, durableRoute{Config: cfg, Decimals: decimals, Output: out})
	}
	return derived.Encode(PolicyVersion, bundle, merged, true)
}
func (p *automatic) restore(out provider.Output) ([]*Provider, []provider.Output, error) {
	var bundle durableRoutes
	inner, matched, err := derived.Decode(PolicyVersion, out, &bundle)
	if err != nil {
		return nil, nil, err
	}
	if !matched {
		return nil, nil, nil
	}
	if bundle.Account == (common.Address{}) {
		return nil, nil, errors.New("persisted issuer account missing")
	}
	if len(bundle.Routes) == 0 {
		return nil, nil, errors.New("Securitize envelope has no routes")
	}
	var runtimes []*Provider
	var outputs []provider.Output
	inputs := map[uint]string{}
	actions := map[string]hexutil.Bytes{}
	targets := map[common.Address]bool{}
	for _, route := range bundle.Routes {
		if route.Config.Catalog != "" || route.Config.Symbol != "" || route.Decimals > 77 {
			return nil, nil, errors.New("persisted Securitize route requires external catalogue")
		}
		if err := p.validateKnownRoute(route.Config, route.Decimals); err != nil {
			return nil, nil, err
		}
		runtime, err := New(route.Config, p.deps)
		if err != nil {
			return nil, nil, err
		}
		if _, err = runtime.validatePersisted(route.Output, bundle.Account); err != nil {
			return nil, nil, err
		}
		decimals := route.Decimals
		runtime.decimals = &decimals
		for _, target := range runtime.ExclusiveTargets() {
			if targets[target] {
				return nil, nil, errors.New("persisted Securitize target repeated")
			}
			targets[target] = true
		}
		if len(route.Output.Actions) != 1 || len(route.Output.SkipSteps) != 0 {
			return nil, nil, errors.New("persisted Securitize action scope mismatch")
		}
		payload, err := runtime.payload()
		if err != nil || !bytes.Equal(route.Output.Actions[runtime.cfg.Action], payload) {
			return nil, nil, errors.New("persisted Securitize payload differs from derived policy")
		}
		expectedInputs := map[uint]string{}
		if runtime.cfg.AmountInput != nil {
			expectedInputs[*runtime.cfg.AmountInput] = runtime.amount.String()
		}
		if runtime.cfg.MinOutputInput != nil {
			expectedInputs[*runtime.cfg.MinOutputInput] = runtime.minimum.String()
		}
		if runtime.cfg.GuardMinimumInput != nil {
			expectedInputs[*runtime.cfg.GuardMinimumInput] = runtime.minimum.String()
		}
		if len(route.Output.Inputs) != len(expectedInputs) {
			return nil, nil, errors.New("persisted Securitize input scope differs from derived fields")
		}
		for index, value := range expectedInputs {
			if route.Output.Inputs[index] != value {
				return nil, nil, errors.New("persisted Securitize input differs from payload")
			}
		}
		for i, v := range route.Output.Inputs {
			if _, ok := inputs[i]; ok {
				return nil, nil, errors.New("persisted Securitize input repeated")
			}
			inputs[i] = v
		}
		for key, data := range route.Output.Actions {
			if _, ok := actions[key]; ok {
				return nil, nil, errors.New("persisted Securitize action repeated")
			}
			actions[key] = data
		}
		runtimes = append(runtimes, runtime)
		outputs = append(outputs, route.Output)
	}
	if len(inner.Actions) != len(actions) || len(inner.Inputs) != len(inputs) || len(inner.SkipSteps) != 0 || len(inner.Data) != 0 {
		return nil, nil, errors.New("persisted Securitize aggregate shape mismatch")
	}
	for key, data := range actions {
		if !bytes.Equal(inner.Actions[key], data) {
			return nil, nil, errors.New("persisted Securitize action aggregate mismatch")
		}
	}
	for i, v := range inputs {
		if inner.Inputs[i] != v {
			return nil, nil, errors.New("persisted Securitize input aggregate mismatch")
		}
	}
	return runtimes, outputs, nil
}
func (p *automatic) ExclusiveOutputTargets(out provider.Output) (map[string]common.Address, error) {
	runtimes, _, err := p.restore(out)
	if err != nil {
		return nil, err
	}
	claims := map[string]common.Address{}
	for _, runtime := range runtimes {
		for key, target := range runtime.ExclusiveTargets() {
			if _, ok := claims[key]; ok {
				return nil, errors.New("persisted Securitize ownership repeated")
			}
			claims[key] = target
		}
	}
	return claims, nil
}
func (p *automatic) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	runtimes, outputs, err := p.restore(out)
	if err != nil {
		return provider.Progress{}, err
	}
	if len(runtimes) == 0 {
		return provider.Progress{Done: true, Outcome: "not-applicable"}, nil
	}
	outcome := "redeemed"
	for i, runtime := range runtimes {
		if _, err := runtime.validatePersisted(outputs[i], s.Account); err != nil {
			return provider.Progress{}, err
		}
		progress, err := runtime.Advance(ctx, s, outputs[i], nil)
		if err != nil {
			return provider.Progress{}, err
		}
		if !progress.Done {
			return provider.Progress{}, errors.New("Securitize route unresolved")
		}
		if runtime.cfg.Mode == "wallet" {
			outcome = "requested"
		}
	}
	return provider.Progress{Done: true, Outcome: outcome, Reference: s.Execution.TxHash.Hex()}, nil
}

func argumentType(name string) abi.Type {
	t, err := abi.NewType(name, "", nil)
	if err != nil {
		panic(err)
	}
	return t
}

var queryArguments = abi.Arguments{{Type: argumentType("address")}, {Type: argumentType("bytes")}, {Type: argumentType("uint256[]")}, {Type: argumentType("uint256")}}
var unaryArguments = abi.Arguments{{Type: argumentType("uint256")}}
var binaryArguments = abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}

func decodeValue(raw *big.Int) recipes.Value {
	source := uint8(new(big.Int).Rsh(new(big.Int).Set(raw), 216).Uint64())
	value := new(big.Int).And(raw, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(1)))
	return recipes.Value{Source: source, Value: value}
}
func balanceRead(r recipes.Recipe, value recipes.Value, token, account common.Address, before, after uint) bool {
	if value.Source != recipes.Result || value.Value == nil || !value.Value.IsUint64() {
		return false
	}
	index := uint(value.Value.Uint64())
	if index >= uint(len(r.Steps)) || index >= before || (after != ^uint(0) && index <= after) || r.Steps[index][0] != recipes.Read {
		return false
	}
	values, err := unaryArguments.Unpack(r.Steps[index][1:])
	if err != nil {
		return false
	}
	query := values[0].(*big.Int)
	if !query.IsUint64() || query.Uint64() >= uint64(len(r.Queries)) {
		return false
	}
	values, err = queryArguments.Unpack(r.Queries[query.Uint64()])
	if err != nil {
		return false
	}
	return values[0].(common.Address) == token && bytes.Equal(values[1].([]byte), erc20.NewERC20().PackBalanceOf(account)) && len(values[2].([]*big.Int)) == 0 && values[3].(*big.Int).Sign() == 0
}
func legacyFloors(r recipes.Recipe, action recipes.ManagedAction, payment, account common.Address) ([]recipes.EconomicValue, error) {
	var floors []recipes.EconomicValue
	for i := action.Step + 1; i < uint(len(r.Steps)); i++ {
		if r.Steps[i][0] != recipes.LessOrEqual {
			continue
		}
		args, err := binaryArguments.Unpack(r.Steps[i][1:])
		if err != nil {
			return nil, err
		}
		left, right := decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))
		if right.Source != recipes.Result || right.Value == nil || !right.Value.IsUint64() {
			continue
		}
		delta := uint(right.Value.Uint64())
		if delta >= i || delta <= action.Step || r.Steps[delta][0] != recipes.Sub {
			continue
		}
		args, err = binaryArguments.Unpack(r.Steps[delta][1:])
		if err != nil {
			return nil, err
		}
		after, before := decodeValue(args[0].(*big.Int)), decodeValue(args[1].(*big.Int))
		if !balanceRead(r, after, payment, account, delta, action.Step) || !balanceRead(r, before, payment, account, action.Step, ^uint(0)) {
			continue
		}
		derivedFloors, err := recipes.Floor(r, left, action.Step)
		if err != nil {
			return nil, err
		}
		for j := range derivedFloors {
			if derivedFloors[j].Input == nil {
				derivedFloors[j].Exact = false
			}
		}
		floors = append(floors, derivedFloors...)
	}
	if len(floors) == 0 {
		return nil, errors.New("legacy Securitize recipe requires an explicit read-derived payment balance delta floor")
	}
	return floors, nil
}
