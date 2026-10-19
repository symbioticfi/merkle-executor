package redemptions

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	midas "github.com/symbioticfi/merkle-executor/api/bindings/midasredemption"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	openeden "github.com/symbioticfi/merkle-executor/api/bindings/openedenexpress"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretoqueue"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"gopkg.in/yaml.v3"
)

const CatalogueSHA256 = "e11e2ce31f0e1864d158fd5ecfdd6a6e3624304f8b2bb546601eb5a870f6bb2d"
const PolicyVersion = "issuer-recipes-v1-" + CatalogueSHA256

//go:embed catalogue/liquid-lane.yaml
var issuerCatalogue []byte

type issuerAsset struct {
	Symbol       string   `yaml:"symbol"`
	ChainID      uint64   `yaml:"chainId"`
	Family       string   `yaml:"family"`
	Token        string   `yaml:"token"`
	OutputAsset  string   `yaml:"outputAsset"`
	Target       string   `yaml:"target"`
	Factory      string   `yaml:"factory"`
	RequestModes []string `yaml:"requestModes"`
	ClaimModes   []string `yaml:"claimModes"`
	Settlement   string   `yaml:"settlement"`
	Intermediate string   `yaml:"intermediate"`
	RedeemVault  string   `yaml:"redeemVault"`
	CDO          string   `yaml:"cdo"`
	Strategy     string   `yaml:"strategy"`
	Tranche      string   `yaml:"tranche"`
}
type automatic struct {
	deps   registry.Deps
	assets []issuerAsset
}
type selectedRoute struct {
	Config       Config
	Actions      []recipes.ManagedAction
	Amount       *recipes.EconomicValue
	AmountField  recipes.Value
	Intermediate *recipes.EconomicValue
	Index        *recipes.EconomicValue
	LiteralIndex *big.Int
}
type durableRoute struct {
	Config Config          `json:"config"`
	Output provider.Output `json:"output"`
}
type durableRoutes struct {
	Account common.Address `json:"account"`
	Routes  []durableRoute `json:"routes"`
}

func NewAutomatic(deps registry.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("issuer chain reader required")
	}
	hash := sha256.Sum256(issuerCatalogue)
	if hex.EncodeToString(hash[:]) != CatalogueSHA256 {
		return nil, errors.New("issuer catalogue digest changed")
	}
	var document struct {
		Version       int           `yaml:"version"`
		Provider      string        `yaml:"provider"`
		ChainID       uint64        `yaml:"chainId"`
		Source        string        `yaml:"source"`
		CheckedAt     string        `yaml:"checkedAt"`
		TopologyBlock uint64        `yaml:"topologyBlock"`
		Assets        []issuerAsset `yaml:"assets"`
	}
	var node yaml.Node
	if err := yaml.Unmarshal(issuerCatalogue, &node); err != nil {
		return nil, errors.New("invalid embedded issuer catalogue")
	}
	if err := parse.DecodeStrict(node, &document); err != nil {
		return nil, err
	}
	if document.Version != 1 || document.Provider != "redemptions" || len(document.Assets) == 0 {
		return nil, errors.New("invalid issuer catalogue metadata")
	}
	return &automatic{deps: deps, assets: document.Assets}, nil
}

func (p *automatic) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("issuer automatic provider requires authenticated recipe")
}

func exactAmount(recipe recipes.Recipe, action recipes.ManagedAction, offset int) (*recipes.EconomicValue, error) {
	value, err := action.Field(offset)
	if err != nil {
		return nil, err
	}
	economic, err := recipes.Budget(recipe, value, action.Step)
	if err != nil {
		return nil, err
	}
	return &economic, nil
}
func exactIndex(recipe recipes.Recipe, action recipes.ManagedAction, offset int) (*recipes.EconomicValue, *big.Int, error) {
	value, err := action.Field(offset)
	if err != nil {
		if err = action.Pinned(offset, 32); err != nil {
			return nil, nil, err
		}
		return nil, new(big.Int).SetBytes(action.Leaf.Data[offset : offset+32]), nil
	}
	if value.Source == recipes.Literal {
		return nil, new(big.Int).Set(value.Value), nil
	}
	economic, err := recipes.ExactValue(recipe, value, action.Step)
	if err != nil {
		return nil, nil, err
	}
	if !economic.Exact {
		return nil, nil, errors.New("issuer request index requires exact recipe identity")
	}
	return &economic, nil, nil
}
func selectorMatches(action recipes.ManagedAction, target common.Address, data []byte) bool {
	return action.Leaf.Target == target && len(action.Leaf.Data) >= 4 && bytes.Equal(action.Leaf.Data[:4], data[:4])
}

func (p *automatic) selectRoutes(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) ([]selectedRoute, error) {
	if err := protocolSnapshot(s); err != nil {
		return nil, err
	}
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	var selected []selectedRoute
	used := map[string]bool{}
	targets := map[common.Address]bool{}
	for _, action := range actions {
		if used[action.Key] {
			continue
		}
		var matches []selectedRoute
		for _, asset := range p.assets {
			if asset.ChainID != s.ChainID.Uint64() {
				continue
			}
			token, target, output := common.HexToAddress(asset.Token), common.HexToAddress(asset.Target), common.HexToAddress(asset.OutputAsset)
			cdo := common.HexToAddress(asset.CDO)
			modes := append(append([]string{}, asset.RequestModes...), asset.ClaimModes...)
			for _, mode := range modes {
				cfg := Config{Family: asset.Family, Mode: mode, ChainID: asset.ChainID, Token: token.Hex(), OutputAsset: output.Hex(), Target: target.Hex(), Action: action.Key, CDO: asset.CDO, Strategy: asset.Strategy, Tranche: asset.Tranche}
				destination := target
				var data []byte
				at := -1
				index := false
				switch asset.Family {
				case "midas":
					data = midas.NewMidasRedemption().PackRedeemRequest(output, new(big.Int))
					at = 36
				case "centrifuge":
					if mode == "request" {
						data = async.NewAsyncRedeemVault().PackRequestRedeem(new(big.Int), s.Account, s.Account)
					} else {
						data = async.NewAsyncRedeemVault().PackRedeem(new(big.Int), s.Account, s.Account)
					}
					at = 4
				case "openeden":
					data = openeden.NewOpenEdenExpress().PackRequestRedeem(s.Account, new(big.Int))
					at = 36
				case "figure":
					destination = token
					data = erc4626.NewERC4626().PackRedeem(new(big.Int), s.Account, s.Account)
					at = 4
				case "hastra":
					if mode == "unwrap" {
						data = erc4626.NewERC4626().PackRedeem(new(big.Int), s.Account, s.Account)
					} else {
						data = figure.NewFigureYieldVault().PackRequestRedeem(new(big.Int))
					}
					at = 4
				case "noon":
					switch mode {
					case "request":
						destination = token
						data = erc4626.NewERC4626().PackRedeem(new(big.Int), target, s.Account)
						at = 4
					case "claim":
						data = noonwithdrawalhandler.NewNoonWithdrawalHandler().PackClaimWithdrawal(new(big.Int))
						at = 4
						index = true
					case "redeem-usn":
						data = native.NewNoonRedeemHandlerV2().PackRedeemOnchain(output, new(big.Int))
						at = 36
					}
				case "pareto":
					switch mode {
					case "queue-request":
						data = paretoqueue.NewParetoQueue().PackRequestWithdraw(new(big.Int))
						at = 4
					case "direct-request":
						destination = cdo
						data = paretocdo.NewParetoCDO().PackRequestWithdraw(new(big.Int), token)
						at = 4
					case "queue-claim":
						data = paretoqueue.NewParetoQueue().PackClaimWithdrawRequest(new(big.Int))
						at = 4
						index = true
					case "direct-claim":
						destination = cdo
						data = paretocdo.NewParetoCDO().PackClaimWithdrawRequest()
					case "instant-claim":
						destination = cdo
						data = paretocdo.NewParetoCDO().PackClaimInstantWithdrawRequest()
					}
				}
				if len(data) == 0 || !selectorMatches(action, destination, data) {
					continue
				}
				// The combined Figure route owns both calls when present. An
				// independent unwrap instead settles in the wrapper's asset.
				if asset.Family == "figure" && len(figureRequests(actions, action.Step, target)) == 0 {
					continue
				}
				if asset.Family == "hastra" && mode == "unwrap" && len(figureRequests(actions, action.Step, output)) > 0 {
					continue
				}
				offsets := []int{}
				if at >= 0 {
					offsets = append(offsets, at)
				}
				if err := derived.NumericTemplate(action, data, offsets...); err != nil {
					// One venue can admit several committed output assets. Reject
					// only after every identity candidate has been considered.
					continue
				}
				route := selectedRoute{Config: cfg, Actions: []recipes.ManagedAction{action}}
				if at >= 0 {
					if index {
						route.Index, route.LiteralIndex, err = exactIndex(r, action, at)
					} else {
						route.AmountField, err = action.Field(at)
						if err == nil {
							route.Amount, err = exactAmount(r, action, at)
						}
					}
					if err != nil {
						return nil, err
					}
				}
				if asset.Family == "figure" {
					secondData := figure.NewFigureYieldVault().PackRequestRedeem(new(big.Int))
					seconds := figureRequests(actions, action.Step, target)
					if len(seconds) != 1 {
						return nil, errors.New("Figure requires exactly one ordered output request")
					}
					second := seconds[0]
					if err = derived.NumericTemplate(second, secondData, 4); err != nil {
						return nil, err
					}
					route.Intermediate, err = exactAmount(r, second, 4)
					if err != nil {
						return nil, err
					}
					if err = figureLink(r, route, token); err != nil {
						return nil, err
					}
					route.Config.SecondAction = second.Key
					route.Actions = append(route.Actions, second)
				}
				matches = append(matches, route)
			}
		}
		if len(matches) > 1 {
			return nil, errors.New("issuer action matches ambiguous catalogue routes")
		}
		if len(matches) == 0 {
			if p.unsupportedIssuer(action, s.Account) {
				return nil, errors.New("issuer selector has no supported committed catalogue route on this chain")
			}
			continue
		}
		route := matches[0]
		for _, owned := range route.Actions {
			if used[owned.Key] || targets[owned.Leaf.Target] {
				return nil, errors.New("issuer receipt target or action is repeated")
			}
			used[owned.Key] = true
			targets[owned.Leaf.Target] = true
		}
		selected = append(selected, route)
	}
	for _, action := range actions {
		for _, asset := range p.assets {
			if asset.ChainID == s.ChainID.Uint64() && asset.Family == "figure" && selectorMatches(action, common.HexToAddress(asset.Target), figure.NewFigureYieldVault().PackRequestRedeem(new(big.Int))) && !used[action.Key] {
				return nil, errors.New("Figure request has no uniquely linked unwrap")
			}
		}
	}
	return selected, nil
}

func figureRequests(actions []recipes.ManagedAction, after uint, target common.Address) []recipes.ManagedAction {
	var requests []recipes.ManagedAction
	data := figure.NewFigureYieldVault().PackRequestRedeem(new(big.Int))
	for _, action := range actions {
		if action.Step > after && selectorMatches(action, target, data) {
			requests = append(requests, action)
		}
	}
	return requests
}

func protocolSnapshot(s provider.Snapshot) error {
	if s.ChainID == nil || !s.ChainID.IsUint64() || s.ChainID.Sign() <= 0 || s.Account == (common.Address{}) {
		return errors.New("issuer authenticated snapshot is invalid")
	}
	return nil
}
func routeRequirements(route selectedRoute) (keeper.RecipeRequirements, error) {
	req := keeper.RecipeRequirements{ExclusiveTargets: map[string]common.Address{}}
	for _, e := range []*recipes.EconomicValue{route.Amount, route.Intermediate, route.Index} {
		if e != nil {
			req.Values = append(req.Values, recipes.EconomicExpressions(*e)...)
		}
	}
	seen := map[uint]bool{}
	for _, action := range route.Actions {
		req.ExclusiveTargets[action.Key] = action.Leaf.Target
		req.Actions = append(req.Actions, action.Key)
		for _, b := range action.Bindings {
			if b.Value.Source == recipes.Input {
				index := uint(b.Value.Value.Uint64())
				if seen[index] {
					return req, errors.New("issuer input has multiple field meanings")
				}
				seen[index] = true
				req.Inputs = append(req.Inputs, index)
			}
		}
	}
	return req, nil
}
func (p *automatic) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	routes, err := p.selectRoutes(s, r, t)
	if err != nil {
		return keeper.RecipeRequirements{}, err
	}
	total := keeper.RecipeRequirements{ExclusiveTargets: map[string]common.Address{}}
	seen := map[uint]bool{}
	for _, route := range routes {
		req, err := routeRequirements(route)
		if err != nil {
			return total, err
		}
		for key, target := range req.ExclusiveTargets {
			total.ExclusiveTargets[key] = target
		}
		total.Values = append(total.Values, req.Values...)
		total.Actions = append(total.Actions, req.Actions...)
		for _, i := range req.Inputs {
			if seen[i] {
				return total, errors.New("issuer routes share numerical input ownership")
			}
			seen[i] = true
			total.Inputs = append(total.Inputs, i)
		}
	}
	return total, nil
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
		if route.Amount != nil {
			amount, err := recipes.EconomicResult(*route.Amount, results)
			if err != nil {
				return provider.Output{}, err
			}
			if amount.Sign() == 0 {
				return provider.Output{}, provider.ErrNotReady
			}
			cfg.Amount = amount.String()
			cfg.AmountInput = route.Amount.Input
		}
		if route.Intermediate != nil {
			intermediate, err := recipes.EconomicResult(*route.Intermediate, results)
			if err != nil {
				return provider.Output{}, err
			}
			if intermediate.Sign() == 0 {
				return provider.Output{}, provider.ErrNotReady
			}
			cfg.IntermediateAmount = intermediate.String()
			cfg.IntermediateInput = route.Intermediate.Input
		}
		if route.Index != nil || route.LiteralIndex != nil {
			value := route.LiteralIndex
			if route.Index != nil {
				value, err = recipes.EconomicResult(*route.Index, results)
				if err != nil {
					return provider.Output{}, err
				}
			}
			if cfg.Family == "noon" {
				cfg.RequestID = value.String()
				if route.Index != nil {
					cfg.RequestIDInput = route.Index.Input
				}
			} else {
				cfg.Epoch = value.String()
				if route.Index != nil {
					cfg.EpochInput = route.Index.Input
				}
			}
		}
		runtime, err := New(cfg, p.deps)
		if err != nil {
			return provider.Output{}, err
		}
		out, err := runtime.Prepare(ctx, s)
		if err != nil {
			return provider.Output{}, err
		}
		for _, action := range route.Actions {
			if err = derived.BindOutput(derived.SignTemplate{Action: action}, out.Actions[action.Key], results, &out); err != nil {
				return provider.Output{}, err
			}
		}
		for i, v := range out.Inputs {
			if _, ok := merged.Inputs[i]; ok {
				return provider.Output{}, errors.New("issuer routes duplicate input")
			}
			merged.Inputs[i] = v
		}
		for key, data := range out.Actions {
			if _, ok := merged.Actions[key]; ok {
				return provider.Output{}, errors.New("issuer routes duplicate action")
			}
			merged.Actions[key] = data
		}
		bundle.Routes = append(bundle.Routes, durableRoute{Config: cfg, Output: out})
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
		return nil, nil, errors.New("issuer envelope has no routes")
	}
	var runtimes []*Provider
	var outputs []provider.Output
	inputs := map[uint]string{}
	actions := map[string]hexutil.Bytes{}
	targets := map[common.Address]bool{}
	for _, route := range bundle.Routes {
		if err := p.validateKnownRoute(route.Config); err != nil {
			return nil, nil, err
		}
		runtime, err := New(route.Config, p.deps)
		if err != nil {
			return nil, nil, err
		}
		if err = runtime.validatePersisted(route.Output, bundle.Account); err != nil {
			return nil, nil, err
		}
		for _, target := range runtime.ExclusiveTargets() {
			if targets[target] {
				return nil, nil, errors.New("persisted issuer target is repeated")
			}
			targets[target] = true
		}
		for i, v := range route.Output.Inputs {
			if _, ok := inputs[i]; ok {
				return nil, nil, errors.New("persisted issuer input is repeated")
			}
			inputs[i] = v
		}
		for key, data := range route.Output.Actions {
			if _, ok := actions[key]; ok {
				return nil, nil, errors.New("persisted issuer action is repeated")
			}
			actions[key] = data
		}
		if len(route.Output.Actions) != len(runtime.ExclusiveTargets()) || len(route.Output.SkipSteps) != 0 {
			return nil, nil, errors.New("persisted issuer action scope mismatch")
		}
		runtimes = append(runtimes, runtime)
		outputs = append(outputs, route.Output)
	}
	if len(inner.Actions) != len(actions) || len(inner.Inputs) != len(inputs) || len(inner.SkipSteps) != 0 || len(inner.Data) != 0 {
		return nil, nil, errors.New("persisted issuer aggregate shape mismatch")
	}
	for key, data := range actions {
		if !bytes.Equal(inner.Actions[key], data) {
			return nil, nil, errors.New("persisted issuer aggregate action mismatch")
		}
	}
	for i, v := range inputs {
		if inner.Inputs[i] != v {
			return nil, nil, errors.New("persisted issuer aggregate input mismatch")
		}
	}
	return runtimes, outputs, nil
}
func (p *automatic) ExclusiveOutputTargets(out provider.Output) (map[string]common.Address, error) {
	runtimes, _, err := p.restore(out)
	if err != nil {
		return nil, err
	}
	targets := map[string]common.Address{}
	for _, runtime := range runtimes {
		for key, target := range runtime.ExclusiveTargets() {
			if _, ok := targets[key]; ok {
				return nil, errors.New("persisted issuer action ownership duplicated")
			}
			targets[key] = target
		}
	}
	return targets, nil
}
func (p *automatic) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	runtimes, outputs, err := p.restore(out)
	if err != nil {
		return provider.Progress{}, err
	}
	if len(runtimes) == 0 {
		return provider.Progress{Done: true, Outcome: "not-applicable"}, nil
	}
	outcome := "requested"
	nativeOnly := true
	for i, runtime := range runtimes {
		if err := runtime.validatePersisted(outputs[i], s.Account); err != nil {
			return provider.Progress{}, err
		}
		progress, err := runtime.Advance(ctx, s, outputs[i], nil)
		if err != nil {
			return provider.Progress{}, err
		}
		if !progress.Done {
			return provider.Progress{}, errors.New("issuer route remains unresolved")
		}
		if progress.Outcome != "redeemed" {
			nativeOnly = false
		}
		if strings.HasSuffix(runtime.cfg.Mode, "claim") {
			outcome = "claimed"
		}
	}
	if nativeOnly {
		outcome = "redeemed"
	}
	return provider.Progress{Done: true, Outcome: outcome, Reference: s.Execution.TxHash.Hex()}, nil
}
