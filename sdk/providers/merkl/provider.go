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

	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Config struct {
	HTTP                protocolio.HTTPConfig `yaml:"http"`
	Multicall3          string                `yaml:"multicall3"`
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
	DirectRecipient     bool                  `yaml:"directRecipient"`
	AllowIdle           bool                  `yaml:"allowIdle"`
}
type Client struct{ http *protocolio.Client }

func NewClient(c protocolio.HTTPConfig, h *http.Client, lookup func(string) string) (*Client, error) {
	v, e := protocolio.NewHTTP(c, h, lookup)
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
	multicall   common.Address
	tokens      []common.Address
	minimum     *big.Int
	claimStep   uint
}

// New consumes an immutable carrier derived from the authenticated claim leaf.
func New(c Config, deps provider.Deps) (*Provider, error) {
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
			if err := parse.InputIndex(i); err != nil {
				return nil, err
			}
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
	p.client, e = NewClient(c.HTTP, deps.HTTP, deps.LookupSecret)
	if e != nil {
		return nil, e
	}
	if deps.Chain == nil {
		return nil, errors.New("Merkl chain reader required")
	}
	return p, nil
}
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	return p.prepare(ctx, s, func() ([]multicall.Multicall3Result, error) { return p.readBatch(ctx, s) })
}

// PlanPreparation only encodes configured reads. The strict aggregate stays an
// isolated eth_call; it must not be nested inside the host's safe-getter aggregate.
func (p *Provider) PlanPreparation(pin protocolio.PreparationPin) (protocolio.PreparationPlan, error) {
	if err := pin.Validate(); err != nil {
		return protocolio.PreparationPlan{}, err
	}
	payload, err := p.batchPayload(pin.Account)
	if err != nil {
		return protocolio.PreparationPlan{}, err
	}
	return protocolio.PreparationPlan{
		Requests: []protocolio.ReadRequest{protocolio.ContractRead(p.multicall, payload, new(big.Int).SetUint64(pin.BlockNumber), false)},
		Resolve: func(ctx context.Context, s provider.Snapshot, read protocolio.ReadAccessor) (provider.Output, error) {
			if !pin.Matches(s) {
				return provider.Output{}, errors.New("Merkl staged identity mismatch")
			}
			return p.prepare(ctx, s, func() ([]multicall.Multicall3Result, error) {
				raw, err := read(0).CallData()
				if err != nil {
					return nil, err
				}
				return p.decodeBatch(raw)
			})
		},
	}, nil
}
func (p *Provider) prepare(ctx context.Context, s provider.Snapshot, read func() ([]multicall.Multicall3Result, error)) (provider.Output, error) {
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
	return p.prepareRewards(s, entries, read)
}

func (p *Provider) prepareRewards(s provider.Snapshot, entries []wire.MerklEntry, read func() ([]multicall.Multicall3Result, error)) (provider.Output, error) {
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
	results, e := read()
	if e != nil {
		return provider.Output{}, e
	}
	rootValue, e := a.UnpackGetMerkleRoot(results[0].ReturnData)
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
	var defaultRecipient common.Address
	defaultLoaded := false
	claimable := new(big.Int)
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
		claimedValue, e := a.UnpackClaimed(results[2+2*i].ReturnData)
		if e != nil {
			return provider.Output{}, errors.New("invalid Merkl claimed amount response")
		}
		claimed := claimedValue.Amount
		if claimed.Cmp(amount) > 0 {
			return provider.Output{}, errors.New("Merkl response older than on-chain claimed amount")
		}
		delta := new(big.Int).Sub(amount, claimed)
		recipient, e := a.UnpackClaimRecipient(results[3+2*i].ReturnData)
		if e != nil {
			return provider.Output{}, errors.New("invalid Merkl claim recipient response")
		}
		if recipient == (common.Address{}) {
			if !defaultLoaded {
				defaultRecipient, e = a.UnpackClaimRecipient(results[1].ReturnData)
				if e != nil {
					return provider.Output{}, errors.New("invalid Merkl default claim recipient response")
				}
				defaultLoaded = true
			}
			recipient = defaultRecipient
		}
		if !p.cfg.DirectRecipient && recipient != (common.Address{}) && recipient != s.Account {
			return provider.Output{}, errors.New("Merkl distributor would redirect rewards away from account")
		}
		if delta.Cmp(p.minimum) >= 0 {
			claimable.Add(claimable, delta)
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
	// An early idle token must not hide invalid data in later batch members.
	if claimable.Sign() == 0 && !p.cfg.AllowIdle {
		return p.empty("Merkl rewards below minClaimable")
	}
	payload, e := a.TryPackClaim(users, p.tokens, amounts, proofs)
	if p.cfg.DirectRecipient {
		recipients := append([]common.Address(nil), users...)
		payload, e = a.TryPackClaimWithRecipient(users, p.tokens, amounts, proofs, recipients, make([][]byte, len(users)))
	}
	if e != nil {
		return provider.Output{}, errors.New("encode Merkl claim")
	}
	if len(payload) != p.cfg.CalldataBytes {
		return provider.Output{}, errors.New("Merkl claim does not fit approved calldataBytes layout")
	}
	data, _ := json.Marshal(map[string]string{"root": root.Hex(), "tokens": strconv.Itoa(len(p.tokens)), "claimable": claimable.String()})
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

// readBatch fetches the entire bounded distributor snapshot in one eth_call.
// Every getter is required to execute successfully; decoding the default recipient
// stays lazy because explicit per-token recipients override that value.
func (p *Provider) readBatch(ctx context.Context, s provider.Snapshot) ([]multicall.Multicall3Result, error) {
	payload, err := p.batchPayload(s.Account)
	if err != nil {
		return nil, err
	}
	raw, err := protocolio.CallData(ctx, p.deps.Chain, s, p.multicall, payload)
	if err != nil {
		return nil, err
	}
	return p.decodeBatch(raw)
}
func (p *Provider) batchPayload(account common.Address) ([]byte, error) {
	distributor := binding.NewMerklDistributor()
	calls := make([]multicall.Multicall3Call3, 0, 2+2*len(p.tokens))
	add := func(data []byte) {
		calls = append(calls, multicall.Multicall3Call3{Target: p.distributor, AllowFailure: false, CallData: data})
	}
	add(distributor.PackGetMerkleRoot())
	add(distributor.PackClaimRecipient(account, common.Address{}))
	for _, token := range p.tokens {
		add(distributor.PackClaimed(account, token))
		add(distributor.PackClaimRecipient(account, token))
	}
	aggregate := multicall.NewMulticall3()
	payload, err := aggregate.TryPackAggregate3(calls)
	if err != nil {
		return nil, errors.New("encode Merkl aggregate reads")
	}
	return payload, nil
}
func (p *Provider) decodeBatch(raw []byte) ([]multicall.Multicall3Result, error) {
	results, err := multicall.NewMulticall3().UnpackAggregate3(raw)
	if err != nil {
		return nil, errors.New("invalid Merkl aggregate response")
	}
	if len(results) != 2+2*len(p.tokens) {
		return nil, errors.New("Merkl aggregate result count mismatch")
	}
	for _, result := range results {
		if !result.Success {
			return nil, errors.New("Merkl aggregate getter failed")
		}
	}
	return results, nil
}
