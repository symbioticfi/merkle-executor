package providers_test

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type unusedReader struct{}

func (unusedReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	panic("constructor must not read chain")
}
func TestEveryProviderInputIndexBoundary(t *testing.T) {
	a, b := "0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002"
	deps := registry.Deps{Chain: unusedReader{}, HTTP: http.DefaultClient}
	for _, tc := range []struct {
		name string
		new  func(*uint) error
	}{
		{"cowswap/sell", func(i *uint) error {
			_, err := cowswap.New(cowswap.Config{HTTP: protocolio.HTTPConfig{URL: "http://localhost"}, Settlement: a, SellToken: a, BuyToken: b, SellAmount: "1000", MinBuyAmount: "1", Action: "0:0", SellAmountInput: i}, deps)
			return err
		}},
		{"cowswap/buy", func(i *uint) error {
			_, err := cowswap.New(cowswap.Config{HTTP: protocolio.HTTPConfig{URL: "http://localhost"}, Settlement: a, SellToken: a, BuyToken: b, SellAmount: "1000", MinBuyAmount: "1", Action: "0:0", BuyAmountInput: i}, deps)
			return err
		}},
		{"merkl/amount", func(i *uint) error {
			_, err := merkl.New(merkl.Config{HTTP: protocolio.HTTPConfig{URL: "http://localhost"}, Multicall3: b, Distributor: a, Tokens: []string{a}, ProofLengths: []int{0}, Action: "0:0", CalldataBytes: 4, AmountInputs: []uint{*i}}, deps)
			return err
		}},
		{"merkl/claimable", func(i *uint) error {
			_, err := merkl.New(merkl.Config{HTTP: protocolio.HTTPConfig{URL: "http://localhost"}, Multicall3: b, Distributor: a, Tokens: []string{a}, ProofLengths: []int{0}, Action: "0:0", CalldataBytes: 4, ClaimableInputs: []uint{*i}}, deps)
			return err
		}},
	} {
		for _, index := range []uint{4095, 4096} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, index), func(t *testing.T) {
				if err := tc.new(&index); (err == nil) != (index == 4095) {
					t.Fatalf("input %d: %v", index, err)
				}
			})
		}
	}
}
