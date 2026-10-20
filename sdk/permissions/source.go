// Package permissions retrieves root-addressed permission artifacts for recipe validation.
package permissions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Default resource bounds for the daemon's permission service.
const HostTimeoutSeconds = 10
const HostMaxResponseBytes int64 = 2 << 20

// ValidateConfig performs no network or environment access. Construction with
// New resolves credentials through the host only when a run/plan actually starts.
func ValidateConfig(c protocolio.HTTPConfig) error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("curator.url must be an HTTP(S) base URL without credentials, query or fragment")
	}
	if c.TimeoutSeconds < 0 || c.TimeoutSeconds > 120 {
		return errors.New("curator.timeoutSeconds must be 0..120")
	}
	if c.MaxResponseBytes < 0 || c.MaxResponseBytes > 16<<20 {
		return errors.New("curator.maxResponseBytes must be 0..16777216")
	}
	return nil
}

type Source struct {
	client *protocolio.Client
	chain  provider.Reader
}

func New(c protocolio.HTTPConfig, h *http.Client, chain provider.Reader, lookup func(string) string) (*Source, error) {
	if err := ValidateConfig(c); err != nil {
		return nil, err
	}
	client, err := protocolio.NewHTTP(c, h, lookup)
	if err != nil {
		return nil, err
	}
	if chain == nil {
		return nil, errors.New("permission chain reader required")
	}
	return &Source{client: client, chain: chain}, nil
}
func (s *Source) PermissionTree(ctx context.Context, chainID *big.Int, snapshot recipes.Snapshot) ([]byte, error) {
	if chainID == nil || chainID.Sign() <= 0 || chainID.BitLen() > 256 {
		return nil, errors.New("permission chain ID required")
	}
	binding := merkleaccount.NewMerkleAccount()
	raw, err := protocolio.CallData(ctx, s.chain, provider.Snapshot{ChainID: chainID, Account: snapshot.Account, BlockNumber: new(big.Int).SetUint64(snapshot.BlockNumber)}, snapshot.Account, binding.PackAdapter())
	if err != nil {
		return nil, errors.Errorf("read permission account adapter: %w", err)
	}
	adapter, err := binding.UnpackAdapter(raw)
	if err != nil || len(raw) != 32 || !bytes.Equal(raw[:12], make([]byte, 12)) || adapter == (common.Address{}) {
		return nil, errors.New("invalid permission account adapter")
	}
	var response struct {
		Bundle struct {
			treeScope
			Leaves   []common.Hash   `json:"leaves"`
			Metadata json.RawMessage `json:"metadata"`
		} `json:"bundle"`
	}
	_, err = s.client.JSON(ctx, http.MethodGet, "/merkle/"+strings.ToLower(adapter.Hex())+"/"+snapshot.Root.Hex(), nil, &response)
	if err != nil {
		return nil, err
	}
	bundle := response.Bundle
	if err = bundle.validate(chainID, adapter, snapshot); err != nil {
		return nil, errors.Errorf("permission bundle: %w", err)
	}
	var metadata treeScope
	if err = json.Unmarshal(bundle.Metadata, &metadata); err != nil {
		return nil, errors.New("invalid permission metadata")
	}
	if err = metadata.validate(chainID, adapter, snapshot); err != nil {
		return nil, errors.Errorf("permission metadata: %w", err)
	}
	tree, err := recipes.DecodeTree(bundle.Metadata)
	if err != nil {
		return nil, errors.Errorf("permission metadata tree: %w", err)
	}
	if err = validateLeaves(bundle.Leaves, snapshot.Root, tree.Leaves); err != nil {
		return nil, err
	}
	return bundle.Metadata, nil
}

type treeScope struct {
	ChainID json.Number `json:"chainId"`
	Adapter string      `json:"adapter"`
	Account string      `json:"account"`
	Root    common.Hash `json:"root"`
}

func (scope treeScope) validate(chainID *big.Int, adapter common.Address, snapshot recipes.Snapshot) error {
	id, err := parse.Uint256(string(scope.ChainID))
	if err != nil || id.Cmp(chainID) != 0 {
		return errors.New("chain mismatch")
	}
	a, err := parse.Address(scope.Adapter)
	if err != nil || a != adapter {
		return errors.New("adapter mismatch")
	}
	account, err := parse.Address(scope.Account)
	if err != nil || account != snapshot.Account {
		return errors.New("account mismatch")
	}
	if scope.Root != snapshot.Root {
		return errors.New("root mismatch")
	}
	return nil
}

// Bundles may include API policy leaves in addition to executable call leaves.
func validateLeaves(leaves []common.Hash, root common.Hash, calls []recipes.Leaf) error {
	if len(leaves) == 0 || len(leaves) > 4096 {
		return errors.New("permission bundle requires 1..4096 leaves")
	}
	known := make(map[common.Hash]bool, len(leaves))
	tree := make([]common.Hash, 2*len(leaves)-1)
	for i, leaf := range leaves {
		if i > 0 && leaves[i-1].Cmp(leaf) >= 0 {
			return errors.New("permission bundle leaves must be sorted and unique")
		}
		known[leaf] = true
		tree[len(tree)-1-i] = leaf
	}
	for i := len(leaves) - 2; i >= 0; i-- {
		a, b := tree[2*i+1], tree[2*i+2]
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		tree[i] = crypto.Keccak256Hash(a[:], b[:])
	}
	if tree[0] != root {
		return errors.New("permission bundle leaves do not match root")
	}
	for _, call := range calls {
		if !known[call.Hash] {
			return errors.New("permission metadata call is absent from bundle leaves")
		}
	}
	return nil
}
