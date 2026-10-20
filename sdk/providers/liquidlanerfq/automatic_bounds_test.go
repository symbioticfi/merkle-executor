package liquidlanerfq

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func groupedAuthorizationFixture(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	raw, err := os.ReadFile("testdata/frontend-grouped.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Recipe  hexutil.Bytes
		Account common.Address
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	return provider.Snapshot{ChainID: big.NewInt(1), Account: fixture.Account}, recipe, tree
}

func TestAutomaticBoundsAuthorizationsBeforeHTTP(t *testing.T) {
	snapshot, recipe, tree := groupedAuthorizationFixture(t)
	p := &Automatic{}
	requests, err := p.derive(snapshot, recipe, tree)
	if err != nil {
		t.Fatal(err)
	}
	step := requests[0].template.Action.Step
	for count := len(requests); count < maxRecipeRequests; count++ {
		recipe.Steps = append(recipe.Steps, append(hexutil.Bytes(nil), recipe.Steps[step]...))
	}
	requests, err = p.derive(snapshot, recipe, tree)
	if err != nil || len(requests) != maxRecipeRequests {
		t.Fatalf("maximum authorization count rejected: %v", err)
	}
	recipe.Steps = append(recipe.Steps, append(hexutil.Bytes(nil), recipe.Steps[step]...))
	_, err = p.RecipeRequirements(snapshot, recipe, tree)
	if err == nil || !strings.Contains(err.Error(), "exceeds 64 authorizations") {
		t.Fatalf("unbounded authorization inventory accepted: %v", err)
	}
	// No HTTP dependencies are installed: the bound must fail before network use.
	_, err = p.PrepareRecipe(context.Background(), snapshot, recipe, tree, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds 64 authorizations") {
		t.Fatalf("preparation bypassed the authorization bound: %v", err)
	}
}

func TestAutomaticRejectsAmbiguousAuthenticatedResultsBeforeHTTP(t *testing.T) {
	snapshot, recipe, tree := groupedAuthorizationFixture(t)
	p := &Automatic{}
	requirements, err := p.RecipeRequirements(snapshot, recipe, tree)
	if err != nil {
		t.Fatal(err)
	}
	results := map[uint]*big.Int{}
	for _, value := range requirements.Values {
		if value.Source == recipes.Result {
			results[uint(value.Value.Uint64())] = big.NewInt(1)
		}
	}
	_, err = p.PrepareRecipe(context.Background(), snapshot, recipe, tree, results)
	if err == nil || !strings.Contains(err.Error(), "activates multiple authorizations") {
		t.Fatalf("ambiguous authenticated routes reached HTTP: %v", err)
	}
}
