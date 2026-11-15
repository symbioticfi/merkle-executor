package recipes

import (
	"context"
	"testing"

	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

func TestPreparedAdmissionBoundsSimulationGas(t *testing.T) {
	for _, mode := range []string{"direct", "external"} {
		for _, tc := range []struct {
			name      string
			maxGas    []uint64
			blockGas  uint64
			wantGas   uint64
			wantError bool
		}{
			{"omitted default", nil, 30_000_000, 16_777_216, false},
			{"zero default", []uint64{0}, 30_000_000, 16_777_216, false},
			{"lower explicit budget", []uint64{2_000_000}, 30_000_000, 2_000_000, false},
			{"higher explicit budget", []uint64{25_000_000}, 30_000_000, 25_000_000, false},
			{"fresh block limit", []uint64{25_000_000}, 10_000_000, 10_000_000, false},
			{"multiple budgets", []uint64{1_000_000, 2_000_000}, 30_000_000, 0, true},
			{"zero block limit", nil, 0, 0, true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				r, prepared, calls, leaves, _ := externalFixture(t, 1)
				header := r.members[0].headers[11]
				header.GasLimit = tc.blockGas
				hash := header.Hash()
				var gas uint64
				var err error
				if mode == "external" {
					err = ValidatePreparedBatchWithSimulation(t.Context(), r, prepared, calls, leaves, r.members[0].caller, func(_ context.Context, request PreparedBatchSimulationRequest) error {
						gas = request.GasLimit
						return nil
					}, tc.maxGas...)
				} else {
					err = ValidatePreparedBatch(t.Context(), r, prepared, calls, leaves, []map[uint]string{nil}, r.members[0].caller, tc.maxGas...)
					for _, group := range r.groups {
						for _, request := range group {
							if request.Kind == protocolio.ReadCall && request.Call.Gas != 0 {
								gas = request.Call.Gas
							}
						}
					}
				}
				if (err != nil) != tc.wantError || gas != tc.wantGas || header.Hash() != hash {
					t.Fatalf("admission err=%v gas=%d wantGas=%d, canonical header changed=%v", err, gas, tc.wantGas, header.Hash() != hash)
				}
			})
		}
	}
}
