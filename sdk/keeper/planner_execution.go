package keeper

import (
	"context"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// PrepareExecution preserves configuration order. Idle on-chain gates are evaluated
// again after the selected prefix, so earlier proceeds can activate later work.
func (p *Planner) PrepareExecution(ctx context.Context, strategies []Strategy) (ExecutionPlan, error) {
	batch := ExecutionPlan{To: p.Caller}
	reader, supportsBatch := p.Chain.(recipes.BatchReader)
	delegated := false
	if supportsBatch {
		var err error
		delegated, err = recipes.SupportsDelegatedBatch(ctx, reader, p.Caller, nil)
		if err != nil {
			return batch, err
		}
	}
	reserved := map[common.Address]bool{}
	for _, strategy := range strategies {
		if err := ctx.Err(); err != nil {
			return batch, err
		}
		if reserved[common.HexToAddress(strategy.Config.Account)] {
			continue
		}
		plan, err := p.Prepare(ctx, strategy)
		if err != nil && !errors.Is(err, provider.ErrNotReady) {
			return batch, err
		}
		if len(plan.Calldata) == 0 {
			continue
		}
		if !delegated {
			if errors.Is(err, provider.ErrNotReady) {
				continue
			}
			return ExecutionPlan{To: plan.Snapshot.Executor, Members: []PlanMember{{Name: strategy.Config.Name, Plan: plan}}}, nil
		}
		candidate := ExecutionPlan{To: p.Caller, Members: append(append([]PlanMember(nil), batch.Members...), PlanMember{Name: strategy.Config.Name, Plan: plan})}
		if err := validateExecutionComposition(candidate, strategies); err != nil {
			// Keep exclusive receipt evidence in separate transactions.
			if len(batch.Members) != 0 {
				continue
			}
			return batch, err
		}
		header, err := p.Chain.HeaderByNumber(ctx, nil)
		if err != nil {
			return batch, err
		}
		if err = validHeader(header, nil); err != nil {
			return batch, err
		}
		calls, _, _ := candidate.calls()
		results, err := recipes.SimulateExecutors(ctx, reader, p.Caller, calls, header.Number, header.GasLimit)
		if err != nil {
			if recipes.BatchRejected(err) && len(batch.Members) != 0 {
				continue
			}
			return batch, err
		}
		last := len(results) - 1
		if len(plan.Results) != 0 {
			if err := recipes.VerifyResults(plan.Calldata, results[last], plan.Results); err != nil {
				// State-bound runtime data must be prepared in the next transaction,
				// never reused against a prefix that changes its witnessed inputs.
				if len(batch.Members) != 0 {
					continue
				}
				return batch, err
			}
		}
		work, err := recipes.HasWork(plan.Calldata, results[last])
		if err != nil {
			return batch, err
		}
		if !work {
			continue
		}
		batch = candidate
		for i, runtime := range strategy.Providers {
			targets, err := exclusiveTargets(runtime, plan.Outputs, i)
			if err != nil {
				return batch, err
			}
			if len(targets) > 0 {
				reserved[plan.Snapshot.Account] = true
			}
		}
	}
	if len(batch.Members) == 0 {
		return batch, provider.ErrNotReady
	}
	calls, snapshots, witnesses := batch.calls()
	if err := recipes.ValidateExecutorBatch(ctx, reader, p.Caller, snapshots, calls, witnesses); err != nil {
		return ExecutionPlan{}, err
	}
	return batch, nil
}

func (b ExecutionPlan) calls() ([]recipes.ExecutorCall, []recipes.Snapshot, []map[uint]string) {
	calls := make([]recipes.ExecutorCall, len(b.Members))
	snapshots := make([]recipes.Snapshot, len(b.Members))
	witnesses := make([]map[uint]string, len(b.Members))
	for i, member := range b.Members {
		calls[i] = recipes.ExecutorCall{Executor: member.Plan.Snapshot.Executor, Data: member.Plan.Calldata}
		snapshots[i], witnesses[i] = member.Plan.Recipe, member.Plan.Results
	}
	return calls, snapshots, witnesses
}

// Exclusive providers consume the transaction's full receipt. A shared target
// must not also be called by another batch member, even on a different account.
func validateExecutionComposition(batch ExecutionPlan, strategies []Strategy) error {
	owners, claims, targets := map[common.Address]int{}, map[string]common.Address{}, map[string]common.Address{}
	owner := 0
	for i, member := range batch.Members {
		var strategy *Strategy
		for j := range strategies {
			if strategies[j].Config.Name == member.Name {
				strategy = &strategies[j]
				break
			}
		}
		if strategy == nil {
			return errors.New("batch contains an unconfigured strategy")
		}
		if err := validatePlanMember(member.Plan, *strategy); err != nil {
			return err
		}
		if len(strategy.Providers) != len(member.Plan.Outputs) {
			return errors.New("provider output count differs from prepared plan")
		}
		for _, group := range member.Plan.Outputs {
			if len(group) == 0 {
				return errors.New("provider returned no execution outputs")
			}
		}
		prefix := strconv.Itoa(i) + "/"
		for j, p := range strategy.Providers {
			exclusive, err := exclusiveTargets(p, member.Plan.Outputs, j)
			if err != nil {
				return err
			}
			qualified := map[string]common.Address{}
			for key, target := range exclusive {
				qualified[prefix+key] = target
			}
			if err := addTargetClaims(qualified, owner, owners, claims); err != nil {
				return err
			}
			owner++
		}
	}
	if len(claims) == 0 {
		return nil
	}
	for i, member := range batch.Members {
		managed, err := recipes.ManagedTargets(member.Plan.Calldata)
		if err != nil {
			return err
		}
		prefix := strconv.Itoa(i) + "/"
		for key, target := range managed {
			targets[prefix+key] = target
		}
	}
	return validateTargetClaims(targets, owners, claims)
}
