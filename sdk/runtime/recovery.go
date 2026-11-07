package runtime

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

type Deployment struct {
	Kind    string         `json:"kind"`
	Version int            `json:"version"`
	ChainID *big.Int       `json:"chainId"`
	Caller  common.Address `json:"caller"`
}

func (r *Runtime) validateWorkflow(w Workflow, strategies map[string]Strategy) error {
	if w.Version != 2 || (w.IdentityVersion != 1 && w.IdentityVersion != 2) || w.Name == "" || w.ID == "" || w.ConfigHash == "" {
		return errors.New("invalid workflow schema or identity")
	}
	switch w.Phase {
	case "prepared", "pending", "authorized", "settled", "failed":
	default:
		return errors.New("invalid workflow phase")
	}
	if err := validatePlanIdentity(w.Plan); err != nil {
		return err
	}
	if w.Plan.Snapshot.ChainID.Cmp(r.ChainID) != 0 || w.Plan.Snapshot.Caller != r.Caller {
		return errors.New("workflow deployment identity mismatch")
	}
	if err := validateTransaction(w, r.ChainID, r.Caller); err != nil {
		return err
	}
	if s, ok := strategies[w.Name]; ok {
		if w.active() || w.Reserved {
			expected := s.Config.Fingerprint()
			if w.IdentityVersion == 1 {
				expected = s.LegacyConfigHash
			}
			if expected == "" || expected != w.ConfigHash || common.HexToAddress(s.Config.Executor) != w.Plan.Snapshot.Executor || common.HexToAddress(s.Config.Account) != w.Plan.Snapshot.Account {
				return errors.New("active workflow configuration changed")
			}
			if _, err := RestoreOperations(w.Plan, s.Integrations); err != nil {
				return err
			}
		}
	} else if w.active() || w.Reserved {
		return errors.New("active workflow removed from configuration")
	}
	if (w.Phase == "settled" || w.Phase == "failed") && w.FinishedAt.IsZero() {
		return errors.New("terminal workflow has no completion time")
	}
	return nil
}

// Start validates the complete inventory before reserving the host signer. No
// discovery, preparation or provider I/O is needed to recover local ownership.
func (r *Runtime) Start(ctx context.Context) error {
	r.tickMu.Lock()
	defer r.tickMu.Unlock()
	return r.start(ctx)
}
func (r *Runtime) start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return r.fault
	}
	if r.closed {
		return errors.New("runtime closed")
	}
	if r.Store == nil || r.Planner == nil || r.ChainID == nil || r.ChainID.Sign() <= 0 || r.Caller == (common.Address{}) || r.OperationTimeout <= 0 || (r.Mode != "send" && r.Mode != "simulate") || r.Mode == "send" && r.Driver == nil {
		return errors.New("runtime dependencies, identity, mode and timeout required")
	}
	strategies := make(map[string]Strategy, len(r.Strategies))
	for _, s := range r.Strategies {
		if s.Config.Name == "" {
			return errors.New("strategy name required")
		}
		if _, exists := strategies[s.Config.Name]; exists {
			return errors.New("duplicate strategy identity")
		}
		strategies[s.Config.Name] = s
	}
	var identity Deployment
	found, err := r.Store.LoadNamespace(Namespace, "deployment", &identity)
	if err != nil {
		return err
	}
	if !found {
		if err = r.importLegacy(strategies); err != nil {
			r.fault = err
			return err
		}
		found, err = r.Store.LoadNamespace(Namespace, "deployment", &identity)
		if err != nil {
			return err
		}
	}
	if !found || identity.Kind != "deployment" || identity.Version != 2 || identity.ChainID == nil || identity.ChainID.Cmp(r.ChainID) != 0 || identity.Caller != r.Caller {
		return errors.New("state deployment identity mismatch")
	}
	rows, err := r.Store.ListNamespace(Namespace)
	if err != nil {
		return err
	}
	workflows := make(map[string]Workflow)
	accounts := make(map[common.Address]string)
	owner := ""
	for _, raw := range rows {
		var header struct {
			Kind string `json:"kind"`
		}
		if err = json.Unmarshal(raw, &header); err != nil {
			return err
		}
		if header.Kind == "deployment" {
			continue
		}
		var w Workflow
		if err = json.Unmarshal(raw, &w); err != nil {
			return errors.New("invalid workflow checkpoint")
		}
		if err = r.validateWorkflow(w, strategies); err != nil {
			return err
		}
		if _, exists := workflows[w.Name]; exists {
			return errors.New("duplicate workflow checkpoint")
		}
		if w.active() {
			a := w.Plan.Snapshot.Account
			if accounts[a] != "" {
				return errors.New("multiple active workflows own one account")
			}
			accounts[a] = w.Name
		}
		if w.Reserved {
			if owner != "" {
				return errors.New("multiple workflows own signer")
			}
			owner = w.Name
		}
		workflows[w.Name] = w.Clone()
	}
	if owner != "" {
		if r.Mode != "send" {
			return errors.New("occupied signer requires send recovery")
		}
		if err = r.Driver.Reserve(ctx, workflows[owner].ID); err != nil {
			return err
		}
	}
	r.workflows, r.accounts, r.owner = workflows, accounts, owner
	r.strategies = strategies
	r.process, r.cancel = context.WithCancel(context.WithoutCancel(ctx))
	r.started = true
	return nil
}
