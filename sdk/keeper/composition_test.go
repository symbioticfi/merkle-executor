package keeper

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type exclusiveProvider struct {
	recoveryProvider
	claims map[string]common.Address
}

func (p *exclusiveProvider) ExclusiveTargets() map[string]common.Address { return p.claims }

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

func TestRecoveryRejectsAmbiguousCompositionBeforeRelease(t *testing.T) {
	for _, phase := range []string{"prepared", "authorized"} {
		t.Run(phase, func(t *testing.T) {
			k, store, _, sender, _ := recoveryHarness(t)
			target := common.HexToAddress("0x123")
			first := &exclusiveProvider{claims: map[string]common.Address{"0:0": target}}
			second := &exclusiveProvider{claims: map[string]common.Address{"1:0": target}}
			k.Strategies[0].Providers = []provider.Provider{first, second}
			job := Job{ID: "owned", ConfigHash: k.Strategies[0].Config.Fingerprint(), Phase: phase,
				Plan:     Plan{Calldata: compositionCalldata(t, target, target), Outputs: make([]provider.Output, 2)},
				Progress: []provider.Progress{{Done: true}, {}}}
			if err := store.Save(key("cow"), job); err != nil {
				t.Fatal(err)
			}
			sender.pending = job.ID
			for range 2 { // completed claims must remain reserved after restart.
				restarted := *k
				if err := restarted.Tick(context.Background()); err == nil {
					t.Fatal("ambiguous durable composition accepted")
				}
			}
			if sender.pending != job.ID || len(sender.requests) != 0 || second.calls != 0 {
				t.Fatal("ambiguous work submitted, released or advanced")
			}
			got := loadRecoveryJob(t, store, "cow")
			if got.Phase != phase || !got.Progress[0].Done {
				t.Fatal("durable job mutated")
			}
		})
	}
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
			var providers []provider.Provider
			for _, claims := range tc.claims {
				providers = append(providers, &exclusiveProvider{claims: claims})
			}
			gate := int64(1)
			if tc.name == "zero gated static duplicate" {
				gate = 0
			}
			err := ValidateComposition(compositionCalldataWithGate(t, gate, tc.targets...), providers)
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
	if err := ValidateComposition(calldata, []provider.Provider{pr}); err == nil {
		t.Fatal("direct account.manage can supply zero-gated provider evidence")
	}
}

type compositionPlanner struct {
	recoveryPlanner
	plan Plan
}

func (p *compositionPlanner) Prepare(context.Context, Strategy) (Plan, error) { return p.plan, nil }
func TestFreshCompositionFailsBeforeSigning(t *testing.T) {
	k, store, _, sender, _ := recoveryHarness(t)
	target := common.HexToAddress("0x123")
	pr := &exclusiveProvider{claims: map[string]common.Address{"0:0": target}}
	k.Strategies[0].Providers = []provider.Provider{pr}
	k.Planner = &compositionPlanner{plan: Plan{Calldata: compositionCalldata(t, target, target), Outputs: []provider.Output{{}}}}
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("fresh ambiguous composition accepted")
	}
	if len(sender.requests) != 0 || sender.pending != "" || pr.calls != 0 {
		t.Fatal("ambiguous composition signed or advanced")
	}
	var job Job
	if found, err := store.Load(key("cow"), &job); err != nil || found {
		t.Fatal("invalid composition persisted as executable work")
	}
}
