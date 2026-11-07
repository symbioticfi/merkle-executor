package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Config struct {
	Name, Executor, Account, RecipeHash, Tree string
	Interval                                  time.Duration
	Revision                                  string
}

func (c Config) Fingerprint() string {
	raw, _ := json.Marshal(c)
	return crypto.Keccak256Hash(raw).Hex()
}

type Strategy struct {
	Config           Config
	Integrations     []provider.Integration
	LegacyConfigHash string
}
type Plan struct {
	Snapshot    chainio.Snapshot  `json:"snapshot"`
	RecipeBlock uint64            `json:"recipeBlock"`
	Calldata    hexutil.Bytes     `json:"calldata"`
	Leaves      []common.Hash     `json:"leaves"`
	Results     map[uint]string   `json:"results,omitempty"`
	Operations  []provider.Record `json:"operations,omitempty"`
}

func (p Plan) RecipeSnapshot() recipes.Snapshot {
	return recipes.Snapshot{BlockNumber: p.Snapshot.BlockNumber.Uint64(), BlockHash: p.Snapshot.BlockHash, Timestamp: p.Snapshot.Timestamp, Executor: p.Snapshot.Executor, Account: p.Snapshot.Account, Caller: p.Snapshot.Caller, RecipeHash: p.Snapshot.RecipeHash, Root: p.Snapshot.Root, RecipeBlock: p.RecipeBlock}
}
func (p Plan) Clone() Plan {
	p.Snapshot = provider.Program{Snapshot: p.Snapshot}.Clone().Snapshot
	p.Calldata = bytes.Clone(p.Calldata)
	p.Leaves = slices.Clone(p.Leaves)
	if p.Results != nil {
		results := make(map[uint]string, len(p.Results))
		for i, v := range p.Results {
			results[i] = v
		}
		p.Results = results
	}
	p.Operations = slices.Clone(p.Operations)
	for i := range p.Operations {
		p.Operations[i] = p.Operations[i].Clone()
	}
	return p
}

type PermissionSource interface {
	PermissionTree(context.Context, *big.Int, recipes.Snapshot) ([]byte, error)
}
type Backend interface {
	recipes.Reader
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
}

type Planner struct {
	PermissionSource PermissionSource
	Chain            Backend
	ChainID          *big.Int
	Caller           common.Address
	Confirmations    uint64
	RecipeCache      *recipes.RecipeCache
	MaxConcurrent    int
}
type PreparationResult struct {
	Plan Plan
	Err  error
}
type compiledOperation struct {
	operation provider.Operation
	record    provider.Record
}

func compileOperations(program provider.Program, integrations []provider.Integration) ([]compiledOperation, error) {
	ids := map[string]bool{}
	for _, in := range integrations {
		if in == nil || in.ID() == "" || in.Policy() == "" || strings.TrimSpace(in.ID()) != in.ID() || ids[in.ID()] {
			return nil, errors.New("invalid or duplicate integration identity")
		}
		ids[in.ID()] = true
	}
	var compiled []compiledOperation
	inputs, actions := map[uint]bool{}, map[string]bool{}
	for _, in := range integrations {
		operations, err := in.Compile(program.Clone())
		if err != nil {
			return nil, errors.Errorf("compile integration %s: %w", in.ID(), err)
		}
		seen := map[string]bool{}
		for _, operation := range operations {
			if operation == nil || operation.ID() == "" || seen[operation.ID()] {
				return nil, errors.New("invalid or duplicate operation identity")
			}
			seen[operation.ID()] = true
			req := operation.Requirements().Clone()
			for _, index := range req.Inputs {
				if index > recipes.MaxInputIndex || inputs[index] {
					return nil, errors.New("multiple operations own the same or invalid recipe input")
				}
				inputs[index] = true
			}
			for _, key := range req.Actions {
				if err := parseAction(key); err != nil {
					return nil, err
				}
				if actions[key] {
					return nil, errors.New("multiple operations own the same recipe action")
				}
				actions[key] = true
			}
			for key := range req.ExclusiveTargets {
				if !slices.Contains(req.Actions, key) {
					return nil, errors.New("exclusive target claim has no owned action")
				}
			}
			for _, value := range req.Values {
				if value.Value == nil || value.Value.Sign() < 0 || value.Value.BitLen() > 256 {
					return nil, errors.New("invalid numerical requirement")
				}
			}
			compiled = append(compiled, compiledOperation{operation: operation, record: provider.Record{Integration: in.ID(), Policy: in.Policy(), ID: operation.ID(), Requirements: req}})
		}
	}
	slices.SortFunc(compiled, func(a, b compiledOperation) int {
		if c := strings.Compare(a.record.Integration, b.record.Integration); c != 0 {
			return c
		}
		return strings.Compare(a.record.ID, b.record.ID)
	})
	return compiled, nil
}
func parseAction(key string) error {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return errors.New("invalid recipe action key")
	}
	for _, part := range parts {
		value, ok := new(big.Int).SetString(part, 10)
		if !ok || value.Sign() < 0 || !value.IsUint64() || value.String() != part {
			return errors.New("invalid recipe action key")
		}
	}
	return nil
}
func validateOutput(req provider.Requirements, out provider.Output) error {
	inputs, actions := map[uint]bool{}, map[string]bool{}
	for _, index := range req.Inputs {
		inputs[index] = true
	}
	for _, key := range req.Actions {
		actions[key] = true
	}
	for index, value := range out.Inputs {
		if !inputs[index] {
			return errors.New("operation assigned an undeclared input")
		}
		number, err := parse.Uint256(value)
		if err != nil || number.String() != value {
			return errors.New("operation input is not canonical uint256")
		}
	}
	for key := range out.Actions {
		if !actions[key] {
			return errors.New("operation assigned an undeclared action")
		}
	}
	for _, step := range out.SkipSteps {
		prefix := new(big.Int).SetUint64(uint64(step)).String() + ":"
		owned := false
		for key := range actions {
			owned = owned || strings.HasPrefix(key, prefix)
		}
		if !owned {
			return errors.New("operation skipped an unowned step")
		}
	}
	return nil
}

func (p *Planner) Prepare(ctx context.Context, strategy Strategy) (Plan, error) {
	return p.prepare(ctx, strategy, nil)
}
func (p *Planner) PrepareAll(ctx context.Context, strategies []Strategy) []PreparationResult {
	results := make([]PreparationResult, len(strategies))
	if len(results) == 0 {
		return results
	}
	if p.Chain == nil {
		for i := range results {
			results[i].Err = errors.New("planner chain required")
		}
		return results
	}
	pin, err := p.Chain.HeaderByNumber(ctx, nil)
	if err != nil || pin == nil || pin.Number == nil || !pin.Number.IsUint64() {
		for i := range results {
			results[i].Err = errors.New("pin preparation block failed")
		}
		return results
	}
	runBounded(len(strategies), p.MaxConcurrent, func(i int) {
		if err := ctx.Err(); err != nil {
			results[i].Err = err
			return
		}
		results[i].Plan, results[i].Err = p.prepare(ctx, strategies[i], types.CopyHeader(pin))
	})
	return results
}
func (p *Planner) prepare(ctx context.Context, strategy Strategy, pin *types.Header) (Plan, error) {
	var plan Plan
	if p.Chain == nil || p.ChainID == nil || p.ChainID.Sign() <= 0 || p.ChainID.BitLen() > 256 {
		return plan, errors.New("planner requires chain and positive uint256 chain ID")
	}
	var returned []byte
	options := recipes.PreparationOptions{ChainID: p.ChainID, Cache: p.RecipeCache, Pin: pin, VerifySimulation: func(raw []byte) error {
		if err := recipes.VerifyResults(plan.Calldata, raw, plan.Results); err != nil {
			return err
		}
		returned = bytes.Clone(raw)
		return nil
	}}
	err := recipes.PrepareExecutionWithOptions(ctx, p.Chain, common.HexToAddress(strategy.Config.Executor), common.HexToAddress(strategy.Config.Account), p.Caller, options, func(snapshot recipes.Snapshot, recipe recipes.Recipe, _ chainio.ReadAccessor) ([]byte, []common.Hash, error) {
		if strategy.Config.RecipeHash != "" && snapshot.RecipeHash != common.HexToHash(strategy.Config.RecipeHash) {
			return nil, nil, errors.New("configured recipe hash mismatch")
		}
		tree, err := p.permissionTree(ctx, strategy.Config.Tree, snapshot)
		if err != nil {
			return nil, nil, err
		}
		if tree.Root != snapshot.Root || p.PermissionSource != nil && tree.ChainID == nil || tree.ChainID != nil && tree.ChainID.Cmp(p.ChainID) != 0 || tree.Account != nil && *tree.Account != snapshot.Account {
			return nil, nil, errors.New("permission tree does not match authenticated chain, account or root")
		}
		plan.Snapshot = chainio.Snapshot{ChainID: new(big.Int).Set(p.ChainID), BlockNumber: new(big.Int).SetUint64(snapshot.BlockNumber), BlockHash: snapshot.BlockHash, Timestamp: snapshot.Timestamp, Executor: snapshot.Executor, Account: snapshot.Account, Caller: snapshot.Caller, RecipeHash: snapshot.RecipeHash, Root: snapshot.Root}
		plan.RecipeBlock = snapshot.RecipeBlock
		program := provider.Program{Snapshot: plan.Snapshot, Recipe: recipe, Tree: tree}
		compiled, err := compileOperations(program, strategy.Integrations)
		if err != nil {
			return nil, nil, err
		}
		reqs := make([]provider.Requirements, len(compiled))
		owned, actionOwners := map[uint]bool{}, map[string]bool{}
		inputs, actions := map[uint]string{}, map[string]hexutil.Bytes{}
		var skips []uint
		var expressions []recipes.Value
		prefix := recipes.PrefixRuntime{Tree: tree, Inputs: inputs, Actions: actions, AvailableInputs: map[uint]bool{}, BlockedActions: map[string]bool{}}
		for i, c := range compiled {
			reqs[i] = c.record.Requirements
			expressions = append(expressions, reqs[i].Values...)
			for _, index := range reqs[i].Inputs {
				owned[index] = true
				if len(reqs[i].Values) == 0 {
					prefix.AvailableInputs[index] = true
				}
			}
			for _, key := range reqs[i].Actions {
				actionOwners[key] = true
				if len(reqs[i].Values) > 0 {
					prefix.BlockedActions[key] = true
				}
			}
		}
		if err = ValidateComposition(recipe, tree, reqs); err != nil {
			return nil, nil, err
		}
		if err = recipes.ValidateMutableActions(recipe, tree, actionOwners); err != nil {
			return nil, nil, err
		}
		anchors, err := recipes.AnchoredInputs(recipe, owned)
		if err != nil {
			return nil, nil, err
		}
		for _, anchor := range anchors {
			expressions = append(expressions, recipes.EconomicExpressions(anchor)...)
		}
		if err = recipes.ValidatePrefix(recipe, expressions, prefix); err != nil {
			return nil, nil, err
		}
		runWave := func(numerical bool, values map[uint]*big.Int) error {
			outputs := make([]provider.Output, len(compiled))
			errs := make([]error, len(compiled))
			runBounded(len(compiled), p.MaxConcurrent, func(i int) {
				if (len(reqs[i].Values) > 0) != numerical {
					return
				}
				if err := ctx.Err(); err != nil {
					errs[i] = err
					return
				}
				copied := make(map[uint]*big.Int, len(values))
				for index, value := range values {
					copied[index] = new(big.Int).Set(value)
				}
				out, err := compiled[i].operation.Prepare(ctx, provider.Preparation{Program: program.Clone(), Results: copied, Reader: p.Chain})
				if err == nil {
					err = validateOutput(reqs[i], out)
				}
				if err == nil {
					err = validateSkipOwnership(recipe, reqs[i], out)
				}
				outputs[i], errs[i] = out.Clone(), err
			})
			for i := range compiled {
				if (len(reqs[i].Values) > 0) != numerical {
					continue
				}
				if errs[i] != nil {
					return errors.Errorf("prepare operation %s/%s: %w", compiled[i].record.Integration, compiled[i].record.ID, errs[i])
				}
				if err := mergeOutputs(inputs, actions, &skips, outputs[i]); err != nil {
					return err
				}
				compiled[i].record.Output = outputs[i]
			}
			return nil
		}
		if err = runWave(false, nil); err != nil {
			return nil, nil, err
		}
		prefix.SkipSteps = skips
		prefix.AvailableInputs = nil
		values, err := recipes.ResolveValuesWithRuntime(ctx, p.Chain, snapshot, recipe, expressions, prefix)
		if err != nil {
			return nil, nil, err
		}
		for index, anchor := range anchors {
			value, err := recipes.EconomicResult(anchor, values)
			if err != nil {
				return nil, nil, err
			}
			inputs[index] = value.String()
		}
		plan.Results = recipes.ResultsWitness(values)
		if err = runWave(true, values); err != nil {
			return nil, nil, err
		}
		plan.Calldata, plan.Leaves, err = recipes.Assemble(recipe, tree, inputs, actions, skips)
		if err != nil {
			return nil, nil, err
		}
		plan.Operations = make([]provider.Record, len(compiled))
		for i, c := range compiled {
			plan.Operations[i] = c.record.Clone()
		}
		if err = ValidatePlanComposition(plan.Calldata, plan.Operations); err != nil {
			return nil, nil, err
		}
		return plan.Calldata, plan.Leaves, nil
	})
	if err != nil {
		return Plan{}, err
	}
	work, err := recipes.HasWork(plan.Calldata, returned)
	if err != nil {
		return Plan{}, err
	}
	if !work {
		return Plan{}, provider.ErrNotReady
	}
	return plan, nil
}
func (p *Planner) permissionTree(ctx context.Context, path string, snapshot recipes.Snapshot) (*recipes.Tree, error) {
	if p.PermissionSource == nil {
		return recipes.LoadTree(path)
	}
	raw, err := p.PermissionSource.PermissionTree(ctx, new(big.Int).Set(p.ChainID), snapshot)
	if err != nil {
		return nil, errors.Errorf("permission source failed: %w", err)
	}
	return recipes.DecodeTree(raw)
}
func mergeOutputs(inputs map[uint]string, actions map[string]hexutil.Bytes, skips *[]uint, out provider.Output) error {
	for index, value := range out.Inputs {
		if _, ok := inputs[index]; ok {
			return errors.New("conflicting operation input")
		}
		inputs[index] = value
	}
	for key, value := range out.Actions {
		if _, ok := actions[key]; ok {
			return errors.New("conflicting operation action")
		}
		actions[key] = bytes.Clone(value)
	}
	for _, step := range out.SkipSteps {
		if slices.Contains(*skips, step) {
			return errors.New("duplicate skipped step")
		}
		*skips = append(*skips, step)
	}
	return nil
}
func (p *Planner) Validate(ctx context.Context, plan Plan) error {
	if err := validatePlanIdentity(plan); err != nil {
		return err
	}
	if plan.Snapshot.Caller != p.Caller || plan.Snapshot.ChainID.Cmp(p.ChainID) != 0 {
		return errors.New("prepared caller or chain mismatch")
	}
	work, err := recipes.ExecutionHasWork(ctx, p.Chain, plan.RecipeSnapshot(), p.Caller, plan.Calldata, plan.Results)
	if err != nil {
		return err
	}
	if !work {
		return provider.ErrNotReady
	}
	return nil
}
