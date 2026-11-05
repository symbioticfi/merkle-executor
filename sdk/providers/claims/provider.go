// Package claims discovers settled native issuer claims before recipe execution.
package claims

import (
	"bytes"
	"context"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	interfaces "github.com/symbioticfi/merkle-executor/sdk/abi"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const policy = "native-claim/v1"

var contractABI = func() abi.ABI {
	a, err := abi.JSON(bytes.NewReader(interfaces.NativeClaims))
	if err != nil {
		panic(err)
	}
	return a
}()

var queryArgs = func() abi.Arguments {
	var args abi.Arguments
	for _, name := range []string{"address", "bytes", "uint256[]", "uint256"} {
		ty, err := abi.NewType(name, "", nil)
		if err != nil {
			panic(err)
		}
		args = append(args, abi.Argument{Type: ty})
	}
	return args
}()

type Provider struct{ chain provider.Reader }
type config struct {
	Key          string
	Target       common.Address
	Data         hexutil.Bytes
	Mask         hexutil.Bytes
	AmountResult *uint `json:",omitempty"`
}
type candidate struct {
	config
	method string
	args   []any
}

func NewAutomatic(deps provider.Deps) (keeper.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("native claims require chain reader")
	}
	return &Provider{chain: deps.Chain}, nil
}

// A native claim is one unconditional call, preceded only by argument reads.
func match(r recipes.Recipe, t *recipes.Tree) (candidate, bool, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return candidate{}, false, err
	}
	if len(actions) != 1 {
		return candidate{}, false, nil
	}
	a := actions[0]
	if a.Step != uint(len(r.Steps)-1) || a.Slot.Gate.Source != recipes.Literal || a.Slot.Gate.Value.Cmp(big.NewInt(1)) != 0 {
		return candidate{}, false, nil
	}
	for _, step := range r.Steps[:len(r.Steps)-1] {
		if step[0] != recipes.Read {
			return candidate{}, false, nil
		}
	}
	if len(a.Leaf.Data) < 4 {
		return candidate{}, false, nil
	}
	m, err := contractABI.MethodById(a.Leaf.Data[:4])
	if err != nil {
		return candidate{}, false, nil
	}
	name := m.RawName
	if name == "claimWithdrawRequest" && len(m.Inputs) == 1 {
		name = "claimWithdrawRequestForEpoch"
	}
	var amountResult *uint
	switch name {
	case "claimWithdrawal", "claimDepositRequest", "claimWithdrawRequestForEpoch", "claimWithdrawRequest", "claimInstantWithdrawRequest", "claimDepositEscrow", "claimRedeemEscrow", "claimCancelDepositRequest", "claimCancelRedeemRequest":
	case "deposit", "mint", "withdraw", "redeem":
		// These selectors also denote synchronous operations. Only the native maximum-bound
		// argument shape is eligible, and preparation checks the async entitlement separately.
		if len(a.Bindings) != 1 || a.Bindings[0].Offset != 4 || a.Bindings[0].Value.Source != recipes.Result {
			return candidate{}, false, nil
		}
		// Only a direct native max* read is an asynchronous amount candidate.
		index := a.Bindings[0].Value.Value
		n := uint(index.Uint64())
		amountResult = &n
		if !index.IsUint64() || index.Uint64() >= uint64(len(r.Steps)-1) {
			return candidate{}, false, nil
		}
		step := r.Steps[index.Uint64()]
		if len(step) != 33 || step[0] != recipes.Read {
			return candidate{}, false, nil
		}
		queryIndex := new(big.Int).SetBytes(step[1:])
		if !queryIndex.IsUint64() || queryIndex.Uint64() >= uint64(len(r.Queries)) {
			return candidate{}, false, nil
		}
		query := r.Queries[queryIndex.Uint64()]
		values, err := queryArgs.Unpack(query)
		if err != nil || values[0].(common.Address) != a.Leaf.Target || len(values[2].([]*big.Int)) != 0 || values[3].(*big.Int).Sign() != 0 {
			return candidate{}, false, nil
		}
		read := values[1].([]byte)
		want := map[string]string{"deposit": "maxDeposit", "mint": "maxMint", "withdraw": "maxWithdraw", "redeem": "maxRedeem"}[name]
		if len(read) != 36 || !bytes.Equal(read[:4], contractABI.Methods[want].ID) {
			return candidate{}, false, nil
		}
	default:
		return candidate{}, false, nil
	}
	if err := a.Pinned(0, 4); err != nil {
		return candidate{}, false, err
	}
	args, err := m.Inputs.Unpack(a.Leaf.Data[4:])
	if err != nil {
		return candidate{}, false, err
	}
	return candidate{config{a.Key, a.Leaf.Target, a.Leaf.Data, a.Leaf.DataMask, amountResult}, name, args}, true, nil
}
func (*Provider) RecipeRequirements(_ provider.Snapshot, r recipes.Recipe, t *recipes.Tree) (keeper.RecipeRequirements, error) {
	c, ok, err := match(r, t)
	if !ok || err != nil {
		return keeper.RecipeRequirements{}, err
	}
	requirements := keeper.RecipeRequirements{Actions: []string{c.Key}}
	if c.AmountResult != nil {
		requirements.Values = []recipes.Value{{Source: recipes.Result, Value: new(big.Int).SetUint64(uint64(*c.AmountResult))}}
	}
	return requirements, nil
}
func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, t *recipes.Tree, results map[uint]*big.Int) ([]provider.Output, error) {
	c, ok, err := match(r, t)
	if err != nil {
		return nil, err
	}
	if !ok {
		return derived.Empty(policy)
	}
	if err = protocolio.Snapshot(s); err != nil {
		return nil, err
	}
	ids, ready, err := p.ready(ctx, s, c)
	if err != nil {
		if (c.method == "deposit" || c.method == "mint" || c.method == "withdraw" || c.method == "redeem") && isRevert(err) {
			return derived.Empty(policy)
		}
		return nil, err
	}
	if !ready {
		return nil, errors.Errorf("%w: no settled native claims", provider.ErrNotReady)
	}
	if c.AmountResult != nil {
		amount, ok := results[*c.AmountResult]
		if !ok || amount == nil {
			return nil, errors.New("native claim amount result was not resolved")
		}
		if amount.Sign() == 0 {
			return nil, errors.Errorf("%w: native claim amount is zero", provider.ErrNotReady)
		}
	}

	var outputs []provider.Output
	if ids == nil {
		ids = []*big.Int{nil}
	}
	for _, id := range ids {
		data := bytes.Clone(c.Data)
		if id != nil {
			if len(data) < 36 {
				return nil, errors.New("claim ID exceeds calldata")
			}
			id.FillBytes(data[4:36])
		}
		allowed := true
		for i := range data {
			if (data[i]^c.Data[i]) & ^c.Mask[i] != 0 {
				allowed = false
				break
			}
		}
		if !allowed {
			continue
		}
		// A native eth_call catches settlement or liquidity changes without sending a transaction.
		// Amount-bound calls are checked by the planner after their query bindings are resolved.
		if c.method != "deposit" && c.method != "mint" && c.method != "withdraw" && c.method != "redeem" {
			valid, err := p.nativeReady(ctx, s, c.Target, data)
			if err != nil {
				return nil, err
			}
			if !valid {
				continue
			}
		}

		out, err := derived.Encode(policy, c.config, provider.Output{Actions: map[string]hexutil.Bytes{c.Key: data}}, true)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, out...)
	}
	if len(outputs) == 0 {
		return nil, errors.Errorf("%w: native claims are not callable", provider.ErrNotReady)
	}
	return outputs, nil
}
func (p *Provider) nativeReady(ctx context.Context, s provider.Snapshot, target common.Address, data []byte) (bool, error) {
	_, err := p.chain.CallContract(ctx, ethereum.CallMsg{From: s.Account, To: &target, Data: data}, s.BlockNumber)
	if err == nil {
		return true, nil
	}
	if isRevert(err) {
		return false, nil
	}
	return false, err
}
func isRevert(err error) bool {
	var coded interface{ ErrorCode() int }
	return (errors.As(err, &coded) && coded.ErrorCode() == 3) || strings.HasPrefix(err.Error(), "execution reverted")
}

func (p *Provider) read(ctx context.Context, s provider.Snapshot, target common.Address, name string, args ...any) ([]any, error) {
	data, err := contractABI.Pack(name, args...)
	if err != nil {
		return nil, err
	}
	raw, err := protocolio.CallData(ctx, p.chain, s, target, data)
	if err != nil {
		return nil, err
	}
	out, err := contractABI.Unpack(name, raw)
	if err != nil {
		return nil, errors.Errorf("decode %s: %w", name, err)
	}
	return out, nil
}
func (p *Provider) number(ctx context.Context, s provider.Snapshot, target common.Address, name string, args ...any) (*big.Int, error) {
	out, err := p.read(ctx, s, target, name, args...)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("missing claim amount")
	}
	return out[0].(*big.Int), nil
}
func (p *Provider) ready(ctx context.Context, s provider.Snapshot, c candidate) ([]*big.Int, bool, error) {
	switch c.method {
	case "claimWithdrawal":
		return p.noon(ctx, s, c)
	case "claimDepositRequest", "claimWithdrawRequestForEpoch":
		return p.paretoQueue(ctx, s, c)
	case "claimWithdrawRequest", "claimInstantWithdrawRequest":
		out, err := p.read(ctx, s, c.Target, "strategy")
		if err != nil {
			return nil, false, err
		}
		target := out[0].(common.Address)
		getter := "withdrawsRequests"
		if c.method == "claimInstantWithdrawRequest" {
			getter = "instantWithdrawsRequests"
		}
		amount, err := p.number(ctx, s, target, getter, s.Account)
		if err != nil {
			return nil, false, err
		}
		if c.method == "claimWithdrawRequest" {
			values, err := p.read(ctx, s, target, "apr0Users", s.Account)
			if err != nil {
				return nil, false, err
			}
			for _, i := range []int{0, 2, 3} {
				amount.Add(amount, values[i].(*big.Int))
			}
		}
		return nil, amount.Sign() > 0, nil
	case "claimDepositEscrow":
		if c.args[0].(common.Address) != s.Account {
			return nil, false, errors.New("claim recipient is not the account")
		}
		n, err := p.number(ctx, s, c.Target, "depositEscrowBalance", c.args...)
		return nil, err == nil && n.Sign() > 0, err
	case "claimRedeemEscrow":
		if c.args[0].(common.Address) != s.Account {
			return nil, false, errors.New("claim recipient is not the account")
		}
		n, err := p.number(ctx, s, c.Target, "redeemEscrowBalance", s.Account)
		return nil, err == nil && n.Sign() > 0, err
	default:
		getter := "claimableDepositRequest"
		if c.method == "redeem" || c.method == "withdraw" {
			getter = "claimableRedeemRequest"
		}
		if c.method == "claimCancelDepositRequest" {
			getter = "claimableCancelDepositRequest"
		}
		if c.method == "claimCancelRedeemRequest" {
			getter = "claimableCancelRedeemRequest"
		}
		id := new(big.Int)
		if c.method == "claimCancelDepositRequest" || c.method == "claimCancelRedeemRequest" {
			id = c.args[0].(*big.Int)
		}
		n, err := p.number(ctx, s, c.Target, getter, id, s.Account)
		return nil, err == nil && n.Sign() > 0, err
	}
}
func (p *Provider) noon(ctx context.Context, s provider.Snapshot, c candidate) ([]*big.Int, bool, error) {
	count, err := p.number(ctx, s, c.Target, "getUserNextRequestId", s.Account)
	if err != nil {
		return nil, false, err
	}
	period, err := p.number(ctx, s, c.Target, "withdrawPeriod")
	if err != nil {
		return nil, false, err
	}
	var ids []*big.Int
	for id := new(big.Int); id.Cmp(count) < 0; id.Add(id, big.NewInt(1)) {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		v, err := p.read(ctx, s, c.Target, "getWithdrawalRequest", s.Account, id)
		if err != nil {
			return nil, false, err
		}
		request := *abi.ConvertType(v[0], new(struct {
			Amount    *big.Int
			Timestamp *big.Int
			Claimed   bool
		})).(*struct {
			Amount    *big.Int
			Timestamp *big.Int
			Claimed   bool
		})
		deadline := new(big.Int).Add(request.Timestamp, period)
		if request.Amount.Sign() > 0 && !request.Claimed && deadline.Cmp(new(big.Int).SetUint64(s.Timestamp)) <= 0 {
			ids = append(ids, new(big.Int).Set(id))
		}
	}
	return ids, len(ids) > 0, nil
}
func (p *Provider) paretoQueue(ctx context.Context, s provider.Snapshot, c candidate) ([]*big.Int, bool, error) {
	strategy, err := p.read(ctx, s, c.Target, "strategy")
	if err != nil {
		return nil, false, err
	}
	current, err := p.number(ctx, s, strategy[0].(common.Address), "epochNumber")
	if err != nil {
		return nil, false, err
	}
	end := new(big.Int).Add(current, big.NewInt(2))
	deposit := c.method == "claimDepositRequest"
	owner, price, pending := "userWithdrawalsEpochs", "epochWithdrawPrice", "epochPendingClaims"
	if deposit {
		owner, price, pending = "userDepositsEpochs", "epochPrice", "epochPendingDeposits"
	}
	var ids []*big.Int
	for id := new(big.Int); id.Cmp(end) < 0; id.Add(id, big.NewInt(1)) {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		amount, err := p.number(ctx, s, c.Target, owner, s.Account, id)
		if err != nil {
			return nil, false, err
		}
		if amount.Sign() == 0 {
			continue
		}
		value, err := p.number(ctx, s, c.Target, price, id)
		if err != nil {
			return nil, false, err
		}
		if value.Sign() == 0 {
			continue
		}
		wait, err := p.number(ctx, s, c.Target, pending, id)
		if err != nil {
			return nil, false, err
		}
		if wait.Sign() != 0 {
			continue
		}
		if deposit {
			wait, err = p.number(ctx, s, c.Target, "epochPrefundedDeposits", id)
			if err != nil {
				return nil, false, err
			}
			if wait.Sign() != 0 {
				continue
			}
		}
		ids = append(ids, new(big.Int).Set(id))
	}
	return ids, len(ids) > 0, nil
}
func (*Provider) Advance(_ context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	var c config
	inner, matched, err := derived.Decode(policy, out, &c)
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true}, nil
	}
	if err = s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	data, ok := inner.Actions[c.Key]
	if !ok || len(inner.Actions) != 1 || len(data) != len(c.Data) || len(c.Mask) != len(data) || len(inner.Inputs)+len(inner.Results)+len(inner.SkipSteps) != 0 {
		return provider.Progress{}, errors.New("invalid persisted native claim")
	}
	for i := range data {
		if (data[i]^c.Data[i]) & ^c.Mask[i] != 0 {
			return provider.Progress{}, errors.New("persisted claim changed pinned data")
		}
	}
	return provider.Progress{Done: true}, nil
}
