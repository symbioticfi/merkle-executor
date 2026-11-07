package runtime

import (
	"bytes"
	"encoding/json"
	"math/big"
	"reflect"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// LegacyConverter decodes one known v1 positional slot entirely offline. The
// host binds protocol-specific validated converters in the original slot order.
type LegacyConverter func(chainio.Snapshot, provider.Output, provider.Progress) ([]provider.Record, error)
type legacyPlan struct {
	Snapshot chainio.Snapshot  `json:"snapshot"`
	Recipe   recipes.Snapshot  `json:"recipe"`
	Calldata hexutil.Bytes     `json:"calldata"`
	Leaves   []common.Hash     `json:"leaves"`
	Outputs  []provider.Output `json:"outputs"`
	Results  map[uint]string   `json:"results,omitempty"`
}
type legacyResult struct {
	ID                           string
	Hash                         common.Hash
	Attempts                     []common.Hash
	Confirmed, Reverted, Pending bool
	Execution                    *chainio.Execution
}
type legacyJob struct {
	ID         string              `json:"id"`
	ConfigHash string              `json:"configHash"`
	Phase      string              `json:"phase"`
	Plan       legacyPlan          `json:"plan"`
	Progress   []provider.Progress `json:"progress"`
	Result     legacyResult        `json:"result"`
	FinishedAt time.Time           `json:"finishedAt"`
}
type legacyJournal struct {
	Version       int                `json:"version"`
	ID            string             `json:"id"`
	ChainID       string             `json:"chainId"`
	Sender        common.Address     `json:"sender"`
	Raw           hexutil.Bytes      `json:"raw"`
	Attempts      []hexutil.Bytes    `json:"attempts,omitempty"`
	Confirmations uint64             `json:"confirmations"`
	Terminal      string             `json:"terminal,omitempty"`
	Execution     *chainio.Execution `json:"execution,omitempty"`
}
type legacyProvenance struct {
	Deployment json.RawMessage `json:"deployment"`
	Job        json.RawMessage `json:"job"`
	Journal    json.RawMessage `json:"journal,omitempty"`
}

func emptyLegacySlot(out provider.Output, progress provider.Progress) bool {
	return len(out.Inputs) == 0 && len(out.Actions) == 0 && len(out.SkipSteps) == 0 && len(out.Data) == 0 && len(progress.State) == 0 && progress.Reference == "" && progress.Outcome == ""
}

func (r *Runtime) importLegacy(strategies map[string]Strategy) error {
	var deploymentRaw, journalRaw json.RawMessage
	legacy, err := r.Store.Load("deployment.v1", &deploymentRaw)
	if err != nil {
		return err
	}
	var names []string
	inventory, err := r.Store.Load("strategies.v1", &names)
	if err != nil {
		return err
	}
	hasJournal, err := r.Store.Load("txmanager.active.v1", &journalRaw)
	if err != nil {
		return err
	}
	if !legacy && (inventory || hasJournal) {
		return errors.New("legacy work has no deployment identity")
	}
	if legacy {
		var old struct {
			ChainID *big.Int       `json:"chainId"`
			Caller  common.Address `json:"caller"`
		}
		if err = json.Unmarshal(deploymentRaw, &old); err != nil {
			return errors.New("invalid legacy deployment")
		}
		if old.ChainID == nil || old.ChainID.Cmp(r.ChainID) != 0 || old.Caller != r.Caller {
			return errors.New("legacy deployment belongs to another chain or caller")
		}
	}
	var journal legacyJournal
	if hasJournal {
		if err = json.Unmarshal(journalRaw, &journal); err != nil {
			return errors.New("invalid legacy signed journal")
		}
		if journal.Version != 1 || journal.ID == "" || journal.ChainID != r.ChainID.String() || journal.Sender != r.Caller || journal.Confirmations > 1024 {
			return errors.New("legacy signed journal identity mismatch")
		}
	}
	values := map[string]any{"deployment": Deployment{Kind: "deployment", Version: 2, ChainID: new(big.Int).Set(r.ChainID), Caller: r.Caller}}
	seen := map[string]bool{}
	matched := !hasJournal
	for _, name := range names {
		if name == "" || seen[name] {
			return errors.New("invalid or duplicate legacy inventory identity")
		}
		seen[name] = true
		var raw json.RawMessage
		found, err := r.Store.Load("strategy."+name+".v1", &raw)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		var old legacyJob
		if err = json.Unmarshal(raw, &old); err != nil {
			return errors.New("invalid legacy job")
		}
		s, configured := strategies[name]
		active := old.Phase == "prepared" || old.Phase == "authorized"
		owns := hasJournal && journal.ID == old.ID
		if active || owns {
			if !configured {
				return errors.New("active legacy executor removed from configuration")
			}
			if s.LegacyConfigHash == "" || old.ConfigHash != s.LegacyConfigHash {
				return errors.New("legacy active workflow configuration changed")
			}
		}
		if old.ID == "" || old.ConfigHash == "" || (old.Phase != "prepared" && old.Phase != "authorized" && old.Phase != "settled" && old.Phase != "failed") {
			return errors.New("invalid legacy job identity or phase")
		}
		if old.Phase != "prepared" && old.Result.ID != old.ID {
			return errors.New("legacy result workflow identity mismatch")
		}
		plan := Plan{Snapshot: old.Plan.Snapshot, RecipeBlock: old.Plan.Recipe.RecipeBlock, Calldata: old.Plan.Calldata, Leaves: old.Plan.Leaves, Results: old.Plan.Results}
		if err = validatePlanIdentity(plan); err != nil {
			return errors.Errorf("invalid legacy prepared identity: %w", err)
		}
		if !reflect.DeepEqual(plan.RecipeSnapshot(), old.Plan.Recipe) {
			return errors.New("legacy recipe snapshot differs from prepared identity")
		}
		if len(old.Plan.Outputs) != len(old.Progress) {
			return errors.New("legacy provider progress count mismatch")
		}
		converters := r.LegacyConverters[name]
		if len(old.Plan.Outputs) > 0 && len(converters) != len(old.Plan.Outputs) && (active || owns) {
			return errors.New("legacy positional provider mapping unavailable")
		}
		for i, out := range old.Plan.Outputs {
			if !active && !owns && len(converters) != len(old.Plan.Outputs) {
				break
			}
			if converters[i] == nil {
				if !emptyLegacySlot(out, old.Progress[i]) {
					return errors.New("nonempty legacy intrinsic input slot")
				}
				continue
			}
			records, err := converters[i](plan.Snapshot, out.Clone(), old.Progress[i].Clone())
			if err != nil {
				return errors.Errorf("validate legacy operation slot %d: %w", i, err)
			}
			plan.Operations = append(plan.Operations, records...)
		}
		sort.Slice(plan.Operations, func(i, j int) bool {
			if plan.Operations[i].Integration != plan.Operations[j].Integration {
				return plan.Operations[i].Integration < plan.Operations[j].Integration
			}
			return plan.Operations[i].ID < plan.Operations[j].ID
		})
		provenance := legacyProvenance{Deployment: deploymentRaw, Job: raw}
		if owns {
			provenance.Journal = journalRaw
		}
		preserved, err := json.Marshal(provenance)
		if err != nil {
			return err
		}
		w := Workflow{Version: 2, IdentityVersion: 1, Name: name, ID: old.ID, ConfigHash: old.ConfigHash, Phase: old.Phase, Plan: plan, FinishedAt: old.FinishedAt, Legacy: preserved}
		if owns {
			if matched {
				return errors.New("legacy journal has multiple workflow owners")
			}
			matched = true
			tx := Transaction{Intent: Intent{ID: old.ID, To: plan.Snapshot.Executor, Data: plan.Calldata}, Sender: journal.Sender, ChainID: new(big.Int).Set(r.ChainID), Admitted: true, Attempts: append([]hexutil.Bytes{journal.Raw}, journal.Attempts...), Terminal: journal.Terminal, Execution: journal.Execution.Clone()}
			if len(journal.Raw) == 0 {
				return errors.New("legacy journal has no signed transaction")
			}
			var first types.Transaction
			if err = first.UnmarshalBinary(journal.Raw); err != nil {
				return errors.New("invalid legacy signed original")
			}
			tx.Nonce = first.Nonce()
			w.Transaction = &tx
			w.Reserved = true
			if journal.Terminal != "" {
				w.Phase = "authorized"
				if journal.Terminal == "reverted" {
					w.Phase = "failed"
					if w.FinishedAt.IsZero() {
						w.FinishedAt = time.Now().UTC()
					}
				}
				if old.Phase == "settled" {
					w.Phase = "settled"
				}
				if old.Phase != "prepared" {
					if err = validateLegacyResult(old.Result, journal.Terminal, journal.Execution); err != nil {
						return err
					}
				}
			} else {
				if old.Phase != "prepared" {
					return errors.New("legacy confirmed workflow has pending journal")
				}
				w.Phase = "pending"
			}
		} else if old.Phase != "prepared" {
			terminal := "confirmed"
			if old.Phase == "failed" {
				terminal = "reverted"
			}
			if err = validateLegacyResult(old.Result, terminal, old.Result.Execution); err != nil {
				return err
			}
			w.Transaction = &Transaction{Intent: Intent{ID: old.ID, To: plan.Snapshot.Executor, Data: plan.Calldata}, Sender: r.Caller, ChainID: new(big.Int).Set(r.ChainID), Admitted: true, Terminal: terminal, Execution: old.Result.Execution.Clone()}
		}
		if err = r.validateWorkflow(w, strategies); err != nil {
			return errors.Errorf("validate imported workflow: %w", err)
		}
		values[WorkflowKey(name)] = w
	}
	if !matched {
		return errors.New("orphan legacy signed journal has no workflow owner")
	}
	// No mutation occurs until every identity and operation has passed validation.
	if err = r.Store.ActivateNamespace(Namespace, values); err != nil {
		return errors.Errorf("activate imported state; reload required: %w", err)
	}
	return nil
}

func validateLegacyResult(result legacyResult, terminal string, execution *chainio.Execution) error {
	if result.ID == "" || result.Pending || result.Confirmed != (terminal == "confirmed") || result.Reverted != (terminal == "reverted") || execution == nil || result.Hash != execution.TxHash || result.Execution == nil || !reflect.DeepEqual(result.Execution.Clone(), execution.Clone()) {
		return errors.New("legacy result differs from canonical journal evidence")
	}
	return execution.Validate()
}
func validateLegacyTerminal(w Workflow) error {
	var provenance legacyProvenance
	if err := json.Unmarshal(w.Legacy, &provenance); err != nil || len(provenance.Job) == 0 || len(provenance.Journal) > 0 {
		return errors.New("legacy terminal provenance missing signed evidence")
	}
	var job legacyJob
	if err := json.Unmarshal(provenance.Job, &job); err != nil {
		return errors.New("invalid legacy terminal provenance")
	}
	if job.ID != w.ID || job.Result.ID != w.ID || job.ConfigHash != w.ConfigHash || !bytes.Equal(job.Plan.Calldata, w.Plan.Calldata) {
		return errors.New("legacy terminal provenance differs from workflow")
	}
	return validateLegacyResult(job.Result, w.Transaction.Terminal, w.Transaction.Execution)
}
