package redemptions

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func packedStep(t *testing.T, op uint8, args abi.Arguments, values ...any) []byte {
	t.Helper()
	raw, err := args.Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{op}, raw...)
}
func packedValue(source uint8, value uint64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(source)), 216), new(big.Int).SetUint64(value))
}
func packedBinding(offset uint32, source uint8, value uint64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(offset)), 224), packedValue(source, value))
}
func automaticConfig(asset issuerAsset, mode string) Config {
	cfg := Config{Family: asset.Family, Mode: mode, ChainID: asset.ChainID, Token: asset.Token, OutputAsset: asset.OutputAsset, Target: asset.Target, CDO: asset.CDO, Strategy: asset.Strategy, Tranche: asset.Tranche, Amount: "100", Action: "1:0"}
	if asset.Family == "figure" {
		cfg.Action, cfg.SecondAction, cfg.IntermediateAmount = "2:0", "3:0", "125"
	}
	if mode == "queue-claim" {
		cfg.Amount = ""
		cfg.Epoch = "7"
	}
	if asset.Family == "noon" && mode == "claim" {
		cfg.Amount = ""
		cfg.RequestID = "7"
	}
	if mode == "direct-claim" || mode == "instant-claim" {
		cfg.Amount = ""
	}
	return cfg
}
func sealAutomaticTree(t *testing.T, recipe *recipes.Recipe, tree *recipes.Tree) {
	t.Helper()
	for i := range tree.Leaves {
		leaf := &tree.Leaves[i]
		old := leaf.Hash
		hash, err := recipes.HashLeaf(*leaf)
		if err != nil {
			t.Fatal(err)
		}
		leaf.Hash = hash
		for j := range recipe.Steps {
			recipe.Steps[j] = bytes.ReplaceAll(recipe.Steps[j], old[:], hash[:])
		}
	}
	if len(tree.Leaves) == 1 {
		tree.Root = tree.Leaves[0].Hash
		tree.Leaves[0].Proof = nil
		return
	}
	if len(tree.Leaves) != 2 {
		t.Fatal("fixture supports one or two permission leaves")
	}
	a, b := tree.Leaves[0].Hash, tree.Leaves[1].Hash
	tree.Leaves[0].Proof = []common.Hash{b}
	tree.Leaves[1].Proof = []common.Hash{a}
	if bytes.Compare(a[:], b[:]) > 0 {
		a, b = b, a
	}
	tree.Root = crypto.Keccak256Hash(a[:], b[:])
}

func automaticRecipe(t *testing.T, cfg Config, s provider.Snapshot, reader provider.Reader) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	runtime, err := New(cfg, registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := runtime.expectedActions(s.Account)
	if err != nil {
		t.Fatal(err)
	}
	query, err := queryArguments.Pack(runtime.token, erc20.NewERC20().PackBalanceOf(s.Account), []*big.Int{}, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{packedStep(t, recipes.Read, readArguments, new(big.Int))}}
	if cfg.Family == "figure" {
		query, err = queryArguments.Pack(runtime.token, erc4626.NewERC4626().PackPreviewRedeem(new(big.Int)), []*big.Int{packedBinding(4, recipes.Result, 0)}, new(big.Int))
		if err != nil {
			t.Fatal(err)
		}
		recipe.Queries = append(recipe.Queries, query)
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Read, readArguments, big.NewInt(1)))
	}
	tree := &recipes.Tree{ChainID: s.ChainID}
	keys := []string{cfg.Action}
	if cfg.SecondAction != "" {
		keys = append(keys, cfg.SecondAction)
	}
	manageArgs := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	for index, key := range keys {
		data := bytes.Clone(payloads[key])
		mask := make([]byte, len(data))
		var bindings []*big.Int
		amount := cfg.Amount != ""
		offset := 4
		if cfg.Family == "midas" || cfg.Family == "openeden" || (cfg.Family == "noon" && cfg.Mode == "redeem-usn") {
			offset = 36
		}
		if amount {
			for i := offset; i < offset+32; i++ {
				mask[i] = 255
			}
			bindings = []*big.Int{packedBinding(uint32(offset), recipes.Result, uint64(index))}
		}
		target := runtime.ExclusiveTargets()[key]
		leaf := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		leaf.Hash, err = recipes.HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		tree.Leaves = append(tree.Leaves, leaf)
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Manage, manageArgs, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{bindings}))
	}
	sealAutomaticTree(t, &recipe, tree)
	return recipe, tree
}
func TestAutomaticAllEmbeddedIssuerRoutes(t *testing.T) {
	initial := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: initial})
	if err != nil {
		t.Fatal(err)
	}
	catalogue := raw.(*automatic)

	for _, asset := range catalogue.assets {
		for _, mode := range append(append([]string{}, asset.RequestModes...), asset.ClaimModes...) {
			t.Run(asset.Symbol+"/"+mode, func(t *testing.T) {
				cfg := automaticConfig(asset, mode)
				s, reader := issuerFixture(t, cfg)
				if mode == "redeem-usn" {
					_, s, reader = nativeNoonFixture(t)
					reader.reply(common.HexToAddress(cfg.Target), native.NewNoonRedeemHandlerV2().GetABI(), "calculateCollateralAmount", []any{common.HexToAddress(cfg.OutputAsset), big.NewInt(100)}, big.NewInt(1_234_567))
				}
				if cfg.Family == "noon" && mode == "claim" {
					a := noonwithdrawalhandler.NewNoonWithdrawalHandler().GetABI()
					reader.reply(common.HexToAddress(cfg.Target), a, "getWithdrawalRequest", []any{s.Account, big.NewInt(7)}, struct {
						Amount    *big.Int
						Timestamp *big.Int
						Claimed   bool
					}{big.NewInt(50), big.NewInt(100), false})
					reader.reply(common.HexToAddress(cfg.Target), a, "withdrawPeriod", nil, big.NewInt(200))
				}
				runtime, err := NewAutomatic(registry.Deps{Chain: reader})
				if err != nil {
					t.Fatal(err)
				}
				p := runtime.(*automatic)
				recipe, tree := automaticRecipe(t, cfg, s, reader)
				req, err := p.RecipeRequirements(s, recipe, tree)
				if err != nil || reader.reads != 0 {
					t.Fatalf("pure route extraction: %v reads=%d", err, reader.reads)
				}
				if len(req.Inputs) != 0 {
					t.Fatal("result binding incorrectly owns an input")
				}
				if err := keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); err != nil {
					t.Fatalf("pure valid composition rejected: %v", err)
				}
				out, err := p.PrepareRecipe(context.Background(), s, recipe, tree, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(125)})
				if err != nil {
					t.Fatal(err)
				}
				claims, err := p.ExclusiveOutputTargets(out)
				if err != nil || len(claims) != len(out.Actions) {
					t.Fatalf("restore ownership: %v", err)
				}
				if !maps.Equal(req.ExclusiveTargets, claims) {
					t.Fatalf("pure and durable exclusive targets differ: %+v %+v", req.ExclusiveTargets, claims)
				}
				var calls []protocolio.ManagedCall
				keys := []string{cfg.Action}
				if cfg.SecondAction != "" {
					keys = append(keys, cfg.SecondAction)
				}
				for _, key := range keys {
					target := claims[key]
					calls = append(calls, protocolio.ManagedCall{Target: target, Data: out.Actions[key]})
				}
				s.Execution.Logs = append(s.Execution.Logs, issuerManageLog(t, s, calls))
				if mode == "redeem-usn" {
					s.Execution.Logs = nativeNoonReceipt(t, cfg, s, out)
					for i := range s.Execution.Logs {
						s.Execution.Logs[i].Index = uint(i)
					}
				}
				// Recovery uses only the durable policy and canonical receipt, never repeats preparation reads.
				before := reader.reads
				restarted, err := NewAutomatic(registry.Deps{Chain: reader})
				if err != nil {
					t.Fatal(err)
				}
				progress, err := restarted.Advance(context.Background(), s, out, nil)
				if err != nil || !progress.Done || reader.reads != before {
					t.Fatalf("durable issuer recovery: %+v %v", progress, err)
				}
			})
		}
	}
}
func TestAutomaticIssuerRejectsInvalidEconomicsAndIdentityBeforeReads(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	asset := p.assets[0]
	cfg := automaticConfig(asset, "request")
	s, _ := issuerFixture(t, cfg)
	for _, fault := range []string{"free amount", "masked pair", "wrong pair", "wrong chain", "duplicate target"} {
		t.Run(fault, func(t *testing.T) {
			recipe, tree := automaticRecipe(t, cfg, s, reader)
			switch fault {
			case "free amount":
				method := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
				recipe.Steps[1] = packedStep(t, recipes.Manage, method, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 3)}})
			case "masked pair":
				tree.Leaves[0].DataMask[4] = 255
			case "wrong pair":
				tree.Leaves[0].Data[35] ^= 1
			case "wrong chain":
				s.ChainID = big.NewInt(10)
				defer func() { s.ChainID = big.NewInt(1) }()
			case "unknown target":
				tree.Leaves[0].Target = issuerAddress(987)
			case "duplicate target":
				recipe.Steps = append(recipe.Steps, bytes.Clone(recipe.Steps[1]))
			}
			sealAutomaticTree(t, &recipe, tree)
			_, err := p.RecipeRequirements(s, recipe, tree)
			if err == nil || reader.reads != 0 {
				t.Fatalf("invalid recipe accepted: %v reads=%d", err, reader.reads)
			}
		})
	}
}
func TestAutomaticIssuerRejectsDurablePolicyAndPayloadChanges(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	cfg := automaticConfig(p.assets[0], "request")
	s, reader := issuerFixture(t, cfg)
	p.deps.Chain = reader
	recipe, tree := automaticRecipe(t, cfg, s, reader)
	out, err := p.PrepareRecipe(context.Background(), s, recipe, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	var bundle durableRoutes
	inner, _, err := derived.Decode(PolicyVersion, out, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"route", "amount", "action", "input", "account"} {
		t.Run(fault, func(t *testing.T) {
			encoded, _ := json.Marshal(bundle)
			var changed durableRoutes
			_ = json.Unmarshal(encoded, &changed)
			switch fault {
			case "route":
				changed.Routes[0].Config.Target = issuerAddress(987).Hex()
			case "amount":
				changed.Routes[0].Config.Amount = "101"
			case "action":
				changed.Routes[0].Output.Actions[cfg.Action][len(changed.Routes[0].Output.Actions[cfg.Action])-1] ^= 1
			case "input":
				changed.Routes[0].Output.Inputs = map[uint]string{77: "100"}
			case "account":
				changed.Routes[0].Output.Data = json.RawMessage(`"0x0000000000000000000000000000000000000011"`)
			}
			corrupt, err := derived.Encode(PolicyVersion, changed, inner, true)
			if err != nil {
				t.Fatal(err)
			}
			before := reader.reads
			if _, err = p.ExclusiveOutputTargets(corrupt); err == nil || reader.reads != before {
				t.Fatalf("corrupt durable route accepted: %v", err)
			}
		})
	}
	empty, err := derived.Empty(PolicyVersion)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := p.Advance(context.Background(), provider.Snapshot{}, empty, nil)
	if err != nil || !progress.Done {
		t.Fatal("unmatched issuer blocked unrelated recipe")
	}
	wrong := out
	wrong.Data = []byte(`{}`)
	if _, err = p.Advance(context.Background(), s, wrong, nil); err == nil || errors.Is(err, provider.ErrNotReady) {
		t.Fatal("malformed recovery treated as idle")
	}
}

func TestAutomaticIssuerBoundedAmountAndExactClaimIdentity(t *testing.T) {
	raw, err := NewAutomatic(registry.Deps{Chain: &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	cfg := automaticConfig(p.assets[0], "request")
	s, reader := issuerFixture(t, cfg)
	p.deps.Chain = reader
	r, tree := automaticRecipe(t, cfg, s, reader)
	guards := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	r.Steps = [][]byte{r.Steps[0], packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Literal, 80)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 7)}})}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil || len(req.Values) != 3 || len(req.Actions) != 1 || len(req.Inputs) != 1 || req.Inputs[0] != 7 {
		t.Fatalf("bounded requirements %+v %v", req, err)
	}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if err != nil || out.Inputs[7] != "80" {
		t.Fatalf("bounded amount %+v %v", out, err)
	}
	var bundle durableRoutes
	_, _, err = derived.Decode(PolicyVersion, out, &bundle)
	if err != nil || bundle.Routes[0].Config.Amount != "80" {
		t.Fatalf("bounded durable amount %+v %v", bundle, err)
	}
	for _, asset := range p.assets {
		if asset.Family == "noon" {
			cfg = automaticConfig(asset, "claim")
			s, _ = issuerFixture(t, cfg)
			r, tree = automaticRecipe(t, cfg, s, reader)
			for i := 4; i < 36; i++ {
				tree.Leaves[0].DataMask[i] = 255
			}
			r.Steps = [][]byte{r.Steps[0], packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Input, 7)}})}
			sealAutomaticTree(t, &r, tree)
			if _, err = p.RecipeRequirements(s, r, tree); err == nil {
				t.Fatal("bounded arbitrary request ID accepted")
			}
			break
		}
	}
}

func TestAutomaticIssuerLeavesUnrelatedGenericRedemptionUnclaimed(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	s, _ := issuerFixture(t, automaticConfig(p.assets[0], "request"))
	data := append(bytes.Clone(crypto.Keccak256([]byte("redeem(uint256)"))[:4]), common.LeftPadBytes(big.NewInt(100).Bytes(), 32)...)
	leaf := recipes.Leaf{Target: issuerAddress(987), Value: new(big.Int), Data: data, DataMask: make([]byte, len(data))}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	r := recipes.Recipe{Steps: [][]byte{packedStep(t, recipes.Manage, args, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})}}
	tree := &recipes.Tree{ChainID: s.ChainID, Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil || len(req.Actions) != 0 || reader.reads != 0 {
		t.Fatalf("generic redemption misclassified: %+v %v", req, err)
	}
}

func TestAutomaticIssuerUsesComputedClaimIndex(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var asset issuerAsset
	for _, candidate := range p.assets {
		if candidate.Family == "pareto" {
			asset = candidate
			break
		}
	}
	for _, anchor := range []string{"result", "input equality"} {
		t.Run(anchor, func(t *testing.T) {
			cfg := automaticConfig(asset, "queue-claim")
			s, chain := issuerFixture(t, cfg)
			p.deps.Chain = chain
			r, tree := automaticRecipe(t, cfg, s, chain)
			for i := 4; i < 36; i++ {
				tree.Leaves[0].DataMask[i] = 255
			}
			binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
			manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
			r.Steps = [][]byte{packedStep(t, recipes.Add, binary, packedValue(recipes.Literal, 3), packedValue(recipes.Literal, 4))}
			source := uint8(recipes.Result)
			index := uint64(0)
			if anchor == "input equality" {
				r.Steps = append(r.Steps, packedStep(t, recipes.Equal, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)))
				source = recipes.Input
				index = 7
			}
			r.Steps = append(r.Steps, packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, source, index)}}))
			sealAutomaticTree(t, &r, tree)
			req, err := p.RecipeRequirements(s, r, tree)
			if err != nil || len(req.Values) != 1 {
				t.Fatalf("computed index requirements %+v %v", req, err)
			}
			out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(7)})
			if err != nil {
				t.Fatal(err)
			}
			var bundle durableRoutes
			_, _, err = derived.Decode(PolicyVersion, out, &bundle)
			if err != nil || bundle.Routes[0].Config.Epoch != "7" {
				t.Fatalf("computed claim identity %+v %v", bundle, err)
			}
			if anchor == "input equality" && out.Inputs[7] != "7" {
				t.Fatal("claim input lost exact anchor")
			}
		})
	}
}

func TestAutomaticFigureLinksBoundedUnwrapInput(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	var asset issuerAsset
	for _, candidate := range p.assets {
		if candidate.Family == "figure" {
			asset = candidate
			break
		}
	}
	cfg := automaticConfig(asset, "request")
	s, chain := issuerFixture(t, cfg)
	p.deps.Chain = chain
	r, tree := automaticRecipe(t, cfg, s, chain)
	r.Queries[1], err = queryArguments.Pack(common.HexToAddress(cfg.Token), erc4626.NewERC4626().PackPreviewRedeem(new(big.Int)), []*big.Int{packedBinding(4, recipes.Input, 7)}, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	binary := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	r.Steps = [][]byte{r.Steps[0], packedStep(t, recipes.LessOrEqual, binary, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), r.Steps[1], packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Input, 7)}}), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[1].Hash}, [][]*big.Int{{packedBinding(4, recipes.Result, 2)}})}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil || len(req.Inputs) != 1 || req.Inputs[0] != 7 || len(req.Actions) != 2 {
		t.Fatalf("bounded Figure requirements %+v %v", req, err)
	}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100), 2: big.NewInt(125)})
	if err != nil || out.Inputs[7] != "100" {
		t.Fatalf("bounded Figure output %+v %v", out, err)
	}
	if _, err = p.ExclusiveOutputTargets(out); err != nil {
		t.Fatal(err)
	}
	if _, err = p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100), 2: big.NewInt(126)}); err == nil {
		t.Fatal("Figure preview witness drift accepted")
	}
}

func TestAutomaticIssuerExclusiveCompositionRejectsBeforeReads(t *testing.T) {
	for _, fault := range []string{"another provider", "unowned manage", "direct call"} {
		t.Run(fault, func(t *testing.T) {
			reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
			raw, err := NewAutomatic(registry.Deps{Chain: reader})
			if err != nil {
				t.Fatal(err)
			}
			p := raw.(*automatic)
			cfg := automaticConfig(p.assets[0], "request")
			s, _ := issuerFixture(t, cfg)
			r, tree := automaticRecipe(t, cfg, s, reader)
			if fault == "unowned manage" {
				leaf := recipes.Leaf{Target: tree.Leaves[0].Target, Value: new(big.Int), Data: []byte{0xde, 0xad, 0xbe, 0xef}, DataMask: make([]byte, 4)}
				leaf.Hash, err = recipes.HashLeaf(leaf)
				if err != nil {
					t.Fatal(err)
				}
				tree.Leaves = append(tree.Leaves, leaf)
				args := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
				r.Steps = append(r.Steps, packedStep(t, recipes.Manage, args, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}}))
				sealAutomaticTree(t, &r, tree)
			}
			if fault == "direct call" {
				args := abi.Arguments{{Type: argumentType("address")}, {Type: argumentType("bytes")}, {Type: argumentType("uint256[]")}}
				r.Steps = append(r.Steps, packedStep(t, recipes.Call, args, s.Account, []byte{0xde, 0xad, 0xbe, 0xef}, []*big.Int{}))
			}
			req, err := p.RecipeRequirements(s, r, tree)
			if err != nil || len(req.ExclusiveTargets) != 1 {
				t.Fatalf("pure route claims %+v %v", req, err)
			}
			requirements := []keeper.RecipeRequirements{req}
			if fault == "another provider" {
				requirements = append(requirements, req)
			}
			if err = keeper.ValidateRecipeComposition(r, tree, requirements); err == nil || reader.reads != 0 {
				t.Fatalf("composition conflict reached reads: %v reads=%d", err, reader.reads)
			}
		})
	}
}

func TestAutomaticNativeNoonSelectionAndRestoration(t *testing.T) {
	cfg, snapshot, reader := nativeNoonFixture(t)
	cfg.AmountInput = nil
	cfg.Action = "1:0"
	amount, _ := new(big.Int).SetString(cfg.Amount, 10)
	payload := native.NewNoonRedeemHandlerV2().PackRedeemOnchain(common.HexToAddress(cfg.OutputAsset), amount)
	runtime, err := New(cfg, registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("persisted action", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("native restoration attempted wrong ABI: %v", r)
			}
		}()
		actions, e := runtime.expectedActions(snapshot.Account)
		if e != nil || !bytes.Equal(actions[cfg.Action], payload) {
			t.Fatalf("native restoration changed exact calldata: %v", e)
		}
	})
	query, err := queryArguments.Pack(common.HexToAddress(cfg.Token), erc20.NewERC20().PackBalanceOf(snapshot.Account), []*big.Int{}, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	leaf := recipes.Leaf{Target: common.HexToAddress(cfg.Target), Value: new(big.Int), Data: payload, DataMask: make([]byte, len(payload))}
	for i := 36; i < 68; i++ {
		leaf.DataMask[i] = 255
	}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	recipe := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
		packedStep(t, recipes.Read, readArguments, new(big.Int)),
		packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{packedBinding(36, recipes.Result, 0)}}),
	}}
	tree := &recipes.Tree{ChainID: snapshot.ChainID, Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	p, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	req, err := p.(*automatic).RecipeRequirements(snapshot, recipe, tree)
	if err != nil || len(req.ExclusiveTargets) != 1 || reader.reads != 0 {
		t.Fatalf("native automatic selection missing: %+v %v reads=%d", req, err, reader.reads)
	}
	out, err := p.(*automatic).PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: amount})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Execution.Logs = nativeNoonReceipt(t, cfg, snapshot, out)
	for i := range snapshot.Execution.Logs {
		snapshot.Execution.Logs[i].Index = uint(i)
	}
	before := reader.reads
	restarted, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := restarted.Advance(t.Context(), snapshot, out, nil)
	if err != nil || !progress.Done || reader.reads != before {
		t.Fatalf("native durable restoration: %+v %v", progress, err)
	}
}

func TestAutomaticNativeNoonBudgetAndPreflight(t *testing.T) {
	for _, fault := range []string{"valid", "free amount", "masked pair", "bound pair", "wrong collateral", "known wrong target", "wrong chain", "misaligned binding", "ETH value", "ambiguous route"} {
		t.Run(fault, func(t *testing.T) {
			cfg, snapshot, reader := nativeNoonFixture(t)
			cfg.Amount = "100"
			cfg.Action = "3:0"
			cfg.AmountInput = nil
			recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
			guards := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("uint256")}}
			manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
			recipe.Steps = [][]byte{recipe.Steps[0], packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Literal, 80)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 7)}})}
			raw, e := NewAutomatic(registry.Deps{Chain: reader})
			if e != nil {
				t.Fatal(e)
			}
			p := raw.(*automatic)
			switch fault {
			case "free amount":
				recipe.Steps = [][]byte{recipe.Steps[0], recipe.Steps[3]}
			case "masked pair":
				tree.Leaves[0].DataMask[4] = 255
			case "bound pair":
				for i := 4; i < 36; i++ {
					tree.Leaves[0].DataMask[i] = 255
				}
				recipe.Steps[3] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Result, 0), packedBinding(36, recipes.Input, 7)}})
			case "wrong collateral":
				tree.Leaves[0].Data[35] ^= 1
			case "known wrong target":
				tree.Leaves[0].Target = common.HexToAddress("0x0DaBc0D9B270c9B0C4C77AaCeAa712b56D0F9178")
			case "wrong chain":
				snapshot.ChainID = big.NewInt(10)
			case "misaligned binding":
				recipe.Steps[3] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(37, recipes.Input, 7)}})
			case "ETH value":
				tree.Leaves[0].Value = big.NewInt(1)
			case "ambiguous route":
				for _, asset := range p.assets {
					if asset.Family == "noon" && asset.RequestModes[0] == "redeem-usn" {
						p.assets = append(p.assets, asset)
						break
					}
				}
			}
			sealAutomaticTree(t, &recipe, tree)
			req, e := p.RecipeRequirements(snapshot, recipe, tree)
			if fault != "valid" {
				if e == nil || reader.reads != 0 {
					t.Fatalf("invalid native preflight accepted or read: %v reads=%d", e, reader.reads)
				}
				return
			}
			if e != nil || len(req.Inputs) != 1 || req.Inputs[0] != 7 || reader.reads != 0 {
				t.Fatalf("bounded native requirements: %+v %v", req, e)
			}
			if e = keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); e != nil {
				t.Fatal(e)
			}
			reader.reply(common.HexToAddress(cfg.Target), native.NewNoonRedeemHandlerV2().GetABI(), "calculateCollateralAmount", []any{common.HexToAddress(cfg.OutputAsset), big.NewInt(80)}, big.NewInt(1_234_567))
			out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: big.NewInt(100)})
			if e != nil || out.Inputs[7] != "80" {
				t.Fatalf("native maximum bounded amount: %+v %v", out.Inputs, e)
			}
			var bundle durableRoutes
			_, _, e = derived.Decode(PolicyVersion, out, &bundle)
			if e != nil || bundle.Routes[0].Config.Amount != "80" || bundle.Routes[0].Config.Mode != "redeem-usn" {
				t.Fatalf("native durable budget differs: %+v %v", bundle, e)
			}
			// Every read-derived upper bound participates; the smaller read also beats the literal cap.
			tighter := recipe
			tighter.Steps = [][]byte{recipe.Steps[0], recipe.Steps[0], packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Input, 7), packedValue(recipes.Result, 1)), recipe.Steps[1], recipe.Steps[2], recipe.Steps[3]}
			reader.reply(common.HexToAddress(cfg.Target), native.NewNoonRedeemHandlerV2().GetABI(), "calculateCollateralAmount", []any{common.HexToAddress(cfg.OutputAsset), big.NewInt(70)}, big.NewInt(1_234_567))
			tighterOut, e := p.PrepareRecipe(t.Context(), snapshot, tighter, tree, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(70)})
			if e != nil || tighterOut.Inputs[7] != "70" {
				t.Fatalf("second native read cap ignored: %+v %v", tighterOut.Inputs, e)
			}
			lower := recipe
			lower.Steps = [][]byte{recipe.Steps[0], recipe.Steps[1], recipe.Steps[2], packedStep(t, recipes.LessOrEqual, guards, packedValue(recipes.Literal, 50), packedValue(recipes.Input, 7)), recipe.Steps[3]}
			before := reader.reads
			if _, e = p.PrepareRecipe(t.Context(), snapshot, lower, tree, map[uint]*big.Int{0: big.NewInt(40)}); e == nil || reader.reads != before {
				t.Fatal("native minimum bound ignored/read", e)
			}
			if _, e = p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: big.NewInt(0)}); e == nil || reader.reads != before {
				t.Fatal("zero native derived amount accepted/read", e)
			}
		})
	}
}

func TestAutomaticNativeNoonRecoveryPolicyAndEvidence(t *testing.T) {
	for _, fault := range []string{"valid", "target", "pair", "chain", "amount", "action", "input", "account", "policy", "missing burn", "missing payment", "duplicate redeemed"} {
		t.Run(fault, func(t *testing.T) {
			cfg, snapshot, reader := nativeNoonFixture(t)
			cfg.Action = "1:0"
			cfg.AmountInput = nil
			amount, _ := new(big.Int).SetString(cfg.Amount, 10)
			recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
			raw, e := NewAutomatic(registry.Deps{Chain: reader})
			if e != nil {
				t.Fatal(e)
			}
			p := raw.(*automatic)
			out, e := p.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: amount})
			if e != nil {
				t.Fatal(e)
			}
			var bundle durableRoutes
			inner, _, e := derived.Decode(PolicyVersion, out, &bundle)
			if e != nil {
				t.Fatal(e)
			}
			snapshot.Execution.Logs = nativeNoonReceipt(t, cfg, snapshot, out)
			policy := PolicyVersion
			switch fault {
			case "target":
				bundle.Routes[0].Config.Target = issuerAddress(123).Hex()
			case "pair":
				bundle.Routes[0].Config.OutputAsset = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
			case "chain":
				bundle.Routes[0].Config.ChainID = 10
			case "amount":
				bundle.Routes[0].Config.Amount = "101"
			case "action":
				bundle.Routes[0].Output.Actions[cfg.Action][67] ^= 1
			case "input":
				bundle.Routes[0].Output.Inputs = map[uint]string{7: "101"}
			case "account":
				bundle.Account = issuerAddress(123)
			case "policy":
				policy = "issuer-recipes-v1-d29862ea24a7968ca7161780d272f70cfe314418c603839fb884985aaebd95f2"
			case "missing burn":
				snapshot.Execution.Logs = append(snapshot.Execution.Logs[:2], snapshot.Execution.Logs[3:]...)
			case "missing payment":
				snapshot.Execution.Logs = snapshot.Execution.Logs[:3]
			case "duplicate redeemed":
				snapshot.Execution.Logs = append(snapshot.Execution.Logs, snapshot.Execution.Logs[1])
			}
			for i := range snapshot.Execution.Logs {
				snapshot.Execution.Logs[i].Index = uint(i)
			}
			out, e = derived.Encode(policy, bundle, inner, true)
			if e != nil {
				t.Fatal(e)
			}
			before := reader.reads
			restarted, e := NewAutomatic(registry.Deps{Chain: reader})
			if e != nil {
				t.Fatal(e)
			}
			progress, e := restarted.Advance(t.Context(), snapshot, out, nil)
			if fault == "valid" {
				if e != nil || !progress.Done || progress.Outcome != "redeemed" {
					t.Fatalf("valid native durable recovery: %+v %v", progress, e)
				}
			} else if e == nil || progress.Done || errors.Is(e, provider.ErrNotReady) {
				t.Fatalf("corrupt native recovery accepted/idle: %+v %v", progress, e)
			}
			if reader.reads != before {
				t.Fatal("native recovery repeated preparation reads")
			}
		})
	}
}

func TestAutomaticNativeSelectorDoesNotOwnUnknownTarget(t *testing.T) {
	cfg, snapshot, reader := nativeNoonFixture(t)
	cfg.Action = "1:0"
	cfg.AmountInput = nil
	recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
	tree.Leaves[0].Target = issuerAddress(987)
	sealAutomaticTree(t, &recipe, tree)
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	req, err := raw.(*automatic).RecipeRequirements(snapshot, recipe, tree)
	if err != nil || len(req.ExclusiveTargets) != 0 || len(req.Actions) != 0 || reader.reads != 0 {
		t.Fatalf("native selector stole unrelated action: %+v %v", req, err)
	}
}
