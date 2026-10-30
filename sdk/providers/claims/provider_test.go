package claims

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type testReader func(ethereum.CallMsg) ([]byte, error)

func (f testReader) CallContract(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	if b == nil || b.Int64() != 12 {
		return nil, errors.New("wrong snapshot")
	}
	return f(m)
}

type reverted struct{}

func (reverted) Error() string  { return "execution reverted" }
func (reverted) ErrorCode() int { return 3 }
func num(n int64) *big.Int      { return big.NewInt(n) }
func snapshot() provider.Snapshot {
	return provider.Snapshot{Account: common.HexToAddress("0x1234"), ChainID: num(1), BlockNumber: num(12), BlockHash: common.HexToHash("0xbeef"), Timestamp: 1000}
}
func claimRecipe(t *testing.T, signature string, words ...*big.Int) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	data := selector(signature)
	for _, n := range words {
		data = append(data, n.FillBytes(make([]byte, 32))...)
	}
	mask := make([]byte, len(data))
	if len(data) >= 36 {
		for i := 4; i < 36; i++ {
			mask[i] = 255
		}
	}
	leaf := recipes.Leaf{Target: common.HexToAddress("0x2222"), Value: num(0), Data: data, DataMask: mask}
	var err error
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Type: abiType("uint256")}, {Type: abiType("bytes32[]")}, {Type: abiType("uint256[][]")}}
	encoded, err := args.Pack(num(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})
	if err != nil {
		t.Fatal(err)
	}
	return recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, encoded...)}}, &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
}
func TestNoonClaimsEveryReadyIDIncludingZero(t *testing.T) {
	s := snapshot()
	r, tree := claimRecipe(t, "claimWithdrawal(uint256)", num(0))
	p := &Provider{reader: testReader(func(m ethereum.CallMsg) ([]byte, error) {
		if bytes.Equal(m.Data[:4], selector("claimWithdrawal(uint256)")) {
			return nil, nil
		}
		method, err := readABI.MethodById(m.Data[:4])
		if err != nil {
			return nil, err
		}
		args, _ := method.Inputs.Unpack(m.Data[4:])
		switch method.Name {
		case "getUserNextRequestId":
			return method.Outputs.Pack(num(4))
		case "withdrawPeriod":
			return method.Outputs.Pack(num(100))
		case "getWithdrawalRequest":
			id := args[1].(*big.Int).Int64()
			stamp := num(800)
			if id == 2 {
				stamp = num(999)
			}
			return method.Outputs.Pack(num(10), stamp, id == 1)
		}
		return nil, errors.New("unexpected read")
	})}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Actions) != 1 || len(out.Repeats) != 1 {
		t.Fatalf("expected ready requests 0 and 3, got %+v", out)
	}
	if new(big.Int).SetBytes(out.Actions["0:0"][4:]).Int64() != 0 || new(big.Int).SetBytes(out.Repeats[0]["0:0"][4:]).Int64() != 3 {
		t.Fatal("request IDs changed")
	}
	if len(out.Inputs) != 0 || len(out.Results) != 0 {
		t.Fatal("claim introduced numeric witnesses")
	}
}

func TestClaimRejectsMissingRequestWord(t *testing.T) {
	r, tree := claimRecipe(t, "claimWithdrawal(uint256)")
	if _, err := derive(snapshot(), r, tree); err == nil || err.Error() != "claim field exceeds calldata" {
		t.Fatalf("expected missing request word error, got %v", err)
	}
}
func TestClaimDiscoveryDistinguishesUnreadyAndFailedReads(t *testing.T) {
	s := snapshot()
	r, tree := claimRecipe(t, "claimWithdrawal(uint256)", num(0))
	for _, tc := range []struct {
		name             string
		failed, overflow bool
		want             error
	}{
		{name: "empty", want: provider.ErrNotReady}, {name: "transport", failed: true}, {name: "complete bound", overflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Provider{reader: testReader(func(m ethereum.CallMsg) ([]byte, error) {
				if tc.failed {
					return nil, errors.New("RPC offline")
				}
				method, _ := readABI.MethodById(m.Data[:4])
				n := num(0)
				if tc.overflow {
					n = num(maxScan + 1)
				}
				return method.Outputs.Pack(n)
			})}
			_, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
			if err == nil {
				t.Fatal("empty/failed discovery accepted")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}
func TestNativeClaimRejectsPinnedAmountAndSkipsProtocolReverts(t *testing.T) {
	s := snapshot()
	r, tree := claimRecipe(t, "redeem(uint256,address,address)", num(0), new(big.Int).SetBytes(s.Account[:]), new(big.Int).SetBytes(s.Account[:]))
	p := &Provider{reader: testReader(func(m ethereum.CallMsg) ([]byte, error) {
		if bytes.Equal(m.Data[:4], selector("redeem(uint256,address,address)")) {
			return nil, reverted{}
		}
		method, _ := readABI.MethodById(m.Data[:4])
		return method.Outputs.Pack(num(100))
	})}
	_, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if !errors.Is(err, provider.ErrNotReady) {
		t.Fatal(err)
	}
	rt, _ := derive(s, r, tree)
	if rt == nil {
		t.Fatal("missing route")
	}
	rt.action.Leaf.DataMask = make([]byte, len(rt.action.Leaf.Data))
	_, err = p.discover(context.Background(), s, *rt)
	if err == nil || err.Error() != "claim field is pinned by its permission" {
		t.Fatal(err)
	}
}
func TestThreeFDiscoveryIncludesTransferredOlderRequests(t *testing.T) {
	s := snapshot()
	factory := common.HexToAddress("0xfac")
	owned := crypto.CreateAddress(factory, 7)
	r, tree := claimRecipe(t, "claimWithdrawal(uint256)", num(0))
	a, _ := recipes.ManagedActions(r, tree)
	rt := route{action: a[0], kind: "threef", factory: factory, field: 4}
	p := &Provider{reader: testReader(func(m ethereum.CallMsg) ([]byte, error) {
		if bytes.Equal(m.Data[:4], selector("claimWithdrawal(uint256)")) {
			return nil, nil
		}
		method, _ := readABI.MethodById(m.Data[:4])
		args, _ := method.Inputs.Unpack(m.Data[4:])
		switch method.Name {
		case "isRequest":
			return method.Outputs.Pack(args[0].(common.Address) == owned)
		case "canWithdraw":
			return method.Outputs.Pack(true)
		case "balancesOf":
			return method.Outputs.Pack(num(1), num(0))
		}
		return nil, errors.New("unexpected read")
	})}
	// A dynamic target is encoded into the already-authenticated capability input.
	for i := 4; i < 36; i++ {
		rt.action.Leaf.DataMask[i] = 255
	}
	out, err := p.discover(context.Background(), s, rt)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || common.BytesToAddress(out[0]["0:0"][4:]) != owned {
		t.Fatal("older request excluded")
	}
}

func TestAggregateClaimsUseNativeEntitlementsAndPreserveRecipients(t *testing.T) {
	s := snapshot()
	owner := new(big.Int).SetBytes(s.Account[:])
	for _, tc := range []struct {
		signature       string
		words           []*big.Int
		ledger, maximum string
		amount          bool
	}{
		{"mint(uint256,address,address)", []*big.Int{num(0), owner, owner}, "claimableDepositRequest", "maxMint", true},
		{"deposit(uint256,address)", []*big.Int{num(0), owner}, "claimableDepositRequest", "maxDeposit", true},
		{"redeem(uint256,address,address)", []*big.Int{num(0), owner, owner}, "claimableRedeemRequest", "maxRedeem", true},
		{"withdraw(uint256,address,address)", []*big.Int{num(0), owner, owner}, "claimableRedeemRequest", "maxWithdraw", true},
		{"claimCancelDepositRequest(uint256,address,address)", []*big.Int{num(0), owner, owner}, "claimableCancelDepositRequest", "", false},
		{"claimCancelRedeemRequest(uint256,address,address)", []*big.Int{num(0), owner, owner}, "claimableCancelRedeemRequest", "", false},
		{"claimDepositEscrow(address,address)", []*big.Int{owner, num(99)}, "depositEscrowBalance", "", false},
		{"claimRedeemEscrow(address)", []*big.Int{owner}, "redeemEscrowBalance", "", false},
	} {
		t.Run(tc.signature, func(t *testing.T) {
			r, tree := claimRecipe(t, tc.signature, tc.words...)
			simulated := false
			p := &Provider{reader: testReader(func(m ethereum.CallMsg) ([]byte, error) {
				if bytes.Equal(m.Data[:4], selector(tc.signature)) {
					simulated = true
					if m.From != s.Account || !bytes.Equal(m.Data[36:], tree.Leaves[0].Data[36:]) {
						t.Fatal("claim changed its caller or fixed recipients")
					}
					if tc.amount && new(big.Int).SetBytes(m.Data[4:36]).Cmp(num(17)) != 0 {
						t.Fatal("claim did not use the native maximum")
					}
					return nil, nil
				}
				method, err := readABI.MethodById(m.Data[:4])
				if err != nil {
					return nil, err
				}
				if method.Name != tc.ledger && method.Name != tc.maximum {
					t.Fatalf("unexpected claim read %s", method.Name)
				}
				args, err := method.Inputs.Unpack(m.Data[4:])
				if err != nil {
					return nil, err
				}
				found := false
				for _, arg := range args {
					if address, ok := arg.(common.Address); ok && address == s.Account {
						found = true
					}
				}
				if !found {
					t.Fatal("entitlement not scoped to account")
				}
				return method.Outputs.Pack(num(17))
			})}
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !simulated || len(out.Actions) != 1 || len(out.Repeats) != 0 {
				t.Fatal("claim was not prepared and simulated")
			}
		})
	}
}

func TestThreeFClaimCapabilityLocatesOnlyRequestInput(t *testing.T) {
	factory := common.HexToAddress("0xfac")
	request := common.HexToAddress("0xbeef")
	query, err := queryArgs.Pack(factory, append(selector("isRequest(address)"), make([]byte, 32)...), []*big.Int{}, num(0))
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := capabilityArgs.Pack([][]byte{query}, [][]byte{{recipes.Read}, {recipes.Equal}, {recipes.Call}}, []*big.Int{new(big.Int).SetBytes(request[:])})
	if err != nil {
		t.Fatal(err)
	}
	data := append(selector("execute(bytes[],bytes[],uint256[])"), arguments...)
	found, offset, err := threeFLayout(data)
	if err != nil || found != factory {
		t.Fatalf("factory=%s error=%v", found, err)
	}
	if common.BytesToAddress(data[offset:offset+32]) != request {
		t.Fatal("wrong mutable request word")
	}
	changed := bytes.Clone(data)
	num(42).FillBytes(changed[offset : offset+32])
	values, err := capabilityArgs.Unpack(changed[4:])
	if err != nil {
		t.Fatal(err)
	}
	if values[2].([]*big.Int)[0].Cmp(num(42)) != 0 || !bytes.Equal(values[0].([][]byte)[0], query) {
		t.Fatal("request patch changed the factory query")
	}
}
