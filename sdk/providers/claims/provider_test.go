package claims

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/nativeclaims"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type chain struct {
	t           *testing.T
	fail        bool
	ready       bool
	maximum     *big.Int
	synchronous bool
	rejectClaim bool
	failMaximum bool
	malformed   string
	failClaim   bool
	readError   error
}

func (c *chain) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	c.t.Helper()
	if block == nil || block.Int64() != 12 {
		c.t.Fatal("read is not pinned")
	}
	if c.readError != nil {
		return nil, c.readError
	}
	if c.fail {
		return nil, errors.New("RPC unavailable")
	}
	m, err := contractABI.MethodById(msg.Data[:4])
	if err != nil {
		return nil, err
	}
	if c.malformed == m.Name {
		return []byte{1}, nil
	}
	args, err := m.Inputs.Unpack(msg.Data[4:])
	if err != nil {
		return nil, err
	}
	n := int64(0)
	switch m.Name {
	case "deposit", "mint", "withdraw", "redeem", "claimWithdrawal", "claimDepositRequest", "claimWithdrawRequest0", "claimWithdrawRequest", "claimInstantWithdrawRequest", "claimDepositEscrow", "claimRedeemEscrow", "claimCancelDepositRequest", "claimCancelRedeemRequest":
		if msg.From != common.HexToAddress("0xa1") {
			c.t.Fatal("native preflight sender differs from account")
		}
		if c.failClaim {
			return nil, errors.New("RPC unavailable during claim preflight")
		}
		if c.rejectClaim {
			return nil, errors.New("execution reverted: unavailable")
		}
		return nil, nil
	case "maxDeposit", "maxMint", "maxWithdraw", "maxRedeem":
		if c.failMaximum {
			return nil, errors.New("execution reverted: broken maximum")
		}
		if c.maximum != nil {
			return m.Outputs.Pack(c.maximum)
		}
		return m.Outputs.Pack(big.NewInt(100))
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
		return m.Outputs.Pack(nativeclaims.Struct0{Amount: big.NewInt(amount), Timestamp: big.NewInt(timestamp), Claimed: claimed})
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
		if c.synchronous {
			return nil, errors.New("execution reverted: unsupported selector")
		}
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
	case "deposit", "mint", "withdraw", "redeem", "claimWithdrawal", "claimDepositRequest", "claimWithdrawRequestForEpoch":
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
func TestSelectsReadyNoonIDsAsIndependentStatelessOutputs(t *testing.T) {
	s, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	originalRecipe := bytes.Clone(r.Steps[0])
	originalPermission := bytes.Clone(tree.Leaves[0].Data)
	p := &Provider{&chain{t: t, ready: true}}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("want two standalone claims, got %d", len(out))
	}
	for i, id := range []int64{0, 3} {
		execution := out[i]
		if execution.Task != nil || len(execution.Data) != 0 || len(execution.Inputs)+len(execution.Results)+len(execution.SkipSteps) != 0 || len(execution.Actions) != 1 {
			t.Fatalf("claim retained settlement or unauthorized state: %+v", execution)
		}
		if new(big.Int).SetBytes(execution.Actions["0:0"][4:36]).Int64() != id {
			t.Fatal("wrong ready ID")
		}
	}
	clear(out[0].Actions["0:0"])
	if new(big.Int).SetBytes(out[1].Actions["0:0"][4:36]).Int64() != 3 || !bytes.Equal(tree.Leaves[0].Data, originalPermission) || !bytes.Equal(r.Steps[0], originalRecipe) {
		t.Fatal("prepared claim aliases a sibling, permission or recipe")
	}
}
func TestOnlySingleUnconditionalClaimMatches(t *testing.T) {
	_, r, tree := fixture(t, "claimWithdrawal", big.NewInt(0))
	r.Steps = append(r.Steps, r.Steps[0])
	if _, ok, err := match(r, tree); err != nil || ok {
		t.Fatalf("multi-claim recipe matched %v", err)
	}
}

func TestAsyncClaimsSupplyNativeMaximumWithoutRecipeReads(t *testing.T) {
	account := common.HexToAddress("0xa1")
	for _, method := range []string{"deposit", "mint", "withdraw", "redeem"} {
		t.Run(method, func(t *testing.T) {
			args := []any{big.NewInt(0), account}
			if method == "withdraw" || method == "redeem" {
				args = append(args, account)
			}
			s, r, tree := fixture(t, method, args...)
			if len(r.Queries) != 0 || len(r.Steps) != 1 {
				t.Fatal("claim recipe contains reads")
			}
			p := &Provider{&chain{t: t, ready: true, maximum: big.NewInt(75)}}
			req, err := p.RecipeRequirements(s, r, tree)
			if err != nil || len(req.Values) != 0 || len(req.Inputs) != 0 || len(req.Actions) != 1 {
				t.Fatalf("claim must require calldata only: %+v %v", req, err)
			}
			out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
			if err != nil || len(out) != 1 {
				t.Fatalf("ready async claim: %v", err)
			}
			data := out[0].Actions["0:0"]
			if new(big.Int).SetBytes(data[4:36]).Cmp(big.NewInt(75)) != 0 {
				t.Fatal("did not use native maximum")
			}
			if !bytes.Equal(data[36:], tree.Leaves[0].Data[36:]) || new(big.Int).SetBytes(tree.Leaves[0].Data[4:36]).Sign() != 0 {
				t.Fatal("changed recipient/controller or permission")
			}
		})
	}
}

func TestAsyncClaimsStayIdleWithoutEntitlementOrCapacity(t *testing.T) {
	account := common.HexToAddress("0xa1")
	s, r, tree := fixture(t, "withdraw", big.NewInt(0), account, account)
	for _, state := range []chain{
		{t: t},
		{t: t, ready: true, maximum: new(big.Int)},
		{t: t, ready: true, rejectClaim: true},
	} {
		out, err := (&Provider{&state}).PrepareRecipe(context.Background(), s, r, tree, nil)
		if !errors.Is(err, provider.ErrNotReady) || len(out) != 0 {
			t.Fatalf("unavailable claim: %v %v", out, err)
		}
	}
}

func TestBrokenNativeMaximumFailsPreparation(t *testing.T) {
	account := common.HexToAddress("0xa1")
	s, r, tree := fixture(t, "withdraw", big.NewInt(0), account, account)
	_, err := (&Provider{&chain{t: t, ready: true, failMaximum: true}}).PrepareRecipe(context.Background(), s, r, tree, nil)
	if err == nil || errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("broken maximum was treated as empty: %v", err)
	}
}

func TestOrdinarySynchronousOperationIsNotPreparedAsAClaim(t *testing.T) {
	account := common.HexToAddress("0xa1")
	other := common.HexToAddress("0xbad")
	for _, tc := range []struct {
		method   string
		receiver common.Address
		owner    common.Address
	}{
		{"deposit", other, common.Address{}},
		{"mint", other, common.Address{}},
		{"withdraw", account, account},
		{"withdraw", other, account},
		{"withdraw", account, other},
		{"redeem", other, account},
		{"redeem", account, other},
	} {
		t.Run(tc.method+"/"+tc.receiver.Hex()+"/"+tc.owner.Hex(), func(t *testing.T) {
			args := []any{new(big.Int), tc.receiver}
			if tc.method == "withdraw" || tc.method == "redeem" {
				args = append(args, tc.owner)
			}
			s, r, tree := fixture(t, tc.method, args...)
			for _, synchronous := range []bool{true, false} {
				out, err := (&Provider{&chain{t: t, ready: true, synchronous: synchronous}}).PrepareRecipe(t.Context(), s, r, tree, nil)
				if !synchronous && (tc.receiver != account || tc.owner != (common.Address{}) && tc.owner != account) {
					if err == nil || err.Error() != "claim recipient or controller is not the account" {
						t.Fatalf("async claim accepted a foreign receiver/controller: %+v %v", out, err)
					}
					continue
				}
				if synchronous {
					if err != nil || len(out) != 1 {
						t.Fatalf("unmatched synchronous call: %v", err)
					}
					if len(out[0].Actions)+len(out[0].Inputs)+len(out[0].Results)+len(out[0].SkipSteps)+len(out[0].Data) != 0 || out[0].Task != nil {
						t.Fatalf("synchronous call became a claim: %+v", out[0])
					}
				} else if err != nil || len(out) != 1 || len(out[0].Actions) != 1 {
					t.Fatalf("authorized async claim was lost: %+v %v", out, err)
				}
			}
		})
	}
}

func TestClaimReadPreservesContextCancellationCause(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			s, r, tree := fixture(t, "claimWithdrawal", new(big.Int))
			p := &Provider{&chain{t: t, readError: errors.Errorf("transport interrupted: %w", cause)}}
			_, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			if !errors.Is(err, cause) {
				t.Fatalf("claim read lost cancellation cause: %v", err)
			}
		})
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

func TestFixedNativeAmountMustMatchMaximum(t *testing.T) {
	account := common.HexToAddress("0xa1")
	for _, amount := range []int64{0, 74, 75, 100} {
		t.Run(big.NewInt(amount).String(), func(t *testing.T) {
			s, r, tree := fixture(t, "redeem", big.NewInt(amount), account, account)
			clear(tree.Leaves[0].DataMask)
			old := tree.Leaves[0].Hash
			tree.Leaves[0].Hash, _ = recipes.HashLeaf(tree.Leaves[0])
			tree.Root = tree.Leaves[0].Hash
			r.Steps[0] = bytes.ReplaceAll(r.Steps[0], old[:], tree.Root[:])
			out, err := (&Provider{&chain{t: t, ready: true, maximum: big.NewInt(75)}}).PrepareRecipe(t.Context(), s, r, tree, nil)
			if amount != 75 {
				if !errors.Is(err, provider.ErrNotReady) || len(out) != 0 {
					t.Fatalf("fixed amount changed to native maximum: %+v %v", out, err)
				}
				return
			}
			if err != nil || len(out) != 1 || !bytes.Equal(out[0].Actions["0:0"], tree.Leaves[0].Data) || out[0].Task != nil {
				t.Fatalf("matching fixed amount was not preserved: %+v %v", out, err)
			}
		})
	}
}

func TestMalformedReadsAndPreflightTransportFailuresAreErrors(t *testing.T) {
	account := common.HexToAddress("0xa1")
	for _, method := range []string{"getUserNextRequestId", "getWithdrawalRequest", "claimableRedeemRequest", "maxRedeem", "preflight"} {
		t.Run(method, func(t *testing.T) {
			s, r, tree := fixture(t, "claimWithdrawal", new(big.Int))
			if strings.Contains(method, "Redeem") {
				s, r, tree = fixture(t, "redeem", new(big.Int), account, account)
			}
			state := &chain{t: t, ready: true, malformed: method, failClaim: method == "preflight"}
			out, err := (&Provider{state}).PrepareRecipe(t.Context(), s, r, tree, nil)
			if err == nil || errors.Is(err, provider.ErrNotReady) || len(out) != 0 {
				t.Fatalf("invalid claim evidence became idle or usable: %+v %v", out, err)
			}
		})
	}
}
