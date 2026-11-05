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
	t                *testing.T
	account, factory common.Address
	count            uint64
	groups           []int
	owned            map[common.Address]bool
	ready            map[common.Address]bool
	calls            int
}

func (c *chainFixture) CallContract(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	c.t.Helper()
	c.calls++
	if b == nil || b.Int64() != 12 {
		c.t.Fatal("unpinned read")
	}
	if *m.To == c.factory {
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
			variants := append([]map[string]hexutil.Bytes{out.Actions}, out.Repeats...)
			want := int((count + 3) / 4)
			if len(variants) != want {
				t.Fatalf("got %d want %d", len(variants), want)
			}
			for i, v := range variants {
				a := common.BytesToAddress(v[cfg.Key][cfg.Offset : cfg.Offset+32])
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
