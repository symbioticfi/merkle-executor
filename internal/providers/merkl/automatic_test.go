package merkl

import (
	"context"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"strings"
	"testing"
)

func TestAutomaticRestoreValidatesProofAndPreservesLegacyAssignmentsOffline(t *testing.T) {
	deps := registry.Deps{Chain: reader(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
		t.Fatal("restore performed chain I/O")
		return nil, nil
	})}
	integration, err := NewAutomatic(deps)
	if err != nil {
		t.Fatal(err)
	}
	sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
	root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
	payload, err := binding.NewMerklDistributor().TryPackClaim([]common.Address{addr(3)}, []common.Address{addr(1)}, []*big.Int{big.NewInt(100)}, [][][32]byte{{sibling}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{HTTP: rewardHTTP(), Multicall3: protocolio.StandardMulticall3, Distributor: addr(4).Hex(), Action: "1:0", CalldataBytes: len(payload), MinClaimable: "1", Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}}
	evidence, _ := json.Marshal(map[string]string{"root": root.Hex(), "tokens": "1"})
	inner := provider.Output{Inputs: map[uint]string{7: "100"}, Actions: map[string]hexutil.Bytes{"1:0": payload}, Data: evidence}
	legacy, err := derived.Encode(automaticPolicy, cfg, inner, true)
	if err != nil {
		t.Fatal(err)
	}
	records, err := ImportLegacy(deps, provider.Snapshot{ChainID: big.NewInt(1), Account: addr(3)}, legacy, provider.Progress{Done: true})
	if err != nil || len(records) != 1 || !records[0].Progress.Done || records[0].ID != "1:0" {
		t.Fatalf("offline import %+v %v", records, err)
	}
	if _, err = ImportLegacy(deps, provider.Snapshot{ChainID: big.NewInt(1), Account: addr(99)}, legacy, provider.Progress{}); err == nil {
		t.Fatal("legacy account mismatch accepted")
	}
	if _, err = integration.Restore(records[0]); err != nil {
		t.Fatal(err)
	}
	inner.Inputs[7] = "99"
	corrupt, err := provider.EncodeOperation(integration.Policy(), cfg, inner)
	if err != nil {
		t.Fatal(err)
	}
	records[0].Output = corrupt
	if _, err = integration.Restore(records[0]); err == nil {
		t.Fatal("input differing from proved cumulative amount accepted")
	}
}
func TestAutomaticUnmatchedProducesNoOperation(t *testing.T) {
	integration, err := NewAutomatic(registry.Deps{Chain: reader(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
		t.Fatal("compile read chain")
		return nil, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := integration.Compile(provider.Program{Tree: &recipes.Tree{}})
	if err != nil || len(operations) != 0 {
		t.Fatalf("unmatched compile %+v %v", operations, err)
	}
}
