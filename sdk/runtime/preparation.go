package runtime

import (
	"strconv"
	"sync"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// runBounded allocates workers rather than one goroutine per queued operation.
func runBounded(n, concurrency int, run func(int)) {
	work := make(chan int)
	var wg sync.WaitGroup
	for range min(n, max(1, concurrency)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				run(i)
			}
		}()
	}
	// Deliver canceled indices too: callers must receive an explicit result for each.
	for i := range n {
		work <- i
	}
	close(work)
	wg.Wait()
}

// A whole Manage slot may be empty only when the operation owns every action.
func validateSkipOwnership(recipe recipes.Recipe, req provider.Requirements, out provider.Output) error {
	if len(out.SkipSteps) == 0 {
		return nil
	}
	inspection, err := recipes.Inspect(recipe)
	if err != nil {
		return err
	}
	owned := make(map[string]bool, len(req.Actions))
	for _, key := range req.Actions {
		owned[key] = true
	}
	counts := map[uint]int{}
	for _, slot := range inspection.Slots {
		if slot.Operation == recipes.Manage {
			counts[slot.StepIndex] = len(slot.Leaves)
		}
	}
	for _, step := range out.SkipSteps {
		if counts[step] == 0 {
			return errors.New("operation skipped a non-managed step")
		}
		for action := range counts[step] {
			key := strconv.FormatUint(uint64(step), 10) + ":" + strconv.Itoa(action)
			if !owned[key] {
				return errors.New("operation skipped a step containing an unowned action")
			}
		}
	}
	return nil
}
