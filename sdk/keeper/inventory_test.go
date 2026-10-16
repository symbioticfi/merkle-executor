package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type inventoryStore struct {
	Store
	loads       int
	failSave    func(string, any) bool
	failDelete  bool
	afterWrite  bool
	writeFailed bool
}

var errInventoryWrite = errors.New("synthetic-private-provider-payload")

func (s *inventoryStore) Load(name string, out any) (bool, error) {
	s.loads++
	return s.Store.Load(name, out)
}

func (s *inventoryStore) Save(name string, value any) error {
	if s.failSave != nil && s.failSave(name, value) {
		s.writeFailed = true
		if s.afterWrite {
			if err := s.Store.Save(name, value); err != nil {
				return err
			}
		}
		return errInventoryWrite
	}
	return s.Store.Save(name, value)
}

func (s *inventoryStore) Delete(name string) error {
	if s.failDelete {
		s.writeFailed = true
		if s.afterWrite {
			if err := s.Store.Delete(name); err != nil {
				return err
			}
		}
		return errInventoryWrite
	}
	return s.Store.Delete(name)
}

func inventoryJob(t *testing.T, k *Keeper, s Strategy, phase string) Job {
	t.Helper()
	plan, err := k.Planner.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	return Job{ID: s.Config.Name + "-owned", ConfigHash: s.Config.Fingerprint(), Phase: phase, Plan: plan, Progress: make([]provider.Progress, len(s.Providers))}
}

func TestInventoryRecoverySafety(t *testing.T) {
	for _, mode := range []string{"removed-active", "changed-active", "removed-terminal-owner", "unknown-owner", "invalid-phase"} {
		t.Run(mode, func(t *testing.T) {
			k, store, _, sender, _ := recoveryHarness(t)
			s := k.Strategies[0]
			job := inventoryJob(t, k, s, "authorized")
			if err := k.CheckRecovery(); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "removed-active":
				k.Strategies = nil
			case "changed-active":
				k.Strategies[0].Config.Revision = "changed"
			case "removed-terminal-owner":
				job.Phase = "settled"
				sender.pending = job.ID
				k.Strategies = nil
			case "unknown-owner":
				sender.pending = "unowned"
			case "invalid-phase":
				job.Phase = "unrecognized"
			}
			if err := store.Save(key(s.Config.Name), job); err != nil {
				t.Fatal(err)
			}
			if err := k.CheckRecovery(); err == nil {
				t.Fatal("unsafe persisted recovery passed")
			}
			if err := k.Tick(context.Background()); err == nil {
				t.Fatal("unsafe persisted recovery admitted a strategy")
			}
			if len(sender.requests) != 0 {
				t.Fatal("recovery validation submitted work")
			}
		})
	}
}

func TestInventorySenderFirst(t *testing.T) {
	k, store, _, sender, _ := recoveryHarness(t)
	owner := k.Strategies[0]
	job := inventoryJob(t, k, owner, "prepared")
	if err := store.Save(key(owner.Config.Name), job); err != nil {
		t.Fatal(err)
	}
	other := owner
	other.Config.Name = "other"
	other.Config.Account = common.HexToAddress("0x40").Hex()
	k.Strategies = []Strategy{other, owner}
	sender.pending = job.ID
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 2 || sender.requests[0].ID != job.ID {
		t.Fatalf("occupied sender was not recovered before new admission: %+v", sender.requests)
	}
}

func idleInventory(tb testing.TB, historical, current int) (*Keeper, *inventoryStore) {
	tb.Helper()
	disk, err := state.Open(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = disk.Close() })
	store := &inventoryStore{Store: disk}
	k := &Keeper{Store: store, Sender: &recoverySender{}, Planner: waitingPlanner{&recoveryPlanner{}, provider.ErrNotReady}, Mode: "send", OperationTimeout: time.Second, Log: logr.Discard()}
	names := make([]string, historical)
	for i := range historical {
		name := fmt.Sprintf("strategy-%03d", i)
		names[i] = name
		account := common.BigToAddress(big.NewInt(int64(i + 1)))
		s := Strategy{Config: Config{Name: name, Account: account.Hex(), Interval: time.Hour}}
		if i < current {
			k.Strategies = append(k.Strategies, s)
		}
		job := Job{ID: name + "-settled", Phase: "settled", ConfigHash: s.Config.Fingerprint(), FinishedAt: time.Now().Add(-2 * time.Hour),
			Plan:     Plan{Snapshot: provider.Snapshot{Account: account}, Outputs: []provider.Output{{Data: json.RawMessage(`{"fixture":"` + strings.Repeat("a", 4096) + `"}`)}}},
			Progress: []provider.Progress{{Done: true, State: json.RawMessage(`{"phase":"completed"}`)}}}
		job.Result.Execution = &provider.Execution{TxHash: common.HexToHash("0xfeed"), BlockNumber: big.NewInt(10), Logs: []types.Log{{Topics: []common.Hash{common.HexToHash("0x123")}, Data: bytes.Repeat([]byte{1}, 1024)}}}
		if err := store.Save(key(name), job); err != nil {
			tb.Fatal(err)
		}
	}
	if err := store.Save("strategies.v1", names); err != nil {
		tb.Fatal(err)
	}
	store.loads = 0
	return k, store
}

func TestInventoryLinearLoads(t *testing.T) {
	for _, tc := range []struct{ historical, current int }{{1, 1}, {16, 16}, {16, 4}, {256, 256}, {256, 8}} {
		t.Run(fmt.Sprintf("H%d-S%d", tc.historical, tc.current), func(t *testing.T) {
			k, store := idleInventory(t, tc.historical, tc.current)
			if err := k.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			bound := 2*tc.historical + 2*tc.current + 16
			t.Logf("loads=%d bound=%d", store.loads, bound)
			if store.loads > bound {
				t.Fatalf("inventory loads=%d exceed linear bound=%d", store.loads, bound)
			}
		})
	}
}

func TestInventoryWriteFailureInvalidatesIteration(t *testing.T) {
	for _, operation := range []string{"inventory", "prepared", "authorized", "progress", "settled", "delete"} {
		for _, afterWrite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/after-write-%t", operation, afterWrite), func(t *testing.T) {
				k, disk, _, sender, _ := recoveryHarness(t)
				first := k.Strategies[0]
				other := first
				other.Config.Name = "other"
				other.Config.Account = common.HexToAddress("0x40").Hex()
				k.Strategies = append(k.Strategies, other)
				if operation != "inventory" {
					if err := k.CheckRecovery(); err != nil {
						t.Fatal(err)
					}
				}
				if operation == "delete" {
					if err := disk.Save(key(first.Config.Name), inventoryJob(t, k, first, "prepared")); err != nil {
						t.Fatal(err)
					}
				}
				store := &inventoryStore{Store: disk, afterWrite: afterWrite, failDelete: operation == "delete"}
				store.failSave = func(name string, value any) bool {
					if operation == "inventory" {
						return name == "strategies.v1"
					}
					job, ok := value.(Job)
					if !ok || name != key(first.Config.Name) {
						return false
					}
					if operation == "progress" {
						return job.Phase == "authorized" && job.Progress[0].Done
					}
					return job.Phase == operation
				}
				k.Store = store
				var observations []string
				k.Observe = func(strategy, outcome string) { observations = append(observations, strategy+":"+outcome) }
				var logs bytes.Buffer
				k.Log = logr.New(captureLog{&logs})
				if err := k.Tick(context.Background()); !errors.Is(err, errInventoryWrite) {
					t.Fatalf("lost uncertain write error: %v", err)
				}
				if !store.writeFailed || len(observations) != 1 || observations[0] != "_global:error" {
					t.Fatalf("uncertain write did not stop with one global observation: %v", observations)
				}
				if strings.Contains(logs.String(), errInventoryWrite.Error()) {
					t.Fatal("uncertain write leaked provider payload in a strategy log")
				}
				for _, req := range sender.requests {
					if strings.HasPrefix(req.ID, "other-") {
						t.Fatal("later strategy admitted after uncertain write")
					}
				}
				var later Job
				if found, err := disk.Load(key(other.Config.Name), &later); err != nil || found {
					t.Fatalf("later strategy persisted after uncertain write: %t %v", found, err)
				}
				store.failSave, store.failDelete, store.loads = nil, false, 0
				if err := k.Tick(context.Background()); err != nil {
					t.Fatal(err)
				}
				if store.loads < 3 || sender.broadcasts != 2 || sender.PendingID() != "" {
					t.Fatalf("retry did not reload and reconcile actual durable writes: loads=%d broadcasts=%d pending=%s", store.loads, sender.broadcasts, sender.PendingID())
				}
				for _, s := range k.Strategies {
					if job := loadRecoveryJob(t, disk, s.Config.Name); job.Phase != "settled" {
						t.Fatalf("durable retry did not finish %s: %+v", s.Config.Name, job)
					}
				}
			})
		}
	}
}

func TestInventoryReservationTracksDurableTransitions(t *testing.T) {
	for _, done := range []bool{false, true} {
		t.Run(fmt.Sprintf("provider-done-%t", done), func(t *testing.T) {
			k, store, planner, sender, pr := recoveryHarness(t)
			pr.done = done
			k.Strategies[0].Config.Account = strings.ToLower(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Hex())
			other := k.Strategies[0]
			other.Config.Name = "other"
			other.Config.Account = common.HexToAddress(other.Config.Account).Hex()
			k.Strategies = append(k.Strategies, other)
			want := 1
			if done {
				want = 2
			}
			sender.beforeAck = func(id string) error {
				for _, s := range k.Strategies {
					var job Job
					found, err := store.Load(key(s.Config.Name), &job)
					if err != nil {
						return err
					}
					if found && job.ID == id && job.Phase == "authorized" {
						return nil
					}
				}
				return errors.New("acknowledgement preceded durable authorization")
			}
			if err := k.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if planner.prepared != want || sender.broadcasts != want {
				t.Fatalf("same-iteration account reservation prepared=%d broadcasts=%d want=%d", planner.prepared, sender.broadcasts, want)
			}
		})
	}
}

func TestInventoryFreshBetweenTicks(t *testing.T) {
	k, store, planner, sender, pr := recoveryHarness(t)
	pr.done = false
	other := k.Strategies[0]
	other.Config.Name = "other"
	k.Strategies = append(k.Strategies, other)
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if planner.prepared != 1 {
		t.Fatal("active provider work failed to reserve account")
	}
	job := loadRecoveryJob(t, store, "cow")
	job.Phase, job.FinishedAt = "settled", time.Now().UTC()
	if err := store.Save(key("cow"), job); err != nil {
		t.Fatal(err)
	}
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if planner.prepared != 2 || sender.broadcasts != 2 {
		t.Fatal("fresh Tick retained an external completed account reservation")
	}
}

func TestInventoryCopiesDoNotPromoteUnpersistedState(t *testing.T) {
	k, store, _, _, _ := recoveryHarness(t)
	strategy := k.Strategies[0]
	job := inventoryJob(t, k, strategy, "authorized")
	job.Plan.Results = map[uint]string{1: "1"}
	if err := store.Save(key(strategy.Config.Name), job); err != nil {
		t.Fatal(err)
	}
	view, err := k.loadInventory()
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"loaded", "saved"} {
		t.Run(stage, func(t *testing.T) {
			working, found, err := view.job(strategy.Config.Name)
			if err != nil || !found {
				t.Fatalf("load working job: %t %v", found, err)
			}
			if stage == "saved" {
				if err := view.save(store, strategy.Config.Name, working); err != nil {
					t.Fatal(err)
				}
			}
			working.Progress[0].Done = true
			working.Plan.Results[1] = "999"
			working.Plan.Outputs[0].Data[0] = '['
			working.Plan.Snapshot.Account = common.HexToAddress("0x40")
			persisted, _, err := view.job(strategy.Config.Name)
			if err != nil || persisted.Progress[0].Done || persisted.Plan.Results[1] != "1" || persisted.Plan.Outputs[0].Data[0] != '{' || persisted.Plan.Snapshot.Account != job.Plan.Snapshot.Account {
				t.Fatalf("unpersisted working fields aliased the iteration view: %+v %v", persisted, err)
			}
			other := strategy
			other.Config.Name = "other"
			if !view.accountBusy(other) {
				t.Fatal("unpersisted working fields released an account reservation")
			}
		})
	}
}
