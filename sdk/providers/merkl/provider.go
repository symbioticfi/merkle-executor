// Package merkl prepares cumulative Merkl rewards for an account self-claim.
package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"

	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// Config is the direct typed carrier derived from an authenticated claim leaf.
type Config struct {
	HTTP            protocolio.HTTPConfig
	Multicall3      string
	Distributor     string
	Tokens          []string
	ProofCapacities []int
	Action          string
	CalldataBytes   int
	ClaimableInputs []uint
}

type rewardSet map[common.Address]wire.MerklReward

func (set rewardSet) tokens() []string {
	tokens := make([]string, 0, len(set))
	for token := range set {
		tokens = append(tokens, token.Hex())
	}
	sort.Strings(tokens)
	return tokens
}

var errAbsentReward = errors.Errorf("%w: Merkl reward absent", provider.ErrNotReady)

func rewards(ctx context.Context, client *protocolio.Client, account common.Address, chainID *big.Int) (rewardSet, error) {
	var v []wire.MerklEntry
	_, e := client.JSON(ctx, http.MethodGet, "/v4/users/"+strings.ToLower(account.Hex())+"/rewards?chainId="+chainID.String(), nil, &v)
	if e != nil {
		return nil, e
	}
	if v == nil {
		return nil, errors.New("Merkl response must be a rewards array")
	}
	return indexRewards(v, chainID.Int64())
}

func indexRewards(entries []wire.MerklEntry, chainID int64) (rewardSet, error) {
	var selected rewardSet
	for _, entry := range entries {
		if entry.Chain.ID <= 0 || entry.Rewards == nil {
			return nil, errors.New("Merkl malformed chain entry")
		}
		if entry.Chain.ID != chainID {
			continue
		}
		if selected != nil {
			return nil, errors.New("Merkl response contains duplicate chain entries")
		}
		selected = make(rewardSet, len(entry.Rewards))
		for _, reward := range entry.Rewards {
			token, err := parse.Address(reward.Token.Address)
			if err != nil {
				return nil, errors.New("Merkl malformed token address")
			}
			if _, duplicate := selected[token]; duplicate {
				return nil, errors.New("Merkl duplicate reward token")
			}
			selected[token] = reward
		}
	}
	// Pending rewards have no finalized leaf and must stay out of claim selection.
	for token, reward := range selected {
		if reward.Amount != "0" || reward.Claimed != "0" || len(reward.Proofs) != 0 {
			continue
		}
		if _, err := parse.Uint256(reward.Pending); err == nil && reward.Token.Decimals >= 0 && reward.Token.Decimals <= 255 {
			delete(selected, token)
		}
	}
	return selected, nil
}

type Provider struct {
	cfg         Config
	deps        provider.Deps
	client      *protocolio.Client
	distributor common.Address
	multicall   common.Address
	tokens      []common.Address
}

// New consumes an immutable carrier derived from the authenticated claim leaf.
func New(c Config, deps provider.Deps) (*Provider, error) {
	if e := protocolio.Action(c.Action); e != nil {
		return nil, e
	}
	if len(c.Tokens) == 0 || len(c.Tokens) > 32 || len(c.ProofCapacities) != len(c.Tokens) {
		return nil, errors.New("Merkl requires 1..32 tokens and one proofCapacities entry per token")
	}
	if c.CalldataBytes < 4 || c.CalldataBytes > 131072 {
		return nil, errors.New("Merkl calldataBytes outside 4..131072")
	}
	if len(c.ClaimableInputs) != 0 && len(c.ClaimableInputs) != len(c.Tokens) {
		return nil, errors.New("Merkl input indexes must be empty or match tokens length")
	}
	seenInputs := map[uint]bool{}
	for _, i := range c.ClaimableInputs {
		if err := parse.InputIndex(i); err != nil {
			return nil, err
		}
		if seenInputs[i] {
			return nil, errors.New("Merkl duplicate input index")
		}
		seenInputs[i] = true
	}
	p := &Provider{cfg: c, deps: deps}
	var e error
	p.distributor, e = parse.Address(c.Distributor)
	if e != nil {
		return nil, e
	}
	p.multicall, e = parse.Address(c.Multicall3)
	if e != nil {
		return nil, errors.New("Merkl multicall3 must be a nonzero address")
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
		if c.ProofCapacities[i] < 0 || c.ProofCapacities[i] > 64 {
			return nil, errors.New("Merkl proof capacity outside 0..64")
		}
	}
	p.client, e = protocolio.NewHTTP(c.HTTP, deps.HTTP)
	if e != nil {
		return nil, e
	}
	if deps.Chain == nil {
		return nil, errors.New("Merkl chain reader required")
	}
	return p, nil
}

// Prepare serves one directly configured claim through the shared reward reads.
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if e := protocolio.Snapshot(s); e != nil {
		return provider.Output{}, e
	}
	if !s.ChainID.IsInt64() {
		return provider.Output{}, errors.New("Merkl API chain ID exceeds int64")
	}
	entries, e := rewards(ctx, p.client, s.Account, s.ChainID)
	if e != nil {
		return provider.Output{}, e
	}
	reads := newRewardReads(ctx, p.deps, s, nil, nil)
	out, claimable, e := p.prepareRewards(s, entries, func() ([]multicall.Multicall3Result, error) { return reads.forProvider(p) })
	if e == nil && claimable.Sign() == 0 {
		// Idle members are reported only after the whole claim validates.
		return provider.Output{}, errors.Errorf("%w: no unclaimed Merkl rewards", provider.ErrNotReady)
	}
	return out, e
}

// prepareRewards returns the claim and its total cumulative-minus-claimed delta.
func (p *Provider) prepareRewards(s provider.Snapshot, selected rewardSet, read func() ([]multicall.Multicall3Result, error)) (provider.Output, *big.Int, error) {
	for _, token := range p.tokens {
		if _, ok := selected[token]; !ok {
			return provider.Output{}, nil, errAbsentReward
		}
	}
	a := binding.NewMerklDistributor()
	results, e := read()
	if e != nil {
		return provider.Output{}, nil, e
	}
	rootValue, e := a.UnpackGetMerkleRoot(results[0].ReturnData)
	if e != nil {
		return provider.Output{}, nil, errors.New("invalid Merkl effective root response")
	}
	root := common.Hash(rootValue)
	if root == (common.Hash{}) {
		return provider.Output{}, nil, errors.New("Merkl effective root is empty")
	}
	users := make([]common.Address, len(p.tokens))
	amounts := make([]*big.Int, len(p.tokens))
	proofs := make([][][32]byte, len(p.tokens))
	inputs := map[uint]string{}
	var defaultRecipient common.Address
	defaultLoaded := false
	claimable := new(big.Int)
	for i, t := range p.tokens {
		reward := selected[t]
		amount, e := parse.Uint256(reward.Amount)
		if e != nil || amount.BitLen() > 208 {
			return provider.Output{}, nil, errors.New("Merkl cumulative amount exceeds distributor uint208")
		}
		apiClaimed, e := parse.Uint256(reward.Claimed)
		if e != nil || apiClaimed.Cmp(amount) > 0 {
			return provider.Output{}, nil, errors.New("Merkl invalid claimed amount")
		}
		if _, e = parse.Uint256(reward.Pending); e != nil {
			return provider.Output{}, nil, errors.New("Merkl invalid pending amount")
		}
		if reward.Token.Decimals < 0 || reward.Token.Decimals > 255 {
			return provider.Output{}, nil, errors.New("Merkl invalid token decimals")
		}
		if len(reward.Proofs) > p.cfg.ProofCapacities[i] {
			return provider.Output{}, nil, errors.New("Merkl proof does not fit approved proofCapacities layout")
		}
		for _, raw := range reward.Proofs {
			h, e := protocolio.Hash(raw)
			if e != nil {
				return provider.Output{}, nil, e
			}
			proofs[i] = append(proofs[i], h)
		}
		if !VerifyProof(s.Account, t, amount, proofs[i], root) {
			return provider.Output{}, nil, errors.New("Merkl proof does not match effective on-chain root")
		}
		claimedValue, e := a.UnpackClaimed(results[2+2*i].ReturnData)
		if e != nil {
			return provider.Output{}, nil, errors.New("invalid Merkl claimed amount response")
		}
		claimed := claimedValue.Amount
		if claimed.Cmp(amount) > 0 {
			return provider.Output{}, nil, errors.New("Merkl response older than on-chain claimed amount")
		}
		delta := new(big.Int).Sub(amount, claimed)
		recipient, e := a.UnpackClaimRecipient(results[3+2*i].ReturnData)
		if e != nil {
			return provider.Output{}, nil, errors.New("invalid Merkl claim recipient response")
		}
		if recipient == (common.Address{}) {
			if !defaultLoaded {
				defaultRecipient, e = a.UnpackClaimRecipient(results[1].ReturnData)
				if e != nil {
					return provider.Output{}, nil, errors.New("invalid Merkl default claim recipient response")
				}
				defaultLoaded = true
			}
			recipient = defaultRecipient
		}
		if recipient != (common.Address{}) && recipient != s.Account {
			return provider.Output{}, nil, errors.New("Merkl distributor would redirect rewards away from account")
		}
		claimable.Add(claimable, delta)
		users[i] = s.Account
		amounts[i] = amount
		if len(p.cfg.ClaimableInputs) > 0 {
			inputs[p.cfg.ClaimableInputs[i]] = delta.String()
		}
	}
	reserved := make([][][32]byte, len(proofs))
	for i, proof := range proofs {
		reserved[i] = make([][32]byte, p.cfg.ProofCapacities[i])
		copy(reserved[i], proof)
	}
	payload, e := a.TryPackClaim(users, p.tokens, amounts, reserved)
	if e == nil {
		proofArray := 4 + int(new(big.Int).SetBytes(payload[100:132]).Uint64())
		for i, proof := range proofs {
			relative := int(new(big.Int).SetBytes(payload[proofArray+32+32*i : proofArray+64+32*i]).Uint64())
			big.NewInt(int64(len(proof))).FillBytes(payload[proofArray+32+relative : proofArray+64+relative])
		}
	}
	if e != nil {
		return provider.Output{}, nil, errors.New("encode Merkl claim")
	}
	if len(payload) != p.cfg.CalldataBytes {
		return provider.Output{}, nil, errors.New("Merkl claim does not fit approved calldataBytes layout")
	}
	data, _ := json.Marshal(map[string]string{"root": root.Hex(), "tokens": strconv.Itoa(len(p.tokens)), "claimable": claimable.String()})
	return provider.Output{Inputs: inputs, Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Data: data}, claimable, nil
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
