package cowswap

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const automaticPolicy = PolicyVersion + "/recipe-v1"

type Automatic struct{ deps registry.Deps }
type recipeOrder struct {
	template     derived.SignTemplate
	cfg          Config
	budget       recipes.EconomicValue
	floors       []recipes.EconomicValue
	requirements provider.Requirements
}

func NewAutomatic(deps registry.Deps) (provider.Integration, error) {
	if deps.Chain == nil {
		return nil, errors.New("automatic CoW requires chain reader")
	}
	return &Automatic{deps: deps}, nil
}

func (p *Automatic) derive(s chainio.Snapshot, r recipes.Recipe, t *recipes.Tree) (*recipeOrder, error) {
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
		economics := append([]recipes.EconomicValue{budget}, floors...)
		for _, binding := range action.Bindings {
			if binding.Value.Source == recipes.Result {
				economics = append(economics, recipes.EconomicValue{Expression: binding.Value, Exact: true})
			}
		}
		requirements, err := derived.Requirements(sign, economics, map[int]bool{4: true, 5: true, 6: true, 7: true})
		if err != nil {
			return nil, err
		}
		result = &recipeOrder{template: sign, cfg: Config{HTTP: http, Settlement: settlement.Hex(), SellToken: sell.Hex(), BuyToken: buy.Hex(), Action: action.Key, ValiditySeconds: DefaultValiditySeconds, SellAmountInput: budget.Input}, budget: budget, floors: floors, requirements: requirements}
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

type automaticOperation struct {
	snapshot     chainio.Snapshot
	integration  *Automatic
	compiled     *recipeOrder
	requirements provider.Requirements
	saved        *provider.Record
}

func (*Automatic) ID() string     { return "cowswap" }
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
	order := *o.compiled
	s, results := preparation.Program.Snapshot, preparation.Results
	deps := o.integration.deps
	deps.Chain = preparation.Reader

	amount, err := recipes.EconomicResult(order.budget, results)
	if err != nil {
		return provider.Output{}, err
	}
	minimum := new(big.Int)
	for _, floor := range order.floors {
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
	order.cfg.SellAmount = amount.String()
	order.cfg.MinBuyAmount = minimum.String()
	runtime, err := New(order.cfg, deps)
	if err != nil {
		return provider.Output{}, err
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return provider.Output{}, err
	}
	payload := out.Actions[order.cfg.Action]
	for _, floor := range order.floors {
		if !floor.Exact {
			continue
		}
		expected, err := recipes.EconomicResult(floor, results)
		if err != nil {
			return provider.Output{}, err
		}
		if new(big.Int).SetBytes(payload[order.template.Offset+160:order.template.Offset+192]).Cmp(expected) != 0 {
			return provider.Output{}, errors.New("CoW dynamic minimum conflicts with exact recipe result binding")
		}
	}
	if err = derived.BindOutput(order.template, payload, results, &out); err != nil {
		return provider.Output{}, err
	}
	return provider.EncodeOperation(o.integration.Policy(), order.cfg, out)
}

func (p *Automatic) Restore(record provider.Record) (provider.Operation, error) {
	var cfg Config
	inner, err := provider.DecodeOperation(p.Policy(), record.Output, &cfg)
	if err != nil {
		return nil, err
	}

	var saved prepared
	if err := derived.StrictJSON(inner.Data, &saved); err != nil {
		return nil, err
	}
	chainID, err := parse.Uint256(saved.ChainID)
	if err != nil || chainID.Sign() == 0 {
		return nil, errors.New("persisted CoW chain invalid")
	}
	s := chainio.Snapshot{ChainID: chainID}

	http, settlement, _, err := ChainDefaults(s.ChainID)
	if err != nil {
		return nil, err
	}
	if cfg.Settlement != settlement.Hex() || !reflect.DeepEqual(cfg.HTTP, http) || cfg.ValiditySeconds != DefaultValiditySeconds {
		return nil, errors.New("persisted CoW automatic constants differ")
	}
	runtime, err := New(cfg, p.deps)
	if err != nil {
		return nil, err
	}

	if saved.Version != 1 || saved.Account == (common.Address{}) || saved.SlippageBPS == 0 || saved.SlippageBPS > 10000 || saved.Order.QuoteID < 0 || saved.Order.AppDataHash != "" {
		return nil, errors.New("persisted CoW identity or policy invalid")
	}
	if _, err := parse.Uint256(saved.PreparedBlock); err != nil {
		return nil, err
	}
	full, app := appData(saved.SlippageBPS)
	if full != saved.FullAppData {
		return nil, errors.New("persisted CoW appData differs")
	}
	if err := runtime.validateOrder(saved.Order, saved.Account, app); err != nil {
		return nil, err
	}
	buffer, err := EncodeOrder(saved.Order)
	if err != nil {
		return nil, err
	}
	domain := protocolDomain(chainID, runtime.settlement)
	digest := protocolio.Digest(domain, crypto.Keccak256Hash(buffer))
	payload, err := account.NewMerkleAccount().TryPackSign(domain, buffer, []account.IMerkleAccountHash{{Offset: big.NewInt(0), Length: big.NewInt(int64(len(buffer))), Destination: big.NewInt(0)}})
	if err != nil || digest != saved.Digest || UID(digest, saved.Account, uint32(saved.Order.ValidTo)) != saved.UID || !bytes.Equal(payload, inner.Actions[cfg.Action]) {
		return nil, errors.New("persisted CoW authorization identity differs")
	}
	expectedInputs := map[uint]string{}
	if cfg.SellAmountInput != nil {
		expectedInputs[*cfg.SellAmountInput] = saved.Order.SellAmount
	}
	if cfg.BuyAmountInput != nil {
		expectedInputs[*cfg.BuyAmountInput] = saved.Order.BuyAmount
	}
	for index, value := range expectedInputs {
		if inner.Inputs[index] != value {
			return nil, errors.New("persisted CoW input differs")
		}
	}
	// Expiry and appData may also be authorized Input bindings. They are
	// selected protocol values rather than financial configuration fields.
	extra := map[string]bool{new(big.Int).SetBytes(buffer[192:224]).String(): true, new(big.Int).SetBytes(buffer[224:256]).String(): true}
	for index, value := range inner.Inputs {
		if _, known := expectedInputs[index]; known {
			continue
		}
		if !extra[value] {
			return nil, errors.New("persisted CoW input scope differs")
		}
		delete(extra, value)
	}

	if record.Progress.Reference != "" && record.Progress.Reference != saved.UID {
		return nil, errors.New("CoW progress UID differs")
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

	http, settlement, _, err := ChainDefaults(s.ChainID)
	if err != nil {
		return provider.Progress{}, err
	}
	if cfg.Settlement != settlement.Hex() || !reflect.DeepEqual(cfg.HTTP, http) || cfg.ValiditySeconds != DefaultValiditySeconds {
		return provider.Progress{}, errors.New("persisted CoW automatic constants differ")
	}
	runtime, err := New(cfg, o.integration.deps)
	if err != nil {
		return provider.Progress{}, err
	}
	var saved prepared
	if err := derived.StrictJSON(inner.Data, &saved); err != nil {
		return provider.Progress{}, err
	}
	if err := chainio.VerifyExecution(ctx, o.integration.deps.Chain, s); err != nil {
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
		if err != nil || !bytes.Equal(data, inner.Actions[cfg.Action]) {
			return provider.Progress{}, errors.New("CoW confirmed Sign differs from persisted order")
		}
		count++
	}
	if count != 1 {
		return provider.Progress{}, errors.New("CoW exact confirmed authorizing Sign event is required")
	}
	return runtime.Advance(ctx, s, inner, state)
}
