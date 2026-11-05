package keeper

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/submission"
)

// ExecutionKey stores the complete submission until every member journal is durable.
const ExecutionKey = "executor-batch.v1"

type PlanMember struct {
	Name string `json:"name"`
	Plan Plan   `json:"plan"`
}

type ExecutionPlan struct {
	To      common.Address `json:"to"`
	Members []PlanMember   `json:"members"`
}

// ExecutionMember retains the member's own provider outputs and recovery identity.
type ExecutionMember struct {
	Name       string `json:"name"`
	ConfigHash string `json:"configHash"`
	Plan       Plan   `json:"plan"`
}

// ExecutionJob owns one transaction until every member journal is durable.
type ExecutionJob struct {
	ID      string            `json:"id"`
	Phase   string            `json:"phase"`
	To      common.Address    `json:"to"`
	Data    hexutil.Bytes     `json:"data"`
	Members []ExecutionMember `json:"members"`
	Result  submission.Result `json:"result"`
}

// Transaction encodes a direct call or an ordered delegated self-call.
func (b ExecutionPlan) Transaction() (common.Address, []byte, error) {
	if len(b.Members) == 0 {
		return common.Address{}, nil, errors.New("empty executor batch")
	}
	caller := b.Members[0].Plan.Snapshot.Caller
	if caller == (common.Address{}) {
		return caller, nil, errors.New("executor batch caller is zero")
	}
	calls := make([]recipes.ExecutorCall, len(b.Members))
	names := make(map[string]bool, len(b.Members))
	for i, member := range b.Members {
		if member.Name == "" || names[member.Name] {
			return caller, nil, errors.New("executor batch has an empty or duplicate member")
		}
		names[member.Name] = true
		if member.Plan.Snapshot.Executor == (common.Address{}) {
			return b.To, nil, errors.New("execution plan executor is zero")
		}
		if member.Plan.Snapshot.Caller != caller {
			return caller, nil, errors.New("executor batch mixes callers")
		}
		if _, err := recipes.Executions(member.Plan.Calldata); err != nil {
			return b.To, nil, err
		}
		calls[i] = recipes.ExecutorCall{Executor: member.Plan.Snapshot.Executor, Data: member.Plan.Calldata}
	}
	if len(calls) == 1 && b.To == calls[0].Executor {
		return b.To, bytes.Clone(calls[0].Data), nil
	}
	if b.To != caller {
		return b.To, nil, errors.New("execution plan target does not match its caller")
	}
	data, err := recipes.DelegatedMulticall(calls)
	return b.To, data, err
}

func (b ExecutionJob) plan() ExecutionPlan {
	plan := ExecutionPlan{To: b.To, Members: make([]PlanMember, len(b.Members))}
	for i, member := range b.Members {
		plan.Members[i] = PlanMember{Name: member.Name, Plan: member.Plan}
	}
	return plan
}

func (k *Keeper) checkExecutionRecovery(configured map[string]Strategy) (map[string]common.Address, string, error) {
	var batch ExecutionJob
	found, err := k.Store.Load(ExecutionKey, &batch)
	if err != nil || !found {
		return nil, "", err
	}
	if batch.ID == "" || (batch.Phase != "prepared" && batch.Phase != "authorized" && batch.Phase != "failed") {
		return nil, "", errors.New("invalid durable executor batch")
	}
	to, data, err := batch.plan().Transaction()
	if err != nil || to != batch.To || !bytes.Equal(data, batch.Data) {
		return nil, "", errors.New("durable executor batch payload mismatch")
	}
	owners := make(map[string]common.Address)
	for _, member := range batch.Members {
		s, ok := configured[member.Name]
		if !ok {
			return nil, "", errors.New("active batch strategy removed from configuration")
		}
		if _, duplicate := owners[member.Name]; duplicate {
			return nil, "", errors.New("duplicate durable batch member")
		}
		if member.ConfigHash != s.Config.Fingerprint() {
			return nil, "", errors.Errorf("strategy %s configuration changed while work is active", member.Name)
		}
		owners[member.Name] = member.Plan.Snapshot.Account
	}
	if err := validateExecutionComposition(batch.plan(), k.Strategies); err != nil {
		return nil, "", err
	}
	return owners, batch.ID, nil
}

func validatePlanMember(plan Plan, strategy Strategy) error {
	if plan.Snapshot.Account != common.HexToAddress(strategy.Config.Account) || plan.Snapshot.Executor != common.HexToAddress(strategy.Config.Executor) || plan.Recipe.Account != plan.Snapshot.Account || plan.Recipe.Executor != plan.Snapshot.Executor || plan.Recipe.Caller != plan.Snapshot.Caller {
		return errors.New("executor batch member identity mismatch")
	}
	return nil
}

func (k *Keeper) tickExecution(ctx context.Context, owners map[string]common.Address, execution ExecutionJob, found bool) error {
	if k.Mode == "simulate" {
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		defer cancel()
		plan, err := k.Planner.PrepareExecution(operation, k.Strategies)
		if err == nil {
			k.Log.Info("execution simulation succeeded", "executors", len(plan.Members))
			for _, member := range plan.Members {
				k.observe(member.Name, nil)
			}
		} else {
			for _, s := range k.Strategies {
				k.observe(s.Config.Name, err)
			}
		}
		if errors.Is(err, provider.ErrNotReady) {
			return nil
		}
		return err
	}
	if k.Sender == nil {
		return errors.New("send mode requires transaction manager")
	}
	if found {
		if execution.Phase == "prepared" && k.Sender.PendingID() == "" {
			if err := k.Store.Delete(ExecutionKey); err != nil {
				return err
			}
			for _, member := range execution.Members {
				delete(owners, member.Name)
			}
		} else {
			operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
			err := k.submitExecution(operation, &execution)
			cancel()
			if err != nil {
				return err
			}
		}
	}
	var failures []error
	var candidates []Strategy
	// Reconcile existing work first, refreshing reservations from durable state.
	for _, s := range k.Strategies {
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		err := k.advanceStrategy(operation, s)
		cancel()
		if err != nil {
			k.observe(s.Config.Name, err)
		}
		if err != nil && !errors.Is(err, provider.ErrNotReady) {
			failures = append(failures, err)
		}
		var job Job
		found, loadErr := k.Store.Load(key(s.Config.Name), &job)
		if loadErr != nil {
			return k.globalFailure(loadErr)
		}
		delete(owners, s.Config.Name)
		if found && job.Phase == "authorized" {
			owners[s.Config.Name] = job.Plan.Snapshot.Account
		}
		if err == nil && (!found || job.Phase == "settled" && time.Since(job.FinishedAt) >= s.Config.Interval) {
			candidates = append(candidates, s)
		}
	}
	for len(candidates) > 0 && k.Sender.PendingID() == "" {
		eligible := candidates[:0]
		for _, s := range candidates {
			reserved := false
			for _, account := range owners {
				reserved = reserved || account == common.HexToAddress(s.Config.Account)
			}
			if !reserved {
				eligible = append(eligible, s)
			}
		}
		candidates = eligible
		if len(candidates) == 0 {
			break
		}
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		plan, err := k.Planner.PrepareExecution(operation, candidates)
		if errors.Is(err, provider.ErrNotReady) {
			for _, s := range candidates {
				k.observe(s.Config.Name, err)
			}
			cancel()
			break
		}
		if err == nil {
			execution, err = newExecution(plan, candidates)
		}
		if err == nil {
			err = k.Store.Save(ExecutionKey, execution)
		}
		if err == nil {
			err = k.submitExecution(operation, &execution)
		}
		if err != nil {
			cancel()
			return errors.Join(append(failures, err)...)
		}
		selected := make(map[string]bool, len(plan.Members))
		for _, member := range plan.Members {
			selected[member.Name] = true
		}
		remaining := candidates[:0]
		for _, s := range candidates {
			if !selected[s.Config.Name] {
				remaining = append(remaining, s)
				continue
			}
			err := k.advanceStrategy(operation, s)
			k.observe(s.Config.Name, err)
			if err != nil && !errors.Is(err, provider.ErrNotReady) {
				failures = append(failures, err)
			}
			// A failed Save may have committed. Never use stale account ownership.
			var job Job
			found, loadErr := k.Store.Load(key(s.Config.Name), &job)
			if loadErr != nil {
				cancel()
				return k.globalFailure(loadErr)
			}
			if found && job.Phase == "authorized" {
				owners[s.Config.Name] = job.Plan.Snapshot.Account
			} else {
				delete(owners, s.Config.Name)
			}
		}
		candidates = remaining
		cancel()
	}
	return errors.Join(failures...)
}

func newExecution(plan ExecutionPlan, strategies []Strategy) (ExecutionJob, error) {
	to, data, err := plan.Transaction()
	if err != nil {
		return ExecutionJob{}, err
	}
	if err := validateExecutionComposition(plan, strategies); err != nil {
		return ExecutionJob{}, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return ExecutionJob{}, errors.New("generate execution identity failed")
	}
	execution := ExecutionJob{ID: "execution-" + hex.EncodeToString(random[:]), Phase: "prepared", To: to, Data: data}
	for _, member := range plan.Members {
		for _, strategy := range strategies {
			if strategy.Config.Name == member.Name {
				execution.Members = append(execution.Members, ExecutionMember{Name: member.Name, ConfigHash: strategy.Config.Fingerprint(), Plan: member.Plan})
				break
			}
		}
	}
	return execution, nil
}

func (k *Keeper) observe(name string, err error) {
	outcome := "ok"
	if errors.Is(err, provider.ErrNotReady) {
		outcome = "idle"
	} else if err != nil {
		outcome = "error"
		k.Log.Error(err, "strategy iteration failed", "strategy", name)
	}
	if k.Observe != nil {
		k.Observe(name, outcome)
	}
}

func (k *Keeper) submitExecution(ctx context.Context, batch *ExecutionJob) error {
	if k.Sender == nil {
		return errors.New("send mode requires transaction manager")
	}
	if batch.Phase == "prepared" {
		if k.Sender.PendingID() != "" && k.Sender.PendingID() != batch.ID {
			return errors.New("another transaction owns the sender")
		}
		result, err := k.Sender.Send(ctx, submission.Request{ID: batch.ID, To: batch.To, Data: batch.Data})
		if err != nil {
			if result.Pending && errors.Is(err, context.DeadlineExceeded) {
				return provider.ErrNotReady
			}
			return err
		}
		if !result.Confirmed && !result.Reverted {
			return provider.ErrNotReady
		}
		batch.Result, batch.Phase = result, "authorized"
		if result.Reverted {
			batch.Phase = "failed"
		}
		if err := k.Store.Save(ExecutionKey, *batch); err != nil {
			return err
		}
	}
	for _, member := range batch.Members {
		job := Job{ID: batch.ID, ConfigHash: member.ConfigHash, Plan: member.Plan, Result: batch.Result, Phase: batch.Phase,
			Progress: make([][]provider.Progress, len(member.Plan.Outputs))}
		for i, outputs := range member.Plan.Outputs {
			job.Progress[i] = make([]provider.Progress, len(outputs))
		}
		if job.Phase == "failed" {
			job.FinishedAt = time.Now().UTC()
		}
		if err := k.Store.Save(key(member.Name), job); err != nil {
			return err
		}
	}
	if k.Sender.PendingID() == batch.ID {
		if err := k.Sender.Acknowledge(ctx, batch.ID); err != nil {
			return err
		}
	}
	if err := k.Store.Delete(ExecutionKey); err != nil {
		return err
	}
	if batch.Phase == "failed" {
		return errors.New("executor batch transaction reverted")
	}
	return nil
}
