// Package keeper coordinates generic recipe execution and durable settlement.
package keeper

import (
	"context"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/submission"
)

type Store interface {
	Load(string, any) (bool, error)
	Save(string, any) error
	Delete(string) error
}
type Sender interface {
	Send(context.Context, submission.Request) (submission.Result, error)
	Acknowledge(context.Context, string) error
	PendingID() string
}
type Planning interface {
	PrepareExecution(context.Context, []Strategy) (ExecutionPlan, error)
	ConfirmedSnapshot(context.Context, provider.Snapshot) (provider.Snapshot, error)
	VerifyConfirmedSnapshot(context.Context, provider.Snapshot) error
}

type Job struct {
	ID         string                `json:"id"`
	ConfigHash string                `json:"configHash"`
	Phase      string                `json:"phase"`
	Plan       Plan                  `json:"plan"`
	Progress   [][]provider.Progress `json:"progress"`
	Result     submission.Result     `json:"result"`
	FinishedAt time.Time             `json:"finishedAt"`
}

type Keeper struct {
	Store            Store
	Sender           Sender
	Planner          Planning
	Strategies       []Strategy
	Mode             string
	OperationTimeout time.Duration
	Log              logr.Logger
	Observe          func(strategy, outcome string)
}

func key(name string) string { return "strategy." + name + ".v1" }

// CheckRecovery rejects an orphan transaction or changed workflow configuration.
// A deployment must retain configured strategies until their durable work ends.
func (k *Keeper) CheckRecovery() error {
	_, err := k.checkRecovery()
	return err
}

// checkRecovery also collects active account owners while each journal is read.
func (k *Keeper) checkRecovery() (map[string]common.Address, error) {
	owners := make(map[string]common.Address)
	names, err := k.inventory()
	if err != nil {
		return nil, err
	}
	configured := make(map[string]Strategy)
	for _, s := range k.Strategies {
		configured[s.Config.Name] = s
	}
	pending := ""
	if k.Sender != nil {
		pending = k.Sender.PendingID()
	}
	matched := pending == ""
	batchOwners, batchID, err := k.checkExecutionRecovery(configured)
	if err != nil {
		return nil, err
	}
	for name, account := range batchOwners {
		owners[name] = account
	}
	if pending != "" && batchID == pending {
		matched = true
	}
	for _, name := range names {
		var job Job
		found, err := k.Store.Load(key(name), &job)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if job.Phase == "authorized" {
			owners[name] = job.Plan.Snapshot.Account
		}
		s, exists := configured[name]
		if !exists {
			if job.Phase != "settled" && job.Phase != "failed" {
				return nil, errors.Errorf("active strategy %s removed from configuration", name)
			}
		} else if err := validateJob(job, s); err != nil {
			return nil, err
		}
		if pending != "" && job.ID == pending {
			if !exists {
				return nil, errors.Errorf("transaction owner strategy %s removed from configuration", name)
			}
			matched = true
		}
	}
	if !matched {
		return nil, errors.New("unresolved transaction has no configured workflow; restore its strategy configuration")
	}
	return owners, nil
}

// Inventory precedes job creation so removing a configured strategy cannot hide
// an external order after its authorization transaction has been acknowledged.
func (k *Keeper) inventory() ([]string, error) {
	var names []string
	_, err := k.Store.Load("strategies.v1", &names)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, s := range k.Strategies {
		if !slices.Contains(names, s.Config.Name) {
			names = append(names, s.Config.Name)
			changed = true
		}
	}
	if changed {
		slices.Sort(names)
		if err := k.Store.Save("strategies.v1", names); err != nil {
			return nil, err
		}
	}
	return names, nil
}

func validateJob(job Job, s Strategy) error {
	switch job.Phase {
	case "authorized", "settled", "failed":
	default:
		return errors.New("invalid durable job phase")
	}
	if job.ID == "" {
		return errors.New("durable job has no identity")
	}
	if job.Phase != "settled" && job.Phase != "failed" {
		if job.ConfigHash != s.Config.Fingerprint() {
			return errors.Errorf("strategy %s configuration changed while work is active", s.Config.Name)
		}
		if err := ValidatePlanComposition(job.Plan.Calldata, s.Providers, job.Plan.Outputs); err != nil {
			return err
		}
		if len(job.Plan.Outputs) != len(s.Providers) || len(job.Progress) != len(s.Providers) {
			return errors.New("durable provider state count mismatch")
		}
		for i, outputs := range job.Plan.Outputs {
			if len(outputs) == 0 || len(outputs) != len(job.Progress[i]) {
				return errors.New("durable provider execution count mismatch")
			}
		}
	}
	return nil
}

func (k *Keeper) Tick(ctx context.Context) error {
	owners, err := k.checkRecovery()
	if err != nil {
		return k.globalFailure(err)
	}
	var execution ExecutionJob
	found, err := k.Store.Load(ExecutionKey, &execution)
	if err != nil {
		return k.globalFailure(err)
	}
	return k.tickExecution(ctx, owners, execution, found)
}

// advanceStrategy only reconciles durable work; new transactions use ExecutionJob.
func (k *Keeper) advanceStrategy(ctx context.Context, s Strategy) error {
	var job Job
	found, err := k.Store.Load(key(s.Config.Name), &job)
	if err != nil || !found {
		return err
	}
	if job.Phase == "settled" || job.Phase == "failed" {
		if k.Sender.PendingID() == job.ID {
			if err := k.Sender.Acknowledge(ctx, job.ID); err != nil {
				return err
			}
		}
		if job.Phase == "failed" {
			return errors.New("strategy halted on a confirmed reverted transaction; inspect durable job")
		}
		return nil
	}

	if job.Phase == "authorized" {
		if k.Sender.PendingID() == job.ID {
			if err := k.Sender.Acknowledge(ctx, job.ID); err != nil {
				return err
			}
		}
		original := job.Plan.Snapshot
		original.Execution = job.Result.Execution.Clone()
		if original.Execution != nil && original.Execution.TxHash != job.Result.Hash {
			return errors.New("durable execution does not match the recipe transaction")
		}
		snapshot, err := k.Planner.ConfirmedSnapshot(ctx, original)
		if err != nil {
			return err
		}
		done := true
		for i, p := range s.Providers {
			for j, output := range job.Plan.Outputs[i] {
				if job.Progress[i][j].Done {
					continue
				}
				progress, err := p.Advance(ctx, snapshot, output, job.Progress[i][j].State)
				if err != nil {
					return errors.Errorf("reconcile provider %d: %w", i, err)
				}
				if err := k.Planner.VerifyConfirmedSnapshot(ctx, snapshot); err != nil {
					return err
				}
				job.Progress[i][j] = progress
				if err := k.Store.Save(key(s.Config.Name), job); err != nil {
					return err
				}
				done = done && progress.Done
			}
		}
		if done {
			job.Phase = "settled"
			job.FinishedAt = time.Now().UTC()
			if err := k.Store.Save(key(s.Config.Name), job); err != nil {
				return err
			}
			var outcomes []string
			for _, group := range job.Progress {
				for _, p := range group {
					outcomes = append(outcomes, p.Outcome)
				}
			}
			k.Log.Info("strategy cycle finished", "strategy", s.Config.Name, "transaction", job.Result.Hash.Hex(), "outcomes", outcomes)
		}
	}
	return nil
}

// Store and recovery errors may carry raw persisted payloads or filesystem data.
// Keep one bounded global observation; strategy failures are observed separately.
func (k *Keeper) globalFailure(err error) error {
	k.Log.Error(errors.New("global recovery or state validation failed"), "keeper iteration failed")
	if k.Observe != nil {
		k.Observe("_global", "error")
	}
	return err
}
