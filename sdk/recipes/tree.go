package recipes

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
)

type leafJSON struct {
	Hash     string   `json:"hash"`
	Target   string   `json:"target"`
	Value    string   `json:"value"`
	Data     string   `json:"data"`
	DataMask string   `json:"dataMask"`
	Proof    []string `json:"proof"`
}

// LoadTree accepts both the Solidity/frontend leaves artifact and the permission
// service calls artifact. Display metadata is ignored; all cryptographic data is
// verified, including unused leaves. Proofs may describe a subset of the root.
func LoadTree(path string) (*Tree, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Errorf("open tree: %w", err)
	}
	defer file.Close()
	const maxTreeBytes = 64 << 20
	data, err := io.ReadAll(io.LimitReader(file, maxTreeBytes+1))
	if err != nil {
		return nil, errors.Errorf("read tree: %w", err)
	}
	if len(data) > maxTreeBytes {
		return nil, errors.New("tree exceeds 64 MiB")
	}
	return DecodeTree(data)
}

func DecodeTree(data []byte) (*Tree, error) {
	var document struct {
		Root    string          `json:"root"`
		ChainID json.RawMessage `json:"chainId"`
		Account *string         `json:"account"`
		Leaves  *[]leafJSON     `json:"leaves"`
		Calls   *[]leafJSON     `json:"calls"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, errors.Errorf("decode tree: %w", err)
	}
	tree := new(Tree)
	var err error
	if tree.Root, err = decodeHash(document.Root); err != nil {
		return nil, errors.Errorf("tree root: %w", err)
	}
	if len(document.ChainID) > 0 && !bytes.Equal(document.ChainID, []byte("null")) {
		value := string(document.ChainID)
		if document.ChainID[0] == '"' {
			if err = json.Unmarshal(document.ChainID, &value); err != nil {
				return nil, errors.Errorf("chain ID: %w", err)
			}
		}
		tree.ChainID, err = parse.Uint256(value)
		if err != nil || tree.ChainID.Sign() == 0 {
			return nil, errors.New("invalid tree chain ID")
		}
	}
	if document.Account != nil {
		value, err := parse.Address(*document.Account)
		if err != nil {
			return nil, errors.Errorf("tree account: %w", err)
		}
		tree.Account = &value
	}
	if (document.Leaves == nil) == (document.Calls == nil) {
		return nil, errors.New("tree requires exactly one of leaves or calls")
	}
	entries := document.Leaves
	if entries == nil {
		entries = document.Calls
	}
	for index, entry := range *entries {
		leaf, err := decodeLeaf(entry)
		if err != nil {
			return nil, errors.Errorf("leaf %d: %w", index, err)
		}
		derived, err := HashLeaf(leaf)
		if err != nil {
			return nil, errors.Errorf("leaf %d: %w", index, err)
		}
		if derived != leaf.Hash {
			return nil, errors.Errorf("leaf %d hash mismatch", index)
		}
		if !Proof(tree.Root, leaf.Hash, leaf.Proof) {
			return nil, errors.Errorf("leaf %d proof does not match root", index)
		}
		tree.Leaves = append(tree.Leaves, leaf)
	}
	return tree, nil
}

func decodeLeaf(entry leafJSON) (Leaf, error) {
	var leaf Leaf
	var err error
	if leaf.Hash, err = decodeHash(entry.Hash); err != nil {
		return leaf, errors.Errorf("hash: %w", err)
	}
	if !common.IsHexAddress(entry.Target) {
		return leaf, errors.New("invalid target address")
	}
	leaf.Target = common.HexToAddress(entry.Target)
	value := entry.Value
	if value == "zero" {
		value = "0"
	}
	if leaf.Value, err = parse.Uint256(value); err != nil || leaf.Value.BitLen() > 96 {
		return leaf, errors.New("value does not fit uint96")
	}
	if leaf.Data, err = hexutil.Decode(entry.Data); err != nil {
		return leaf, errors.Errorf("calldata: %w", err)
	}
	if leaf.DataMask, err = hexutil.Decode(entry.DataMask); err != nil {
		return leaf, errors.Errorf("data mask: %w", err)
	}
	for _, text := range entry.Proof {
		hash, err := decodeHash(text)
		if err != nil {
			return leaf, errors.Errorf("proof: %w", err)
		}
		leaf.Proof = append(leaf.Proof, hash)
	}
	return leaf, nil
}

func decodeHash(value string) (common.Hash, error) {
	data, err := hexutil.Decode(value)
	if err != nil || len(data) != 32 {
		return common.Hash{}, errors.New("expected a 32-byte hexadecimal hash")
	}
	return common.BytesToHash(data), nil
}

func (tree *Tree) lookup(hash common.Hash) (Leaf, error) {
	if tree == nil {
		return Leaf{}, errors.New("permission tree required")
	}
	for _, leaf := range tree.Leaves {
		if leaf.Hash == hash {
			derived, err := HashLeaf(leaf)
			if err != nil {
				return Leaf{}, err
			}
			if derived != hash || !Proof(tree.Root, hash, leaf.Proof) {
				return Leaf{}, errors.New("invalid leaf artifact")
			}
			if leaf.Value.Cmp(new(big.Int)) != 0 {
				return Leaf{}, errors.New("recipe managed actions cannot carry native value")
			}
			return leaf, nil
		}
	}
	return Leaf{}, errors.Errorf("leaf %s is absent from permission tree", hash)
}
