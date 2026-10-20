package liquidlanerfq

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestRecipeSelectsOneRFQAuthorization(t *testing.T) {
	result := func(index int64) recipes.EconomicValue {
		return recipes.EconomicValue{Expression: recipes.Value{Source: recipes.Result, Value: big.NewInt(index)}}
	}
	requests := []*recipeRequest{
		{budget: result(0), floors: []recipes.EconomicValue{result(1)}},
		{budget: result(2), floors: []recipes.EconomicValue{result(3)}},
	}
	for _, tc := range []struct {
		name        string
		a, b, floor int64
		selected    int
		want        string
	}{
		{"all idle", 0, 0, 9, -1, "idle"},
		{"first active", 11, 0, 9, 0, ""},
		{"second active", 0, 17, 9, 1, ""},
		{"two active", 11, 17, 9, -1, "activates multiple"},
		{"active without floor", 11, 0, 0, -1, "zero output floor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[uint]*big.Int{0: big.NewInt(tc.a), 1: big.NewInt(tc.floor), 2: big.NewInt(tc.b), 3: big.NewInt(tc.floor)}
			got, amount, minimum, err := selectRequest(requests, values)
			if tc.want == "idle" {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("idle accepted: %v", err)
				}
				return
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("wanted %s: %v", tc.want, err)
				}
				return
			}
			if err != nil || got != requests[tc.selected] || amount.Cmp(values[uint(tc.selected*2)]) != 0 || minimum.Int64() != tc.floor {
				t.Fatalf("incorrect selected recipe route: %v", err)
			}
		})
	}
	_, _, _, err := selectRequest(requests, map[uint]*big.Int{})
	if err == nil {
		t.Fatal("missing authenticated result accepted")
	}
}

func TestAutomaticDerivesGroupedCanonicalRequests(t *testing.T) {
	raw, err := os.ReadFile("testdata/frontend-grouped.json")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
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
	recipe, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	p := &Automatic{}
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: fixture.Account}
	requests, err := p.derive(snapshot, recipe, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 13 {
		t.Fatalf("wanted all13 authenticated routes: %d", len(requests))
	}
	requirements, err := p.RecipeRequirements(snapshot, recipe, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(requirements.Actions) != 13 || len(requirements.Inputs) != 3 {
		t.Fatalf("incomplete grouped ownership: %+v", requirements)
	}
	// A zero amount must skip its authorization, even if another route is ready.
	bad := recipes.CloneRecipe(recipe)
	step := requests[0].template.Action.Step
	bad.Steps[step][1:33][31] = 1
	for i := 1; i < 32; i++ {
		bad.Steps[step][i] = 0
	}
	_, err = p.derive(snapshot, bad, tree)
	if err == nil || !strings.Contains(err.Error(), "gate must equal its signed amount") {
		t.Fatalf("ungated inactive authorization accepted: %v", err)
	}
}
