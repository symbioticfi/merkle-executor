package merkl

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// Includes per-token proof verification and calldata assembly with already pinned getter replies.
func BenchmarkRewardInventory(b *testing.B) {
	for _, count := range []int{32, 128, 512} {
		b.Run(fmt.Sprintf("%d_rewards", count), func(b *testing.B) {
			s := provider.Snapshot{Account: addr(3), ChainID: big.NewInt(1)}
			rows, root, proofs := rewardInventory(b, s.Account, count)
			entries := []wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}}
			contract := binding.NewMerklDistributor()
			methods := contract.GetABI().Methods
			rawRoot, _ := methods["getMerkleRoot"].Outputs.Pack(root)
			rawRecipient, _ := methods["claimRecipient"].Outputs.Pack(common.Address{})
			rawClaimed, _ := methods["claimed"].Outputs.Pack(new(big.Int), new(big.Int), root)
			replies := []multicall.Multicall3Result{{Success: true, ReturnData: rawRoot}, {Success: true, ReturnData: rawRecipient}, {Success: true, ReturnData: rawClaimed}, {Success: true, ReturnData: rawRecipient}}
			read := func() ([]multicall.Multicall3Result, error) { return replies, nil }
			runtimes := make([]Provider, count)
			for i, row := range rows {
				token := common.HexToAddress(row.Token.Address)
				payload, err := contract.TryPackClaim([]common.Address{s.Account}, []common.Address{token}, []*big.Int{big.NewInt(100)}, [][][32]byte{proofs[i]})
				if err != nil {
					b.Fatal(err)
				}
				runtimes[i] = Provider{tokens: []common.Address{token}, cfg: Config{Action: "0:0", ProofCapacities: []int{len(proofs[i])}, CalldataBytes: len(payload)}}
			}
			b.ReportAllocs()
			for b.Loop() {
				set, err := indexRewards(entries, 1)
				if err != nil {
					b.Fatal(err)
				}
				for i := range runtimes {
					if _, _, err := runtimes[i].prepareRewards(s, set, read); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
