package recipes

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
)

func TestProbePrefixBoundsAndCancellation(t *testing.T) {
	if err := ValidateProbePrefix(Recipe{}, 1, PrefixRuntime{}); err == nil {
		t.Fatal("out-of-program prefix accepted")
	}
	recipe := Recipe{Steps: [][]byte{step(Min, pack(t, binaryArgs, input(0), literal(10)))}}
	if err := ValidateProbePrefix(recipe, 1, PrefixRuntime{}); err == nil {
		t.Fatal("circular unresolved input accepted")
	}
	recipe = Recipe{Queries: [][]byte{pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, new(big.Int))}, Steps: [][]byte{step(Read, pack(t, readArgs, new(big.Int)))}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := ProbePrefix(ctx, canceledLens{}, Snapshot{Executor: common.HexToAddress("0x22")}, recipe, 1, PrefixRuntime{}, []byte{1}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	_, _, err = ProbePrefix(t.Context(), canceledLens{}, Snapshot{Executor: testTarget}, recipe, 1, PrefixRuntime{}, []byte{1}, nil)
	if err == nil || !strings.Contains(err.Error(), "self-query") {
		t.Fatal("self-query accepted", err)
	}
}
func TestProbeRejectsUnpreparedMutablePrefixAction(t *testing.T) {
	leaf := Leaf{Target: testTarget, Value: new(big.Int), Data: []byte{1, 2, 3, 4, 0}, DataMask: []byte{0, 0, 0, 0, 255}}
	leaf.Hash, _ = HashLeaf(leaf)
	tree := &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}
	recipe := Recipe{Steps: [][]byte{manageStep(t, leaf.Hash, literal(1), nil)}}
	if err := ValidateProbePrefix(recipe, 1, PrefixRuntime{Tree: tree}); err == nil || !strings.Contains(err.Error(), "mutable bytes") {
		t.Fatal("unprepared provider action accepted", err)
	}
}
