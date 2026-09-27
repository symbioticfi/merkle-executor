// Package merkl prepares cumulative Merkl rewards for an account self-claim.
package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	"github.com/symbioticfi/merkle-executor/api/protocolio"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/parse"
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

type Config struct {
	HTTP                protocolio.HTTPConfig `yaml:"http"`
	Distributor         string                `yaml:"distributor"`
	Tokens              []string              `yaml:"tokens"`
	ProofLengths        []int                 `yaml:"proofLengths"`
	Action              string                `yaml:"action"`
	CalldataBytes       int                   `yaml:"calldataBytes"`
	MinClaimable        string                `yaml:"minClaimable"`
	AmountInputs        []uint                `yaml:"amountInputs"`
	ClaimableInputs     []uint                `yaml:"claimableInputs"`
	SkipIfEmpty         bool                  `yaml:"skipIfEmpty"`
	DedicatedClaimGroup bool                  `yaml:"dedicatedClaimGroup"`
}
type Client struct{ http *protocolio.Client }

func NewClient(c protocolio.HTTPConfig, h *http.Client) (*Client, error) {
	v, e := protocolio.NewHTTP(c, h)
	if e != nil {
		return nil, e
	}
	return &Client{http: v}, nil
}
func (c *Client) Rewards(ctx context.Context, account common.Address, chainID *big.Int) ([]wire.MerklEntry, error) {
	var v []wire.MerklEntry
	_, e := c.http.JSON(ctx, http.MethodGet, "/v4/users/"+strings.ToLower(account.Hex())+"/rewards?chainId="+chainID.String(), nil, &v)
	if e != nil {
		return nil, e
	}
	if v == nil {
		return nil, errors.New("Merkl response must be a rewards array")
	}
	for _, entry := range v {
		if entry.Chain.ID <= 0 || entry.Rewards == nil {
			return nil, errors.New("Merkl malformed chain entry")
		}
	}
	return v, nil
}

type Provider struct {
	cfg         Config
	deps        provider.Deps
	client      *Client
	distributor common.Address
	tokens      []common.Address
	minimum     *big.Int
	claimStep   uint
}

func Register(r *provider.Registry) error { return r.Register("merkl", Factory) }
func Factory(raw yaml.Node, deps provider.Deps) (provider.Provider, error) {
	var c Config
	if e := parse.DecodeStrict(raw, &c); e != nil {
		return nil, e
	}
	if e := protocolio.Action(c.Action); e != nil {
		return nil, e
	}
	if len(c.Tokens) == 0 || len(c.Tokens) > 32 || len(c.ProofLengths) != len(c.Tokens) {
		return nil, errors.New("Merkl requires 1..32 tokens and an exact proofLengths entry per token")
	}
	if c.CalldataBytes < 4 || c.CalldataBytes > 131072 {
		return nil, errors.New("Merkl calldataBytes outside 4..131072")
	}
	if len(c.AmountInputs) != 0 && len(c.AmountInputs) != len(c.Tokens) || len(c.ClaimableInputs) != 0 && len(c.ClaimableInputs) != len(c.Tokens) {
		return nil, errors.New("Merkl input indexes must be empty or match tokens length")
	}
	seenInputs := map[uint]bool{}
	for _, indices := range [][]uint{c.AmountInputs, c.ClaimableInputs} {
		for _, i := range indices {
			if seenInputs[i] {
				return nil, errors.New("Merkl duplicate input index")
			}
			seenInputs[i] = true
		}
	}
	p := &Provider{cfg: c, deps: deps}
	if c.SkipIfEmpty {
		parts := strings.Split(c.Action, ":")
		if len(c.Tokens) != 1 || parts[1] != "0" || !c.DedicatedClaimGroup {
			return nil, errors.New("Merkl skipIfEmpty requires one token, action index 0 and dedicatedClaimGroup acknowledgement")
		}
		step, e := strconv.ParseUint(parts[0], 10, strconv.IntSize)
		if e != nil {
			return nil, errors.New("Merkl skip step exceeds uint range")
		}
		p.claimStep = uint(step)
	}
	var e error
	p.distributor, e = parse.Address(c.Distributor)
	if e != nil {
		return nil, e
	}
	seen := map[common.Address]bool{}
	for i, raw := range c.Tokens {
		t, e := parse.Address(raw)
		if e != nil {
			return nil, e
		}
		if seen[t] {
			return nil, errors.New("Merkl duplicate configured token")
		}
		seen[t] = true
		p.tokens = append(p.tokens, t)
		if c.ProofLengths[i] < 0 || c.ProofLengths[i] > 64 {
			return nil, errors.New("Merkl proof length outside 0..64")
		}
	}
	if c.SkipIfEmpty {
		// Validate the configured ABI shape even when no reward row is available.
		payload, e := binding.NewMerklDistributor().TryPackClaim([]common.Address{{}}, p.tokens, []*big.Int{new(big.Int)}, [][][32]byte{make([][32]byte, c.ProofLengths[0])})
		if e != nil || len(payload) != c.CalldataBytes {
			return nil, errors.New("Merkl calldataBytes does not match configured single-claim layout")
		}
	}
	if c.MinClaimable == "" {
		c.MinClaimable = "1"
	}
	p.minimum, e = parse.Uint256(c.MinClaimable)
	if e != nil || p.minimum.Sign() == 0 {
		return nil, errors.New("Merkl minClaimable must be positive uint256")
	}
	p.client, e = NewClient(c.HTTP, deps.HTTP)
	if e != nil {
		return nil, e
	}
	if deps.Chain == nil {
		return nil, errors.New("Merkl chain reader required")
	}
	return p, nil
}
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if e := protocolio.Snapshot(s); e != nil {
		return provider.Output{}, e
	}
	if !s.ChainID.IsInt64() {
		return provider.Output{}, errors.New("Merkl API chain ID exceeds int64")
	}
	entries, e := p.client.Rewards(ctx, s.Account, s.ChainID)
	if e != nil {
		return provider.Output{}, e
	}
	var rewards []wire.MerklReward
	found := false
	for _, entry := range entries {
		if entry.Chain.ID == s.ChainID.Int64() {
			if found {
				return provider.Output{}, errors.New("Merkl response contains duplicate chain entries")
			}
			found = true
			rewards = entry.Rewards
		}
	}
	if !found {
		return p.empty("Merkl response has no configured chain")
	}
	selected := map[common.Address]wire.MerklReward{}
	wanted := map[common.Address]bool{}
	for _, t := range p.tokens {
		wanted[t] = true
	}
	for _, reward := range rewards {
		t, e := parse.Address(reward.Token.Address)
		if e != nil {
			return provider.Output{}, errors.New("Merkl malformed token address")
		}
		if !wanted[t] {
			continue
		}
		if _, ok := selected[t]; ok {
			return provider.Output{}, errors.New("Merkl duplicate reward for configured token")
		}
		selected[t] = reward
	}
	for _, token := range p.tokens {
		if _, ok := selected[token]; !ok {
			return p.empty("Merkl configured reward absent")
		}
	}
	a := binding.NewMerklDistributor()
	call := func(data []byte) ([]byte, error) {
		return protocolio.CallData(ctx, p.deps.Chain, s, p.distributor, data)
	}
	raw, e := call(a.PackGetMerkleRoot())
	if e != nil {
		return provider.Output{}, e
	}
	rootValue, e := a.UnpackGetMerkleRoot(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid Merkl effective root response")
	}
	root := common.Hash(rootValue)
	if root == (common.Hash{}) {
		return provider.Output{}, errors.New("Merkl effective root is empty")
	}
	users := make([]common.Address, len(p.tokens))
	amounts := make([]*big.Int, len(p.tokens))
	proofs := make([][][32]byte, len(p.tokens))
	inputs := map[uint]string{}
	for i, t := range p.tokens {
		reward := selected[t]
		amount, e := parse.Uint256(reward.Amount)
		if e != nil || amount.BitLen() > 208 {
			return provider.Output{}, errors.New("Merkl cumulative amount exceeds distributor uint208")
		}
		apiClaimed, e := parse.Uint256(reward.Claimed)
		if e != nil || apiClaimed.Cmp(amount) > 0 {
			return provider.Output{}, errors.New("Merkl invalid claimed amount")
		}
		if _, e = parse.Uint256(reward.Pending); e != nil {
			return provider.Output{}, errors.New("Merkl invalid pending amount")
		}
		if reward.Token.Decimals < 0 || reward.Token.Decimals > 255 {
			return provider.Output{}, errors.New("Merkl invalid token decimals")
		}
		if len(reward.Proofs) != p.cfg.ProofLengths[i] {
			return provider.Output{}, errors.New("Merkl proof does not fit approved proofLengths layout")
		}
		for _, raw := range reward.Proofs {
			h, e := protocolio.Hash(raw)
			if e != nil {
				return provider.Output{}, e
			}
			proofs[i] = append(proofs[i], h)
		}
		if !VerifyProof(s.Account, t, amount, proofs[i], root) {
			return provider.Output{}, errors.New("Merkl proof does not match effective on-chain root")
		}
		raw, e = call(a.PackClaimed(s.Account, t))
		if e != nil {
			return provider.Output{}, e
		}
		claimedValue, e := a.UnpackClaimed(raw)
		if e != nil {
			return provider.Output{}, errors.New("invalid Merkl claimed amount response")
		}
		claimed := claimedValue.Amount
		if claimed.Cmp(amount) > 0 {
			return provider.Output{}, errors.New("Merkl response older than on-chain claimed amount")
		}
		delta := new(big.Int).Sub(amount, claimed)
		raw, e = call(a.PackClaimRecipient(s.Account, t))
		if e != nil {
			return provider.Output{}, e
		}
		recipient, e := a.UnpackClaimRecipient(raw)
		if e != nil {
			return provider.Output{}, errors.New("invalid Merkl claim recipient response")
		}
		if recipient == (common.Address{}) {
			raw, e = call(a.PackClaimRecipient(s.Account, common.Address{}))
			if e != nil {
				return provider.Output{}, e
			}
			recipient, e = a.UnpackClaimRecipient(raw)
			if e != nil {
				return provider.Output{}, errors.New("invalid Merkl default claim recipient response")
			}
		}
		if recipient != (common.Address{}) && recipient != s.Account {
			return provider.Output{}, errors.New("Merkl distributor would redirect rewards away from account")
		}
		if delta.Cmp(p.minimum) < 0 {
			return p.empty("Merkl rewards below minClaimable")
		}
		users[i] = s.Account
		amounts[i] = amount
		if len(p.cfg.AmountInputs) > 0 {
			inputs[p.cfg.AmountInputs[i]] = amount.String()
		}
		if len(p.cfg.ClaimableInputs) > 0 {
			inputs[p.cfg.ClaimableInputs[i]] = delta.String()
		}
	}
	payload, e := a.TryPackClaim(users, p.tokens, amounts, proofs)
	if e != nil {
		return provider.Output{}, errors.New("encode Merkl claim")
	}
	if len(payload) != p.cfg.CalldataBytes {
		return provider.Output{}, errors.New("Merkl claim does not fit approved calldataBytes layout")
	}
	data, _ := json.Marshal(map[string]string{"root": root.Hex(), "tokens": strconv.Itoa(len(p.tokens))})
	return provider.Output{Inputs: inputs, Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Data: data}, nil
}

// empty either leaves the strategy idle or supplies a zero gate for a dedicated optional claim group.
func (p *Provider) empty(reason string) (provider.Output, error) {
	if !p.cfg.SkipIfEmpty {
		return provider.Output{}, errors.Errorf("%w: %s", provider.ErrNotReady, reason)
	}
	inputs := map[uint]string{}
	for _, indexes := range [][]uint{p.cfg.AmountInputs, p.cfg.ClaimableInputs} {
		for _, index := range indexes {
			inputs[index] = "0"
		}
	}
	data, _ := json.Marshal(struct {
		Skipped bool   `json:"skipped"`
		Reason  string `json:"reason"`
	}{Skipped: true, Reason: reason})
	return provider.Output{Inputs: inputs, SkipSteps: []uint{p.claimStep}, Data: data}, nil
}

// VerifyProof mirrors Merkl's single keccak256(abi.encode(user,token,amount)) leaf and sorted pairs.
func VerifyProof(user, token common.Address, amount *big.Int, proof [][32]byte, root common.Hash) bool {
	h := crypto.Keccak256Hash(protocolio.AddressWord(user), protocolio.AddressWord(token), protocolio.Word(amount))
	for _, sibling := range proof {
		b := common.Hash(sibling)
		if bytes.Compare(h[:], b[:]) < 0 {
			h = crypto.Keccak256Hash(h[:], b[:])
		} else {
			h = crypto.Keccak256Hash(b[:], h[:])
		}
	}
	return h == root
}
func (p *Provider) Advance(_ context.Context, _ provider.Snapshot, _ provider.Output, _ json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}
