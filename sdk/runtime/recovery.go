package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

// Preflight rejects incompatible local ownership before a host performs RPC or
// loads signing credentials. Full config/operation validation remains in Start.
func Preflight(store CheckpointStore, chainID *big.Int, callerAssertion common.Address, configuredNames []string) error {
	if store == nil || chainID == nil || chainID.Sign() <= 0 {
		return errors.New("preflight store and chain identity required")
	}
	configured := map[string]bool{}
	for _, name := range configuredNames {
		configured[name] = true
	}
	identity, workflows, found, err := checkpointInventory(store)
	if err != nil {
		return err
	}
	if found {
		if identity.ChainID == nil || identity.ChainID.Cmp(chainID) != 0 || identity.Caller == (common.Address{}) || callerAssertion != (common.Address{}) && identity.Caller != callerAssertion {
			return errors.New("state directory belongs to another chain or caller")
		}
		owner := ""
		for _, w := range workflows {
			if w.Version != 2 || w.ID == "" {
				return errors.New("invalid workflow identity")
			}
			switch w.Phase {
			case "prepared", "pending", "authorized", "settled", "failed":
			default:
				return errors.New("invalid workflow phase")
			}
			if (w.active() || w.Reserved) && !configured[w.Name] {
				return errors.New("active workflow removed from configuration")
			}
			if w.Reserved {
				if owner != "" {
					return errors.New("multiple workflows own signer")
				}
				owner = w.ID
				if w.Transaction == nil || w.Transaction.Intent.ID != w.ID || w.Transaction.Sender != identity.Caller || w.Transaction.ChainID == nil || w.Transaction.ChainID.Cmp(chainID) != 0 {
					return errors.New("reserved workflow identity mismatch")
				}
			}
		}
		return nil
	}
	var old struct {
		ChainID *big.Int       `json:"chainId"`
		Caller  common.Address `json:"caller"`
	}
	legacy, err := store.Load("deployment.v1", &old)
	if err != nil {
		return err
	}
	var names []string
	inventory, err := store.Load("strategies.v1", &names)
	if err != nil {
		return err
	}
	var journal legacyJournal
	occupied, err := store.Load("txmanager.active.v1", &journal)
	if err != nil {
		return err
	}
	if !legacy {
		if inventory || occupied {
			return errors.New("legacy work has no deployment identity")
		}
		return nil
	}
	if old.ChainID == nil || old.ChainID.Cmp(chainID) != 0 || old.Caller == (common.Address{}) || callerAssertion != (common.Address{}) && old.Caller != callerAssertion {
		return errors.New("state directory belongs to another chain or caller")
	}
	if occupied && (journal.Version != 1 || journal.ID == "" || journal.ChainID != chainID.String() || journal.Sender != old.Caller) {
		return errors.New("legacy signed journal identity mismatch")
	}
	seen := map[string]bool{}
	matched := !occupied
	for _, name := range names {
		if name == "" || seen[name] {
			return errors.New("invalid legacy inventory identity")
		}
		seen[name] = true
		var job legacyJob
		found, err := store.Load("strategy."+name+".v1", &job)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		owns := occupied && job.ID == journal.ID
		if job.ID == "" {
			return errors.New("invalid legacy job identity")
		}
		switch job.Phase {
		case "prepared", "authorized":
			if !configured[name] {
				return errors.New("active legacy executor removed from configuration")
			}
		case "settled", "failed":
		default:
			return errors.New("invalid legacy job phase")
		}
		if owns {
			if !configured[name] {
				return errors.New("legacy transaction owner removed from configuration")
			}
			if matched {
				return errors.New("legacy journal has multiple owners")
			}
			matched = true
		}
	}
	if !matched {
		return errors.New("orphan legacy signed journal has no workflow owner")
	}
	return nil
}

func checkpointInventory(store CheckpointStore) (Deployment, []Workflow, bool, error) {
	var identity Deployment
	var identityRaw json.RawMessage
	found, err := store.LoadNamespace(Namespace, "deployment", &identityRaw)
	if err != nil {
		return identity, nil, false, err
	}
	rows, err := store.ListNamespace(Namespace)
	if err != nil {
		return identity, nil, false, err
	}
	if !found {
		if len(rows) > 0 {
			return identity, nil, false, errors.New("active namespace has no deployment identity")
		}
		return identity, nil, false, nil
	}
	if err = json.Unmarshal(identityRaw, &identity); err != nil || identity.Kind != "deployment" || identity.Version != 2 {
		return identity, nil, false, errors.New("invalid deployment checkpoint")
	}
	var workflows []Workflow
	seen := map[string]bool{}
	identities := 0
	for _, raw := range rows {
		var header struct {
			Kind string `json:"kind"`
		}
		if err = json.Unmarshal(raw, &header); err != nil {
			return identity, nil, false, err
		}
		if header.Kind != "" {
			if header.Kind != "deployment" || !bytes.Equal(raw, identityRaw) {
				return identity, nil, false, errors.New("unexpected deployment checkpoint")
			}
			identities++
			if identities > 1 {
				return identity, nil, false, errors.New("duplicate deployment checkpoint")
			}
			continue
		}
		var w Workflow
		if err = json.Unmarshal(raw, &w); err != nil || w.Name == "" || seen[w.Name] {
			return identity, nil, false, errors.New("invalid or duplicate workflow checkpoint")
		}
		var actual json.RawMessage
		exists, err := store.LoadNamespace(Namespace, WorkflowKey(w.Name), &actual)
		if err != nil {
			return identity, nil, false, err
		}
		if !exists || !bytes.Equal(raw, actual) {
			return identity, nil, false, errors.New("workflow checkpoint filename differs from identity")
		}
		seen[w.Name] = true
		workflows = append(workflows, w)
	}
	if identities != 1 {
		return identity, nil, false, errors.New("deployment checkpoint missing from namespace")
	}
	return identity, workflows, true, nil
}

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
	identity, rows, found, err := checkpointInventory(r.Store)
	if err != nil {
		return err
	}
	if !found {
		if err = r.importLegacy(strategies); err != nil {
			r.fault = err
			return err
		}
		identity, rows, found, err = checkpointInventory(r.Store)
		if err != nil {
			return err
		}
	}
	if !found || identity.Kind != "deployment" || identity.Version != 2 || identity.ChainID == nil || identity.ChainID.Cmp(r.ChainID) != 0 || identity.Caller != r.Caller {
		return errors.New("state deployment identity mismatch")
	}
	workflows := make(map[string]Workflow)
	accounts := make(map[common.Address]string)
	owner := ""
	for _, w := range rows {
		if err = r.validateWorkflow(w, strategies); err != nil {
			return err
		}
		if _, exists := workflows[w.Name]; exists {
			return errors.New("duplicate workflow checkpoint")
		}
		if w.ownsAccount() {
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
