package securitize

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
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
func packedBinding(at uint32, source uint8, value uint64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(at)), 224), packedValue(source, value))
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
	data, err := runtime.payload()
	if err != nil {
		t.Fatal(err)
	}
	amountOffset := 4
	if cfg.Mode == "wallet" {
		amountOffset = 36
	}
	mask := make([]byte, len(data))
	for i := amountOffset; i < amountOffset+32; i++ {
		mask[i] = 255
	}
	bindings := []*big.Int{packedBinding(uint32(amountOffset), recipes.Result, 0)}
	query, err := queryArguments.Pack(runtime.token, erc20.NewERC20().PackBalanceOf(s.Account), []*big.Int{}, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{packedStep(t, recipes.Read, unaryArguments, new(big.Int))}}
	if cfg.Mode != "wallet" {
		query, err = queryArguments.Pack(runtime.payment, erc20.NewERC20().PackBalanceOf(s.Account), []*big.Int{}, new(big.Int))
		if err != nil {
			t.Fatal(err)
		}
		recipe.Queries = append(recipe.Queries, query)
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Read, unaryArguments, big.NewInt(1)))
	}
	if cfg.Mode == "modern" {
		for i := 36; i < 68; i++ {
			mask[i] = 255
		}
		bindings = append(bindings, packedBinding(36, recipes.Result, 1))
	}
	if cfg.Mode == "legacy" {
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Read, unaryArguments, big.NewInt(1)))
	}
	target := runtime.target
	if cfg.Mode == "wallet" {
		target = runtime.token
	}
	leaf := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	manageArgs := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Manage, manageArgs, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{bindings}))
	if cfg.Mode == "legacy" {
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Read, unaryArguments, big.NewInt(1)))
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Sub, binaryArguments, packedValue(recipes.Result, 4), packedValue(recipes.Result, 1)))
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.LessOrEqual, binaryArguments, packedValue(recipes.Result, 2), packedValue(recipes.Result, 5)))
	}
	tree := &recipes.Tree{ChainID: s.ChainID, Leaves: []recipes.Leaf{leaf}}
	sealAutomaticTree(t, &recipe, tree)
	return recipe, tree
}
func TestAutomaticSecuritizeAllEmbeddedRoutes(t *testing.T) {
	_, _, initial := fixture(t, "wallet")
	raw, err := NewAutomatic(registry.Deps{Chain: initial})
	if err != nil {
		t.Fatal(err)
	}
	catalogue := raw.(*automatic)

	for _, asset := range catalogue.assets {
		routes := append([]Route{}, asset.Offramps...)
		if asset.RedemptionWallet != "" {
			routes = append(routes, Route{Mode: "wallet", Target: asset.RedemptionWallet})
		}
		for _, route := range routes {
			t.Run(asset.Symbol+"/"+new(big.Int).SetUint64(asset.ChainID).String()+"/"+route.Mode+"/"+route.Target, func(t *testing.T) {
				cfg, s, reader := fixture(t, route.Mode)
				cfg.Token, cfg.Target, cfg.PaymentToken, cfg.ChainID = asset.Token, route.Target, route.PaymentToken, asset.ChainID
				cfg.AmountInput, cfg.MinOutputInput = nil, nil
				cfg.MinOutput = "50"
				if cfg.Mode == "wallet" {
					cfg.MinOutput = ""
				}
				cfg.Action = "1:0"
				if cfg.Mode == "modern" {
					cfg.Action = "2:0"
				}
				if cfg.Mode == "legacy" {
					cfg.Action = "3:0"
				}
				s.ChainID = new(big.Int).SetUint64(asset.ChainID)
				reader.configure(cfg)
				recipe, tree := automaticRecipe(t, cfg, s, reader)
				runtime, err := NewAutomatic(registry.Deps{Chain: reader})
				if err != nil {
					t.Fatal(err)
				}
				p := runtime.(*automatic)
				req, err := p.RecipeRequirements(s, recipe, tree)
				if err != nil || len(req.Inputs) != 0 {
					t.Fatalf("extract issuer route: %+v %v", req, err)
				}
				if err := keeper.ValidateRecipeComposition(recipe, tree, []keeper.RecipeRequirements{req}); err != nil {
					t.Fatalf("pure valid composition rejected: %v", err)
				}
				out, err := p.PrepareRecipe(context.Background(), s, recipe, tree, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(50), 2: big.NewInt(50)})
				if err != nil {
					t.Fatal(err)
				}
				claims, err := p.ExclusiveOutputTargets(out)
				if err != nil || len(claims) != 1 {
					t.Fatalf("durable exclusive ownership: %v", err)
				}
				if !maps.Equal(req.ExclusiveTargets, claims) {
					t.Fatalf("pure and durable exclusive targets differ: %+v %+v", req.ExclusiveTargets, claims)
				}
				runtimes, outputs, err := p.restore(out)
				if err != nil || len(runtimes) != 1 {
					t.Fatal(err)
				}
				s = execution(t, runtimes[0], s, outputs[0])
				restarted, err := NewAutomatic(registry.Deps{Chain: reader})
				if err != nil {
					t.Fatal(err)
				}
				progress, err := restarted.Advance(context.Background(), s, out, nil)
				if err != nil || !progress.Done {
					t.Fatalf("durable issuer receipt: %+v %v", progress, err)
				}
			})
		}
	}
}
func TestAutomaticSecuritizeRejectsFreeFieldsAndMissingLegacyFloor(t *testing.T) {
	for _, mode := range []string{"wallet", "modern", "legacy"} {
		for _, fault := range []string{"free amount", "masked identity", "duplicate target", "free minimum"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				cfg, s, reader := fixture(t, mode)
				raw, err := NewAutomatic(registry.Deps{Chain: reader})
				if err != nil {
					t.Fatal(err)
				}
				p := raw.(*automatic)
				asset := p.assets[0]
				cfg.Token = asset.Token
				cfg.AmountInput, cfg.MinOutputInput = nil, nil
				cfg.Action = "1:0"
				if mode == "wallet" {
					cfg.Target = asset.RedemptionWallet
				} else {
					var route Route
					for _, candidate := range asset.Offramps {
						if candidate.Mode == mode {
							route = candidate
							break
						}
					}
					cfg.Target, cfg.PaymentToken = route.Target, route.PaymentToken
					cfg.Action = "2:0"
					if mode == "legacy" {
						cfg.Action = "3:0"
					}
				}
				reader.configure(cfg)
				r, tree := automaticRecipe(t, cfg, s, reader)
				actionStep := len(r.Steps) - 1
				if mode == "legacy" {
					actionStep = 3
				}
				amountOffset := uint32(4)
				if mode == "wallet" {
					amountOffset = 36
				}
				manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
				switch fault {
				case "free amount":
					r.Steps[actionStep] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(amountOffset, recipes.Input, 3)}})
				case "masked identity":
					tree.Leaves[0].DataMask[0] = 255
				case "unknown target":
					tree.Leaves[0].Target = address(987)
					if mode == "wallet" {
						return
					} // An unrelated ERC20 transfer is outside issuer discovery.
				case "duplicate target":
					r.Steps = append(r.Steps, bytes.Clone(r.Steps[actionStep]))
				case "free minimum":
					if mode == "wallet" {
						return
					}
					if mode == "legacy" {
						r.Steps = r.Steps[:4]
					} else {
						r.Steps[actionStep] = packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(4, recipes.Result, 0), packedBinding(36, recipes.Input, 3)}})
					}
				}
				sealAutomaticTree(t, &r, tree)
				if _, err := p.RecipeRequirements(s, r, tree); err == nil {
					t.Fatal("ambiguous issuer recipe accepted")
				}
			})
		}
	}
}
func TestAutomaticSecuritizeRejectsPersistedRouteAndFieldMutation(t *testing.T) {
	cfg, s, reader := fixture(t, "wallet")
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	asset := p.assets[0]
	cfg.Token, cfg.Target = asset.Token, asset.RedemptionWallet
	cfg.AmountInput = nil
	reader.configure(cfg)
	r, tree := automaticRecipe(t, cfg, s, reader)
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	var bundle durableRoutes
	inner, _, err := derived.Decode(PolicyVersion, out, &bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"route", "amount", "decimals", "action", "input", "account", "expected"} {
		t.Run(fault, func(t *testing.T) {
			data, _ := json.Marshal(bundle)
			var changed durableRoutes
			_ = json.Unmarshal(data, &changed)
			switch fault {
			case "route":
				changed.Routes[0].Config.Target = address(987).Hex()
			case "amount":
				changed.Routes[0].Config.Amount = "101"
			case "decimals":
				changed.Routes[0].Decimals = 7
			case "action":
				changed.Routes[0].Output.Actions[cfg.Action][67] ^= 1
			case "input":
				changed.Routes[0].Output.Inputs = map[uint]string{77: "100"}
			case "account":
				var identity prepared
				_ = json.Unmarshal(changed.Routes[0].Output.Data, &identity)
				identity.Account = address(987)
				changed.Routes[0].Output.Data, _ = json.Marshal(identity)
			case "expected":
				var identity prepared
				_ = json.Unmarshal(changed.Routes[0].Output.Data, &identity)
				identity.Expected = "1"
				changed.Routes[0].Output.Data, _ = json.Marshal(identity)
			}
			corrupt, err := derived.Encode(PolicyVersion, changed, inner, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.ExclusiveOutputTargets(corrupt); err == nil {
				t.Fatal("corrupt durable issuer route accepted")
			}
		})
	}
	empty, err := derived.Empty(PolicyVersion)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := p.Advance(context.Background(), provider.Snapshot{}, empty, nil)
	if err != nil || !progress.Done {
		t.Fatal("empty issuer provider blocked unrelated recipe")
	}
	if got := common.HexToAddress(cfg.Token); got == (common.Address{}) {
		t.Fatal("fixture missing token")
	}
}

func TestAutomaticSecuritizeBoundedAmount(t *testing.T) {
	cfg, s, reader := fixture(t, "wallet")
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	asset := p.assets[0]
	cfg.Token, cfg.Target = asset.Token, asset.RedemptionWallet
	cfg.AmountInput = nil
	reader.configure(cfg)
	r, tree := automaticRecipe(t, cfg, s, reader)
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	r.Steps = [][]byte{r.Steps[0], packedStep(t, recipes.LessOrEqual, binaryArguments, packedValue(recipes.Input, 7), packedValue(recipes.Result, 0)), packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{packedBinding(36, recipes.Input, 7)}})}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil || len(req.Values) != 2 || len(req.Actions) != 1 || len(req.Inputs) != 1 || req.Inputs[0] != 7 {
		t.Fatalf("bounded requirements %+v %v", req, err)
	}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if err != nil || out.Inputs[7] != "100" {
		t.Fatalf("bounded amount %+v %v", out, err)
	}
	if _, err = p.ExclusiveOutputTargets(out); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticSecuritizeLeavesUnrelatedGenericRedemptionUnclaimed(t *testing.T) {
	cfg, s, reader := fixture(t, "wallet")
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	_ = cfg
	data := append(bytes.Clone(crypto.Keccak256([]byte("redeem(uint256)"))[:4]), common.LeftPadBytes(big.NewInt(100).Bytes(), 32)...)
	leaf := recipes.Leaf{Target: address(987), Value: new(big.Int), Data: data, DataMask: make([]byte, len(data))}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	r := recipes.Recipe{Steps: [][]byte{packedStep(t, recipes.Manage, args, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})}}
	tree := &recipes.Tree{ChainID: s.ChainID, Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil || len(req.Actions) != 0 {
		t.Fatalf("generic redemption misclassified: %+v %v", req, err)
	}
}

func TestAutomaticSecuritizeLegacyGuardMinimumInput(t *testing.T) {
	for _, operation := range []uint8{recipes.LessOrEqual, recipes.Equal} {
		t.Run(new(big.Int).SetUint64(uint64(operation)).String(), func(t *testing.T) {
			cfg, s, reader := fixture(t, "legacy")
			raw, err := NewAutomatic(registry.Deps{Chain: reader})
			if err != nil {
				t.Fatal(err)
			}
			p := raw.(*automatic)
			asset := p.assets[0]
			cfg.Token = asset.Token
			for _, route := range asset.Offramps {
				if route.Mode == "legacy" {
					cfg.Target, cfg.PaymentToken = route.Target, route.PaymentToken
					break
				}
			}
			cfg.AmountInput, cfg.MinOutputInput = nil, nil
			cfg.MinOutput = "50"
			cfg.Action = "3:0"
			reader.configure(cfg)
			r, tree := automaticRecipe(t, cfg, s, reader)
			r.Steps = [][]byte{r.Steps[0], r.Steps[1], r.Steps[2], packedStep(t, operation, binaryArguments, packedValue(recipes.Result, 2), packedValue(recipes.Input, 7)), r.Steps[3], r.Steps[4], packedStep(t, recipes.Sub, binaryArguments, packedValue(recipes.Result, 5), packedValue(recipes.Result, 1)), packedStep(t, recipes.LessOrEqual, binaryArguments, packedValue(recipes.Input, 7), packedValue(recipes.Result, 6))}
			req, err := p.RecipeRequirements(s, r, tree)
			if err != nil || len(req.Inputs) != 1 || req.Inputs[0] != 7 || len(req.Actions) != 1 || req.Actions[0] != "4:0" {
				t.Fatalf("guard ownership %+v %v", req, err)
			}
			out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100), 2: big.NewInt(50)})
			if err != nil || out.Inputs[7] != "50" || len(out.Actions["4:0"]) != 36 {
				t.Fatalf("legacy guard output %+v %v", out, err)
			}
			var bundle durableRoutes
			_, _, err = derived.Decode(PolicyVersion, out, &bundle)
			if err != nil || bundle.Routes[0].Config.MinOutputInput != nil || bundle.Routes[0].Config.GuardMinimumInput == nil || *bundle.Routes[0].Config.GuardMinimumInput != 7 {
				t.Fatalf("guard durable policy %+v %v", bundle, err)
			}
			runtimes, outputs, err := p.restore(out)
			if err != nil {
				t.Fatal(err)
			}
			s = execution(t, runtimes[0], s, outputs[0])
			if progress, err := p.Advance(context.Background(), s, out, nil); err != nil || !progress.Done {
				t.Fatalf("legacy guard recovery %+v %v", progress, err)
			}
			r.Steps[3] = packedStep(t, recipes.LessOrEqual, binaryArguments, packedValue(recipes.Input, 7), packedValue(recipes.Result, 2))
			if _, err = p.RecipeRequirements(s, r, tree); err == nil {
				t.Fatal("minimum ceiling accepted as proceeds floor")
			}
		})
	}
}

func TestAutomaticSecuritizeLegacyCombinesPaymentFloors(t *testing.T) {
	cfg, s, reader := fixture(t, "legacy")
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*automatic)
	asset := p.assets[0]
	cfg.Token = asset.Token
	for _, route := range asset.Offramps {
		if route.Mode == "legacy" {
			cfg.Target, cfg.PaymentToken = route.Target, route.PaymentToken
			break
		}
	}
	cfg.AmountInput, cfg.MinOutputInput = nil, nil
	cfg.MinOutput = "50"
	cfg.Action = "3:0"
	reader.configure(cfg)
	r, tree := automaticRecipe(t, cfg, s, reader)
	r.Steps = append(r.Steps, packedStep(t, recipes.LessOrEqual, binaryArguments, packedValue(recipes.Result, 1), packedValue(recipes.Result, 5)))
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(70), 2: big.NewInt(50)})
	if err != nil {
		t.Fatal(err)
	}
	var bundle durableRoutes
	_, _, err = derived.Decode(PolicyVersion, out, &bundle)
	if err != nil || bundle.Routes[0].Config.MinOutput != "70" {
		t.Fatalf("combined floors %+v %v", bundle, err)
	}
}

type preparationReadCounter struct {
	provider.Reader
	reads int
}

func (r *preparationReadCounter) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.reads++
	return r.Reader.CallContract(ctx, msg, block)
}

func TestAutomaticSecuritizeExclusiveCompositionRejectsBeforeReads(t *testing.T) {
	for _, fault := range []string{"another provider", "unowned manage", "direct call"} {
		t.Run(fault, func(t *testing.T) {
			cfg, s, reader := fixture(t, "wallet")
			counter := &preparationReadCounter{Reader: reader}
			raw, err := NewAutomatic(registry.Deps{Chain: counter})
			if err != nil {
				t.Fatal(err)
			}
			p := raw.(*automatic)
			asset := p.assets[0]
			cfg.Token, cfg.Target = asset.Token, asset.RedemptionWallet
			cfg.AmountInput = nil
			cfg.Action = "1:0"
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
			if err = keeper.ValidateRecipeComposition(r, tree, requirements); err == nil || counter.reads != 0 {
				t.Fatalf("composition conflict reached reads: %v reads=%d", err, counter.reads)
			}
		})
	}
}
