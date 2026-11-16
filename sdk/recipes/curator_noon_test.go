package recipes

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// This is the frontend's actual standalone recipe, not the separately authored
// capped/input-driven Noon program used by the native issuer integration test.
func TestCuratorNoonRedemption(t *testing.T) {
	raw, err := os.ReadFile("testdata/curator-noon-redemption.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Recipe     hexutil.Bytes
		RecipeHash common.Hash
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	recipe, err := DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	if hash, err := HashRecipe(recipe); err != nil || hash != fixture.RecipeHash {
		t.Fatal("frontend recipe commitment changed", err)
	}
	if len(recipe.Queries) != 1 || len(recipe.Steps) != 3 {
		t.Fatal("standalone redemption must only read USN, approve and redeem")
	}
	for i, want := range []uint8{Read, Manage, Manage} {
		if len(recipe.Steps[i]) == 0 || recipe.Steps[i][0] != want {
			t.Fatalf("step %d must have opcode %d", i, want)
		}
	}
	tree, err := DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(recipe)
	if err != nil || len(inspection.Inputs) != 0 || len(inspection.Slots) != 2 {
		t.Fatal("redemption must need no provider inputs", err)
	}
	calldata, leaves, err := Assemble(recipe, tree, nil, nil, nil)
	if err != nil || len(leaves) != 2 {
		t.Fatal("frontend permissions cannot assemble the standalone redemption", err)
	}
	for _, tc := range []struct {
		name    string
		balance int64
		work    bool
	}{
		{name: "empty account"},
		{name: "received USN", balance: 1e18, work: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := make([]*big.Int, len(recipe.Steps))
			for i := range values {
				values[i] = new(big.Int)
			}
			values[0].SetInt64(tc.balance)
			returned, err := executorABI.GetABI().Methods["execute"].Outputs.Pack(values)
			if err != nil {
				t.Fatal(err)
			}
			if work, err := HasWork(calldata, returned); err != nil || work != tc.work {
				t.Fatalf("work=%v want=%v: %v", work, tc.work, err)
			}
		})
	}
	tree.Leaves = nil
	if _, _, err := Assemble(recipe, tree, nil, nil, nil); err == nil {
		t.Fatal("missing active-root permissions accepted")
	}
}
