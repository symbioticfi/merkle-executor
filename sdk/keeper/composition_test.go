package keeper

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type exclusiveProvider struct {
	testProvider
	claims map[string]common.Address
}

func (p *exclusiveProvider) ExclusiveOutputTargets(provider.Output) (map[string]common.Address, error) {
	return p.claims, nil
}

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

func TestCompositionIncludesStaticAndOtherProviders(t *testing.T) {
	target, other := common.HexToAddress("0x123"), common.HexToAddress("0x456")
	for _, tc := range []struct {
		name    string
		targets []common.Address
		claims  []map[string]common.Address
		ok      bool
	}{
		{"single", []common.Address{target}, []map[string]common.Address{{"0:0": target}}, true},
		{"two actual identical calls unsupported", []common.Address{target, target}, []map[string]common.Address{{"0:0": target}, {"1:0": target}}, false},
		{"zero gated static duplicate", []common.Address{target, target}, []map[string]common.Address{{"0:0": target}}, false},
		{"mixed provider duplicate", []common.Address{target, target}, []map[string]common.Address{{"0:0": target}, nil}, false},
		{"unrelated interleaving", []common.Address{target, other, target}, []map[string]common.Address{{"0:0": target, "2:0": target}}, true},
		{"ordered two target owner", []common.Address{target, other}, []map[string]common.Address{{"0:0": target, "1:0": other}}, true},
		{"wrong target", []common.Address{other}, []map[string]common.Address{{"0:0": target}}, false},
		{"missing action", []common.Address{target}, []map[string]common.Address{{"1:0": target}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var providers []Provider
			for _, claims := range tc.claims {
				providers = append(providers, &exclusiveProvider{claims: claims})
			}
			gate := int64(1)
			if tc.name == "zero gated static duplicate" {
				gate = 0
			}
			err := ValidatePlanComposition(compositionCalldataWithGate(t, gate, tc.targets...), providers, emptyOutputs(len(providers)))
			if (err == nil) != tc.ok {
				t.Fatalf("composition validation: %v", err)
			}
		})
	}
}

func TestCompositionRejectsDirectAccountManageCall(t *testing.T) {
	target := common.HexToAddress("0x123")
	method := executor.NewRecipeExecutor().GetABI().Methods["execute"]
	args, err := method.Inputs.Unpack(compositionCalldataWithGate(t, 0, target)[4:])
	if err != nil {
		t.Fatal(err)
	}
	addressType, _ := abi.NewType("address", "", nil)
	dataType, _ := abi.NewType("bytes", "", nil)
	bindingsType, _ := abi.NewType("uint256[]", "", nil)
	callArgs := abi.Arguments{{Type: addressType}, {Type: dataType}, {Type: bindingsType}}
	nested := account.NewMerkleAccount().PackManage([]account.IMerkleAccountAction{{Target: target, Value: new(big.Int), Data: []byte{1, 2, 3, 4}}})
	call, err := callArgs.Pack(common.HexToAddress("0x20"), nested, []*big.Int{})
	if err != nil {
		t.Fatal(err)
	}
	steps := append(args[1].([][]byte), append([]byte{11}, call...))
	calldata := executor.NewRecipeExecutor().PackExecute(args[0].([][]byte), steps, args[2].([]*big.Int), args[3].([][]byte))
	pr := &exclusiveProvider{claims: map[string]common.Address{"0:0": target}}
	if err := ValidatePlanComposition(calldata, []Provider{pr}, emptyOutputs(1)); err == nil {
		t.Fatal("direct account.manage can supply zero-gated provider evidence")
	}
}

func TestCompositionChecksEveryProviderExecution(t *testing.T) {
	target, other := common.HexToAddress("0x123"), common.HexToAddress("0x456")
	runtime := &exclusiveRecipeProvider{target: target}
	for _, tc := range []struct {
		name   string
		second common.Address
		ok     bool
	}{
		{"all owned", target, true}, {"later target changed", other, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch, err := recipes.Multicall([][]byte{compositionCalldata(t, target), compositionCalldata(t, tc.second)})
			if err != nil {
				t.Fatal(err)
			}
			err = ValidatePlanComposition(batch, []Provider{runtime}, [][]provider.Output{{{}, {}}})
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
		})
	}
}

func emptyOutputs(count int) [][]provider.Output {
	outputs := make([][]provider.Output, count)
	for i := range outputs {
		outputs[i] = []provider.Output{{}}
	}
	return outputs
}
