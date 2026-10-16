package keeper

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkKeeperIdleInventory(b *testing.B) {
	for _, historical := range []int{16, 256} {
		b.Run(fmt.Sprintf("H%d", historical), func(b *testing.B) {
			k, store := idleInventory(b, historical, historical)
			b.ResetTimer()
			for range b.N {
				if err := k.Tick(context.Background()); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(store.loads)/float64(b.N), "loads/op")
		})
	}
}
