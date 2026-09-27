package keeper

import (
	"context"
	"testing"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

type waitingPlanner struct {
	*recoveryPlanner
	err error
}

func (p waitingPlanner) Prepare(context.Context, Strategy) (Plan, error) { return Plan{}, p.err }

func TestNormalIdleDoesNotSendOrReportFailure(t *testing.T) {
	k, _, p, sender, _ := recoveryHarness(t)
	k.Planner = waitingPlanner{p, errors.Errorf("no eligible rewards: %w", provider.ErrNotReady)}
	outcome := ""
	k.Observe = func(_, got string) { outcome = got }
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if outcome != "idle" || len(sender.requests) != 0 {
		t.Fatalf("idle outcome=%s requests=%d", outcome, len(sender.requests))
	}
}

func TestInvalidProviderDataStillFailsIteration(t *testing.T) {
	k, _, p, _, _ := recoveryHarness(t)
	k.Planner = waitingPlanner{p, errors.New("invalid proof")}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("invalid proof became idle")
	}
}
