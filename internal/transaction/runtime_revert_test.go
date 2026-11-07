package transaction

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func TestCanonicalRevertReturnsEvidenceWithoutObservationError(t *testing.T) {
	for _, resume := range []bool{false, true} {
		name := "execute"
		if resume {
			name = "resume"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			d := driver(t, f)
			reserve(t, d)
			var saved rt.Transaction
			var result rt.TransactionResult
			if resume {
				saved = signedRecord(t, d)
				f.include(f.sent()[0], 1, types.ReceiptStatusFailed)
				d = driver(t, f)
				reserve(t, d)
				result = d.Resume(t.Context(), saved, persist(&saved))
			} else {
				f.onSend = func(tx *types.Transaction, _ int) error { f.include(tx, 1, types.ReceiptStatusFailed); return nil }
				result = d.Execute(t.Context(), intent(), persist(&saved))
			}
			if result.Err != nil || !result.Reverted || result.Pending || result.NotSubmitted || result.Execution == nil {
				t.Fatalf("canonical revert observation = %+v", result)
			}
			if saved.Terminal != "reverted" || saved.Execution == nil || saved.Execution.TxHash != result.Execution.TxHash {
				t.Fatal("reverted evidence was not durably accepted before returning")
			}
			if err := d.Release(t.Context(), "job"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type revertBoundaryPlanner struct {
	plans map[string]rt.Plan
	calls map[string]int
}

func (p *revertBoundaryPlanner) PrepareAll(_ context.Context, strategies []rt.Strategy) []rt.PreparationResult {
	out := make([]rt.PreparationResult, len(strategies))
	for i, strategy := range strategies {
		p.calls[strategy.Config.Name]++
		out[i].Plan = p.plans[strategy.Config.Name].Clone()
	}
	return out
}
func (*revertBoundaryPlanner) Validate(context.Context, rt.Plan) error { return nil }
func (*revertBoundaryPlanner) ConfirmedSnapshot(_ context.Context, snapshot chainio.Snapshot) (chainio.Snapshot, error) {
	return snapshot, nil
}
func (*revertBoundaryPlanner) VerifyConfirmedSnapshot(context.Context, chainio.Snapshot) error {
	return nil
}

type releaseCheckpointAudit struct {
	*Driver
	store    *state.Store
	names    []string
	mu       sync.Mutex
	releases int
}

func (a *releaseCheckpointAudit) Release(ctx context.Context, id string) error {
	for _, name := range a.names {
		var workflow rt.Workflow
		found, err := a.store.LoadNamespace(rt.Namespace, rt.WorkflowKey(name), &workflow)
		if err != nil {
			return err
		}
		if !found || workflow.ID != id {
			continue
		}
		if !workflow.Reserved || workflow.Transaction == nil || workflow.Transaction.Execution == nil {
			return errors.New("signer released before durable canonical checkpoint")
		}
		if workflow.Transaction.Terminal == "reverted" && workflow.Phase != "failed" {
			return errors.New("signer released before durable failed workflow phase")
		}
		if err := a.Driver.Release(ctx, id); err != nil {
			return err
		}
		a.mu.Lock()
		a.releases++
		a.mu.Unlock()
		return nil
	}
	return errors.New("release has no durable workflow")
}
func (a *releaseCheckpointAudit) releaseCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.releases
}

func TestRuntimeCanonicalRevertReleasesLaneAfterCheckpointAndHaltsOnlyFailedStrategy(t *testing.T) {
	f := newFixture()
	d := driver(t, f)
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"reverting", "unrelated"}
	audit := &releaseCheckpointAudit{Driver: d, store: store, names: names}
	planner := &revertBoundaryPlanner{plans: map[string]rt.Plan{}, calls: map[string]int{}}
	var strategies []rt.Strategy
	recipe := recipes.Recipe{}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	header, err := f.HeaderByNumber(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		executorAddress := common.BigToAddress(big.NewInt(int64(100 + i)))
		accountAddress := common.BigToAddress(big.NewInt(int64(200 + i)))
		planner.plans[name] = rt.Plan{Snapshot: chainio.Snapshot{ChainID: big.NewInt(31337), BlockNumber: new(big.Int).Set(header.Number), BlockHash: header.Hash(), Executor: executorAddress, Account: accountAddress, Caller: testSigner(t).Address(), RecipeHash: hash}, Calldata: executor.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)}
		strategies = append(strategies, rt.Strategy{Config: rt.Config{Name: name, Executor: executorAddress.Hex(), Account: accountAddress.Hex(), Interval: time.Hour}})
	}
	runtime := &rt.Runtime{Store: store, Planner: planner, Driver: audit, Strategies: strategies, Mode: "send", ChainID: big.NewInt(31337), Caller: testSigner(t).Address(), OperationTimeout: time.Second, ShutdownTimeout: time.Second, MaxConcurrent: 2, Log: logr.Discard()}
	t.Cleanup(func() { _ = runtime.Close(); _ = store.Close() })
	f.onSend = func(tx *types.Transaction, n int) error {
		status := uint64(types.ReceiptStatusSuccessful)
		if n == 1 {
			status = types.ReceiptStatusFailed
		}
		f.include(tx, 1, status)
		return nil
	}
	if err := runtime.Tick(t.Context()); err == nil || !strings.Contains(err.Error(), "strategy halted on confirmed revert") {
		t.Fatalf("reverted strategy did not report business failure: %v", err)
	}
	var failed rt.Workflow
	found, err := store.LoadNamespace(rt.Namespace, rt.WorkflowKey(names[0]), &failed)
	if err != nil || !found || failed.Phase != "failed" || failed.Reserved || audit.releaseCount() != 1 {
		t.Fatalf("canonical revert wedged lane: found=%v phase=%s reserved=%v releases=%d err=%v", found, failed.Phase, failed.Reserved, audit.releaseCount(), err)
	}
	if err := runtime.Tick(t.Context()); err == nil || !strings.Contains(err.Error(), "strategy halted on confirmed revert") {
		t.Fatalf("sticky business failure vanished: %v", err)
	}
	sent := f.sent()
	if len(sent) != 2 || sent[0].Nonce() != 0 || sent[1].Nonce() != 1 || *sent[1].To() != planner.plans[names[1]].Snapshot.Executor || audit.releaseCount() != 2 {
		t.Fatalf("unrelated strategy did not use next nonce: sends=%d releases=%d", len(sent), audit.releaseCount())
	}
	if planner.calls[names[0]] != 1 {
		t.Fatalf("halted strategy was prepared again: %d", planner.calls[names[0]])
	}
	_ = runtime.Tick(t.Context())
	if len(f.sent()) != 2 {
		t.Fatal("halted strategy or completed unrelated strategy was replayed")
	}
}
