package recipes

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestManagedTargetsPreservesApprovalAndExplicitSkipSemantics(t *testing.T) {
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	approval := step(Approve, pack(t, approveArgs, literal(1), leaf.Target, common.HexToAddress("0x1234"), leaf.Hash))
	recipe := Recipe{Steps: [][]byte{approval, manageStep(t, leaf.Hash, literal(0), nil)}}
	for _, skip := range []bool{false, true} {
		var skips []uint
		if skip {
			skips = []uint{1}
		}
		calldata, _, err := Assemble(recipe, tree, nil, nil, skips)
		if err != nil {
			t.Fatal(err)
		}
		targets, err := ManagedTargets(calldata)
		if err != nil {
			t.Fatal(err)
		}
		if skip {
			if len(targets) != 0 {
				t.Fatal("empty skipped runtime acquired a target")
			}
		} else if len(targets) != 1 || targets["1:0"] != leaf.Target {
			t.Fatalf("approval affected managed claims: %v", targets)
		}
	}
}
func TestManagedTargetsKeepsBoundOutputTargets(t *testing.T) {
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	recipe := Recipe{Steps: [][]byte{step(Add, pack(t, binaryArgs, literal(1), literal(2))), manageStep(t, leaf.Hash, literal(1), []*big.Int{binding(36, result(0))})}}
	calldata, _, err := Assemble(recipe, tree, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := ManagedTargets(calldata)
	if err != nil || targets["1:0"] != leaf.Target {
		t.Fatalf("result binding changed composition: %v %v", targets, err)
	}
}
