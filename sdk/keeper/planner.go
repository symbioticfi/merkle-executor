package keeper

import (
	"bytes"
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/sdk/protocolio"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Backend interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

type Strategy struct {
	Config    Config
	Providers []provider.Provider
}

type Plan struct {
	Snapshot provider.Snapshot `json:"snapshot"`
	Recipe   recipes.Snapshot  `json:"recipe"`
	Calldata hexutil.Bytes     `json:"calldata"`
	Leaves   []common.Hash     `json:"leaves"`
	Outputs  []provider.Output `json:"outputs"`
	Results  map[uint]string   `json:"results,omitempty"`
}

// PermissionSource supplies encoded artifacts; the SDK validates every leaf and
// proof itself. Errors must omit credentials, URLs and remote payloads.
// It is called only during new preparation, never durable recovery.
type PermissionSource interface {
	PermissionTree(context.Context, *big.Int, recipes.Snapshot) ([]byte, error)
}

type Planner struct {
	PermissionSource PermissionSource
	Chain            Backend
	ChainID          *big.Int
	Caller           common.Address
	Confirmations    uint64
	RecipeCache      *recipes.RecipeCache
}

func (p *Planner) Prepare(ctx context.Context, strategy Strategy) (Plan, error) {
	var plan Plan
	if p.PermissionSource != nil && (p.ChainID == nil || p.ChainID.Sign() <= 0 || p.ChainID.BitLen() > 256) {
		return plan, errors.New("permission source requires valid chain ID")
	}
	var preparation *protocolio.Preparation
	var returned []byte
	options := recipes.PreparationOptions{ChainID: p.ChainID, Cache: p.RecipeCache, Stage: func(pin protocolio.PreparationPin) []protocolio.ReadRequest {
		preparation = protocolio.PlanPreparation(strategy.Providers, pin)
		return preparation.Requests()
	}, VerifySimulation: func(raw []byte) error {
		if err := recipes.VerifyResults(plan.Calldata, raw, plan.Results); err != nil {
			return err
		}
		returned = bytes.Clone(raw)
		return nil
	}}
	err := recipes.PrepareExecutionWithOptions(ctx, p.Chain, common.HexToAddress(strategy.Config.Executor), common.HexToAddress(strategy.Config.Account), p.Caller, options, func(snap recipes.Snapshot, recipe recipes.Recipe, staged protocolio.ReadAccessor) ([]byte, []common.Hash, error) {
		if strategy.Config.RecipeHash != "" && snap.RecipeHash != common.HexToHash(strategy.Config.RecipeHash) {
			return nil, nil, errors.New("configured recipe hash mismatch")
		}
		tree, err := p.permissionTree(ctx, strategy.Config.Tree, snap)
		if err != nil {
			return nil, nil, errors.Errorf("load permission tree: %w", err)
		}
		if tree.Root != snap.Root {
			return nil, nil, errors.New("permission tree does not match active account root")
		}
		if p.PermissionSource != nil && tree.ChainID == nil || tree.ChainID != nil && tree.ChainID.Cmp(p.ChainID) != 0 {
			return nil, nil, errors.New("permission tree chain mismatch")
		}
		if tree.Account != nil && *tree.Account != snap.Account {
			return nil, nil, errors.New("permission tree account mismatch")
		}
		plan.Recipe = snap
		plan.Snapshot = provider.Snapshot{ChainID: new(big.Int).Set(p.ChainID), BlockNumber: new(big.Int).SetUint64(snap.BlockNumber), BlockHash: snap.BlockHash, Timestamp: snap.Timestamp, Executor: snap.Executor, Account: snap.Account, Caller: snap.Caller, RecipeHash: snap.RecipeHash, Root: snap.Root}
		inputs := make(map[uint]string)
		actions := make(map[string]hexutil.Bytes)
		var skips []uint
		var expressions []recipes.Value
		owned := make(map[uint]bool)
		actionOwners := map[string]bool{}
		authenticated := false
		requirements := make([]RecipeRequirements, len(strategy.Providers))
		ready := make([]bool, len(strategy.Providers))
		plan.Outputs = make([]provider.Output, len(strategy.Providers))
		prefix := recipes.PrefixRuntime{Tree: tree, Inputs: inputs, Actions: actions, AvailableInputs: map[uint]bool{}, BlockedActions: map[string]bool{}}
		for i, runtime := range strategy.Providers {
			if rp, ok := runtime.(RecipeProvider); ok {
				authenticated = true
				requirements[i], err = rp.RecipeRequirements(plan.Snapshot, recipes.CloneRecipe(recipe), recipes.CloneTree(tree))
				if err != nil {
					return nil, nil, errors.Errorf("invalid authenticated recipe: %w", err)
				}
				expressions = append(expressions, requirements[i].Values...)
				for _, index := range requirements[i].Inputs {
					if owned[index] {
						return nil, nil, errors.New("multiple providers own the same recipe input")
					}
					owned[index] = true
					if len(requirements[i].Values) == 0 {
						prefix.AvailableInputs[index] = true
					}
				}
				for _, key := range requirements[i].Actions {
					if actionOwners[key] {
						return nil, nil, errors.New("multiple providers own the same recipe action")
					}
					actionOwners[key] = true
					if len(requirements[i].Values) > 0 {
						prefix.BlockedActions[key] = true
					}
				}
			}
		}
		anchors := map[uint]recipes.EconomicValue{}
		if authenticated {
			if err = ValidateRecipeComposition(recipe, tree, requirements); err != nil {
				return nil, nil, err
			}
			if err = recipes.ValidateMutableActions(recipe, tree, actionOwners); err != nil {
				return nil, nil, err
			}
			anchors, err = recipes.AnchoredInputs(recipe, owned)
			if err != nil {
				return nil, nil, err
			}
			for _, anchor := range anchors {
				expressions = append(expressions, recipes.EconomicExpressions(anchor)...)
			}
			if err = recipes.ValidatePrefix(recipe, expressions, prefix); err != nil {
				return nil, nil, err
			}
		}
		resolve := func(i int, results map[uint]*big.Int) error {
			var out provider.Output
			if rp, ok := strategy.Providers[i].(RecipeProvider); ok {
				copyResults := make(map[uint]*big.Int, len(results))
				for index, value := range results {
					copyResults[index] = new(big.Int).Set(value)
				}
				out, err = rp.PrepareRecipe(ctx, plan.Snapshot, recipes.CloneRecipe(recipe), recipes.CloneTree(tree), copyResults)
			} else {
				out, err = preparation.Resolve(ctx, i, plan.Snapshot, staged)
			}
			if err != nil {
				return errors.Errorf("resolve runtime: %w", err)
			}
			if err = merge(inputs, actions, &skips, out); err != nil {
				return err
			}
			plan.Outputs[i] = out
			ready[i] = true
			return nil
		}
		for i := range strategy.Providers {
			if len(requirements[i].Values) == 0 {
				if err = resolve(i, nil); err != nil {
					return nil, nil, err
				}
			}
		}
		resolved := map[uint]*big.Int{}
		if authenticated {
			prefix.SkipSteps = skips
			prefix.AvailableInputs = nil
			resolved, err = recipes.ResolveValuesWithRuntime(ctx, p.Chain, snap, recipe, expressions, prefix)
			if err != nil {
				return nil, nil, err
			}
			for index, anchor := range anchors {
				value, e := recipes.EconomicResult(anchor, resolved)
				if e != nil {
					return nil, nil, e
				}
				inputs[index] = value.String()
			}
			plan.Results = recipes.ResultsWitness(resolved)
		}
		for i := range strategy.Providers {
			if !ready[i] {
				if err = resolve(i, resolved); err != nil {
					return nil, nil, err
				}
			}
		}
		plan.Calldata, plan.Leaves, err = recipes.Assemble(recipe, tree, inputs, actions, skips)
		if err != nil {
			return nil, nil, errors.Errorf("assemble runtime: %w", err)
		}
		if err := ValidatePlanComposition(plan.Calldata, strategy.Providers, plan.Outputs); err != nil {
			return nil, nil, err
		}
		return plan.Calldata, plan.Leaves, nil
	})
	if err != nil {
		return plan, err
	}
	// Readiness is meaningful only after full simulation and both canonical guards.
	work, err := recipes.HasWork(plan.Calldata, returned)
	if err != nil {
		return plan, err
	}
	if !work {
		return plan, provider.ErrNotReady
	}
	return plan, nil
}

func (p *Planner) permissionTree(ctx context.Context, path string, snapshot recipes.Snapshot) (*recipes.Tree, error) {
	if p.PermissionSource == nil {
		return recipes.LoadTree(path)
	}
	if p.ChainID == nil || p.ChainID.Sign() <= 0 {
		return nil, errors.New("permission source requires chain ID")
	}
	raw, err := p.PermissionSource.PermissionTree(ctx, new(big.Int).Set(p.ChainID), snapshot)
	if err != nil {
		return nil, errors.Errorf("permission source failed: %w", err)
	}
	return recipes.DecodeTree(raw)
}

func merge(inputs map[uint]string, actions map[string]hexutil.Bytes, skips *[]uint, out provider.Output) error {
	for index, value := range out.Inputs {
		if index > recipes.MaxInputIndex {
			return errors.New("input index exceeds limit")
		}
		if _, ok := inputs[index]; ok {
			return errors.Errorf("conflicting providers for input %d", index)
		}
		inputs[index] = value
	}
	for key, value := range out.Actions {
		if _, ok := actions[key]; ok {
			return errors.Errorf("conflicting providers for action %s", key)
		}
		actions[key] = value
	}
	for _, step := range out.SkipSteps {
		for _, old := range *skips {
			if old == step {
				return errors.New("duplicate skipped step")
			}
		}
		*skips = append(*skips, step)
	}
	return nil
}

// Validate re-simulates immutable prepared bytes at current state before admission.
func (p *Planner) Validate(ctx context.Context, plan Plan) error {
	if plan.Snapshot.Executor != plan.Recipe.Executor {
		return errors.New("prepared plan executor mismatch")
	}
	work, err := recipes.ExecutionHasWork(ctx, p.Chain, plan.Recipe, p.Caller, plan.Calldata, plan.Results)
	if err != nil {
		return err
	}
	if !work {
		return provider.ErrNotReady
	}
	return nil
}

func (p *Planner) ConfirmedSnapshot(ctx context.Context, original provider.Snapshot) (provider.Snapshot, error) {
	witness, err := p.Chain.HeaderByNumber(ctx, nil)
	if err != nil {
		return original, errors.Errorf("read confirmation head: %w", err)
	}
	if err := validHeader(witness, nil); err != nil {
		return original, err
	}
	number := new(big.Int).Sub(witness.Number, new(big.Int).SetUint64(p.Confirmations))
	if number.Sign() < 0 {
		return original, errors.New("insufficient blocks for confirmations")
	}
	header, err := p.confirmedAncestor(ctx, witness, number)
	if err != nil {
		return original, err
	}
	original.BlockNumber = new(big.Int).Set(header.Number)
	original.BlockHash = header.Hash()
	original.Timestamp = header.Time
	if original.Execution != nil {
		if err := protocolio.Execution(ctx, p.Chain, original); err != nil {
			return original, err
		}
	}
	return original, nil
}

// VerifyConfirmedSnapshot checks provider reads still refer to a canonical block
// with the configured number of descendants before their progress is committed.
func (p *Planner) VerifyConfirmedSnapshot(ctx context.Context, snapshot provider.Snapshot) error {
	if snapshot.BlockNumber == nil || snapshot.BlockNumber.Sign() < 0 || snapshot.BlockHash == (common.Hash{}) {
		return errors.New("invalid confirmed snapshot")
	}
	witnessNumber := new(big.Int).Add(snapshot.BlockNumber, new(big.Int).SetUint64(p.Confirmations))
	witness, err := p.Chain.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return errors.Errorf("read snapshot confirmation block: %w", err)
	}
	if err := validHeader(witness, witnessNumber); err != nil {
		return err
	}
	header, err := p.confirmedAncestor(ctx, witness, snapshot.BlockNumber)
	if err != nil {
		return err
	}
	if header.Hash() != snapshot.BlockHash || header.Time != snapshot.Timestamp {
		return errors.New("confirmed snapshot was reorganized")
	}
	if snapshot.Execution != nil {
		return protocolio.Execution(ctx, p.Chain, snapshot)
	}
	return nil
}

func (p *Planner) confirmedAncestor(ctx context.Context, witness *types.Header, number *big.Int) (*types.Header, error) {
	ancestor := witness
	for ancestor.Number.Cmp(number) > 0 {
		parent, err := p.Chain.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, errors.Errorf("read confirmation ancestor: %w", err)
		}
		expected := new(big.Int).Sub(ancestor.Number, big.NewInt(1))
		if err := validHeader(parent, expected); err != nil {
			return nil, err
		}
		if parent.Hash() != ancestor.ParentHash {
			return nil, errors.New("inconsistent confirmation ancestry")
		}
		ancestor = parent
	}
	stable, err := p.Chain.HeaderByNumber(ctx, witness.Number)
	if err != nil {
		return nil, errors.Errorf("recheck canonical confirmation block: %w", err)
	}
	if err := validHeader(stable, witness.Number); err != nil {
		return nil, err
	}
	if stable.Hash() != witness.Hash() {
		return nil, errors.New("confirmation block was reorganized")
	}
	return ancestor, nil
}

func validHeader(header *types.Header, number *big.Int) error {
	if header == nil || header.Number == nil || !header.Number.IsUint64() {
		return errors.New("invalid confirmation header")
	}
	if number != nil && header.Number.Cmp(number) != 0 {
		return errors.New("RPC returned another confirmation block")
	}
	return nil
}
