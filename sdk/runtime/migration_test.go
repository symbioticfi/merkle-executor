package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func writeLegacyFixture(t *testing.T, r *Runtime, s *coordinatorStore, w Workflow, journal bool) {
	t.Helper()
	name := r.Strategies[0].Config.Name
	r.Strategies[0].LegacyConfigHash = "old-config"
	w.ConfigHash = "old-config"
	job := legacyJob{ID: w.ID, ConfigHash: w.ConfigHash, Phase: w.Phase, Plan: legacyPlan{Snapshot: w.Plan.Snapshot, Recipe: w.Plan.RecipeSnapshot(), Calldata: w.Plan.Calldata, Leaves: w.Plan.Leaves, Outputs: []provider.Output{{}}, Results: w.Plan.Results}, Progress: []provider.Progress{{}}, FinishedAt: w.FinishedAt}
	if w.Phase == "pending" {
		job.Phase = "prepared"
	} else if w.Transaction != nil {
		tx := w.Transaction
		job.Result = legacyResult{ID: w.ID, Hash: tx.Execution.TxHash, Confirmed: tx.Terminal == "confirmed", Reverted: tx.Terminal == "reverted", Execution: tx.Execution.Clone()}
	}
	if err := s.Save("deployment.v1", struct {
		ChainID uint64         `json:"chainId"`
		Caller  common.Address `json:"caller"`
	}{1, r.Caller}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("strategies.v1", []string{name}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("strategy."+name+".v1", job); err != nil {
		t.Fatal(err)
	}
	if journal {
		tx := w.Transaction
		record := legacyJournal{Version: 1, ID: w.ID, ChainID: r.ChainID.String(), Sender: r.Caller, Raw: tx.Attempts[0], Attempts: tx.Attempts[1:], Confirmations: 1, Terminal: tx.Terminal, Execution: tx.Execution.Clone()}
		if err := s.Save("txmanager.active.v1", record); err != nil {
			t.Fatal(err)
		}
	}
	r.LegacyConverters = map[string][]LegacyConverter{name: {nil}}
}

func TestMigrationRefusesInvalidOwnershipWithoutActivating(t *testing.T) {
	for _, kind := range []string{"orphan", "removed", "changed-config", "wrong-deployment", "nonempty-intrinsic", "unknown-slot"} {
		t.Run(kind, func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			name := r.Strategies[0].Config.Name
			w := signedCoordinatorWorkflow(t, r, name, "pending")
			writeLegacyFixture(t, r, s, w, true)
			switch kind {
			case "orphan":
				if err := s.Save("strategies.v1", []string{}); err != nil {
					t.Fatal(err)
				}
			case "removed":
				r.Strategies = nil
			case "changed-config":
				r.Strategies[0].LegacyConfigHash = "different"
			case "wrong-deployment":
				r.Caller = common.HexToAddress("0x123")
			case "nonempty-intrinsic":
				var job legacyJob
				_, _ = s.Load("strategy."+name+".v1", &job)
				job.Plan.Outputs[0].Data = json.RawMessage(`{"unknown":true}`)
				if err := s.Save("strategy."+name+".v1", job); err != nil {
					t.Fatal(err)
				}
			case "unknown-slot":
				r.LegacyConverters = nil
			}
			if err := r.Start(context.Background()); err == nil {
				t.Fatal("invalid migration accepted")
			}
			var identity Deployment
			if found, err := s.LoadNamespace(Namespace, "deployment", &identity); err != nil || found {
				t.Fatal("failed migration activated v2", found, err)
			}
			if d.owner != "" || d.executions != 0 || d.resumes != 0 {
				t.Fatal("migration performed transaction I/O")
			}
		})
	}
}

func TestMigrationRetainsExactSignedIdentityAndOriginalJSON(t *testing.T) {
	r, s, d := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "pending")
	writeLegacyFixture(t, r, s, w, true)
	var before json.RawMessage
	if found, err := s.Load("strategy."+name+".v1", &before); err != nil || !found {
		t.Fatal(found, err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := r.Snapshot()[0]
	if got.IdentityVersion != 1 || got.ConfigHash != "old-config" || got.ID != w.ID || !bytes.Equal(got.Transaction.Attempts[0], w.Transaction.Attempts[0]) || got.Phase != "pending" || d.owner != w.ID {
		t.Fatal("migration changed durable identity", got)
	}
	var provenance legacyProvenance
	if err := json.Unmarshal(got.Legacy, &provenance); err != nil || !bytes.Equal(provenance.Job, before) {
		t.Fatal("raw v1 provenance lost", err)
	}
	var after json.RawMessage
	if found, err := s.Load("strategy."+name+".v1", &after); err != nil || !found || !bytes.Equal(before, after) {
		t.Fatal("original v1 changed")
	}
	if d.resumes != 0 || d.executions != 0 {
		t.Fatal("offline import invoked observer")
	}
}

func TestMigrationAcknowledgedExternalProgressRestoresByIdentityAfterReorder(t *testing.T) {
	r, s, _ := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "authorized")
	writeLegacyFixture(t, r, s, w, false)
	state := json.RawMessage(`{"cursor":23}`)
	var old legacyJob
	_, _ = s.Load("strategy."+name+".v1", &old)
	old.Plan.Outputs[0].Data = json.RawMessage(`{"order":"external-id"}`)
	old.Progress[0] = provider.Progress{State: state, Reference: "external-id"}
	if err := s.Save("strategy."+name+".v1", old); err != nil {
		t.Fatal(err)
	}
	op := &checkpointOperation{state: state, done: true}
	in := &testIntegration{name: "route", ops: []provider.Operation{op}}
	extra := &testIntegration{name: "new-unmatched"}
	r.Strategies[0].Integrations = []provider.Integration{extra, in}
	r.LegacyConverters[name] = []LegacyConverter{func(_ chainio.Snapshot, out provider.Output, progress provider.Progress) ([]provider.Record, error) {
		return []provider.Record{{Integration: in.ID(), Policy: in.Policy(), ID: op.ID(), Output: out, Progress: progress}}, nil
	}}
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := r.Snapshot()[0]
	if got.Phase != "settled" || got.IdentityVersion != 1 || got.ConfigHash != "old-config" || op.calls != 1 || got.Plan.Operations[0].Progress.Reference != "external-id" {
		t.Fatal("external state/order changed during migration", got, op.calls)
	}
}

func TestMigrationTerminalEvidenceAndUnsignedPreparation(t *testing.T) {
	for _, phase := range []string{"prepared", "settled", "failed"} {
		t.Run(phase, func(t *testing.T) {
			r, s, d := coordinatorHarness(t)
			name := r.Strategies[0].Config.Name
			kind := phase
			if kind == "prepared" {
				kind = "pending"
			}
			w := signedCoordinatorWorkflow(t, r, name, kind)
			w.Phase = phase
			if phase == "prepared" {
				w.Transaction = nil
			}
			writeLegacyFixture(t, r, s, w, false)
			if err := r.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := r.Snapshot()[0]
			if got.Phase != phase || got.IdentityVersion != 1 || d.owner != "" {
				t.Fatal(got)
			}
			if phase != "prepared" && got.Transaction.Execution == nil {
				t.Fatal("acknowledged terminal evidence lost")
			}
		})
	}
}

func TestMigrationAccountConflictDoesNotActivateNamespace(t *testing.T) {
	r, s, _ := coordinatorHarness(t)
	name := r.Strategies[0].Config.Name
	w := signedCoordinatorWorkflow(t, r, name, "pending")
	writeLegacyFixture(t, r, s, w, false)
	second := r.Strategies[0]
	second.Config.Name = "other"
	second.Config.Executor = common.HexToAddress("0x99").Hex()
	r.Strategies = append(r.Strategies, second)
	r.LegacyConverters["other"] = []LegacyConverter{nil}
	var old legacyJob
	if found, err := s.Load("strategy."+name+".v1", &old); err != nil || !found {
		t.Fatal(found, err)
	}
	old.ID = "other-id"
	old.Plan.Snapshot.Executor = common.HexToAddress(second.Config.Executor)
	old.Plan.Recipe.Executor = old.Plan.Snapshot.Executor
	if err := s.Save("strategy.other.v1", old); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("strategies.v1", []string{name, "other"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err == nil {
		t.Fatal("duplicate account owners accepted")
	}
	var identity Deployment
	if found, err := s.LoadNamespace(Namespace, "deployment", &identity); err != nil || found {
		t.Fatal("rejected account conflict activated namespace", found, err)
	}
}
