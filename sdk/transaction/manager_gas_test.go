package transaction

import "testing"

func TestSubmitBoundsGasBeforeSigning(t *testing.T) {
	for _, tc := range []struct {
		name      string
		estimate  uint64
		blockGas  uint64
		maxGas    uint64
		wantGas   uint64
		wantError bool
	}{
		{"headroom crosses transaction cap", 15_521_169, 30_000_000, 0, 16_777_216, false},
		{"estimate equals transaction cap", 16_777_216, 30_000_000, 0, 16_777_216, false},
		{"estimate exceeds transaction cap", 16_777_217, 30_000_000, 0, 0, true},
		{"lower block limit", 15_521_169, 16_000_000, 0, 16_000_000, false},
		{"estimate exceeds block limit", 16_000_001, 16_000_000, 0, 0, true},
		{"explicit higher budget", 18_000_000, 30_000_000, 25_000_000, 19_800_000, false},
		{"explicit lower budget", 21_000, 30_000_000, 22_500, 22_500, false},
		{"estimate exceeds explicit budget", 25_001, 30_000_000, 25_000, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBackend()
			b.gas, b.headers[0].GasLimit = tc.estimate, tc.blockGas
			m := newManagerWithConfig(t, b, Config{MaxFeeGwei: 30, MaxGas: tc.maxGas})
			s := &fundingSigner{Signer: m.signer}
			m.signer = s
			err := m.Submit(t.Context(), testRequest())
			if tc.wantError {
				if err == nil || s.calls != 0 || len(b.sent) != 0 || m.Busy() {
					t.Fatalf("over-budget estimate signed: err=%v signatures=%d sends=%d busy=%v", err, s.calls, len(b.sent), m.Busy())
				}
				return
			}
			if err != nil || s.calls != 1 || len(b.sent) != 1 || b.sent[0].Gas() != tc.wantGas || !m.Busy() {
				t.Fatalf("bounded submission: err=%v signatures=%d sends=%d wantGas=%d", err, s.calls, len(b.sent), tc.wantGas)
			}
		})
	}
}
