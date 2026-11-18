package aave

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func BenchmarkSharedCapacityDependencies(b *testing.B) {
	for _, levels := range []int{4, 8, 12} {
		b.Run(fmt.Sprintf("%d_levels", levels), func(b *testing.B) {
			var r recipes.Recipe
			addMin := func(a, c *big.Int) *big.Int {
				args, err := binaryArgs.Pack(a, c)
				if err != nil {
					b.Fatal(err)
				}
				r.Steps = append(r.Steps, append([]byte{recipes.Min}, args...))
				return value(recipes.Result, int64(len(r.Steps)-1))
			}
			previous, current := big.NewInt(900), addMin(value(recipes.Input, 0), big.NewInt(1000))
			for range levels {
				previous, current = current, addMin(previous, current)
			}
			last := recipes.Value{Source: recipes.Result, Value: big.NewInt(int64(len(r.Steps) - 1))}
			b.ReportAllocs()
			for b.Loop() {
				if ok, err := capacityChain(r, last, map[uint]bool{}); err != nil || !ok {
					b.Fatal(ok, err)
				}
			}
		})
	}
}
