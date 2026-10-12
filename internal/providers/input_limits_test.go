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
	"github.com/symbioticfi/merkle-executor/internal/providers/data"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlane"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	"github.com/symbioticfi/merkle-executor/internal/providers/securitize"
	"gopkg.in/yaml.v3"
)

type unusedReader struct{}

func (unusedReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	panic("factory must not read chain")
}
func TestEveryProviderInputIndexBoundary(t *testing.T) {
	r := registry.NewRegistry()
	for _, register := range []func(*registry.Registry) error{data.Register, cowswap.Register, liquidlane.Register, merkl.Register, redemptions.Register, securitize.Register} {
		if err := register(r); err != nil {
			t.Fatal(err)
		}
	}
	a, b := "'0x0000000000000000000000000000000000000001'", "'0x0000000000000000000000000000000000000002'"
	hash := "'0x0000000000000000000000000000000000000000000000000000000000000000'"
	cow := "http: {url: 'http://localhost'}\nsettlement: " + a + "\nsellToken: " + a + "\nbuyToken: " + b + "\nsellAmount: '1000'\nminBuyAmount: '1'\nvaliditySeconds: 600\nappData: " + hash + "\naction: '0:0'\n"
	ll := "http: {url: 'http://localhost'}\nformat: connector-v1\nconnector: " + a + "\ntoken: " + a + "\noutputAsset: " + b + "\nrecipient: " + a + "\nprotocol: " + a + "\ndiscountId: " + hash + "\namountIn: '1'\nmaxAmountOut: '1'\naction: '0:0'\ncalldataBytes: 4\n"
	rewards := "http: {url: 'http://localhost'}\ndistributor: " + a + "\ntokens: [" + a + "]\nproofLengths: [0]\naction: '0:0'\ncalldataBytes: 4\n"
	redemption := "family: figure\nmode: request\nchainId: 1\ntoken: " + a + "\noutputAsset: " + b + "\ntarget: " + b + "\namount: '1'\naction: '0:0'\nsecondAction: '1:0'\n"
	sec := "mode: modern\nchainId: 1\ntoken: " + a + "\ntarget: " + b + "\npaymentToken: " + b + "\namount: '1'\nminOutput: '1'\naction: '0:0'\n"
	for _, tc := range []struct{ name, base, field string }{
		{"static", "", "inputs: {%d: '1'}"},
		{"http", "url: http://localhost\n", "inputs: {%d: /value}"},
		{"rpc", "", "reads: [{target: " + a + ", data: '0x12345678', input: %d}]"},
		{"rpc", "reads: [{target: " + a + ", data: '0x12345678', input: 0}]\n", "actions: {'0:0': {data: '0x12345678', words: [{offset: 4, input: %d}]}}"},
		{"cowswap", cow, "sellAmountInput: %d"}, {"cowswap", cow, "buyAmountInput: %d"},
		{"liquidlane", ll, "amountInInput: %d"}, {"liquidlane", ll, "amountOutInput: %d"},
		{"merkl", rewards, "amountInputs: [%d]"}, {"merkl", rewards, "claimableInputs: [%d]"},
		{"redemptions", redemption, "amountInput: %d"}, {"redemptions", redemption, "intermediateInput: %d"},
		{"securitize", sec, "amountInput: %d"}, {"securitize", sec, "minOutputInput: %d"},
	} {
		for _, index := range []int{4095, 4096} {
			t.Run(fmt.Sprintf("%s/%s/%d", tc.name, tc.field, index), func(t *testing.T) {
				var raw yaml.Node
				if err := yaml.Unmarshal([]byte(tc.base+fmt.Sprintf(tc.field, index)), &raw); err != nil {
					t.Fatal(err)
				}
				_, err := r.New(tc.name, raw, registry.Deps{Chain: unusedReader{}, HTTP: http.DefaultClient})
				if (err == nil) != (index == 4095) {
					t.Fatalf("input %d: %v", index, err)
				}
			})
		}
	}
}
