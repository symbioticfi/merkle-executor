package runtime

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"math/big"
	"testing"
)

func compositionCalldata(t *testing.T, targets ...common.Address) []byte {
	return compositionCalldataWithGate(t, 1, targets...)
}
func compositionCalldataWithGate(t *testing.T, gate int64, targets ...common.Address) []byte {
	t.Helper()
	word, _ := abi.NewType("uint256", "", nil)
	hashes, _ := abi.NewType("bytes32[]", "", nil)
	bindings, _ := abi.NewType("uint256[][]", "", nil)
	args := abi.Arguments{{Type: word}, {Type: hashes}, {Type: bindings}}
	steps, runtime := make([][]byte, len(targets)), make([][]byte, len(targets))
	for i, target := range targets {
		// Gates cannot establish which committed call supplied the receipt evidence.
		data, err := args.Pack(big.NewInt(gate), [][32]byte{{1}}, [][]*big.Int{{}})
		if err != nil {
			t.Fatal(err)
		}
		steps[i] = append([]byte{9}, data...)
		runtime[i] = account.NewMerkleAccount().PackManage([]account.IMerkleAccountAction{{Target: target, Value: new(big.Int), Data: []byte{1, 2, 3, 4}}})[4:]
	}
	return executor.NewRecipeExecutor().PackExecute(nil, steps, nil, runtime)
}

func TestOperationCompositionRetainsCompletedClaims(t *testing.T) {
	target, other := common.HexToAddress("0x123"), common.HexToAddress("0x456")
	for _, tc := range []struct {
		name    string
		targets []common.Address
		claims  []map[string]common.Address
		ok      bool
	}{
		{"single", []common.Address{target}, []map[string]common.Address{{"0:0": target}}, true},
		{"two owners", []common.Address{target, target}, []map[string]common.Address{{"0:0": target}, {"1:0": target}}, false},
		{"static duplicate", []common.Address{target, target}, []map[string]common.Address{{"0:0": target}}, false},
		{"interleaved same operation", []common.Address{target, other, target}, []map[string]common.Address{{"0:0": target, "2:0": target}}, true},
		{"wrong target", []common.Address{other}, []map[string]common.Address{{"0:0": target}}, false},
		{"missing action", []common.Address{target}, []map[string]common.Address{{"1:0": target}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := make([]provider.Record, len(tc.claims))
			for i, claims := range tc.claims {
				records[i] = provider.Record{Integration: "issuer", ID: big.NewInt(int64(i)).String(), Requirements: provider.Requirements{ExclusiveTargets: claims}, Progress: provider.Progress{Done: true}}
			}
			err := ValidatePlanComposition(compositionCalldata(t, tc.targets...), records)
			if (err == nil) != tc.ok {
				t.Fatalf("valid=%v: %v", tc.ok, err)
			}
		})
	}
}
