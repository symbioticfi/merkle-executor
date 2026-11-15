package merkl

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestRewardReadPlanDeduplicatesAndBoundsAggregates(t *testing.T) {
	for _, count := range []int{2, 130} {
		t.Run(big.NewInt(int64(count)).String(), func(t *testing.T) {
			snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3)}
			claims := make([]recipeClaim, count)
			runtimes := make([]*Provider, count)
			for i := range claims {
				token := addr(int64(100 + i))
				claims[i].cfg = Config{Distributor: addr(4).Hex(), Tokens: []string{token.Hex()}}
				runtimes[i] = &Provider{distributor: addr(4), multicall: addr(5), tokens: []common.Address{token}}
			}
			seen := map[rewardReadKey]bool{}
			aggregates := 0
			chain := reader(func(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
				if msg.To == nil || *msg.To != addr(5) || block.Cmp(snapshot.BlockNumber) != 0 || msg.From != (common.Address{}) {
					t.Fatal("shared reads lost target, block or caller")
				}
				aggregates++
				method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				args, err := method.Inputs.Unpack(msg.Data[4:])
				if err != nil {
					return nil, err
				}
				calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
				if len(calls) > protocolio.MaxReadBatch {
					t.Fatal("unbounded aggregate")
				}
				results := make([]multicall.Multicall3Result, len(calls))
				for i, call := range calls {
					key := rewardReadKey{call.Target, string(call.CallData)}
					if seen[key] || !call.AllowFailure {
						t.Fatal("getter repeated or speculative failure became fatal")
					}
					seen[key] = true
					results[i].Success = true
				}
				return method.Outputs.Pack(results)
			})
			reads := newRewardReads(t.Context(), provider.Deps{Chain: chain}, snapshot, claims, nil)
			for _, runtime := range runtimes {
				if _, err := reads.forProvider(runtime); err != nil {
					t.Fatal(err)
				}
			}
			want := 2 + 2*count
			if len(seen) != want || aggregates != (want+protocolio.MaxReadBatch-1)/protocolio.MaxReadBatch {
				t.Fatalf("getters=%d aggregates=%d", len(seen), aggregates)
			}
		})
	}
}

func TestLaterGetterFailureDoesNotMaskEarlierClaim(t *testing.T) {
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3)}
	claims := []recipeClaim{{cfg: Config{Distributor: addr(4).Hex(), Tokens: []string{addr(100).Hex()}}}, {cfg: Config{Distributor: addr(4).Hex(), Tokens: []string{addr(101).Hex()}}}}
	chain := reader(func(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
		method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
		results := make([]multicall.Multicall3Result, 6)
		for i := range results {
			results[i].Success = i != 4
		}
		return method.Outputs.Pack(results)
	})
	reads := newRewardReads(t.Context(), provider.Deps{Chain: chain}, snapshot, claims, nil)
	first := &Provider{distributor: addr(4), multicall: addr(5), tokens: []common.Address{addr(100)}}
	second := &Provider{distributor: addr(4), multicall: addr(5), tokens: []common.Address{addr(101)}}
	if _, err := reads.forProvider(first); err != nil {
		t.Fatal("later failure masked earlier claim", err)
	}
	if _, err := reads.forProvider(second); err == nil {
		t.Fatal("consumed failed getter accepted")
	}
}

func TestLaterAggregateFailureDoesNotMaskEarlierClaim(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		for _, failure := range []string{"member error", "malformed response", "wrong result count"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3)}
				claims := make([]recipeClaim, 130)
				for i := range claims {
					claims[i].cfg = Config{Distributor: addr(4).Hex(), Tokens: []string{addr(int64(100 + i)).Hex()}}
				}
				requests := 0
				read := reader(func(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
					requests++
					method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
					count := 256
					if requests == 2 {
						switch failure {
						case "member error":
							return nil, errors.New("unavailable later aggregate")
						case "malformed response":
							return []byte{1}, nil
						case "wrong result count":
							count = 5 // The final chunk contains six getters.
						}
					}
					results := make([]multicall.Multicall3Result, count)
					for i := range results {
						results[i].Success = true
					}
					return method.Outputs.Pack(results)
				})
				var chain provider.Reader = read
				if mode == "batch" {
					chain = batchRewardReader{read}
				}
				reads := newRewardReads(t.Context(), provider.Deps{Chain: chain}, snapshot, claims, nil)
				first := &Provider{distributor: addr(4), multicall: addr(5), tokens: []common.Address{addr(100)}}
				last := &Provider{distributor: addr(4), multicall: addr(5), tokens: []common.Address{addr(229)}}
				if _, err := reads.forProvider(first); err != nil {
					t.Fatal("later aggregate masked earlier claim", err)
				}
				if _, err := reads.forProvider(last); err == nil {
					t.Fatal("consumed invalid aggregate accepted")
				}
				if _, err := reads.forProvider(first); err != nil || requests != 2 {
					t.Fatal("shared successful getters were lost or repeated", err, requests)
				}
			})
		}
	}
}
