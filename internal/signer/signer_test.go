package signer

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestSignerSignsConfiguredChainAndHonorsCancellation(t *testing.T) {
	t.Setenv("MERKLE_TEST_KEY", strings.Repeat("0", 63)+"1")
	s, err := FromEnv("MERKLE_TEST_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if s.Address() != common.HexToAddress("0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf") {
		t.Fatal("wrong signer address")
	}
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(1), Nonce: 7, Gas: 21000, GasFeeCap: big.NewInt(2), GasTipCap: big.NewInt(1)})
	signed, err := s.SignTx(context.Background(), tx, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	from, err := types.Sender(types.LatestSignerForChainID(big.NewInt(1)), signed)
	if err != nil || from != s.Address() || signed.Nonce() != 7 {
		t.Fatalf("signed transaction: %v %v", from, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.SignTx(ctx, tx, big.NewInt(1)); err == nil {
		t.Fatal("cancelled signing succeeded")
	}
}

func TestSignerRejectsInvalidSecretWithoutEchoingIt(t *testing.T) {
	const secret = "not-a-valid-private-key"
	t.Setenv("MERKLE_TEST_KEY", secret)
	_, err := FromEnv("MERKLE_TEST_KEY")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error: %v", err)
	}
	if _, err := FromEnv(""); err == nil {
		t.Fatal("empty env reference accepted")
	}
}
