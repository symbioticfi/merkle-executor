package main

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSignerEnvironmentBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, variable, value, want string
	}{
		{"missing name", "", "", "signer env variable name required"},
		{"empty value", "MERKLE_TEST_KEY", " \n", "signer env variable is empty"},
		{"invalid secret", "MERKLE_TEST_KEY", "private-invalid-secret", "signer contains an invalid private key"},
		{"valid", "MERKLE_TEST_KEY", " \n0x" + strings.Repeat("0", 63) + "1 \n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.variable != "" {
				t.Setenv(tc.variable, tc.value)
			}
			s, err := signerFromEnv(tc.variable)
			if tc.want != "" {
				if err == nil || err.Error() != tc.want {
					t.Fatalf("unexpected credential error: %v", err)
				}
				return
			}
			if err != nil || s.Address() != common.HexToAddress("0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf") {
				t.Fatal("configured signer was not preserved", err)
			}
		})
	}
}
