package parse

import (
	"math/big"
	"testing"
)

func TestUint256(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"0", "42", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()} {
		if _, err := Uint256(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"", "-1", "+1", "0x1", "1.0", "1e18", " 1", new(big.Int).Lsh(big.NewInt(1), 256).String()} {
		if _, err := Uint256(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
