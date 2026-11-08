package aave

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func pack(t *testing.T, args abi.Arguments, values ...any) []byte {
	t.Helper()
	b, err := args.Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func value(source uint8, n int64) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(source)), 216), big.NewInt(n))
}
func instruction(t *testing.T, op uint8, args abi.Arguments, values ...any) []byte {
	return append([]byte{op}, pack(t, args, values...)...)
}
func capacityFixture(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	s := provider.Snapshot{Account: common.HexToAddress("0xa1"), Executor: common.HexToAddress("0xe1"), Caller: common.HexToAddress("0xc1"), BlockNumber: big.NewInt(9)}
	pool, asset := common.HexToAddress("0x11"), common.HexToAddress("0x22")
	data := append(selector("borrow(address,uint256,uint256,uint16,address)"), pack(t, borrowArgs, asset, new(big.Int), big.NewInt(2), uint16(0), s.Account)...)
	mask := make([]byte, len(data))
	for i := 36; i < 68; i++ {
		mask[i] = 255
	}
	leaf := recipes.Leaf{Target: pool, Value: new(big.Int), Data: data, DataMask: mask}
	leaf.Hash, _ = recipes.HashLeaf(leaf)
	binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), value(recipes.Result, 0))
	health := append(selector("getUserAccountData(address)"), pack(t, accountArgs, s.Account)...)
	r := recipes.Recipe{
		Queries: [][]byte{pack(t, queryArgs, pool, health, []*big.Int{}, big.NewInt(160))},
		Steps: [][]byte{
			instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 0), big.NewInt(100)),
			instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 0), [][32]byte{leaf.Hash}, [][]*big.Int{{binding}}),
			instruction(t, recipes.Read, uintArgs, new(big.Int)),
			instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 2)),
		},
	}
	return s, r, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
}

type noRPC struct {
	calls int
	raw   []byte
	err   error
}

func TestAuthenticatedCapacityPreflight(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(*recipes.Recipe, *recipes.Tree)
		invalid   bool
		unclaimed bool
	}{
		{"valid", func(*recipes.Recipe, *recipes.Tree) {}, false, false},
		{"mutable asset", func(_ *recipes.Recipe, tree *recipes.Tree) { tree.Leaves[0].DataMask[35] = 255 }, true, false},
		{"other account", func(_ *recipes.Recipe, tree *recipes.Tree) { tree.Leaves[0].Data[163] ^= 1 }, true, false},
		{"stable debt", func(_ *recipes.Recipe, tree *recipes.Tree) { tree.Leaves[0].Data[99] = 1 }, true, false},
		{"referral", func(_ *recipes.Recipe, tree *recipes.Tree) { tree.Leaves[0].Data[131] = 1 }, true, false},
		{"missing health floor", func(r *recipes.Recipe, _ *recipes.Tree) { r.Steps = r.Steps[:2] }, true, false},
		{"wrong health pool", func(r *recipes.Recipe, _ *recipes.Tree) { r.Queries[0][31] ^= 1 }, true, false},
		{"wrong health account", func(r *recipes.Recipe, _ *recipes.Tree) { r.Queries[0][195] ^= 1 }, true, false},
		{"wrong health position", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Queries[0] = pack(t, queryArgs, common.HexToAddress("0x11"), append(selector("getUserAccountData(address)"), pack(t, argTypes("address"), common.HexToAddress("0xa1"))...), []*big.Int{}, big.NewInt(128))
		}, true, false},
		{"unsafe floor", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Steps[3] = instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(999_999_999_999_999_999), value(recipes.Result, 2))
		}, true, false},
		{"health after another effect", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Steps = append(r.Steps[:2], append([][]byte{instruction(t, recipes.Call, callArgs, common.HexToAddress("0x99"), []byte{1}, []*big.Int{})}, r.Steps[2:]...)...)
			r.Steps[4] = instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 3))
		}, true, false},
		{"direct input", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Steps[0] = instruction(t, recipes.Add, binaryArgs, value(recipes.Input, 0), big.NewInt(0))
		}, false, true},
		{"range only", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Steps = append(r.Steps, instruction(t, recipes.LessOrEqual, binaryArgs, value(recipes.Input, 0), big.NewInt(100)))
		}, true, false},
		{"multiple groups", func(r *recipes.Recipe, _ *recipes.Tree) {
			r.Steps = append(r.Steps, r.Steps[1], r.Steps[2], instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 5)))
		}, true, false},
		{"interleaved effect", func(r *recipes.Recipe, _ *recipes.Tree) {
			effect := instruction(t, recipes.Call, callArgs, common.HexToAddress("0x99"), []byte{1, 2, 3, 4}, []*big.Int{})
			r.Steps = append([][]byte{r.Steps[0], effect}, r.Steps[1:]...)
			r.Steps[4] = instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 3))
		}, true, false},
		{"recipe-derived unrelated shape", func(r *recipes.Recipe, tree *recipes.Tree) {
			r.Steps[0] = instruction(t, recipes.Min, binaryArgs, big.NewInt(100), big.NewInt(100))
			tree.Leaves[0].Data = append(tree.Leaves[0].Data, 0)
			tree.Leaves[0].DataMask = append(tree.Leaves[0].DataMask, 0)
		}, false, true},
		{"noncanonical borrow", func(_ *recipes.Recipe, tree *recipes.Tree) {
			tree.Leaves[0].Data = append(tree.Leaves[0].Data, 0)
			tree.Leaves[0].DataMask = append(tree.Leaves[0].DataMask, 0)
		}, true, false},
		{"unresolved prefix", func(r *recipes.Recipe, tree *recipes.Tree) {
			binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), value(recipes.Result, 1))
			r.Steps = [][]byte{
				instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 1), big.NewInt(100)),
				instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 0), value(recipes.Result, 0)),
				instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{binding}}),
				r.Steps[2],
				instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 3)),
			}
		}, true, false},
		{"wrong binding", func(r *recipes.Recipe, tree *recipes.Tree) {
			binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(68), 224), value(recipes.Result, 0))
			r.Steps[1] = instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 0), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{binding}})
		}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tree := capacityFixture(t)
			old := tree.Leaves[0].Hash
			tc.mutate(&r, tree)
			tree.Leaves[0].Hash, _ = recipes.HashLeaf(tree.Leaves[0])
			tree.Root = tree.Leaves[0].Hash
			for i := range r.Steps {
				r.Steps[i] = bytes.ReplaceAll(r.Steps[i], old[:], tree.Root[:])
			}
			reader := &noRPC{}
			req, err := (&Provider{reader: reader}).RecipeRequirements(s, r, tree)
			if tc.invalid {
				if err == nil {
					t.Fatal("unsafe native capacity recipe accepted")
				}
			} else if tc.unclaimed {
				if err != nil || len(req.Inputs) != 0 {
					t.Fatalf("unrelated recipe claimed: %v %v", req, err)
				}
				if _, err = recipes.AnchoredInputs(r, nil); err == nil && tc.name == "direct input" {
					t.Fatal("unrecognized direct input escaped generic exact-source check")
				}
			} else if err != nil || len(req.Inputs) != 1 || req.Inputs[0] != 0 {
				t.Fatalf("native capacity ownership missing: %v %v", req, err)
			}
			if reader.calls != 0 {
				t.Fatal("preflight performed network I/O")
			}
		})
	}
}

func (r *noRPC) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	r.calls++
	return nil, errors.New("unexpected read")
}
func (r *noRPC) CallContractWithCode(context.Context, ethereum.CallMsg, *big.Int, []byte) ([]byte, error) {
	r.calls++
	return r.raw, r.err
}
func TestCapacityProbeErrorsAndOutput(t *testing.T) {
	s, r, tree := capacityFixture(t)
	reader := &noRPC{err: errors.New("secret RPC payload")}
	p := &Provider{reader: reader}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := p.PrepareRecipe(ctx, s, r, tree, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if reader.calls != 0 {
		t.Fatal("canceled preparation read chain")
	}
	if _, err := p.PrepareRecipe(t.Context(), s, r, tree, nil); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	reader.err = nil
	reader.raw = []byte{1}
	if _, err := p.PrepareRecipe(t.Context(), s, r, tree, nil); err == nil {
		t.Fatal("malformed probe reply accepted")
	}
	outer := argTypes("uint256[]", "bytes")
	reader.raw = pack(t, outer, []*big.Int{big.NewInt(0)}, pack(t, uintArgs, big.NewInt(42)))
	outputs, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
	var out provider.Output
	if len(outputs) == 1 {
		out = outputs[0]
	}
	if err != nil || out.Inputs[0] != "42" {
		t.Fatal(out, err)
	}
	if out.Task != nil || len(out.Data) != 0 {
		t.Fatal("capacity probe retained settlement state")
	}
	if _, err = New(nil); err == nil {
		t.Fatal("missing reader accepted")
	}
}
func TestProbeArtifactSources(t *testing.T) {
	var artifact struct{ Sources map[string]string }
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"AaveBorrowProbe.sol": "lens/AaveBorrowProbe.sol", "RecipePrefixLens.sol": "../../recipes/lens/RecipePrefixLens.sol"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if artifact.Sources[name] != hex.EncodeToString(sum[:]) {
			t.Fatalf("stale %s", name)
		}
	}
}

func TestFrontendLendingCapacity(t *testing.T) {
	raw, err := os.ReadFile("testdata/frontend-lending.json")
	if err != nil {
		t.Fatal(err)
	}
	// The frontend fixture carries hex bytes and an authenticated permission package.
	var document struct {
		Account common.Address
		Recipe  string
	}
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	encoded, err := hex.DecodeString(strings.TrimPrefix(document.Recipe, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := recipes.DecodeRecipe(encoded)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{reader: &noRPC{}}
	req, err := p.RecipeRequirements(provider.Snapshot{Account: document.Account}, recipe, tree)
	if err != nil || len(req.Inputs) != 1 || req.Inputs[0] != 0 {
		t.Fatal(req, err)
	}
	if _, err = recipes.AnchoredInputs(recipe, map[uint]bool{0: true}); err != nil {
		t.Fatal(err)
	}
}
