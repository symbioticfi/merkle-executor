package keeper

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type validationReader struct {
	t                  *testing.T
	snapshot           recipes.Snapshot
	header             *types.Header
	reads, simulations int
	payload            []byte
	returned           []byte
	simulationErr      error
	reorgOnSimulation  bool
}

func (r *validationReader) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	r.reads++
	return r.header, nil
}
func (r *validationReader) HeaderByHash(context.Context, common.Hash) (*types.Header, error) {
	r.reads++
	return r.header, nil
}
func (r *validationReader) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	r.reads++
	return nil, errors.New("unexpected logs")
}
func (r *validationReader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.reads++
	if bytes.Equal(msg.Data, r.payload) {
		r.simulations++
		if msg.To == nil || *msg.To != r.snapshot.Executor || msg.From != r.snapshot.Caller || block == nil || block.Cmp(r.header.Number) != 0 || msg.Gas != min(r.header.GasLimit, uint64(16_777_216)) {
			r.t.Fatal("validation changed exact send target/caller/block/gas")
		}
		if r.reorgOnSimulation {
			r.header = types.CopyHeader(r.header)
			r.header.Extra = []byte("reorg after simulation")
		}
		return r.returned, r.simulationErr
	}
	if block == nil || block.Uint64() != r.snapshot.BlockNumber {
		r.t.Fatal("metadata not pinned")
	}
	if msg.To != nil && *msg.To == r.snapshot.Account {
		return account.NewMerkleAccount().GetABI().Methods["root"].Outputs.Pack(r.snapshot.Root)
	}
	a := executor.NewRecipeExecutor().GetABI()
	method, err := a.MethodById(msg.Data[:4])
	if err != nil {
		return nil, err
	}
	var value any
	switch method.Name {
	case "account":
		value = r.snapshot.Account
	case "caller":
		value = r.snapshot.Caller
	case "recipeHash":
		value = r.snapshot.RecipeHash
	case "recipeBlock":
		value = new(big.Int).SetUint64(r.snapshot.RecipeBlock)
	default:
		r.t.Fatal("unexpected getter")
	}
	return method.Outputs.Pack(value)
}
func TestPlanMemberRejectsDifferentDiscoveryAndSendExecutors(t *testing.T) {
	strategy := Strategy{Config: Config{Account: common.Address{2}.Hex(), Executor: common.Address{1}.Hex()}}
	plan := Plan{Snapshot: provider.Snapshot{Executor: common.Address{1}, Account: common.Address{2}, Caller: common.Address{3}}, Recipe: recipes.Snapshot{Executor: common.Address{1}, Account: common.Address{2}, Caller: common.Address{3}}}
	if err := validatePlanMember(plan, strategy); err != nil {
		t.Fatal(err)
	}
	plan.Snapshot.Executor = common.Address{99}
	if err := validatePlanMember(plan, strategy); err == nil {
		t.Fatal("mismatched send target accepted")
	}
}
