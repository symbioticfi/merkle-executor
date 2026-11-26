package iris

import (
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"slices"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/iris"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

//go:embed testdata/claim.json
var fixture []byte

func claimFixture(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	var f struct {
		Account common.Address
		Recipe  hexutil.Bytes
		Leaf    struct {
			Target         common.Address
			Hash           common.Hash
			Data, DataMask hexutil.Bytes
		}
	}
	if err := json.Unmarshal(fixture, &f); err != nil {
		t.Fatal(err)
	}
	recipe, err := recipes.DecodeRecipe(f.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	leaf := recipes.Leaf{Target: f.Leaf.Target, Hash: f.Leaf.Hash, Value: new(big.Int), Data: f.Leaf.Data, DataMask: f.Leaf.DataMask}
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(42), Account: f.Account}
	return snapshot, recipe, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
}

func TestOwnsOnlyGuardedClaimInput(t *testing.T) {
	s, recipe, tree := claimFixture(t)
	p := &Provider{}
	for _, candidate := range []recipes.Recipe{
		recipe,
		claimProgram(s.Account, tree.Root, true, false),
		claimProgram(s.Account, tree.Root, true, true),
	} {
		req, err := p.RecipeRequirements(s, candidate, tree)
		if err != nil || !slices.Equal(req.Inputs, []uint{0}) {
			t.Fatalf("claim input requirement = %v, %v", req.Inputs, err)
		}
		if _, err := recipes.AnchoredInputs(candidate, map[uint]bool{0: true}); err != nil {
			t.Fatal(err)
		}
	}

	modified := recipes.CloneRecipe(recipe)
	modified.Steps[1] = append([]byte{recipes.Equal}, pack(compareArgs, new(big.Int), new(big.Int))...)
	req, err := p.RecipeRequirements(s, modified, tree)
	if err != nil || len(req.Inputs) != 0 {
		t.Fatalf("changed recipe received claim input: %v, %v", req.Inputs, err)
	}
	s.ChainID = big.NewInt(11155111)
	req, err = p.RecipeRequirements(s, recipe, tree)
	if err != nil || len(req.Inputs) != 0 {
		t.Fatalf("non-mainnet recipe received claim input: %v, %v", req.Inputs, err)
	}
}

type chainFixture struct {
	t          *testing.T
	snapshot   provider.Snapshot
	positions  int
	allSettled bool
}

func (c *chainFixture) NonceAt(_ context.Context, account common.Address, block *big.Int) (uint64, error) {
	if account != mainnetCore || block.Cmp(c.snapshot.BlockNumber) != 0 {
		c.t.Fatal("Iris discovery is not pinned to the core and the recipe block")
	}
	return 5, nil
}

func (c *chainFixture) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if call.To == nil || *call.To != mainnetCore || block.Cmp(c.snapshot.BlockNumber) != 0 {
		c.t.Fatal("Iris read is not pinned to the core and the recipe block")
	}
	abi := coreABI.GetABI()
	method, err := abi.MethodById(call.Data[:4])
	if err != nil {
		c.t.Fatal(err)
	}
	pod := common.BytesToAddress(call.Data[4:])
	z := new(big.Int)
	if method.Name == "getLoan" {
		foreign := common.HexToAddress("0xff")
		owner := c.snapshot.Account
		if pod == crypto.CreateAddress(mainnetCore, 3) {
			owner = foreign
		}
		return method.Outputs.Pack(binding.Struct0{Borrower: foreign, Solver: owner, VenueBitmap: z})
	}

	c.positions++
	if pod == crypto.CreateAddress(mainnetCore, 3) {
		c.t.Fatal("read a position for another solver")
	}
	position := binding.Struct1{Collateral: big.NewInt(70), Debt: z, Bond: big.NewInt(20),
		BondRequirement: z, CollateralIndex: z, DebtIndex: z, FixedLeg: z, FloatingLeg: z,
		Surplus: z, LastUpdate: 1}
	switch pod {
	case crypto.CreateAddress(mainnetCore, 1):
		position.Debt = big.NewInt(50)
	case crypto.CreateAddress(mainnetCore, 2):
		if c.allSettled {
			position.Bond = z
		}
	case crypto.CreateAddress(mainnetCore, 4):
		position.Bond = z
	default:
		c.t.Fatal("read an unexpected Iris pod")
	}
	return method.Outputs.Pack(position)
}

func TestSuppliesFinishedOwnedPodWithBond(t *testing.T) {
	s, recipe, tree := claimFixture(t)
	chain := &chainFixture{t: t, snapshot: s}
	p, err := NewAutomatic(provider.Deps{Chain: chain})
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := p.PrepareRecipe(t.Context(), s, recipe, tree, nil)
	if err != nil || len(outputs) != 1 || chain.positions != 3 {
		t.Fatalf("Iris claim discovery = %v, %v, %d position reads", outputs, err, chain.positions)
	}
	output := outputs[0]
	want := crypto.CreateAddress(mainnetCore, 2)
	if output.Inputs[0] != want.Big().String() || len(output.Actions) != 0 {
		t.Fatalf("Iris runtime output = %v, expected only pod %s", output, want)
	}
	if _, _, err := recipes.Assemble(recipe, tree, output.Inputs, nil, nil); err != nil {
		t.Fatal(err)
	}

	chain.allSettled = true
	if _, err := p.PrepareRecipe(t.Context(), s, recipe, tree, nil); !errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("no claimable bonds: %v", err)
	}
}
