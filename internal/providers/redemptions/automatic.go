package redemptions

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"gopkg.in/yaml.v3"
)

const CatalogueSHA256 = "14f5c8979da891a85b913244d9829d93f53d507263413d990d73026ff9139fe6"
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

func NewAutomatic(deps registry.Deps) (provider.Integration, error) {
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
func routeRequirements(route selectedRoute) (provider.Requirements, error) {
	req := provider.Requirements{ExclusiveTargets: map[string]common.Address{}}
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

type routeEnvelope struct {
	Config  Config         `json:"config"`
	Account common.Address `json:"account"`
}
type routeOperation struct {
	integration  *automatic
	route        *selectedRoute
	requirements provider.Requirements
	saved        *provider.Record
}

func (*automatic) ID() string     { return "redemptions" }
func (*automatic) Policy() string { return PolicyVersion + "/operation-v2" }
func (p *automatic) Compile(program provider.Program) ([]provider.Operation, error) {
	program = program.Clone()
	routes, err := p.selectRoutes(program.Snapshot, program.Recipe, program.Tree)
	if err != nil {
		return nil, err
	}
	var operations []provider.Operation
	for i := range routes {
		route := routes[i]
		req, err := routeRequirements(route)
		if err != nil {
			return nil, err
		}
		operations = append(operations, &routeOperation{integration: p, route: &route, requirements: req})
	}
	return operations, nil
}
func (o *routeOperation) ID() string                          { return provider.OperationID(o.requirements.Actions) }
func (o *routeOperation) Requirements() provider.Requirements { return o.requirements.Clone() }
func (o *routeOperation) Prepare(ctx context.Context, preparation provider.Preparation) (provider.Output, error) {
	if o.route == nil {
		return provider.Output{}, errors.New("restored operation cannot prepare")
	}
	if preparation.Reader == nil {
		return provider.Output{}, errors.New("attempt chain reader required")
	}
	route := *o.route
	var err error
	results := preparation.Results
	s := preparation.Program.Snapshot
	deps := o.integration.deps
	deps.Chain = preparation.Reader
	cfg := route.Config
	if route.Amount != nil {
		amount, err := recipes.EconomicResult(*route.Amount, results)
		if err != nil {
			return provider.Output{}, err
		}
		cfg.Amount = amount.String()
		cfg.AmountInput = route.Amount.Input
	}
	if route.Intermediate != nil {
		intermediate, err := recipes.EconomicResult(*route.Intermediate, results)
		if err != nil {
			return provider.Output{}, err
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
	runtime, err := New(cfg, deps)
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
	return provider.EncodeOperation(o.integration.Policy(), routeEnvelope{Config: cfg, Account: s.Account}, out)
}

func (p *automatic) Restore(record provider.Record) (provider.Operation, error) {
	var envelope routeEnvelope
	inner, err := provider.DecodeOperation(p.Policy(), record.Output, &envelope)
	if err != nil {
		return nil, err
	}
	if envelope.Account == (common.Address{}) {
		return nil, errors.New("persisted route account missing")
	}

	if err := p.validateKnownRoute(envelope.Config); err != nil {
		return nil, err
	}
	runtime, err := New(envelope.Config, p.deps)
	if err != nil {
		return nil, err
	}
	if err = runtime.validatePersisted(inner, envelope.Account); err != nil {
		return nil, err
	}
	expected := provider.OutputRequirements(inner, runtime.ExclusiveTargets())

	if err = provider.ValidateRecord(record, p.ID(), p.Policy(), expected); err != nil {
		return nil, err
	}
	saved := record.Clone()
	return &routeOperation{integration: p, requirements: record.Requirements.Clone(), saved: &saved}, nil
}
func (o *routeOperation) Advance(ctx context.Context, s chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	if _, err := o.integration.Restore(provider.Record{Integration: o.integration.ID(), Policy: o.integration.Policy(), ID: o.ID(), Requirements: o.Requirements(), Output: out}); err != nil {
		return provider.Progress{}, err
	}

	if o.saved != nil && !provider.EqualOutput(o.saved.Output, out) {
		return provider.Progress{}, errors.New("operation output differs from restored record")
	}
	var envelope routeEnvelope
	inner, err := provider.DecodeOperation(o.integration.Policy(), out, &envelope)
	if err != nil {
		return provider.Progress{}, err
	}
	if envelope.Account != s.Account {
		return provider.Progress{}, errors.New("operation account differs from snapshot")
	}
	runtime, err := New(envelope.Config, o.integration.deps)
	if err != nil {
		return provider.Progress{}, err
	}
	return runtime.Advance(ctx, s, inner, state)
}
