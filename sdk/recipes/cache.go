package recipes

import (
	"bytes"
	"container/list"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type evidenceKey struct {
	chainID  string
	executor common.Address
}
type recipeEvidence struct {
	key       evidenceKey
	recipe    Recipe
	hash      common.Hash
	block     uint64
	eventHash common.Hash
	version   uint64
	cost      int64
}

const recipeCacheOverhead int64 = 256

// RecipeCache retains only exact, previously authenticated event bodies. It is
// safe to share across planners/goroutines. Entries are copied under the mutex;
// discovery and callbacks never hold the lock. Eviction is insertion-order FIFO,
// bounded by both entries and a conservative retained-byte charge (including
// payloads, slice slots, key and entry/allocation overhead). Transient copies are
// not retained and are released with their preparation attempt.
type RecipeCache struct {
	mu           sync.Mutex
	capacity     int
	budget, used int64
	version      uint64
	entries      list.List
}

// NewRecipeCache returns nil when either bound disables caching. Host callers
// derive capacity from their configured strategy count, never untrusted events.
func NewRecipeCache(capacity int, bytes int64) *RecipeCache {
	if capacity <= 0 || bytes <= recipeCacheOverhead {
		return nil
	}
	return &RecipeCache{capacity: capacity, budget: bytes, used: recipeCacheOverhead}
}
func CloneRecipe(r Recipe) Recipe {
	clone := func(src [][]byte) [][]byte {
		if src == nil {
			return nil
		}
		dst := make([][]byte, len(src))
		for i := range src {
			dst[i] = bytes.Clone(src[i])
		}
		return dst
	}
	return Recipe{Queries: clone(r.Queries), Steps: clone(r.Steps)}
}
func cloneEvidence(e *recipeEvidence) *recipeEvidence {
	if e == nil {
		return nil
	}
	c := *e
	c.key.chainID = strings.Clone(e.key.chainID)
	c.recipe = CloneRecipe(e.recipe)
	return &c
}
func (c *RecipeCache) load(key evidenceKey) *recipeEvidence {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for entry := c.entries.Front(); entry != nil; entry = entry.Next() {
		e := entry.Value.(*recipeEvidence)
		if e.key == key {
			return cloneEvidence(e)
		}
	}
	return nil
}

// discard only removes the frozen hint, never a concurrent newer replacement.
func (c *RecipeCache) discard(hint *recipeEvidence) {
	if c == nil || hint == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for entry := c.entries.Front(); entry != nil; entry = entry.Next() {
		e := entry.Value.(*recipeEvidence)
		if e.key == hint.key && e.version == hint.version {
			c.remove(entry)
			return
		}
	}
}
func (c *RecipeCache) remove(entry *list.Element) {
	c.used -= entry.Value.(*recipeEvidence).cost
	c.entries.Remove(entry)
}

func evidenceCost(e *recipeEvidence) int64 {
	// Charge twice each allocation's payload plus 64 bytes of allocation slack.
	// This covers Go size-class/page rounding, including slice backing arrays;
	// 512 additionally covers the entry, list pointers and fixed object headers.
	allocation := func(n int64) int64 {
		if n == 0 {
			return 0
		}
		return 2*n + 64
	}
	cost := int64(512) + allocation(int64(len(e.key.chainID)))
	for _, entries := range [][][]byte{e.recipe.Queries, e.recipe.Steps} {
		cost += allocation(int64(len(entries)) * 32)
		for _, data := range entries {
			cost += allocation(int64(len(data)))
		}
	}
	return cost
}
func (c *RecipeCache) retainable(e *recipeEvidence) bool {
	return c != nil && evidenceCost(e) <= c.budget-recipeCacheOverhead
}

func (c *RecipeCache) put(evidence *recipeEvidence) {
	if c == nil || evidence == nil {
		return
	}
	cost := evidenceCost(evidence)
	if !c.retainable(evidence) {
		return
	}
	e := cloneEvidence(evidence)
	e.cost = cost
	c.mu.Lock()
	defer c.mu.Unlock()
	for entry := c.entries.Front(); entry != nil; entry = entry.Next() {
		if entry.Value.(*recipeEvidence).key == e.key {
			c.remove(entry)
			break
		}
	}
	for c.entries.Len() >= c.capacity || cost > c.budget-c.used {
		c.remove(c.entries.Front())
	}
	c.version++
	e.version = c.version
	c.entries.PushBack(e)
	c.used += cost
}
