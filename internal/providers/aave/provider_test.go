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
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
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
func capacityFixture(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree, int) {
	s := provider.Snapshot{Account: common.HexToAddress("0xa1"), Executor: common.HexToAddress("0xe1"), Caller: common.HexToAddress("0xc1"), BlockNumber: big.NewInt(9)}
	pool, asset, helper := common.HexToAddress("0x11"), common.HexToAddress("0x22"), common.HexToAddress("0x33")
	borrowData := append(selector("borrow(address,uint256,uint256,uint16,address)"), pack(t, borrowArgs, asset, new(big.Int), big.NewInt(2), uint16(0), s.Account)...)
	health := append(selector("getHealthFactor(address,address)"), pack(t, healthArgs, pool, s.Account)...)
	queries := [][]byte{pack(t, queryArgs, helper, health, []*big.Int{}, new(big.Int))}
	steps := [][]byte{instruction(t, recipes.Call, callArgs, pool, borrowData, []*big.Int{}), instruction(t, recipes.Read, uintArgs, new(big.Int)), instruction(t, recipes.LessOrEqual, binaryArgs, big.NewInt(1_250_000_000_000_000_000), value(recipes.Result, 1))}
	data := append(selector("execute(bytes[],bytes[],uint256[])"), pack(t, executeArgs, queries, steps, []*big.Int{})...)
	offset := bytes.Index(data, borrowData) + 36
	mask := make([]byte, len(data))
	for i := offset; i < offset+32; i++ {
		mask[i] = 255
	}
	leaf := recipes.Leaf{Target: s.Account, Value: new(big.Int), Data: data, DataMask: mask}
	leaf.Hash, _ = recipes.HashLeaf(leaf)
	binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset)), 224), value(recipes.Result, 0))
	r := recipes.Recipe{Steps: [][]byte{instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 0), big.NewInt(100)), instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 0), [][32]byte{leaf.Hash}, [][]*big.Int{{binding}})}}
	return s, r, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}, offset
}

type noRPC struct {
	calls int
	raw   []byte
	err   error
}

func (r *noRPC) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	r.calls++
	return nil, errors.New("unexpected read")
}
func (r *noRPC) CallContractWithCode(context.Context, ethereum.CallMsg, *big.Int, []byte) ([]byte, error) {
	r.calls++
	return r.raw, r.err
}
func TestAuthenticatedCapacityPreflight(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*recipes.Recipe, *recipes.Tree, int)
		want   string
	}{
		{"valid", func(*recipes.Recipe, *recipes.Tree, int) {}, ""},
		{"direct input", func(r *recipes.Recipe, _ *recipes.Tree, _ int) {
			r.Steps[0] = instruction(t, recipes.Add, binaryArgs, value(recipes.Input, 0), big.NewInt(0))
		}, ""},
		{"range only", func(r *recipes.Recipe, _ *recipes.Tree, _ int) {
			r.Steps = append(r.Steps, instruction(t, recipes.LessOrEqual, binaryArgs, value(recipes.Input, 0), big.NewInt(100)))
		}, "only feed"},
		{"multiple groups", func(r *recipes.Recipe, _ *recipes.Tree, _ int) { r.Steps = append(r.Steps, r.Steps[1]) }, "multiple"},
		{"mutable health", func(_ *recipes.Recipe, tree *recipes.Tree, _ int) { tree.Leaves[0].DataMask[4] = 255 }, "unpinned"},
		{"stable rate", func(_ *recipes.Recipe, tree *recipes.Tree, offset int) { tree.Leaves[0].Data[offset+63] = 1 }, "unpinned"},
		{"referral", func(_ *recipes.Recipe, tree *recipes.Tree, offset int) { tree.Leaves[0].Data[offset+95] = 1 }, "unpinned"},
		{"other account", func(_ *recipes.Recipe, tree *recipes.Tree, offset int) { tree.Leaves[0].Data[offset+127] ^= 1 }, "unpinned"},
		{"health pool mismatch", func(_ *recipes.Recipe, tree *recipes.Tree, _ int) {
			data := tree.Leaves[0].Data
			at := bytes.Index(data, selector("getHealthFactor(address,address)"))
			data[at+35] ^= 1
		}, "unpinned"},
		{"health account mismatch", func(_ *recipes.Recipe, tree *recipes.Tree, _ int) {
			data := tree.Leaves[0].Data
			at := bytes.Index(data, selector("getHealthFactor(address,address)"))
			data[at+67] ^= 1
		}, "unpinned"},
		{"interleaved effect", func(r *recipes.Recipe, _ *recipes.Tree, _ int) {
			effect := instruction(t, recipes.Call, callArgs, common.HexToAddress("0x99"), []byte{1, 2, 3, 4}, []*big.Int{})
			r.Steps = [][]byte{r.Steps[0], effect, r.Steps[1]}
		}, "effects between"},
		{"recipe-derived unrelated shape", func(r *recipes.Recipe, tree *recipes.Tree, _ int) {
			r.Steps[0] = instruction(t, recipes.Min, binaryArgs, big.NewInt(100), big.NewInt(100))
			tree.Leaves[0].Data = append(tree.Leaves[0].Data, 0)
			tree.Leaves[0].DataMask = append(tree.Leaves[0].DataMask, 0)
		}, ""},

		{"invalid health floor", func(_ *recipes.Recipe, tree *recipes.Tree, _ int) {
			data := tree.Leaves[0].Data
			needle := pack(t, uintArgs, big.NewInt(1_250_000_000_000_000_000))
			at := bytes.Index(data, needle)
			big.NewInt(999999999999999999).FillBytes(data[at : at+32])
		}, "unpinned"},
		{"noncanonical capacity guard", func(_ *recipes.Recipe, tree *recipes.Tree, _ int) {
			tree.Leaves[0].Data = append(tree.Leaves[0].Data, 0)
			tree.Leaves[0].DataMask = append(tree.Leaves[0].DataMask, 0)
		}, "unpinned"},
		{"unresolved prefix", func(r *recipes.Recipe, tree *recipes.Tree, offset int) {
			binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset)), 224), value(recipes.Result, 1))
			r.Steps = [][]byte{instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 1), big.NewInt(100)), instruction(t, recipes.Min, binaryArgs, value(recipes.Input, 0), value(recipes.Result, 0)), instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{binding}})}
		}, "input"},

		{"wrong binding", func(r *recipes.Recipe, tree *recipes.Tree, offset int) {
			binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset+32)), 224), value(recipes.Result, 0))
			r.Steps[1] = instruction(t, recipes.Manage, manageArgs, value(recipes.Result, 0), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{binding}})
		}, "unpinned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tree, offset := capacityFixture(t)
			tc.mutate(&r, tree, offset)
			old := tree.Leaves[0].Hash
			tree.Leaves[0].Hash, _ = recipes.HashLeaf(tree.Leaves[0])
			tree.Root = tree.Leaves[0].Hash
			for i, step := range r.Steps {
				r.Steps[i] = bytes.ReplaceAll(step, old[:], tree.Root[:])
			}
			reader := &noRPC{}
			p := &Automatic{reader: reader}
			req, err := p.RecipeRequirements(s, r, tree)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q: %v", tc.want, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tc.name == "valid" && (len(req.Inputs) != 1 || req.Inputs[0] != 0) {
				t.Fatal("capacity ownership missing")
			}
			if tc.name == "direct input" {
				if _, err = recipes.AnchoredInputs(r, nil); err == nil {
					t.Fatal("unrecognized direct input escaped generic exact-source check")
				}
			}
			if reader.calls != 0 {
				t.Fatal("preflight performed network I/O")
			}
		})
	}
}
func TestCapacityProbeErrorsAndAdvance(t *testing.T) {
	s, r, tree, _ := capacityFixture(t)
	reader := &noRPC{err: errors.New("secret RPC payload")}
	p := &Automatic{reader: reader}
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
	out, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
	if err != nil || out.Inputs[0] != "42" {
		t.Fatal(out, err)
	}
	if _, err = p.Advance(t.Context(), s, out, nil); err == nil {
		t.Fatal("advanced without confirmed execution")
	}
	s.Execution = &provider.Execution{TxHash: common.HexToHash("0x1"), BlockNumber: big.NewInt(1), BlockHash: common.HexToHash("0x2")}
	progress, err := p.Advance(t.Context(), s, out, nil)
	if err != nil || !progress.Done {
		t.Fatal(progress, err)
	}
	out.Inputs[0] = "43"
	if _, err = p.Advance(t.Context(), s, out, nil); err == nil {
		t.Fatal("modified persisted output accepted")
	}
	if _, err = NewAutomatic(registry.Deps{}); err == nil {
		t.Fatal("missing reader accepted")
	}
}
func TestProbeArtifactSources(t *testing.T) {
	var artifact struct{ Sources map[string]string }
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"AaveBorrowProbe.sol": "lens/AaveBorrowProbe.sol", "RecipePrefixLens.sol": "../../../sdk/recipes/lens/RecipePrefixLens.sol"} {
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
	p := &Automatic{reader: &noRPC{}}
	req, err := p.RecipeRequirements(provider.Snapshot{Account: document.Account}, recipe, tree)
	if err != nil || len(req.Inputs) != 1 || req.Inputs[0] != 0 {
		t.Fatal(req, err)
	}
	if _, err = recipes.AnchoredInputs(recipe, map[uint]bool{0: true}); err != nil {
		t.Fatal(err)
	}
}
