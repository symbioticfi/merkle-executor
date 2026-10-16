package keeper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type countingStore struct {
	Store
	reads             int
	failRead          bool
	failAfterSave     bool
	failReadAfterSave bool
}

func (s *countingStore) Load(k string, out any) (bool, error) {
	s.reads++
	if s.failRead {
		return false, errors.New("injected ownership reload failure")
	}
	return s.Store.Load(k, out)
}
func (s *countingStore) Save(k string, v any) error {
	err := s.Store.Save(k, v)
	if err == nil && s.failAfterSave {
		s.failRead = s.failReadAfterSave
		return errors.New("uncertain persisted write")
	}
	return err
}

type idlePlanner struct{ recoveryPlanner }

func (p *idlePlanner) Prepare(context.Context, Strategy) (Plan, error) {
	return Plan{}, provider.ErrNotReady
}

func TestIdleTickReadsJournalsLinearly(t *testing.T) {
	for _, n := range []int{1, 16, 256} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			k, store, _, _, _ := recoveryHarness(t)
			base := k.Strategies[0]
			k.Strategies = nil
			k.Planner = &idlePlanner{}
			for i := 0; i < n; i++ {
				s := base
				s.Config.Name = fmt.Sprintf("strategy-%d", i)
				s.Config.Account = fmt.Sprintf("0x%040x", i+1)
				k.Strategies = append(k.Strategies, s)
				job := Job{ID: s.Config.Name, Phase: "settled", FinishedAt: time.Now().Add(-2 * time.Hour)}
				job.Plan.Calldata = make([]byte, 5000)
				if err := store.Save(key(s.Config.Name), job); err != nil {
					t.Fatal(err)
				}
			}
			if err := k.CheckRecovery(); err != nil {
				t.Fatal(err)
			}
			counter := &countingStore{Store: store}
			k.Store = counter
			if err := k.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			if counter.reads > 3*n+1 {
				t.Fatalf("%d strategies required %d journal reads; want <= %d", n, counter.reads, 3*n+1)
			}
			t.Logf("%d strategies: %d journal reads", n, counter.reads)
		})
	}
}

func TestTickRefreshesDurableAccountOwnership(t *testing.T) {
	for _, mode := range []string{"reserve", "release", "uncertain write", "unreadable write"} {
		t.Run(mode, func(t *testing.T) {
			k, store, planner, sender, pr := recoveryHarness(t)
			other := k.Strategies[0]
			other.Config.Name = "other"
			switch mode {
			case "reserve":
				pr.done = false
			case "release":
				pr.done = false
				if err := k.Tick(t.Context()); err != nil {
					t.Fatal(err)
				}
				pr.done = true
				planner.prepared = 0
				sender.broadcasts = 0
			case "uncertain write", "unreadable write":
				k.Strategies = append(k.Strategies, other)
				if err := k.CheckRecovery(); err != nil {
					t.Fatal(err)
				}
				k.Strategies = k.Strategies[:1]
				k.Store = &countingStore{Store: store, failAfterSave: true, failReadAfterSave: mode == "unreadable write"}
			}
			k.Strategies = append(k.Strategies, other)
			err := k.Tick(t.Context())
			switch mode {
			case "reserve":
				if err != nil || planner.prepared != 1 || sender.broadcasts != 1 {
					t.Fatalf("reservation lost: %v prepared=%d sent=%d", err, planner.prepared, sender.broadcasts)
				}
			case "release":
				if err != nil || planner.prepared != 1 || sender.broadcasts != 1 {
					t.Fatalf("settled account not released: %v prepared=%d sent=%d", err, planner.prepared, sender.broadcasts)
				}
			case "uncertain write", "unreadable write":
				if err == nil || planner.prepared != 1 || sender.broadcasts != 0 {
					t.Fatalf("continued after unreadable ownership: %v prepared=%d sent=%d", err, planner.prepared, sender.broadcasts)
				}
			}
		})
	}
}
