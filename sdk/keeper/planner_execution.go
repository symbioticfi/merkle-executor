package keeper

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"slices"
)

// PrepareExecution authenticates candidates together and validates one selected
// envelope. Independent reads batch across executors; EVM effects retain order.
func (p *Planner) PrepareExecution(ctx context.Context, strategies []Strategy) (ExecutionPlan, error) {
	var batch ExecutionPlan
	if p.ChainID == nil || p.ChainID.Sign() <= 0 || p.ChainID.BitLen() > 256 {
		return batch, errors.New("positive uint256 chain ID required")
	}
	if len(strategies) == 0 {
		return batch, provider.ErrNotReady
	}
	strategies = slices.Clone(strategies)
	names := make(map[string]bool, len(strategies))
	for i := range strategies {
		if strategies[i].Config.Name == "" {
			strategies[i].Config.Name = strategies[i].Config.Executor
		}
		name := strategies[i].Config.Name
		if name == "" || names[name] {
			return batch, errors.New("planner strategy names must be nonempty and unique")
		}
		names[name] = true
	}
	failures := map[string]error{}
	executors, accounts := make([]common.Address, len(strategies)), make([]common.Address, len(strategies))
	for i, s := range strategies {
		executors[i] = common.HexToAddress(s.Config.Executor)
		accounts[i] = common.HexToAddress(s.Config.Account)
	}
	discovered, err := recipes.DiscoverCandidates(ctx, p.Chain, executors, accounts, p.Caller, recipes.PreparationOptions{ChainID: p.ChainID, Cache: p.RecipeCache})
	if err != nil {
		return batch, err
	}
	reader, supportsBatch := p.Chain.(recipes.BatchReader)
	delegated := false
	var block *big.Int
	for i, result := range discovered {
		if result.Err != nil {
			failures[strategies[i].Config.Name] = result.Err
			continue
		}
		if block == nil {
			block = new(big.Int).SetUint64(result.Prepared.Snapshot.BlockNumber)
		}
	}
	if block == nil {
		return ExecutionPlan{}, candidateFailures(failures)
	}
	batch.Failures = failures
	if supportsBatch && len(strategies) > 1 {
		delegated, err = recipes.SupportsDelegatedBatch(ctx, reader, p.Caller, block)
		if err != nil {
			return batch, err
		}
	}
	header, err := p.Chain.HeaderByNumber(ctx, block)
	if err != nil {
		return batch, err
	}
	if err = validHeader(header, block); err != nil {
		return batch, err
	}
	type treeKey struct {
		account common.Address
		root    common.Hash
		path    string
	}
	trees := map[treeKey]*recipes.Tree{}
	var selected []recipes.PreparedRecipe
	reserved := map[common.Address]bool{}
	composition := newExecutionComposition()
	for i, strategy := range strategies {
		if err := ctx.Err(); err != nil {
			return ExecutionPlan{}, err
		}
		if discovered[i].Err != nil {
			continue
		}
		if reserved[accounts[i]] {
			continue
		}
		snap := discovered[i].Prepared.Snapshot
		key := treeKey{snap.Account, snap.Root, strategy.Config.Tree}
		tree := trees[key]
		if tree == nil {
			tree, err = p.permissionTree(ctx, strategy.Config.Tree, snap)
			if err != nil {
				failures[strategy.Config.Name] = err
				continue
			}
			trees[key] = tree
		}
		plan, repeat, buildErr := p.buildPlan(ctx, strategy, snap, recipes.CloneRecipe(discovered[i].Prepared.Recipe), recipes.CloneTree(tree))
		if errors.Is(buildErr, provider.ErrNotReady) {
			continue
		}
		if buildErr != nil {
			failures[strategy.Config.Name] = buildErr
			continue
		}
		if repeat && supportsBatch {
			repeated, batchErr := recipes.BatchExecution(ctx, reader, snap, plan.Calldata, header.GasLimit)
			err = batchErr
			if errors.Is(err, provider.ErrNotReady) {
				if !delegated {
					continue
				}
				// Earlier members may fund an otherwise idle recipe.
				err = nil
			} else if err == nil {
				plan.Calldata = repeated
			}
			if err != nil {
				failures[strategy.Config.Name] = err
				continue
			}
		}
		name := strategy.Config.Name
		if err = composition.add(plan, strategy); err != nil {
			if len(batch.Members) > 0 {
				continue
			}
			failures[strategy.Config.Name] = err
			continue
		}
		batch.To = p.Caller
		batch.Members = append(batch.Members, PlanMember{Name: name, Plan: plan})
		selected = append(selected, discovered[i].Prepared)
		for _, group := range plan.Outputs {
			for _, out := range group {
				if out.Task != nil {
					reserved[snap.Account] = true
				}
			}
		}
		if !delegated {
			batch.To = snap.Executor
			calls, leaves, witnesses := batch.validation()
			err = recipes.ValidatePreparedBatch(ctx, p.Chain, selected, calls, leaves, witnesses, p.Caller)
			if errors.Is(err, provider.ErrNotReady) {
				batch = ExecutionPlan{Failures: failures}
				selected = nil
				composition = newExecutionComposition()
				clear(reserved)
				continue
			}
			if err == nil {
				return batch, nil
			}
			fitted, fitErr := p.fit(ctx, batch, selected, strategies, err)
			var member *recipes.MemberError
			if fitErr == nil {
				return fitted, nil
			}
			if !recipes.BatchRejected(fitErr) && !errors.As(fitErr, &member) {
				return ExecutionPlan{}, fitErr
			}
			failures[strategy.Config.Name] = fitErr
			batch = ExecutionPlan{Failures: failures}
			selected = nil
			composition = newExecutionComposition()
			clear(reserved)
		}
	}
	if err := ctx.Err(); err != nil {
		return ExecutionPlan{}, err
	}
	if len(batch.Members) == 0 {
		if len(failures) > 0 {
			return batch, candidateFailures(failures)
		}
		return batch, provider.ErrNotReady
	}
	return p.fit(ctx, batch, selected, strategies, nil)
}

func candidateFailures(failures map[string]error) error {
	var joined []error
	for name, err := range failures {
		joined = append(joined, errors.Errorf("executor %s: %w", name, err))
	}
	return errors.Join(joined...)
}

func (p *Planner) fit(ctx context.Context, batch ExecutionPlan, selected []recipes.PreparedRecipe, strategies []Strategy, prior error) (ExecutionPlan, error) {
	for len(batch.Members) > 0 {
		if len(batch.Members) == 1 {
			batch.To = batch.Members[0].Plan.Snapshot.Executor
		}
		calls, leaves, witnesses := batch.validation()
		err := prior
		prior = nil
		if err == nil {
			err = recipes.ValidatePreparedBatch(ctx, p.Chain, selected, calls, leaves, witnesses, p.Caller)
		}
		if err == nil {
			return batch, nil
		}
		var member *recipes.MemberError
		if errors.As(err, &member) && member.Index >= 0 && member.Index < len(batch.Members) {
			if !errors.Is(err, provider.ErrNotReady) && len(batch.Members) == 1 {
				return ExecutionPlan{}, err
			}
			indices := member.Indices
			if len(indices) == 0 {
				indices = []int{member.Index}
			}
			drop := make(map[int]bool, len(indices))
			for _, index := range indices {
				drop[index] = true
			}
			members, prepared := batch.Members[:0], selected[:0]
			for i, entry := range batch.Members {
				if drop[i] && !errors.Is(err, provider.ErrNotReady) {
					if batch.Failures == nil {
						batch.Failures = map[string]error{}
					}
					batch.Failures[entry.Name] = err
				}
				if !drop[i] {
					members = append(members, entry)
					prepared = append(prepared, selected[i])
				}
			}
			batch.Members, selected = members, prepared
			continue
		}
		if !recipes.BatchRejected(err) {
			return ExecutionPlan{}, err
		}
		if len(batch.Members) > 1 {
			// EVM prefixes need not be monotone; test descending prefixes to retain
			// the largest passing configured-order prefix within the operation budget.
			count := len(batch.Members) - 1
			batch.Members = batch.Members[:count]
			selected = selected[:count]
			continue
		}
		plan := &batch.Members[0].Plan
		entries, e := recipes.Executions(plan.Calldata)
		if e != nil || len(entries) < 2 {
			return ExecutionPlan{}, err
		}
		count := len(entries) - 1
		if count == 1 {
			plan.Calldata = entries[0]
		} else {
			plan.Calldata, e = recipes.Multicall(entries[:count])
			if e != nil {
				return ExecutionPlan{}, e
			}
		}
		for i, group := range plan.Outputs {
			if len(group) > 1 {
				plan.Outputs[i] = group[:count]
			}
		}
		if e = validateExecutionComposition(batch, strategies); e != nil {
			return ExecutionPlan{}, e
		}
	}
	if len(batch.Failures) != 0 {
		return ExecutionPlan{Failures: batch.Failures}, candidateFailures(batch.Failures)
	}
	return ExecutionPlan{}, provider.ErrNotReady
}

func (b ExecutionPlan) validation() ([]recipes.ExecutorCall, [][]common.Hash, []map[uint]string) {
	calls := make([]recipes.ExecutorCall, len(b.Members))
	leaves := make([][]common.Hash, len(b.Members))
	witnesses := make([]map[uint]string, len(b.Members))
	for i, m := range b.Members {
		calls[i] = recipes.ExecutorCall{Executor: m.Plan.Snapshot.Executor, Data: m.Plan.Calldata}
		leaves[i] = m.Plan.Leaves
		witnesses[i] = m.Plan.Results
	}
	return calls, leaves, witnesses
}

// Validate selected plans in linear time, including transaction-wide receipt ownership.
func validateExecutionComposition(batch ExecutionPlan, strategies []Strategy) error {
	configured := make(map[string]Strategy, len(strategies))
	for _, strategy := range strategies {
		configured[strategy.Config.Name] = strategy
	}
	composition := newExecutionComposition()
	for _, member := range batch.Members {
		strategy, ok := configured[member.Name]
		if !ok {
			return errors.New("batch contains an unconfigured strategy")
		}
		if err := composition.add(member.Plan, strategy); err != nil {
			return err
		}
	}
	return nil
}
