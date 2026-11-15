package threef

import (
	"bytes"
	"context"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	request "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequest"
	factory "github.com/symbioticfi/merkle-executor/sdk/bindings/threefrequestsfactory"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"os"
	"testing"
)

func fixture(t *testing.T) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	raw, err := os.ReadFile("testdata/frontend-claim.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Account common.Address
		Recipe  hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	r, err := recipes.DecodeRecipe(v.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	return provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: v.Account}, r, tree
}
func TestExactFrontendClaimPermission(t *testing.T) {
	s, r, tree := fixture(t)
	c, ok, err := derive(s, r, tree)
	if err != nil || !ok {
		t.Fatalf("frontend template rejected: %v", err)
	}
	expected, at := claimTemplate(s.Account, c.Factory)
	if at != c.Offset || !bytes.Equal(expected, c.Template) {
		t.Fatal("frontend and SDK template differ")
	}
	// Every fixed byte must stay fixed, and only the one request word is mutable.
	for i := range c.Template {
		changed := recipes.CloneTree(tree)
		changed.Leaves[0].DataMask[i] ^= 1
		actions, _ := recipes.ManagedActions(r, tree)
		action := actions[0]
		action.Leaf = changed.Leaves[0]
		_, _, matches := claimLayout(action, s.Account)
		if matches {
			t.Fatalf("wrong mutable byte %d", i)
		}
	}
}

type chainFixture struct {
	t                                      *testing.T
	account, factory                       common.Address
	count                                  uint64
	groups                                 []int
	owned                                  map[common.Address]bool
	ready                                  map[common.Address]bool
	calls                                  int
	factoryReads, balanceReads, readyReads int
}

func (c *chainFixture) CallContract(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	c.t.Helper()
	c.calls++
	if b == nil || b.Int64() != 12 {
		c.t.Fatal("unpinned read")
	}
	if *m.To == c.factory {
		c.factoryReads++
		a := factory.NewThreeFRequestsFactory().GetABI()
		method := a.Methods["isRequest"]
		v, err := method.Inputs.Unpack(m.Data[4:])
		if err != nil {
			return nil, err
		}
		exists := false
		for i := uint64(0); i < c.count; i++ {
			exists = exists || v[0].(common.Address) == requestAt(c.factory, i)
		}
		return method.Outputs.Pack(exists)
	}
	a := request.NewThreeFRequest().GetABI()
	method, err := a.MethodById(m.Data[:4])
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case "balancesOf":
		c.balanceReads++
		v, _ := method.Inputs.Unpack(m.Data[4:])
		if v[0].(common.Address) != c.account {
			c.t.Fatal("wrong owner")
		}
		n := new(big.Int)
		if c.owned[*m.To] {
			n.SetInt64(1)
		}
		return method.Outputs.Pack(n, new(big.Int))
	case "canWithdraw":
		c.readyReads++
		if !c.owned[*m.To] {
			c.t.Fatal("read readiness for unowned request")
		}
		return method.Outputs.Pack(c.ready[*m.To])
	}
	c.t.Fatal("unexpected getter")
	return nil, nil
}
func (c *chainFixture) BatchRead(ctx context.Context, req []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	c.groups = append(c.groups, len(req))
	out := make([]protocolio.ReadResult, len(req))
	for i, r := range req {
		out[i].Data, out[i].Err = c.CallContract(ctx, r.Call, r.Block)
	}
	return out, nil
}
func TestClaimsAllOwnedReadyRequestsAndSkipsPending(t *testing.T) {
	s, r, tree := fixture(t)
	cfg, _, _ := derive(s, r, tree)
	for _, count := range []uint64{0, 1, 5, 257} {
		t.Run(big.NewInt(int64(count)).String(), func(t *testing.T) {
			chain := &chainFixture{t: t, account: s.Account, factory: cfg.Factory, count: count, owned: map[common.Address]bool{}, ready: map[common.Address]bool{}}
			for i := uint64(0); i < count; i++ {
				if i%2 == 0 {
					a := requestAt(cfg.Factory, i)
					chain.owned[a] = true
					chain.ready[a] = i%4 == 0
				}
			}
			p := &Provider{chain: chain}
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			if count == 0 {
				if err == nil {
					t.Fatal("empty factory runnable")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			variants := out
			want := int((count + 3) / 4)
			if len(variants) != want {
				t.Fatalf("got %d want %d", len(variants), want)
			}
			for i, v := range variants {
				a := common.BytesToAddress(v.Actions[cfg.Key][cfg.Offset : cfg.Offset+32])
				if a != requestAt(cfg.Factory, uint64(i*4)) {
					t.Fatal("missed or repeated request")
				}
			}
			for _, size := range chain.groups {
				if size > protocolio.MaxReadBatch {
					t.Fatal("unbounded read batch")
				}
			}
		})
	}
}

func TestFactoryHintRechecksBoundaryAndAllMutableRequests(t *testing.T) {
	s, recipe, tree := fixture(t)
	cfg, _, _ := derive(s, recipe, tree)
	chain := &chainFixture{t: t, account: s.Account, factory: cfg.Factory, count: 257, owned: map[common.Address]bool{}, ready: map[common.Address]bool{}}
	first, transferred := requestAt(cfg.Factory, 0), requestAt(cfg.Factory, 1)
	chain.owned[first], chain.ready[first] = true, true
	p := &Provider{chain: chain}
	if out, err := p.PrepareRecipe(t.Context(), s, recipe, tree, nil); err != nil || len(out) != 1 {
		t.Fatal(out, err)
	}
	coldReads, balanceReads, readyReads := chain.factoryReads, chain.balanceReads, chain.readyReads
	chain.owned[transferred], chain.ready[transferred] = true, true
	chain.ready[first] = false
	out, err := p.PrepareRecipe(t.Context(), s, recipe, tree, nil)
	if err != nil || len(out) != 1 || common.BytesToAddress(out[0].Actions[cfg.Key][cfg.Offset:cfg.Offset+32]) != transferred {
		t.Fatal("transferred rights or readiness were cached", out, err)
	}
	if chain.factoryReads-coldReads != 2 || coldReads <= 2 || chain.balanceReads-balanceReads != 257 || chain.readyReads-readyReads != 2 {
		t.Fatalf("cold=%d warm=%d balances=%d readiness=%d", coldReads, chain.factoryReads-coldReads, chain.balanceReads-balanceReads, chain.readyReads-readyReads)
	}
	factoryReads := chain.factoryReads
	chain.count = 258
	newRequest := requestAt(cfg.Factory, 257)
	chain.owned[newRequest], chain.ready[newRequest] = true, true
	if out, err = p.PrepareRecipe(t.Context(), s, recipe, tree, nil); err != nil || len(out) != 2 || chain.factoryReads-factoryReads != 3 {
		t.Fatal("new request not discovered at cached boundary", out, err, chain.factoryReads-factoryReads)
	}
	chain.count = 1
	chain.ready[first] = true
	balanceReads = chain.balanceReads
	if out, err = p.PrepareRecipe(t.Context(), s, recipe, tree, nil); err != nil || len(out) != 1 || chain.balanceReads-balanceReads != 1 {
		t.Fatal("replaced factory inventory retained stale lower bound", out, err)
	}
}
