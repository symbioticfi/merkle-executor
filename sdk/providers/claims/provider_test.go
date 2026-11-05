package claims

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type chain struct {
	t     *testing.T
	fail  bool
	ready bool
}

func (c *chain) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	c.t.Helper()
	if block == nil || block.Int64() != 12 {
		c.t.Fatal("read is not pinned")
	}
	if c.fail {
		return nil, errors.New("RPC unavailable")
	}
	m, err := contractABI.MethodById(msg.Data[:4])
	if err != nil {
		return nil, err
	}
	args, err := m.Inputs.Unpack(msg.Data[4:])
	if err != nil {
		return nil, err
	}
	n := int64(0)
	switch m.Name {
	case "claimWithdrawal", "claimDepositRequest", "claimWithdrawRequest0", "claimWithdrawRequest", "claimInstantWithdrawRequest", "claimDepositEscrow", "claimRedeemEscrow", "claimCancelDepositRequest", "claimCancelRedeemRequest":
		if msg.From != common.HexToAddress("0xa1") {
			c.t.Fatal("native preflight sender differs from account")
		}
		return nil, nil
	case "getUserNextRequestId":
		n = 4
	case "withdrawPeriod":
		n = 10
	case "getWithdrawalRequest":
		id := args[1].(*big.Int).Int64()
		amount := int64(0)
		if c.ready {
			amount = 100
		}
		timestamp := int64(80)
		if id == 1 {
			timestamp = 95
		}
		claimed := id == 2
		return m.Outputs.Pack(struct {
			Amount    *big.Int
			Timestamp *big.Int
			Claimed   bool
		}{big.NewInt(amount), big.NewInt(timestamp), claimed})
	case "strategy":
		return m.Outputs.Pack(common.HexToAddress("0x51"))
	case "epochNumber":
		n = 2
	case "userDepositsEpochs", "userWithdrawalsEpochs":
		if c.ready {
			n = 10
		}
	case "epochPrice", "epochWithdrawPrice":
		n = 1
	case "epochPendingDeposits", "epochPrefundedDeposits", "epochPendingClaims":
		if args[0].(*big.Int).Int64() != 1 {
			n = 1
		}
	case "claimableDepositRequest", "claimableRedeemRequest", "depositEscrowBalance", "redeemEscrowBalance", "claimableCancelDepositRequest", "claimableCancelRedeemRequest":
		if c.ready {
			n = 100
		}
	case "withdrawsRequests", "instantWithdrawsRequests":
		if c.ready {
			n = 100
		}
	case "apr0Users":
		return m.Outputs.Pack(big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0))
	default:
		c.t.Fatalf("unexpected read %s", m.Name)
	}
	return m.Outputs.Pack(big.NewInt(n))
}
func fixture(t *testing.T, name string, args ...any) (provider.Snapshot, recipes.Recipe, *recipes.Tree) {
	t.Helper()
	lookup := name
	if lookup == "claimWithdrawRequestForEpoch" {
		lookup = "claimWithdrawRequest0"
	}
	data, err := contractABI.Pack(lookup, args...)
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]byte, len(data))
	switch name {
	case "claimWithdrawal", "claimDepositRequest", "claimWithdrawRequestForEpoch":
		for i := 4; i < 36; i++ {
			mask[i] = 255
		}
	}
	leaf := recipes.Leaf{Target: common.HexToAddress("0x50"), Value: new(big.Int), Data: data, DataMask: mask}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	ty := func(name string) abi.Type {
		v, e := abi.NewType(name, "", nil)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	packed, err := (abi.Arguments{{Type: ty("uint256")}, {Type: ty("bytes32[]")}, {Type: ty("uint256[][]")}}).Pack(big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})
	if err != nil {
		t.Fatal(err)
	}
	r := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, packed...)}}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: common.HexToAddress("0xa1"), Timestamp: 100}
	return s, r, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
}
func TestSelectsEveryReadyNoonIDIncludingZero(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	p := &Provider{&chain{t: t, ready: true}}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want IDs zero and three, got %d", len(out))
	}
	for i, id := range []int64{0, 3} {
		var cfg config
		inner, _, err := derived.Decode(policy, out[i], &cfg)
		if err != nil {
			t.Fatal(err)
		}
		if new(big.Int).SetBytes(inner.Actions["0:0"][4:36]).Int64() != id {
			t.Fatal("wrong ready ID")
		}
		if !bytes.Equal(tree.Leaves[0].Data[4:], make([]byte, 32)) {
			t.Fatal("mutated permission")
		}
	}
}
func TestSelectsReadyParetoEpoch(t *testing.T) {
	for _, method := range []string{"claimDepositRequest", "claimWithdrawRequestForEpoch"} {
		t.Run(method, func(t *testing.T) {
			s, r, tree := fixture(t, method, big.NewInt(0))
			out, err := (&Provider{&chain{t: t, ready: true}}).PrepareRecipe(context.Background(), s, r, tree, nil)
			if err != nil || len(out) != 1 {
				t.Fatalf("outputs %d: %v", len(out), err)
			}
			if new(big.Int).SetBytes(out[0].Actions["0:0"][4:36]).Int64() != 1 {
				t.Fatal("unsettled epoch selected")
			}
		})
	}
}
func TestEmptyIsIdleButFailedRPCIsAnError(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	for _, fail := range []bool{false, true} {
		_, err := (&Provider{&chain{t: t, fail: fail}}).PrepareRecipe(context.Background(), s, r, tree, nil)
		if err == nil || errors.Is(err, provider.ErrNotReady) == fail {
			t.Fatalf("fail=%v error=%v", fail, err)
		}
	}
}
func TestPreparedClaimReconcilesWithoutReplacingRuntime(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	p := &Provider{&chain{t: t, ready: true}}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Execution = &provider.Execution{TxHash: common.HexToHash("0x1"), BlockNumber: big.NewInt(13), BlockHash: common.HexToHash("0x2"), Logs: []types.Log{}}
	if progress, err := p.Advance(context.Background(), s, out[0], nil); err != nil || !progress.Done {
		t.Fatalf("%+v %v", progress, err)
	}
	out[0].Actions["0:0"][0] ^= 1
	if _, err := p.Advance(context.Background(), s, out[0], nil); err == nil {
		t.Fatal("changed action accepted")
	}
}
func TestOnlySingleUnconditionalClaimMatches(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	_ = s
	r.Steps = append(r.Steps, r.Steps[0])
	if _, ok, err := match(r, tree); err != nil || ok {
		t.Fatalf("multi-claim recipe matched %v", err)
	}
}

func TestAsyncClaimUsesNativeEntitlementAndRejectsOrdinaryOperation(t *testing.T) {
	account := common.HexToAddress("0xa1")
	s, r, tree := fixture(t, "withdraw", big.NewInt(0), account, account)
	// An ordinary synchronous call is not claimed by this provider.
	if _, ok, err := match(r, tree); ok || err != nil {
		t.Fatalf("ordinary withdrawal matched: %v", err)
	}
	ty := func(name string) abi.Type {
		v, err := abi.NewType(name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	read, _ := contractABI.Pack("maxWithdraw", account)
	q, err := queryArgs.Pack(tree.Leaves[0].Target, read, []*big.Int{}, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}
	r.Queries = [][]byte{q}
	r.Steps = append([][]byte{append([]byte{recipes.Read}, make([]byte, 32)...)}, r.Steps...)
	result := new(big.Int).Lsh(big.NewInt(int64(recipes.Result)), 216)
	binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(4), 224), result)
	body, err := (abi.Arguments{{Type: ty("uint256")}, {Type: ty("bytes32[]")}, {Type: ty("uint256[][]")}}).Pack(big.NewInt(1), [][32]byte{tree.Leaves[0].Hash}, [][]*big.Int{{binding}})
	if err != nil {
		t.Fatal(err)
	}
	r.Steps[1] = append([]byte{recipes.Manage}, body...)
	for _, ready := range []bool{false, true} {
		out, err := (&Provider{&chain{t: t, ready: ready}}).PrepareRecipe(context.Background(), s, r, tree, nil)
		if ready {
			if err != nil || len(out) != 1 {
				t.Fatalf("ready async claim failed: %v", err)
			}
		} else if !errors.Is(err, provider.ErrNotReady) {
			t.Fatalf("unsettled async claim: %v", err)
		}
	}
}

func TestPinnedIDDoesNotPreventOtherReadyIDsFromBeingScanned(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(3))
	clear(tree.Leaves[0].DataMask)
	// Hash the newly fixed permission and bind the same leaf into the recipe.
	old := tree.Leaves[0].Hash
	tree.Leaves[0].Hash, _ = recipes.HashLeaf(tree.Leaves[0])
	tree.Root = tree.Leaves[0].Hash
	r.Steps[0] = bytes.ReplaceAll(r.Steps[0], old[:], tree.Root[:])
	out, err := (&Provider{&chain{t: t, ready: true}}).PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil || len(out) != 1 {
		t.Fatalf("fixed ID: %v %d", err, len(out))
	}
	if new(big.Int).SetBytes(out[0].Actions["0:0"][4:36]).Int64() != 3 {
		t.Fatal("changed fixed ID")
	}
}
