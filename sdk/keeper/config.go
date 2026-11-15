package keeper

import "time"

// Config identifies one committed executor and its scheduling policy.
// Name must be nonempty and unique within a Keeper.
type Config struct {
	Name, Executor, Account, RecipeHash, Tree string
	Interval                                  time.Duration
}
