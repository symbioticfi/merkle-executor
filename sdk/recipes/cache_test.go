package recipes

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type cachedReader struct {
	*executionReader
	logReads, allReads int
	badHeader          uint64
	headerMode         string
}

func (r *cachedReader) HeaderByNumber(ctx context.Context, b *big.Int) (*types.Header, error) {
	r.allReads++
	if b != nil && b.Uint64() == r.badHeader {
		switch r.headerMode {
		case "error":
			return nil, errors.New("cached header failed")
		case "null":
			return nil, nil
		case "wrong":
			return r.headers[10], nil
		}
	}
	return r.executionReader.HeaderByNumber(ctx, b)
}
func (r *cachedReader) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	r.logReads++
	r.allReads++
	return r.executionReader.FilterLogs(ctx, q)
}
func (r *cachedReader) CallContract(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	r.allReads++
	return r.executionReader.CallContract(ctx, m, b)
}

type cachedBatch struct{ *cachedReader }

func (r cachedBatch) BatchRead(ctx context.Context, qs []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	out := make([]protocolio.ReadResult, len(qs))
	for i, q := range qs {
		switch q.Kind {
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = r.HeaderByNumber(ctx, q.Block)
		case protocolio.ReadCall:
			out[i].Data, out[i].Err = r.CallContract(ctx, q.Call, q.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = r.FilterLogs(ctx, q.Filter)
		}
	}
	return out, nil
}
func cacheFixture(t *testing.T, recipe Recipe) (*cachedReader, *RecipeCache) {
	r := &cachedReader{executionReader: newExecutionReader(t)}
	r.fakeReader = fakeChain(t, recipe)
	return r, NewRecipeCache(2, 1<<20)
}
func cachedPrepare(t *testing.T, r *cachedReader, c *RecipeCache, batched bool, build func(Snapshot, Recipe) error) error {
	t.Helper()
	r.validating = false
	r.headerReads = 0
	var reader Reader = r
	if batched {
		reader = cachedBatch{r}
	}
	return PrepareExecutionWithOptions(t.Context(), reader, r.executor, r.account, common.HexToAddress("0x3333"), PreparationOptions{ChainID: big.NewInt(1), Cache: c}, func(s Snapshot, recipe Recipe, _ protocolio.ReadAccessor) ([]byte, []common.Hash, error) {
		r.validating = true
		if build != nil {
			if err := build(s, recipe); err != nil {
				return nil, nil, err
			}
		}
		return executionPayload, []common.Hash{{1}}, nil
	})
}
func TestRecipeCacheRejectsInvalidMetadataBeforeCachedHeader(t *testing.T) {
	for _, mode := range []string{"zero", "future"} {
		t.Run(mode, func(t *testing.T) {
			r, c := cacheFixture(t, Recipe{})
			if err := cachedPrepare(t, r, c, true, nil); err != nil {
				t.Fatal(err)
			}
			hint := c.load(evidenceKey{"1", r.executor})
			want := "unconfigured"
			if mode == "zero" {
				hint.hash = common.Hash{}
				r.recipeHash = common.Hash{}
			} else {
				hint.block = 11
				r.recipeBlock = 11
				want = "future"
			}
			c.put(hint)
			r.badHeader = hint.block
			r.headerMode = "error"
			err := cachedPrepare(t, r, c, true, nil)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("metadata error masked by speculative header: %v", err)
			}
		})
	}
}
func TestRecipeCacheReusesExactBytesAndIsolatesMutableBuilds(t *testing.T) {
	raw := append(step(Call, pack(t, callArgs, common.HexToAddress("0x9999"), []byte{1, 2, 3, 4}, []*big.Int{})), 0xde, 0xad)
	recipe := Recipe{Steps: [][]byte{raw}}
	for _, batched := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batched], func(t *testing.T) {
			r, c := cacheFixture(t, recipe)
			for attempt := range 3 {
				err := cachedPrepare(t, r, c, batched, func(_ Snapshot, got Recipe) error {
					if !bytes.Equal(got.Steps[0], raw) {
						t.Fatal("cache changed exact trailing bytes")
					}
					got.Steps[0][0] = 255
					got.Steps = append(got.Steps, []byte{255})
					return nil
				})
				if err != nil || r.logReads != 1 {
					t.Fatalf("attempt%d evidence reads%d: %v", attempt, r.logReads, err)
				}
			}
			hint := c.load(evidenceKey{"1", r.executor})
			hint.recipe.Steps[0][0] = 255
			if err := cachedPrepare(t, r, c, batched, nil); err != nil || r.logReads != 1 {
				t.Fatalf("load alias poisoned cache: %v", err)
			}
		})
	}
}
func TestRecipeCacheMissesAndRequiredHeaderFailures(t *testing.T) {
	for _, mode := range []string{"commitment", "new block", "event reorg", "miss ignores old header", "matched error", "matched null", "matched wrong", "header before caller", "caller", "root", "role"} {
		t.Run(mode, func(t *testing.T) {
			r, c := cacheFixture(t, Recipe{})
			if err := cachedPrepare(t, r, c, true, nil); err != nil {
				t.Fatal(err)
			}
			originalRoot := r.root
			expectedLogs := 1
			wantError := false
			switch mode {
			case "commitment":
				recipe := Recipe{Steps: [][]byte{step(Call, pack(t, callArgs, common.HexToAddress("0x9999"), []byte{1, 2, 3, 4}, []*big.Int{}))}}
				r.recipeHash, _ = HashRecipe(recipe)
				r.logs = []types.Log{r.recipeLog(t, recipe, 4)}
				expectedLogs = 2
			case "new block", "miss ignores old header":
				r.recipeBlock = 10
				r.logs = []types.Log{r.recipeLog(t, Recipe{}, 4)}
				expectedLogs = 2
				if mode == "miss ignores old header" {
					r.badHeader = 8
					r.headerMode = "error"
				}
			case "event reorg":
				h := types.CopyHeader(r.headers[8])
				h.Extra = []byte("new canonical event")
				r.headers[8] = h
				r.logs = []types.Log{r.recipeLog(t, Recipe{}, 4)}
				expectedLogs = 2
			case "matched error", "matched null", "matched wrong", "header before caller":
				r.badHeader = 8
				r.headerMode = strings.TrimPrefix(mode, "matched ")
				wantError = true
				if mode == "header before caller" {
					r.headerMode = "error"
					r.caller = common.HexToAddress("0x8888")
				}
			case "caller":
				r.caller = common.HexToAddress("0x8888")
				wantError = true
			case "root":
				r.root = common.HexToHash("0x5678")
				wantError = true
			case "role":
				r.allowed = false
				wantError = true
			}
			err := cachedPrepare(t, r, c, true, func(s Snapshot, _ Recipe) error {
				if s.Root != originalRoot {
					return errors.New("fresh tree root mismatch")
				}
				return nil
			})
			if (err != nil) != wantError || r.logReads != expectedLogs {
				t.Fatalf("error=%v logs%d want%d", err, r.logReads, expectedLogs)
			}
			if mode == "header before caller" && !strings.Contains(err.Error(), "cached header") {
				t.Fatalf("caller masked event evidence: %v", err)
			}
		})
	}
}
func TestRecipeCacheStoresOnlyAfterSuccessfulClosingGuards(t *testing.T) {
	for _, mode := range []string{"build", "simulation error", "closing original"} {
		t.Run(mode, func(t *testing.T) {
			r, c := cacheFixture(t, Recipe{})
			r.mode = mode
			err := cachedPrepare(t, r, c, true, func(Snapshot, Recipe) error {
				if mode == "build" {
					return errors.New("build rejected")
				}
				return nil
			})
			if err == nil || c.load(evidenceKey{"1", r.executor}) != nil {
				t.Fatalf("failure populated cache: %v", err)
			}
			r.mode = ""
			if err = cachedPrepare(t, r, c, true, nil); err != nil || r.logReads != 2 {
				t.Fatalf("retry skipped evidence: %v logs%d", err, r.logReads)
			}
		})
	}
}
func TestRecipeCacheBoundedEvictionIsolationAndConcurrentReplacement(t *testing.T) {
	base := &recipeEvidence{key: evidenceKey{"18446744073709551617", common.HexToAddress("0x01")}, recipe: Recipe{Queries: make([][]byte, 100)}, hash: common.HexToHash("0x01"), block: 8, eventHash: common.HexToHash("0x02")}
	cost := evidenceCost(base) + recipeCacheOverhead
	if cost < 3200 {
		t.Fatal("empty slice entries omitted from budget")
	}
	for _, budget := range []int64{cost - 1, cost, 2 * cost} {
		c := NewRecipeCache(1, budget)
		c.put(base)
		if (c.load(base.key) != nil) != (budget >= cost) {
			t.Fatal("oversized entry retained")
		}
	}
	// With spare entry slots, retained bytes alone must evict the oldest entry.
	byteLimited := NewRecipeCache(3, cost+(cost-recipeCacheOverhead)/2)
	byteLimited.put(base)
	replacement := cloneEvidence(base)
	replacement.key.executor = common.HexToAddress("0x99")
	byteLimited.put(replacement)
	if byteLimited.load(base.key) != nil || byteLimited.load(replacement.key) == nil || byteLimited.count != 1 {
		t.Fatal("total byte budget failed to evict")
	}
	large := cloneEvidence(base)
	large.recipe = Recipe{Queries: [][]byte{make([]byte, 1030)}}
	if evidenceCost(large)-512 < 1152 {
		t.Fatal("allocation rounding undercharged")
	}
	c := NewRecipeCache(2, 3*cost)
	c.put(base)
	frozen := c.load(base.key)
	next := cloneEvidence(base)
	next.hash = common.HexToHash("0x03")
	c.put(next)
	c.discard(frozen)
	if got := c.load(base.key); got == nil || got.hash != next.hash {
		t.Fatal("old miss discarded concurrent replacement")
	}
	other := cloneEvidence(base)
	other.key.chainID = "1"
	c.put(other)
	if c.load(base.key) == nil || c.load(other.key) == nil {
		t.Fatal("full chain ID was truncated")
	}
	third := cloneEvidence(base)
	third.key.executor = common.HexToAddress("0x02")
	c.put(third)
	if c.load(base.key) != nil || c.load(other.key) == nil || c.load(third.key) == nil {
		t.Fatal("FIFO capacity eviction failed")
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for n := range 100 {
				entry := cloneEvidence(base)
				entry.key.executor = common.BigToAddress(big.NewInt(int64(i + 1)))
				c.put(entry)
				hint := c.load(entry.key)
				if n%2 == 0 {
					c.discard(hint)
				}
			}
		})
	}
	wg.Wait()
	if c.count > 2 || c.used > c.budget {
		t.Fatal("concurrent insertion exceeded bounds")
	}
}
func TestPrepareExecutionNilBuildFailsBeforeReads(t *testing.T) {
	r, _ := cacheFixture(t, Recipe{})
	if err := PrepareExecution(t.Context(), r, r.executor, r.account, r.caller, nil); err == nil || r.allReads != 0 {
		t.Fatalf("nil build reached reads: %v reads%d", err, r.allReads)
	}
}

func TestRecipeCacheSamePinHintUsesInitialHeaderWithoutExtraRead(t *testing.T) {
	r, c := cacheFixture(t, Recipe{})
	r.recipeBlock = 10
	r.logs = []types.Log{r.recipeLog(t, Recipe{}, 4)}
	var initialCalls int
	for attempt := range 3 {
		r.allReads = 0
		if err := cachedPrepare(t, r, c, true, nil); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			initialCalls = r.allReads
		} else if r.allReads != initialCalls-1 || r.logReads != 1 {
			t.Fatalf("same-pin hint added a read: reads%d cold%d logs%d", r.allReads, initialCalls, r.logReads)
		}
	}
}
