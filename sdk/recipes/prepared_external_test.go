package recipes

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func externalFixture(t *testing.T, count int) (*preparedReader, []PreparedRecipe, []ExecutorCall, [][]common.Hash, *RecipeCache) {
	t.Helper()
	r, executors, accounts := preparedFixture(t, count)
	cache := NewRecipeCache(count, 1<<20)
	prepared, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{ChainID: big.NewInt(1), Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	calls, leaves := make([]ExecutorCall, count), make([][]common.Hash, count)
	for i, member := range prepared {
		data, _, err := Assemble(member.Recipe, &Tree{}, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		calls[i] = ExecutorCall{Executor: executors[i], Data: data}
		leaves[i] = []common.Hash{{1}}
		r.members[i].head = 11
	}
	r.groups = nil
	return r, prepared, calls, leaves, cache
}

func TestPreparedExternalSimulationUsesFreshPinnedCopiesWithoutResults(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(big.NewInt(int64(count)).String(), func(t *testing.T) {
			r, prepared, calls, leaves, cache := externalFixture(t, count)
			caller := r.members[0].caller
			original := bytes.Clone(calls[0].Data)
			callbacks := 0
			err := ValidatePreparedBatchWithSimulation(t.Context(), r, prepared, calls, leaves, caller, func(_ context.Context, request PreparedBatchSimulationRequest) error {
				callbacks++
				if len(request.Calls) != count || request.Caller != caller || request.BlockNumber.Uint64() != 11 || request.GasLimit != 16_777_216 {
					t.Fatalf("external simulation lost exact context: %+v", request)
				}
				for i, call := range request.Calls {
					if call.Executor != calls[i].Executor || !bytes.Equal(call.Data, calls[i].Data) {
						t.Fatal("external simulation received different selected calls")
					}
				}
				request.Calls[0].Data[0] ^= 0xff
				request.Calls[0].Executor = common.Address{99}
				request.BlockNumber.SetUint64(99)
				return nil
			})
			if err != nil || callbacks != 1 || r.simulations != 0 || len(r.groups) != 2 {
				t.Fatalf("external admission: err=%v callbacks=%d inner=%d groups=%d", err, callbacks, r.simulations, len(r.groups))
			}
			if !bytes.Equal(calls[0].Data, original) || calls[0].Executor != prepared[0].Snapshot.Executor || r.members[0].headers[11].Number.Uint64() != 11 {
				t.Fatal("callback mutated selected bytes or canonical pin")
			}
			for _, member := range prepared {
				if cache.load(evidenceKey{chainID: "1", executor: member.Snapshot.Executor}) == nil {
					t.Fatal("successful external admission did not publish evidence")
				}
			}
		})
	}
}

func TestPreparedExternalSimulationRetainsAuthenticationAndClosingGuards(t *testing.T) {
	for _, failure := range []string{"nil callback", "caller", "executor", "recipe", "calldata", "fresh caller", "fresh account", "fresh recipe", "fresh root", "role", "opening original reorg", "closing original reorg", "closing fresh reorg", "revert", "transport", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			r, prepared, calls, leaves, cache := externalFixture(t, 1)
			caller := r.members[0].caller
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			callbacks := 0
			reorg := func(number uint64) {
				header := types.CopyHeader(r.members[0].headers[number])
				header.Extra = []byte{1}
				r.members[0].headers[number] = header
			}
			simulate := PreparedBatchSimulation(func(context.Context, PreparedBatchSimulationRequest) error {
				callbacks++
				switch failure {
				case "closing original reorg":
					reorg(10)
				case "closing fresh reorg":
					reorg(11)
				case "revert":
					return repeatedProbeError{}
				case "transport":
					return errors.New("outer call unavailable")
				case "cancel":
					cancel()
				}
				return nil
			})
			switch failure {
			case "nil callback":
				simulate = nil
			case "caller":
				caller = common.Address{99}
			case "executor":
				calls[0].Executor = common.Address{99}
			case "recipe":
				prepared[0].Recipe.Steps[0][0] = 255
			case "calldata":
				calls[0].Data = []byte{1, 2, 3}
			case "fresh caller":
				r.members[0].caller = common.Address{99}
			case "fresh account":
				r.members[0].account = common.Address{99}
			case "fresh recipe":
				r.members[0].recipeHash = common.Hash{99}
			case "fresh root":
				r.members[0].root = common.Hash{99}
			case "role":
				r.members[0].allowed = false
			case "opening original reorg":
				reorg(10)
			}
			err := ValidatePreparedBatchWithSimulation(ctx, r, prepared, calls, leaves, caller, simulate)
			if err == nil || cache.load(evidenceKey{chainID: "1", executor: prepared[0].Snapshot.Executor}) != nil || r.simulations != 0 {
				t.Fatalf("failed external admission escaped guard: err=%v inner=%d", err, r.simulations)
			}
			wantCallback := failure == "closing original reorg" || failure == "closing fresh reorg" || failure == "revert" || failure == "transport" || failure == "cancel"
			if (callbacks == 1) != wantCallback {
				t.Fatalf("authentication/callback order changed: callbacks=%d", callbacks)
			}
			if failure == "revert" && !BatchRejected(err) {
				t.Fatal("external revert lost simulation classification")
			}
			if failure == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("external cancellation lost context cause")
			}
		})
	}
}

func TestExternalEnvelopeOwnsUsefulnessWhileGenericChecksResults(t *testing.T) {
	r, executors, accounts := preparedFixture(t, 1)
	recipe := Recipe{Steps: [][]byte{manageStep(t, common.Hash{1}, literal(0), nil)}}
	r.members[0].recipeHash, _ = HashRecipe(recipe)
	r.members[0].logs = []types.Log{r.members[0].recipeLog(t, recipe, 4)}
	prepared, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := executorABI.TryPackExecute(recipe.Queries, recipe.Steps, nil, [][]byte{{}})
	if err != nil {
		t.Fatal(err)
	}
	calls, leaves := []ExecutorCall{{Executor: executors[0], Data: data}}, [][]common.Hash{{common.Hash{1}}}
	if err = ValidatePreparedBatchWithSimulation(t.Context(), r, prepared, calls, leaves, r.members[0].caller, func(context.Context, PreparedBatchSimulationRequest) error { return nil }); err != nil || r.simulations != 0 {
		t.Fatalf("external envelope incorrectly required generic work results: %v", err)
	}
	if err = ValidatePreparedBatch(t.Context(), r, prepared, calls, leaves, []map[uint]string{nil}, r.members[0].caller); !errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("generic readiness changed: %v", err)
	}
	if err = ValidatePreparedBatch(t.Context(), r, prepared, calls, leaves, []map[uint]string{{0: "1"}}, r.members[0].caller); err == nil || errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("generic numerical witness validation changed: %v", err)
	}
}
