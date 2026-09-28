// Package keeper coordinates generic recipe execution and durable settlement.
package keeper

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
)

type Store interface {
	Load(string, any) (bool, error)
	Save(string, any) error
	Delete(string) error
}
type Sender interface {
	Send(context.Context, txmanager.Request) (txmanager.Result, error)
	Acknowledge(string) error
	PendingID() string
}
type Planning interface {
	Prepare(context.Context, Strategy) (Plan, error)
	Validate(context.Context, Plan) error
	ConfirmedSnapshot(context.Context, provider.Snapshot) (provider.Snapshot, error)
	VerifyConfirmedSnapshot(context.Context, provider.Snapshot) error
}

type Job struct {
	ID         string              `json:"id"`
	ConfigHash string              `json:"configHash"`
	Phase      string              `json:"phase"`
	Plan       Plan                `json:"plan"`
	Progress   []provider.Progress `json:"progress"`
	Result     txmanager.Result    `json:"result"`
	FinishedAt time.Time           `json:"finishedAt"`
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
	names, err := k.inventory()
	if err != nil {
		return err
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
	for _, name := range names {
		var job Job
		found, err := k.Store.Load(key(name), &job)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		s, exists := configured[name]
		if !exists {
			if job.Phase != "settled" && job.Phase != "failed" {
				return errors.Errorf("active strategy %s removed from configuration", name)
			}
		} else if err := validateJob(job, s); err != nil {
			return err
		}
		if pending != "" && job.ID == pending {
			if !exists {
				return errors.Errorf("transaction owner strategy %s removed from configuration", name)
			}
			matched = true
		}
	}
	if !matched {
		return errors.New("unresolved transaction has no configured workflow; restore its strategy configuration")
	}
	return nil
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

func (k *Keeper) accountBusy(s Strategy) (bool, error) {
	names, err := k.inventory()
	if err != nil {
		return false, err
	}
	for _, name := range names {
		if name == s.Config.Name {
			continue
		}
		var job Job
		found, err := k.Store.Load(key(name), &job)
		if err != nil {
			return false, err
		}
		if found && job.Plan.Snapshot.Account == common.HexToAddress(s.Config.Account) && (job.Phase == "prepared" || job.Phase == "authorized") {
			return true, nil
		}
	}
	return false, nil
}

func validateJob(job Job, s Strategy) error {
	switch job.Phase {
	case "prepared", "authorized", "settled", "failed":
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
		if len(job.Plan.Outputs) != len(s.Providers) || len(job.Progress) != len(s.Providers) {
			return errors.New("durable provider state count mismatch")
		}
	}
	return nil
}

func (k *Keeper) Tick(ctx context.Context) error {
	if err := k.CheckRecovery(); err != nil {
		return err
	}
	// Recover the occupied nonce first, independently of configuration ordering.
	ordered := append([]Strategy(nil), k.Strategies...)
	if k.Sender != nil && k.Sender.PendingID() != "" {
		for i, s := range ordered {
			var job Job
			found, err := k.Store.Load(key(s.Config.Name), &job)
			if err != nil {
				return err
			}
			if found && job.ID == k.Sender.PendingID() {
				ordered[0], ordered[i] = ordered[i], ordered[0]
				break
			}
		}
	}
	var failures []error
	for _, s := range ordered {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		operation, cancel := context.WithTimeout(ctx, k.OperationTimeout)
		err := k.tickStrategy(operation, s)
		cancel()
		outcome := "ok"
		if errors.Is(err, provider.ErrNotReady) {
			outcome = "idle"
			k.Log.V(1).Info("strategy waiting", "strategy", s.Config.Name)
		} else if err != nil {
			outcome = "error"
			failures = append(failures, errors.Errorf("strategy %s: %w", s.Config.Name, err))
			k.Log.Error(err, "strategy iteration failed", "strategy", s.Config.Name)
		}
		if k.Observe != nil {
			k.Observe(s.Config.Name, outcome)
		}
	}
	return errors.Join(failures...)
}

func (k *Keeper) tickStrategy(ctx context.Context, s Strategy) error {
	if k.Mode == "simulate" {
		_, err := k.Planner.Prepare(ctx, s)
		if err == nil {
			k.Log.Info("recipe simulation succeeded", "strategy", s.Config.Name)
		}
		return err
	}
	if k.Sender == nil {
		return errors.New("send mode requires transaction manager")
	}
	var job Job
	found, err := k.Store.Load(key(s.Config.Name), &job)
	if err != nil {
		return err
	}
	if found {
		if err := validateJob(job, s); err != nil {
			return err
		}
	}
	// No journal means these bytes were never broadcast. Refresh expiring quotes
	// after restart or a failed pre-admission attempt, even if sign() still simulates.
	if found && job.Phase == "prepared" && k.Sender.PendingID() == "" {
		if err := k.Store.Delete(key(s.Config.Name)); err != nil {
			return err
		}
		found = false
	}
	if found && (job.Phase == "settled" || job.Phase == "failed") {
		if k.Sender.PendingID() == job.ID {
			if err := k.Sender.Acknowledge(job.ID); err != nil {
				return err
			}
		}
		// Failed work is sticky: an operator must inspect it before removing the job.
		if job.Phase == "failed" {
			return errors.New("strategy halted on a confirmed reverted transaction; inspect durable job")
		}
		if time.Since(job.FinishedAt) < s.Config.Interval {
			return nil
		}
		found = false
	}
	if !found {
		if k.Sender.PendingID() != "" {
			return nil
		}
		busy, err := k.accountBusy(s)
		if err != nil {
			return err
		}
		if busy {
			return nil
		}
		plan, err := k.Planner.Prepare(ctx, s)
		if err != nil {
			return err
		}
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return errors.New("generate job identity failed")
		}
		job = Job{ID: s.Config.Name + "-" + hex.EncodeToString(random[:]), ConfigHash: s.Config.Fingerprint(), Phase: "prepared", Plan: plan, Progress: make([]provider.Progress, len(s.Providers))}
		if err := k.Store.Save(key(s.Config.Name), job); err != nil {
			return err
		}
	}
	if job.Phase == "prepared" {
		if k.Sender.PendingID() != "" && k.Sender.PendingID() != job.ID {
			return nil
		}
		// Once signed, resume exact bytes even if the recipe changed. Resimulation
		// cannot tell whether an already broadcast transaction landed.
		if k.Sender.PendingID() == "" {
			if err := k.Planner.Validate(ctx, job.Plan); err != nil {
				if deleteErr := k.Store.Delete(key(s.Config.Name)); deleteErr != nil {
					return deleteErr
				}
				return err
			}
		}
		result, err := k.Sender.Send(ctx, txmanager.Request{ID: job.ID, To: job.Plan.Snapshot.Executor, Data: job.Plan.Calldata})
		if err != nil {
			if result.Pending && errors.Is(err, context.DeadlineExceeded) {
				return provider.ErrNotReady
			}
			return err
		}
		if !result.Confirmed && !result.Reverted {
			return nil
		}
		job.Result = result
		job.Phase = "authorized"
		if result.Reverted {
			job.Phase = "failed"
			job.FinishedAt = time.Now().UTC()
		}
		if err := k.Store.Save(key(s.Config.Name), job); err != nil {
			return err
		}
		if err := k.Sender.Acknowledge(job.ID); err != nil {
			return err
		}
		if result.Reverted {
			return errors.New("recipe transaction reverted")
		}
	}
	if job.Phase == "authorized" {
		if k.Sender.PendingID() == job.ID {
			if err := k.Sender.Acknowledge(job.ID); err != nil {
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
			if job.Progress[i].Done {
				continue
			}
			progress, err := p.Advance(ctx, snapshot, job.Plan.Outputs[i], job.Progress[i].State)
			if err != nil {
				return errors.Errorf("reconcile provider %d: %w", i, err)
			}
			if err := k.Planner.VerifyConfirmedSnapshot(ctx, snapshot); err != nil {
				return err
			}
			job.Progress[i] = progress
			if err := k.Store.Save(key(s.Config.Name), job); err != nil {
				return err
			}
			done = done && progress.Done
		}
		if done {
			job.Phase = "settled"
			job.FinishedAt = time.Now().UTC()
			if err := k.Store.Save(key(s.Config.Name), job); err != nil {
				return err
			}
			outcomes := make([]string, len(job.Progress))
			for i, p := range job.Progress {
				outcomes[i] = p.Outcome
			}
			k.Log.Info("strategy cycle finished", "strategy", s.Config.Name, "transaction", job.Result.Hash.Hex(), "outcomes", outcomes)
		}
	}
	return nil
}
