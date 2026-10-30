// Package claims discovers account-owned issuer claims outside committed recipes.
package claims

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	reads "github.com/symbioticfi/merkle-executor/sdk/bindings/claimreads"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const policy = "issuer-claims-v1"
const maxScan = 4096
const maxBatch = 16

var readABI = reads.NewClaimReads().GetABI()

type Provider struct{ reader provider.Reader }
type route struct {
	action  recipes.ManagedAction
	kind    string
	field   int
	factory common.Address
}

func NewAutomatic(deps provider.Deps) (provider.Provider, error) {
	if deps.Chain == nil {
		return nil, errors.New("claim discovery requires a chain reader")
	}
	return &Provider{reader: deps.Chain}, nil
}
func (*Provider) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	return provider.Output{}, errors.New("claims require an authenticated recipe")
}

func (*Provider) RecipeRequirements(s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree) (keeper.RecipeRequirements, error) {
	route, err := derive(s, r, tree)
	if err != nil || route == nil {
		return keeper.RecipeRequirements{}, err
	}
	return keeper.RecipeRequirements{Actions: []string{route.action.Key}}, nil
}

func (p *Provider) PrepareRecipe(ctx context.Context, s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree, _ map[uint]*big.Int) (provider.Output, error) {
	route, err := derive(s, r, tree)
	if err != nil {
		return provider.Output{}, err
	}
	if route == nil {
		return derived.Empty(policy)
	}
	if err = protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	variants, err := p.discover(ctx, s, *route)
	if err != nil {
		return provider.Output{}, err
	}
	if len(variants) == 0 {
		return provider.Output{}, provider.ErrNotReady
	}
	out := provider.Output{Actions: variants[0], Repeats: variants[1:]}
	return derived.Encode(policy, route.action.Key, out, true)
}

func (*Provider) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if err := ctx.Err(); err != nil {
		return provider.Progress{}, err
	}
	var key string
	inner, matched, err := derived.Decode(policy, out, &key)
	if err != nil {
		return provider.Progress{}, err
	}
	if !matched {
		return provider.Progress{Done: true, Outcome: "not-applicable"}, nil
	}
	if len(inner.Actions) != 1 || len(inner.Actions[key]) == 0 || len(inner.Inputs) != 0 || len(inner.Results) != 0 || len(inner.SkipSteps) != 0 || len(inner.Repeats) >= maxBatch {
		return provider.Progress{}, errors.New("invalid persisted claim batch")
	}
	for _, repeat := range inner.Repeats {
		if len(repeat) != 1 || len(repeat[key]) == 0 {
			return provider.Progress{}, errors.New("invalid persisted claim variant")
		}
	}
	if err := s.Execution.Validate(); err != nil {
		return provider.Progress{}, err
	}
	return provider.Progress{Done: true, Outcome: "claimed", Reference: s.Execution.TxHash.Hex()}, nil
}

func selector(signature string) []byte { return crypto.Keccak256([]byte(signature))[:4] }
func matches(data []byte, signature string) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], selector(signature))
}

// Derivation recognizes only unconditional unbound claims, preserving financial
// recipe amount rules for supply, withdrawal, swaps and other managed actions.
func derive(s provider.Snapshot, r recipes.Recipe, tree *recipes.Tree) (*route, error) {
	actions, err := recipes.ManagedActions(r, tree)
	if err != nil {
		return nil, err
	}
	var found *route
	for _, a := range actions {
		if a.Slot.Gate.Source != recipes.Literal || a.Slot.Gate.Value.Sign() == 0 || len(a.Bindings) != 0 {
			continue
		}
		data := a.Leaf.Data
		candidate := &route{action: a, field: -1}
		switch {
		case matches(data, "claimWithdrawal(uint256)"):
			candidate.kind, candidate.field = "noon", 4
		case matches(data, "claimDepositRequest(uint256)"):
			candidate.kind, candidate.field = "pareto-deposit", 4
		case matches(data, "claimWithdrawRequest(uint256)"):
			candidate.kind, candidate.field = "pareto-withdraw", 4
		case matches(data, "claimWithdrawRequest()"):
			candidate.kind = "pareto-normal"
		case matches(data, "claimInstantWithdrawRequest()"):
			candidate.kind = "pareto-instant"
		case matches(data, "mint(uint256,address)"), matches(data, "mint(uint256,address,address)"):
			candidate.kind, candidate.field = "mint", 4
		case matches(data, "deposit(uint256,address)"), matches(data, "deposit(uint256,address,address)"):
			candidate.kind, candidate.field = "deposit", 4
		case matches(data, "redeem(uint256,address,address)"):
			candidate.kind, candidate.field = "redeem", 4
		case matches(data, "withdraw(uint256,address,address)"):
			candidate.kind, candidate.field = "withdraw", 4
		case matches(data, "claimCancelDepositRequest(uint256,address,address)"):
			candidate.kind = "cancel-deposit"
		case matches(data, "claimCancelRedeemRequest(uint256,address,address)"):
			candidate.kind = "cancel-redeem"
		case matches(data, "claimDepositEscrow(address,address)"):
			candidate.kind = "deposit-refund"
		case matches(data, "claimRedeemEscrow(address)"):
			candidate.kind = "redeem-refund"
		case a.Leaf.Target == s.Account && matches(data, "execute(bytes[],bytes[],uint256[])"):
			candidate.factory, candidate.field, err = threeFLayout(data)
			if err != nil {
				return nil, err
			}
			if candidate.factory == (common.Address{}) {
				continue
			}
			candidate.kind = "threef"
		default:
			continue
		}
		if candidate.field >= 0 && candidate.field+32 > len(data) {
			return nil, errors.New("claim field exceeds calldata")
		}
		// ERC-4626 uses the same selectors for synchronous financial actions.
		// Only an explicitly variable, unbound amount identifies a prepared claim.
		if candidate.kind == "mint" || candidate.kind == "deposit" || candidate.kind == "withdraw" || candidate.kind == "redeem" {
			if len(a.Leaf.DataMask) < 36 || !bytes.Equal(a.Leaf.DataMask[4:36], bytes.Repeat([]byte{255}, 32)) {
				continue
			}
		}

		if found != nil {
			return nil, errors.New("use one claim operation per executor; batch its ready requests with multicall")
		}
		if len(a.Slot.Leaves) != 1 {
			return nil, errors.New("claim must have its own managed group")
		}
		found = candidate
	}
	return found, nil
}

func (p *Provider) values(ctx context.Context, s provider.Snapshot, target common.Address, name string, args ...any) ([]any, error) {
	method := readABI.Methods[name]
	data, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	data = append(bytes.Clone(method.ID), data...)
	raw, err := p.reader.CallContract(ctx, ethereum.CallMsg{To: &target, Data: data}, s.BlockNumber)
	if err != nil {
		return nil, errors.Errorf("claim %s read: %w", name, err)
	}
	values, err := method.Outputs.Unpack(raw)
	if err != nil {
		return nil, errors.Errorf("invalid claim %s reply", name)
	}
	return values, nil
}

func (p *Provider) uint(ctx context.Context, s provider.Snapshot, target common.Address, name string, args ...any) (*big.Int, error) {
	v, err := p.values(ctx, s, target, name, args...)
	if err != nil {
		return nil, err
	}
	return v[0].(*big.Int), nil
}

func bounded(n *big.Int) (int, error) {
	if !n.IsInt64() || n.Sign() < 0 || n.Int64() > maxScan {
		return 0, errors.New("claim inventory exceeds complete scan limit")
	}
	return int(n.Int64()), nil
}

func variant(r route, value *big.Int) map[string]hexutil.Bytes {
	data := bytes.Clone(r.action.Leaf.Data)
	if r.field >= 0 {
		value.FillBytes(data[r.field : r.field+32])
	}
	return map[string]hexutil.Bytes{r.action.Key: data}
}

var capabilityArgs = abi.Arguments{{Type: abiType("bytes[]")}, {Type: abiType("bytes[]")}, {Type: abiType("uint256[]")}}
var queryArgs = abi.Arguments{{Type: abiType("address")}, {Type: abiType("bytes")}, {Type: abiType("uint256[]")}, {Type: abiType("uint256")}}

func abiType(name string) abi.Type {
	t, err := abi.NewType(name, "", nil)
	if err != nil {
		panic(err)
	}
	return t
}

func threeFLayout(data []byte) (common.Address, int, error) {
	args, err := capabilityArgs.Unpack(data[4:])
	if err != nil {
		return common.Address{}, 0, errors.New("invalid claim capability")
	}
	queries, steps, inputs := args[0].([][]byte), args[1].([][]byte), args[2].([]*big.Int)
	if len(queries) != 1 || len(steps) != 3 || len(inputs) != 1 {
		return common.Address{}, 0, nil
	}
	query, err := queryArgs.Unpack(queries[0])
	if err != nil || !matches(query[1].([]byte), "isRequest(address)") {
		return common.Address{}, 0, nil
	}
	start := new(big.Int).SetBytes(data[68:100])
	if !start.IsInt64() || start.Int64()+68 > int64(len(data)) {
		return common.Address{}, 0, errors.New("invalid claim request offset")
	}
	return query[0].(common.Address), 4 + int(start.Int64()) + 32, nil
}
