package recipes

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestContractFixture(t *testing.T) {
	tree, err := LoadTree("testdata/merkle-tree.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Leaves) != 33 {
		t.Fatalf("leaves: %d", len(tree.Leaves))
	}
	for _, leaf := range tree.Leaves {
		hash, err := HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		if hash != leaf.Hash || !Proof(tree.Root, hash, leaf.Proof) {
			t.Fatal("Solidity fixture mismatch")
		}
		changed := leaf
		changed.Data = bytes.Clone(leaf.Data)
		changed.Data[0] ^= 1
		hash, err = HashLeaf(changed)
		if err != nil {
			t.Fatal(err)
		}
		if hash == leaf.Hash {
			t.Fatal("selector mutation preserved leaf")
		}
	}
}

func TestAssembleEmpty(t *testing.T) {
	data, leaves, err := Assemble(Recipe{}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 260 || len(leaves) != 0 {
		t.Fatalf("empty execute: %d bytes / %d leaves", len(data), len(leaves))
	}
	if !bytes.Equal(data[:4], common.FromHex("0x66036838")) {
		t.Fatalf("execute selector %x", data[:4])
	}
	if _, _, err := Assemble(Recipe{}, nil, map[uint]string{0: "1"}, nil, nil); err == nil {
		t.Fatal("unused input accepted")
	}
	if _, _, err := Assemble(Recipe{}, nil, nil, map[string]hexutil.Bytes{"0:0": {}}, nil); err == nil {
		t.Fatal("unused action accepted")
	}
	if _, _, err := Assemble(Recipe{}, nil, nil, nil, []uint{0}); err == nil {
		t.Fatal("unknown skip accepted")
	}
}

func TestHashLeafRejectsInvalidNativeValue(t *testing.T) {
	for _, value := range []*big.Int{nil, big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 96)} {
		if _, err := HashLeaf(Leaf{Value: value}); err == nil {
			t.Fatal("invalid uint96 accepted")
		}
	}
}
