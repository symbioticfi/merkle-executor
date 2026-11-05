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
	txmanager "github.com/symbioticfi/merkle-executor/sdk/submission"
)

// BatchKey stores the complete submission until every member journal is durable.
const BatchKey = "executor-batch.v1"

type BatchMember struct {
	Name string `json:"name"`
	Plan Plan   `json:"plan"`
}

type BatchPlan struct {
	Members []BatchMember `json:"members"`
}

type BatchPlanning interface {
	PrepareBatch(context.Context, []Strategy) (BatchPlan, error)
	ValidateBatch(context.Context, BatchPlan) error
}

// BatchJobMember retains the member's own provider outputs and recovery identity.
type BatchJobMember struct {
	Name string `json:"name"`
	Job  Job    `json:"job"`
}

// BatchJob is the durable owner of one atomic cross-executor transaction.
type BatchJob struct {
	ID      string           `json:"id"`
	Phase   string           `json:"phase"`
	To      common.Address   `json:"to"`
	Data    hexutil.Bytes    `json:"data"`
	Members []BatchJobMember `json:"members"`
	Result  txmanager.Result `json:"result"`
}

// Transaction encodes the ordered, atomic self-call for a delegated keeper.
func (b BatchPlan) Transaction() (common.Address, []byte, error) {
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
		if member.Plan.Snapshot.Caller != caller {
			return caller, nil, errors.New("executor batch mixes callers")
		}
		calls[i] = recipes.ExecutorCall{Executor: member.Plan.Snapshot.Executor, Data: member.Plan.Calldata}
	}
	data, err := recipes.DelegatedMulticall(calls)
	return caller, data, err
}

func (b BatchJob) plan() BatchPlan {
	plan := BatchPlan{Members: make([]BatchMember, len(b.Members))}
	for i, member := range b.Members {
		plan.Members[i] = BatchMember{Name: member.Name, Plan: member.Job.Plan}
	}
	return plan
}

func (k *Keeper) checkBatchRecovery(configured map[string]Strategy) (map[string]common.Address, string, error) {
	var batch BatchJob
	found, err := k.Store.Load(BatchKey, &batch)
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
		if member.Job.ID != batch.ID || member.Job.Phase != "prepared" {
			return nil, "", errors.New("invalid durable batch member")
		}
		if err := validateJob(member.Job, s); err != nil {
			return nil, "", err
		}
		if err := validateBatchMember(member.Job.Plan, s); err != nil {
			return nil, "", err
		}
		owners[member.Name] = member.Job.Plan.Snapshot.Account
	}
	if err := validateBatchComposition(batch.plan(), k.Strategies); err != nil {
		return nil, "", err
	}
	return owners, batch.ID, nil
}

func validateBatchMember(plan Plan, strategy Strategy) error {
	if plan.Snapshot.Account != common.HexToAddress(strategy.Config.Account) || plan.Snapshot.Executor != common.HexToAddress(strategy.Config.Executor) || plan.Recipe.Account != plan.Snapshot.Account || plan.Recipe.Executor != plan.Snapshot.Executor || plan.Recipe.Caller != plan.Snapshot.Caller {
		return errors.New("executor batch member identity mismatch")
	}
	return nil
}

func (k *Keeper) tickBatch(ctx context.Context, owners map[string]common.Address, batch BatchJob, found bool) error {
	planner, ok := k.Planner.(BatchPlanning)
	if !ok {
		return errors.New("cross-executor batching requires batch planning")
	}
	if k.Mode == "simulate" {
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		defer cancel()
		plan, err := planner.PrepareBatch(operation, k.Strategies)
		if errors.Is(err, provider.ErrNotReady) {
			return nil
		}
		if err == nil {
			k.Log.Info("executor batch simulation succeeded", "executors", len(plan.Members))
		}
		return err
	}
	if found {
		if batch.Phase == "prepared" && k.Sender != nil && k.Sender.PendingID() == "" {
			// An unsigned batch has no external effects and may contain expired quotes.
			if err := k.Store.Delete(BatchKey); err != nil {
				return err
			}
		} else {
			operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
			err := k.submitBatch(operation, planner, &batch)
			cancel()
			if err != nil {
				return err
			}
		}
	}
	var failures []error
	// Finish old single-executor jobs and external settlement before selecting
	// new work. Their existing account reservations remain authoritative.
	for _, s := range k.Strategies {
		var job Job
		found, err := k.Store.Load(key(s.Config.Name), &job)
		if err != nil {
			return k.globalFailure(err)
		}
		if !found || job.Phase == "settled" && (k.Sender == nil || k.Sender.PendingID() != job.ID) {
			continue
		}
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		err = k.tickStrategy(operation, s, owners)
		cancel()
		if err != nil && !errors.Is(err, provider.ErrNotReady) {
			failures = append(failures, err)
		}
	}
	owners, err := k.checkRecovery()
	if err != nil {
		return k.globalFailure(err)
	}
	if k.Sender != nil && k.Sender.PendingID() != "" {
		return errors.Join(failures...)
	}
	var candidates []Strategy
	for _, s := range k.Strategies {
		var job Job
		found, err := k.Store.Load(key(s.Config.Name), &job)
		if err != nil {
			return k.globalFailure(err)
		}
		if found && (job.Phase != "settled" || time.Since(job.FinishedAt) < s.Config.Interval) {
			continue
		}
		reserved := false
		for _, account := range owners {
			reserved = reserved || account == common.HexToAddress(s.Config.Account)
		}
		if !reserved {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		return errors.Join(failures...)
	}
	operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
	defer cancel()
	plan, err := planner.PrepareBatch(operation, candidates)
	if errors.Is(err, provider.ErrNotReady) {
		return errors.Join(failures...)
	}
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	if k.Sender == nil {
		return errors.New("send mode requires transaction manager")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return errors.New("generate batch identity failed")
	}
	batch = BatchJob{ID: "batch-" + hex.EncodeToString(random[:]), Phase: "prepared"}
	batch.To, batch.Data, err = plan.Transaction()
	if err != nil {
		return err
	}
	for _, member := range plan.Members {
		var strategy *Strategy
		for i := range candidates {
			if candidates[i].Config.Name == member.Name {
				strategy = &candidates[i]
				break
			}
		}
		if strategy == nil {
			return errors.New("batch contains an unconfigured strategy")
		}
		if err := validateBatchMember(member.Plan, *strategy); err != nil {
			return err
		}
		if err := ValidatePlanComposition(member.Plan.Calldata, strategy.Providers, member.Plan.Outputs); err != nil {
			return err
		}
		batch.Members = append(batch.Members, BatchJobMember{Name: member.Name, Job: Job{ID: batch.ID, ConfigHash: strategy.Config.Fingerprint(), Phase: "prepared", Plan: member.Plan, Progress: make([]provider.Progress, len(strategy.Providers))}})
	}
	if err := validateBatchComposition(plan, candidates); err != nil {
		return err
	}
	if err := k.Store.Save(BatchKey, batch); err != nil {
		return err
	}
	if err := k.submitBatch(operation, planner, &batch); err != nil {
		return errors.Join(append(failures, err)...)
	}
	// Let every provider consume the same immutable receipt, with each member's
	// own outputs. No provider is advanced until all members are durable.
	for _, s := range k.Strategies {
		var job Job
		found, err := k.Store.Load(key(s.Config.Name), &job)
		if err != nil {
			return err
		}
		if found && job.ID == batch.ID {
			if err := k.tickStrategy(operation, s, owners); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}

func (k *Keeper) submitBatch(ctx context.Context, planner BatchPlanning, batch *BatchJob) error {
	if k.Sender == nil {
		return errors.New("send mode requires transaction manager")
	}
	if batch.Phase == "prepared" {
		if k.Sender.PendingID() != "" && k.Sender.PendingID() != batch.ID {
			return errors.New("another transaction owns the sender")
		}
		if k.Sender.PendingID() == "" {
			if err := planner.ValidateBatch(ctx, batch.plan()); err != nil {
				if e := k.Store.Delete(BatchKey); e != nil {
					return e
				}
				return err
			}
		}
		result, err := k.Sender.Send(ctx, txmanager.Request{ID: batch.ID, To: batch.To, Data: batch.Data})
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
		if err := k.Store.Save(BatchKey, *batch); err != nil {
			return err
		}
	}
	for _, member := range batch.Members {
		job := member.Job
		job.Result, job.Phase = batch.Result, batch.Phase
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
	if err := k.Store.Delete(BatchKey); err != nil {
		return err
	}
	if batch.Phase == "failed" {
		return errors.New("executor batch transaction reverted")
	}
	return nil
}
