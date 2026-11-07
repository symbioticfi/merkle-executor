package liquidlanerfq

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	reactor "github.com/symbioticfi/merkle-executor/api/bindings/liquidlanereactor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// API deployment provenance: vault-solver cc75494298aa, Makefile RFQ_OPENAPI_URL.
const ConsumerAPI = "https://swap.symbiotic.fi/api/v1"
const automaticPolicy = "liquid-lane-consumer/recipe-v1"
const minValidity = uint64(60)
const maxValidity = uint64(1800)

type Automatic struct{ deps registry.Deps }
type recipeRequest struct {
	template     derived.SignTemplate
	cfg          Config
	budget       recipes.EconomicValue
	floors       []recipes.EconomicValue
	requirements provider.Requirements
}

func NewAutomatic(deps registry.Deps) (provider.Integration, error) {
	if _, ok := deps.Chain.(chainReader); !ok {
		return nil, errors.New("automatic RFQ requires contract, log, receipt and header reads")
	}
	return &Automatic{deps: deps}, nil
}

func consumerHTTP() protocolio.HTTPConfig {
	return protocolio.HTTPConfig{URL: ConsumerAPI, MaxResponseBytes: 4 << 20}
}

func (p *Automatic) derive(s chainio.Snapshot, r recipes.Recipe, t *recipes.Tree) (*recipeRequest, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	inspection, err := recipes.Inspect(r)
	if err != nil {
		return nil, err
	}
	var result *recipeRequest
	for _, action := range actions {
		sign, matched, err := derived.Sign(action, s.Account)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		candidates := map[common.Address]bool{}
		for _, slot := range inspection.Slots {
			if slot.Operation == recipes.Approve && protocolio.Domain("Reactor", "1", s.ChainID, slot.Spender) == sign.Domain {
				candidates[slot.Spender] = true
			}
		}
		for _, other := range actions {
			if other.Leaf.Target != s.Account && protocolio.Domain("Reactor", "1", s.ChainID, other.Leaf.Target) == sign.Domain {
				candidates[other.Leaf.Target] = true
			}
		}
		looksLikeRequest := len(sign.Buffer) >= 32 && common.BytesToHash(sign.Buffer[:32]) == requestTypeHash
		if !looksLikeRequest && len(candidates) == 0 {
			continue
		}
		if result != nil {
			return nil, errors.New("multiple RFQ authorizations require separate executors")
		}
		if len(candidates) != 1 {
			return nil, errors.New("RFQ recipe must commit one unambiguous Reactor approval or invalidation target")
		}
		var address common.Address
		for candidate := range candidates {
			address = candidate
		}
		if len(sign.Buffer) != 352 || !sameHashPlan(sign.Plan) {
			return nil, errors.New("RFQ recipe must commit the single-output Request hash plan")
		}
		for _, index := range []int{0, 1, 3, 6, 7, 8, 10} {
			if err := sign.PinnedWord(index); err != nil {
				return nil, err
			}
		}
		input, err := derived.WordAddress(sign.Buffer[32:64])
		if err != nil {
			return nil, err
		}
		output, err := derived.WordAddress(sign.Buffer[256:288])
		if err != nil {
			return nil, err
		}
		protocol, err := derived.WordAddress(sign.Buffer[192:224])
		if err != nil {
			return nil, err
		}
		recipient, err := derived.WordAddress(sign.Buffer[320:352])
		if err != nil || recipient != s.Account {
			return nil, errors.New("RFQ recipe recipient must be authenticated account")
		}
		if input == output {
			return nil, errors.New("RFQ recipe input and output tokens must differ")
		}
		request := reactor.IReactorRequest{TokenIn: input, AmountIn: new(big.Int).SetBytes(sign.Buffer[64:96]), Deadline: new(big.Int).SetBytes(sign.Buffer[128:160]), Nonce: new(big.Int).SetBytes(sign.Buffer[160:192]), Protocol: protocol, Outputs: []reactor.IReactorOutput{{Token: output, Amount: new(big.Int).SetBytes(sign.Buffer[288:320]), Recipient: recipient}}}
		expected, _, err := authorization(sign.Domain, request)
		if err != nil || !bytes.Equal(expected, action.Leaf.Data) {
			return nil, errors.New("RFQ recipe must pin canonical Request identity and array shape")
		}
		amount, err := sign.Field(2)
		if err != nil {
			return nil, err
		}
		budget, err := recipes.Budget(r, amount, action.Step)
		if err != nil {
			return nil, err
		}
		minimum, err := sign.Field(9)
		if err != nil {
			return nil, err
		}
		floors, err := recipes.Floor(r, minimum, action.Step)
		if err != nil {
			return nil, err
		}
		economics := append([]recipes.EconomicValue{budget}, floors...)
		for _, binding := range action.Bindings {
			if binding.Value.Source == recipes.Result {
				economics = append(economics, recipes.EconomicValue{Expression: binding.Value, Exact: true})
			}
		}
		requirements, err := derived.Requirements(sign, economics, map[int]bool{2: true, 4: true, 5: true, 9: true})
		if err != nil {
			return nil, err
		}
		result = &recipeRequest{template: sign, budget: budget, floors: floors, requirements: requirements, cfg: Config{HTTP: consumerHTTP(), Reactor: address.Hex(), Protocol: protocol.Hex(), TokenIn: input.Hex(), TokenOut: output.Hex(), Action: action.Key, MinValiditySeconds: minValidity, MaxValiditySeconds: maxValidity, MaxLookupPages: 10, EventRangeBlocks: 2000, AmountInInput: budget.Input}}
		if minimum.Source == recipes.Input {
			index := uint(minimum.Value.Uint64())
			result.cfg.AmountOutInput = &index
		}
		for _, binding := range action.Bindings {
			if binding.Value.Source != recipes.Input {
				continue
			}
			index := uint(binding.Value.Value.Uint64())
			switch int(binding.Offset) - sign.Offset {
			case 128:
				result.cfg.DeadlineInput = &index
			case 160:
				result.cfg.NonceInput = &index
			}
		}
	}
	return result, nil
}

func sameHashPlan(actual []account.IMerkleAccountHash) bool {
	expected := hashPlan()
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i].Offset == nil || actual[i].Length == nil || actual[i].Destination == nil || actual[i].Offset.Cmp(expected[i].Offset) != 0 || actual[i].Length.Cmp(expected[i].Length) != 0 || actual[i].Destination.Cmp(expected[i].Destination) != 0 {
			return false
		}
	}
	return true
}

type automaticOperation struct {
	snapshot     chainio.Snapshot
	integration  *Automatic
	compiled     *recipeRequest
	requirements provider.Requirements
	saved        *provider.Record
}

func (*Automatic) ID() string     { return "liquidlanerfq" }
func (*Automatic) Policy() string { return automaticPolicy + "/operation-v2" }
func (p *Automatic) Compile(program provider.Program) ([]provider.Operation, error) {
	program = program.Clone()
	compiled, err := p.derive(program.Snapshot, program.Recipe, program.Tree)
	if err != nil || compiled == nil {
		return nil, err
	}
	return []provider.Operation{&automaticOperation{integration: p, snapshot: program.Snapshot, compiled: compiled, requirements: compiled.requirements}}, nil
}
func (o *automaticOperation) ID() string                          { return provider.OperationID(o.requirements.Actions) }
func (o *automaticOperation) Requirements() provider.Requirements { return o.requirements.Clone() }
func (o *automaticOperation) Prepare(ctx context.Context, preparation provider.Preparation) (provider.Output, error) {
	if o.compiled == nil {
		return provider.Output{}, errors.New("restored operation cannot prepare")
	}
	if err := ctx.Err(); err != nil {
		return provider.Output{}, err
	}
	if err := preparation.ValidateSnapshot(o.snapshot); err != nil {
		return provider.Output{}, err
	}
	if preparation.Reader == nil {
		return provider.Output{}, errors.New("attempt chain reader required")
	}
	request := *o.compiled
	s, results := preparation.Program.Snapshot, preparation.Results
	deps := o.integration.deps
	deps.Chain = preparation.Reader

	amount, err := recipes.EconomicResult(request.budget, results)
	if err != nil {
		return provider.Output{}, err
	}
	minimum := new(big.Int)
	for _, floor := range request.floors {
		value, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return provider.Output{}, err
		}
		if value.Cmp(minimum) > 0 {
			minimum = value
		}
	}
	if amount.Sign() == 0 || minimum.Sign() == 0 {
		return provider.Output{}, provider.ErrNotReady
	}
	request.cfg.AmountIn = amount.String()
	request.cfg.MinAmountOut = minimum.String()
	runtime, err := New(request.cfg, deps)
	if err != nil {
		return provider.Output{}, err
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return provider.Output{}, err
	}
	payload := out.Actions[request.cfg.Action]
	for _, floor := range request.floors {
		if !floor.Exact {
			continue
		}
		expected, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return provider.Output{}, err
		}
		if new(big.Int).SetBytes(payload[request.template.Offset+288:request.template.Offset+320]).Cmp(expected) != 0 {
			return provider.Output{}, errors.New("RFQ selected output conflicts with exact recipe result binding")
		}
	}
	if err = derived.BindOutput(request.template, payload, results, &out); err != nil {
		return provider.Output{}, err
	}
	return provider.EncodeOperation(o.integration.Policy(), request.cfg, out)
}

func (p *Automatic) Restore(record provider.Record) (provider.Operation, error) {
	var cfg Config
	inner, err := provider.DecodeOperation(p.Policy(), record.Output, &cfg)
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(cfg.HTTP, consumerHTTP()) || cfg.MinValiditySeconds != minValidity || cfg.MaxValiditySeconds != maxValidity || cfg.MaxLookupPages != 10 || cfg.EventRangeBlocks != 2000 || len(cfg.SolverIDs) != 0 {
		return nil, errors.New("persisted RFQ automatic constants differ")
	}
	runtime, err := New(cfg, p.deps)
	if err != nil {
		return nil, err
	}

	var saved prepared
	if err := derived.StrictJSON(inner.Data, &saved); err != nil {
		return nil, err
	}
	chainID, err := parse.Uint256(saved.ChainID)
	if err != nil || chainID.Sign() == 0 {
		return nil, errors.New("persisted RFQ chain invalid")
	}
	if _, _, err := runtime.persisted(chainio.Snapshot{ChainID: chainID, Account: saved.Account, BlockNumber: new(big.Int).SetUint64(saved.PreparedBlock)}, inner); err != nil {
		return nil, err
	}

	if len(record.Progress.State) > 0 {
		var state progressState
		if err := strictJSON(record.Progress.State, &state); err != nil || state.Version != 1 || state.Digest != saved.Digest || state.NextBlock < saved.PreparedBlock || (state.NextBlock == saved.PreparedBlock && state.Anchor != (common.Hash{})) || (state.NextBlock > saved.PreparedBlock && state.Anchor == (common.Hash{})) || (state.OrderID != "" && !uuid.MatchString(state.OrderID)) {
			return nil, errors.New("RFQ invalid persisted scan cursor")
		}
		if state.Lookup != nil {
			if err := state.Lookup.validate(); err != nil {
				return nil, err
			}
		}
		if record.Progress.Reference != "" && record.Progress.Reference != state.OrderID {
			return nil, errors.New("RFQ progress order identity differs")
		}
	}

	expected := provider.OutputRequirements(inner, nil)
	if len(expected.Actions) != 1 || expected.Actions[0] != cfg.Action {
		return nil, errors.New("persisted operation action scope differs")
	}
	if err = provider.ValidateRecord(record, p.ID(), p.Policy(), expected); err != nil {
		return nil, err
	}
	savedRecord := record.Clone()
	return &automaticOperation{integration: p, requirements: record.Requirements.Clone(), saved: &savedRecord}, nil
}
func (o *automaticOperation) Advance(ctx context.Context, s chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	if _, err := o.integration.Restore(provider.Record{Integration: o.integration.ID(), Policy: o.integration.Policy(), ID: o.ID(), Requirements: o.Requirements(), Output: out}); err != nil {
		return provider.Progress{}, err
	}

	if o.saved != nil && !provider.EqualOutput(o.saved.Output, out) {
		return provider.Progress{}, errors.New("operation output differs from restored record")
	}
	var cfg Config
	inner, err := provider.DecodeOperation(o.integration.Policy(), out, &cfg)
	if err != nil {
		return provider.Progress{}, err
	}

	if !reflect.DeepEqual(cfg.HTTP, consumerHTTP()) || cfg.MinValiditySeconds != minValidity || cfg.MaxValiditySeconds != maxValidity || cfg.MaxLookupPages != 10 || cfg.EventRangeBlocks != 2000 || len(cfg.SolverIDs) != 0 {
		return provider.Progress{}, errors.New("persisted RFQ automatic constants differ")
	}
	runtime, err := New(cfg, o.integration.deps)
	if err != nil {
		return provider.Progress{}, err
	}
	return runtime.Advance(ctx, s, inner, state)
}
