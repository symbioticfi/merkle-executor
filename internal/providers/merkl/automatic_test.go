package merkl

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAutomaticRestoreValidatesProofAndPreservesAssignmentsOffline(t *testing.T) {
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
	output, err := provider.EncodeOperation(integration.Policy(), cfg, inner)
	if err != nil {
		t.Fatal(err)
	}
	requirements := provider.OutputRequirements(inner, nil)
	record := provider.Record{Integration: integration.ID(), Policy: integration.Policy(), ID: provider.OperationID(requirements.Actions), Requirements: requirements, Output: output, Progress: provider.Progress{Done: true}}
	restored, err := integration.Restore(record)
	if err != nil || restored.ID() != "1:0" || !restored.(*automaticOperation).saved.Progress.Done || !provider.EqualOutput(restored.(*automaticOperation).saved.Output, output) {
		t.Fatalf("offline restore changed assignments: %v", err)
	}
	wrongAccount := inner.Clone()
	wrongAccount.Actions[cfg.Action], err = binding.NewMerklDistributor().TryPackClaim([]common.Address{addr(99)}, []common.Address{addr(1)}, []*big.Int{big.NewInt(100)}, [][][32]byte{{sibling}})
	if err != nil {
		t.Fatal(err)
	}
	record.Output, err = provider.EncodeOperation(integration.Policy(), cfg, wrongAccount)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = integration.Restore(record); err == nil {
		t.Fatal("claim account differing from verified proof accepted")
	}
	inner.Inputs[7] = "99"
	corrupt, err := provider.EncodeOperation(integration.Policy(), cfg, inner)
	if err != nil {
		t.Fatal(err)
	}
	record.Output = corrupt
	if _, err = integration.Restore(record); err == nil {
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
