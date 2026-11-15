package recipes

import (
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"testing"
)

var testTarget = common.HexToAddress("0x11")

func TestBudgetRequiresRecipeSelectedAmount(t *testing.T) {
	read := step(Read, pack(t, readArgs, big.NewInt(0)))
	upper := func(a, b *big.Int) []byte { return step(LessOrEqual, pack(t, binaryArgs, a, b)) }
	equal := func(a, b *big.Int) []byte { return step(Equal, pack(t, binaryArgs, a, b)) }
	query := pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))
	cases := []struct {
		name    string
		steps   [][]byte
		results map[uint]*big.Int
		want    int64
		invalid bool
		exact   bool
	}{
		{"read ceiling leaves amount unspecified", [][]byte{read, upper(input(0), result(0))}, map[uint]*big.Int{0: big.NewInt(100)}, 0, true, false},
		{"literal cap leaves amount unspecified", [][]byte{read, upper(input(0), result(0)), upper(input(0), literal(17))}, map[uint]*big.Int{0: big.NewInt(100)}, 0, true, false},
		{"multiple caps leave amount unspecified", [][]byte{read, step(Min, pack(t, binaryArgs, result(0), literal(50))), upper(input(0), result(0)), upper(input(0), result(1))}, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(50)}, 0, true, false},
		{"explicit minimum", [][]byte{read, step(Min, pack(t, binaryArgs, result(0), literal(17))), equal(input(0), result(1))}, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(17)}, 17, false, true},
		{"equality precedence", [][]byte{read, equal(input(0), result(0)), upper(input(0), literal(200))}, map[uint]*big.Int{0: big.NewInt(100)}, 100, false, true},
		{"exact violates cap", [][]byte{read, equal(input(0), result(0)), upper(input(0), literal(17))}, map[uint]*big.Int{0: big.NewInt(100)}, 0, true, true},
		{"exact violates lower", [][]byte{read, equal(input(0), result(0)), upper(literal(20), input(0))}, map[uint]*big.Int{0: big.NewInt(17)}, 0, true, true},
		{"literal alone", [][]byte{upper(input(0), literal(17))}, nil, 0, true, false},
		{"free", [][]byte{read}, nil, 0, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Recipe{Queries: [][]byte{query}, Steps: tc.steps}
			b, err := Budget(r, Value{Source: Input, Value: big.NewInt(0)}, uint(len(r.Steps)))
			if err == nil {
				var value *big.Int
				value, err = EconomicResult(b, tc.results)
				if err == nil && (value.Int64() != tc.want || b.Exact != tc.exact) {
					t.Fatalf("amount=%s exact=%v", value, b.Exact)
				}
			}
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v want invalid=%v", err, tc.invalid)
			}
		})
	}
}

func TestReadBindingsRequireDeterminateInputs(t *testing.T) {
	baseQuery := pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))
	boundQuery := pack(t, queryArgs, testTarget, make([]byte, 36), []*big.Int{binding(4, input(0))}, big.NewInt(0))
	recipe := Recipe{Queries: [][]byte{baseQuery, boundQuery}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0))), step(Equal, pack(t, binaryArgs, input(0), result(0))), step(Read, pack(t, readArgs, big.NewInt(1)))}}
	if _, err := ExactValue(recipe, Value{Source: Result, Value: big.NewInt(2)}, 3); err != nil {
		t.Fatal(err)
	}
	recipe.Steps[1] = step(LessOrEqual, pack(t, binaryArgs, input(0), result(0)))
	if _, err := ExactValue(recipe, Value{Source: Result, Value: big.NewInt(2)}, 3); err == nil {
		t.Fatal("query accepted input with only an upper bound")
	}
	recipe.Steps[1] = step(LessOrEqual, pack(t, binaryArgs, result(0), input(0)))
	if _, err := ExactValue(recipe, Value{Source: Result, Value: big.NewInt(2)}, 3); err == nil {
		t.Fatal("query accepted input without preceding exact equality")
	}
}

func TestEconomicAnchoredBoundsAndMixedFloors(t *testing.T) {
	query := pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))
	r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0))), step(Equal, pack(t, binaryArgs, input(1), result(0))), step(LessOrEqual, pack(t, binaryArgs, input(0), input(1)))}}
	budget, err := Budget(r, Value{Source: Input, Value: big.NewInt(0)}, uint(len(r.Steps)))
	if err == nil {
		t.Fatalf("input bounded by an exact input still permits multiple values: %+v", budget)
	}
	r.Steps = [][]byte{step(Read, pack(t, readArgs, big.NewInt(0))), step(LessOrEqual, pack(t, binaryArgs, literal(1), input(0))), step(LessOrEqual, pack(t, binaryArgs, result(0), input(0)))}
	floors, err := Floor(r, Value{Source: Input, Value: big.NewInt(0)}, uint(len(r.Steps)))
	if err != nil || len(floors) != 2 {
		t.Fatalf("floors=%v error=%v", floors, err)
	}
	for _, floor := range floors {
		if _, err = EconomicResult(floor, map[uint]*big.Int{0: big.NewInt(50)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDependencyDAGMemoizedAcrossInputAnchors(t *testing.T) {
	query := pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))
	r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}
	previous := int64(0)
	for i := int64(0); i < 128; i++ {
		r.Steps = append(r.Steps, step(Equal, pack(t, binaryArgs, input(i), result(previous))))
		r.Steps = append(r.Steps, step(Add, pack(t, binaryArgs, input(i), input(i))))
		previous = int64(len(r.Steps) - 1)
	}
	needed, err := dependencies(r, []Value{{Source: Result, Value: big.NewInt(previous)}})
	if err != nil || len(needed) != 129 {
		t.Fatalf("DAG dependencies=%d error=%v", len(needed), err)
	}
}

func TestExactNonfinancialValues(t *testing.T) {
	r := Recipe{Steps: [][]byte{step(Add, pack(t, binaryArgs, literal(4), literal(5))), step(Equal, pack(t, binaryArgs, input(0), result(0)))}}
	exact, err := ExactValue(r, Value{Source: Input, Value: big.NewInt(0)}, 2)
	if err != nil || !exact.Exact {
		t.Fatal(err)
	}
	v, err := EconomicResult(exact, map[uint]*big.Int{0: big.NewInt(9)})
	if err != nil || v.Int64() != 9 {
		t.Fatalf("index=%v error=%v", v, err)
	}
	if _, err = Budget(r, Value{Source: Input, Value: big.NewInt(0)}, 2); err != nil {
		t.Fatal(err)
	}
}

func TestFixedRecipeAmounts(t *testing.T) {
	for _, source := range []Value{{Source: Literal, Value: big.NewInt(17)}, {Source: Input, Value: big.NewInt(0)}} {
		r := Recipe{Steps: [][]byte{step(Equal, pack(t, binaryArgs, input(0), literal(17)))}}
		budget, err := Budget(r, source, 1)
		if err != nil {
			t.Fatal(err)
		}
		amount, err := EconomicResult(budget, nil)
		if err != nil || amount.Int64() != 17 {
			t.Fatalf("fixed budget = %v, %v", amount, err)
		}
		floors, err := Floor(r, source, 1)
		if err != nil || len(floors) != 1 {
			t.Fatalf("fixed floor = %v, %v", floors, err)
		}
		amount, err = EconomicResult(floors[0], nil)
		if err != nil || amount.Int64() != 17 {
			t.Fatalf("fixed proceeds = %v, %v", amount, err)
		}
	}
}
