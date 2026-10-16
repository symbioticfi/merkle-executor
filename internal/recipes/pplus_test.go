package recipes

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	account "github.com/symbioticfi/merkle-executor/api/bindings/merkleaccount"
)

// PPLUS requires only native calls committed by the frontend, not a keeper provider.
func TestPplusFrontendNativeCallsUseGenericAssembly(t *testing.T) {
	tree, err := LoadTree("testdata/pplus-frontend.json")
	if err != nil {
		t.Fatal(err)
	}
	pplus := common.HexToAddress("0xbc5d3722376d44f3bc316c6ea61c9bd553be8cbe")
	usdc := common.HexToAddress("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
	find := func(target common.Address, signature string) Leaf {
		t.Helper()
		selector := crypto.Keccak256([]byte(signature))[:4]
		for _, leaf := range tree.Leaves {
			if leaf.Target == target && bytes.Equal(leaf.Data[:4], selector) {
				return leaf
			}
		}
		t.Fatalf("missing %s", signature)
		return Leaf{}
	}
	for _, tc := range []struct {
		name, signature string
		token           common.Address
		amount          int64
	}{
		{"deposit", "deposit(uint256,address)", usdc, 1000000},
		{"queued request", "redeemShares(uint256,address)", pplus, 100000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			approval := find(tc.token, "approve(address,uint256)")
			native := find(pplus, tc.signature)
			if common.BytesToAddress(approval.Data[4:36]) != pplus || common.BytesToAddress(native.Data[36:68]) != *tree.Account {
				t.Fatal("frontend changed fixed spender/receiver")
			}
			recipe := Recipe{Steps: []Step{
				{Operation: Approve, Data: pack(t, approveArgs, input(0), tc.token, pplus, approval.Hash)},
				manageStep(t, native.Hash, literal(1), []Binding{{Offset: big.NewInt(4), Value: input(0)}}),
				{Operation: Approve, Data: pack(t, approveArgs, literal(0), tc.token, pplus, approval.Hash)},
			}}
			replacement := bytes.Clone(native.Data)
			big.NewInt(tc.amount).FillBytes(replacement[4:36])
			data, used, err := Assemble(recipe, tree, map[uint]string{0: big.NewInt(tc.amount).String()}, map[string]hexutil.Bytes{"1:0": replacement}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(used) != 2 {
				t.Fatal("approval/native leaf coverage changed")
			}
			decoded, err := executorABI.GetABI().Methods["execute"].Inputs.Unpack(data[4:])
			if err != nil {
				t.Fatal(err)
			}
			if decoded[2].([]*big.Int)[0].Int64() != tc.amount {
				t.Fatal("input changed")
			}
			runtime := decoded[3].([][]byte)
			for _, index := range []int{0, 2} {
				permission, err := permissionArgs.Unpack(runtime[index])
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(permission[0].([]byte), approval.DataMask) {
					t.Fatal("approval mask changed")
				}
			}
			unpacked, err := accountABI.GetABI().Methods["manage"].Inputs.Unpack(runtime[1])
			if err != nil {
				t.Fatal(err)
			}
			actions := *abi.ConvertType(unpacked[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
			if len(actions) != 1 || actions[0].Target != pplus || !bytes.Equal(actions[0].Data, replacement) || !bytes.Equal(actions[0].DataMask, native.DataMask) {
				t.Fatal("native permission changed")
			}
			if !Proof(tree.Root, native.Hash, native.Proof) {
				t.Fatal("native proof changed")
			}
			replacement[67] ^= 1
			if _, _, err = Assemble(recipe, tree, map[uint]string{0: "1"}, map[string]hexutil.Bytes{"1:0": replacement}, nil); err == nil {
				t.Fatal("foreign receiver accepted")
			}
			broken := *tree
			broken.Leaves = append([]Leaf(nil), tree.Leaves...)
			for i := range broken.Leaves {
				if broken.Leaves[i].Hash == native.Hash {
					broken.Leaves[i].DataMask = bytes.Clone(native.DataMask)
					broken.Leaves[i].DataMask[67] = 0xff
				}
			}
			if _, _, err = Assemble(recipe, &broken, map[uint]string{0: "1"}, nil, nil); err == nil {
				t.Fatal("loosened receiver mask accepted")
			}
			broken = *tree
			broken.Leaves = append([]Leaf(nil), tree.Leaves...)
			for i := range broken.Leaves {
				if broken.Leaves[i].Hash == native.Hash {
					broken.Leaves[i].Proof = []common.Hash{{}}
				}
			}
			if _, _, err = Assemble(recipe, &broken, map[uint]string{0: "1"}, nil, nil); err == nil {
				t.Fatal("invalid native proof accepted")
			}
			recipe.Steps[0].Data = pack(t, approveArgs, input(0), tc.token, usdc, approval.Hash)
			if _, _, err = Assemble(recipe, tree, map[uint]string{0: "1"}, nil, nil); err == nil {
				t.Fatal("foreign approval spender accepted")
			}
		})
	}
}
