package keeper

import (
	"context"
	"strings"
	"testing"

	"github.com/go-errors/errors"
	"github.com/go-logr/logr/funcr"
)

type failedPreparation struct {
	Planning
	plan ExecutionPlan
	err  error
}

func (p failedPreparation) PrepareExecution(context.Context, []Strategy) (ExecutionPlan, error) {
	return p.plan, p.err
}

func TestPreparationFailuresAreAttributedOnlyToTheirOwner(t *testing.T) {
	for _, mode := range []string{"send", "simulate"} {
		for _, failure := range []string{"shared", "member", "partial"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				k, planner, sender := runtimeHarness(t)
				k.Mode = mode
				addRuntimeStrategy(t, k, planner, "two", 0x11, 0x21)
				cause := errors.New("read failed")
				var memberCause error = cause
				plan := ExecutionPlan{}
				if failure != "shared" {
					plan.Failures = map[string]error{"one": memberCause}
				}
				if failure == "partial" {
					cause = errors.New("shared discovery failed after one member failed")
				}
				k.Planner = failedPreparation{Planning: planner, plan: plan, err: cause}
				var logs []string
				k.Log = funcr.New(func(_, message string) { logs = append(logs, message) }, funcr.Options{})
				outcomes := map[string]string{}
				k.Observe = func(name, outcome string) { outcomes[name] = outcome }
				if err := k.Tick(t.Context()); !errors.Is(err, cause) {
					t.Fatalf("runtime lost the failure: %v", err)
				} else if failure == "partial" && !errors.Is(err, memberCause) {
					t.Fatalf("runtime lost the earlier member failure: %v", err)
				}
				if len(sender.requests) != 0 || k.Pending() {
					t.Fatal("failed planning submitted work")
				}
				if failure == "shared" && (len(logs) != 0 || len(outcomes) != 0) {
					t.Fatalf("shared failure was assigned to strategies: logs=%v outcomes=%v", logs, outcomes)
				}
				if _, reported := outcomes["two"]; reported {
					t.Fatalf("unrelated strategy was assigned an outcome: %v", outcomes)
				}
				if failure != "shared" && (len(logs) != 1 || !strings.Contains(logs[0], `"strategy"="one"`) || outcomes["one"] != "error") {
					t.Fatalf("failed owner was not reported once: logs=%v outcomes=%v", logs, outcomes)
				}
			})
		}
	}
}
