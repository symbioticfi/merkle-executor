package keeper

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/recipes"
)

type Backend interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

type Strategy struct {
	Config    config.Strategy
	Providers []provider.Provider
}

type Plan struct {
	Snapshot provider.Snapshot `json:"snapshot"`
	Recipe   recipes.Snapshot  `json:"recipe"`
	Calldata hexutil.Bytes     `json:"calldata"`
	Leaves   []common.Hash     `json:"leaves"`
	Outputs  []provider.Output `json:"outputs"`
}

type Planner struct {
	Chain         Backend
	ChainID       *big.Int
	Caller        common.Address
	Confirmations uint64
}

func (p *Planner) Prepare(ctx context.Context, strategy Strategy) (Plan, error) {
	var plan Plan
	snap, recipe, err := recipes.Discover(ctx, p.Chain, common.HexToAddress(strategy.Config.Executor), nil)
	if err != nil {
		return plan, errors.Errorf("discover recipe: %w", err)
	}
	if snap.Caller != p.Caller || snap.Account != common.HexToAddress(strategy.Config.Account) {
		return plan, errors.New("configured caller/account mismatch")
	}
	if strategy.Config.RecipeHash != "" && snap.RecipeHash != common.HexToHash(strategy.Config.RecipeHash) {
		return plan, errors.New("configured recipe hash mismatch")
	}
	tree, err := recipes.LoadTree(strategy.Config.Tree)
	if err != nil {
		return plan, errors.Errorf("load permission tree: %w", err)
	}
	if tree.Root != snap.Root {
		return plan, errors.New("permission tree does not match active account root")
	}
	if tree.ChainID != nil && tree.ChainID.Cmp(p.ChainID) != 0 {
		return plan, errors.New("permission tree chain mismatch")
	}
	if tree.Account != nil && *tree.Account != snap.Account {
		return plan, errors.New("permission tree account mismatch")
	}
	plan.Recipe = snap
	plan.Snapshot = provider.Snapshot{ChainID: new(big.Int).Set(p.ChainID), BlockNumber: new(big.Int).SetUint64(snap.BlockNumber), BlockHash: snap.BlockHash, Timestamp: snap.Timestamp, Executor: snap.Executor, Account: snap.Account, Caller: snap.Caller, RecipeHash: snap.RecipeHash, Root: snap.Root}
	inputs := make(map[uint]string)
	actions := make(map[string]hexutil.Bytes)
	var skips []uint
	for _, pr := range strategy.Providers {
		out, err := pr.Prepare(ctx, plan.Snapshot)
		if err != nil {
			return plan, errors.Errorf("resolve runtime: %w", err)
		}
		if err := merge(inputs, actions, &skips, out); err != nil {
			return plan, err
		}
		plan.Outputs = append(plan.Outputs, out)
	}
	plan.Calldata, plan.Leaves, err = recipes.Assemble(recipe, tree, inputs, actions, skips)
	if err != nil {
		return plan, errors.Errorf("assemble runtime: %w", err)
	}
	if err := recipes.VerifyRoles(ctx, p.Chain, snap, plan.Leaves); err != nil {
		return plan, err
	}
	if _, err := p.Chain.CallContract(ctx, ethereum.CallMsg{From: p.Caller, To: &snap.Executor, Data: plan.Calldata}, plan.Snapshot.BlockNumber); err != nil {
		return plan, errors.New("full recipe simulation failed")
	}
	if err := recipes.ValidateSnapshot(ctx, p.Chain, snap, p.Caller); err != nil {
		return plan, err
	}
	return plan, nil
}

func merge(inputs map[uint]string, actions map[string]hexutil.Bytes, skips *[]uint, out provider.Output) error {
	for index, value := range out.Inputs {
		if index > 4095 {
			return errors.New("input index exceeds limit")
		}
		if _, ok := inputs[index]; ok {
			return errors.Errorf("conflicting providers for input %d", index)
		}
		inputs[index] = value
	}
	for key, value := range out.Actions {
		if _, ok := actions[key]; ok {
			return errors.Errorf("conflicting providers for action %s", key)
		}
		actions[key] = value
	}
	for _, step := range out.SkipSteps {
		for _, old := range *skips {
			if old == step {
				return errors.New("duplicate skipped step")
			}
		}
		*skips = append(*skips, step)
	}
	return nil
}

// Validate re-simulates immutable prepared bytes at current state before admission.
func (p *Planner) Validate(ctx context.Context, plan Plan) error {
	if err := recipes.ValidateSnapshot(ctx, p.Chain, plan.Recipe, p.Caller); err != nil {
		return err
	}
	_, err := p.Chain.CallContract(ctx, ethereum.CallMsg{From: p.Caller, To: &plan.Snapshot.Executor, Data: plan.Calldata}, nil)
	if err != nil {
		return errors.New("prepared recipe no longer simulates")
	}
	return nil
}

func (p *Planner) ConfirmedSnapshot(ctx context.Context, original provider.Snapshot) (provider.Snapshot, error) {
	witness, err := p.Chain.HeaderByNumber(ctx, nil)
	if err != nil {
		return original, errors.Errorf("read confirmation head: %w", err)
	}
	if err := validHeader(witness, nil); err != nil {
		return original, err
	}
	number := new(big.Int).Sub(witness.Number, new(big.Int).SetUint64(p.Confirmations))
	if number.Sign() < 0 {
		return original, errors.New("insufficient blocks for confirmations")
	}
	header, err := p.confirmedAncestor(ctx, witness, number)
	if err != nil {
		return original, err
	}
	original.BlockNumber = new(big.Int).Set(header.Number)
	original.BlockHash = header.Hash()
	original.Timestamp = header.Time
	return original, nil
}

// VerifyConfirmedSnapshot checks provider reads still refer to a canonical block
// with the configured number of descendants before their progress is committed.
func (p *Planner) VerifyConfirmedSnapshot(ctx context.Context, snapshot provider.Snapshot) error {
	if snapshot.BlockNumber == nil || snapshot.BlockNumber.Sign() < 0 || snapshot.BlockHash == (common.Hash{}) {
		return errors.New("invalid confirmed snapshot")
	}
	witnessNumber := new(big.Int).Add(snapshot.BlockNumber, new(big.Int).SetUint64(p.Confirmations))
	witness, err := p.Chain.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return errors.Errorf("read snapshot confirmation block: %w", err)
	}
	if err := validHeader(witness, witnessNumber); err != nil {
		return err
	}
	header, err := p.confirmedAncestor(ctx, witness, snapshot.BlockNumber)
	if err != nil {
		return err
	}
	if header.Hash() != snapshot.BlockHash || header.Time != snapshot.Timestamp {
		return errors.New("confirmed snapshot was reorganized")
	}
	return nil
}

func (p *Planner) confirmedAncestor(ctx context.Context, witness *types.Header, number *big.Int) (*types.Header, error) {
	ancestor := witness
	for ancestor.Number.Cmp(number) > 0 {
		parent, err := p.Chain.HeaderByHash(ctx, ancestor.ParentHash)
		if err != nil {
			return nil, errors.Errorf("read confirmation ancestor: %w", err)
		}
		expected := new(big.Int).Sub(ancestor.Number, big.NewInt(1))
		if err := validHeader(parent, expected); err != nil {
			return nil, err
		}
		if parent.Hash() != ancestor.ParentHash {
			return nil, errors.New("inconsistent confirmation ancestry")
		}
		ancestor = parent
	}
	stable, err := p.Chain.HeaderByNumber(ctx, witness.Number)
	if err != nil {
		return nil, errors.Errorf("recheck canonical confirmation block: %w", err)
	}
	if err := validHeader(stable, witness.Number); err != nil {
		return nil, err
	}
	if stable.Hash() != witness.Hash() {
		return nil, errors.New("confirmation block was reorganized")
	}
	return ancestor, nil
}

func validHeader(header *types.Header, number *big.Int) error {
	if header == nil || header.Number == nil || !header.Number.IsUint64() {
		return errors.New("invalid confirmation header")
	}
	if number != nil && header.Number.Cmp(number) != 0 {
		return errors.New("RPC returned another confirmation block")
	}
	return nil
}
