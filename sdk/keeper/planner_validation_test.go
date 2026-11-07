package keeper

import (
	"bytes"
	"context"
	"math/big"
	"strings"
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
		if msg.To == nil || *msg.To != r.snapshot.Executor || msg.From != r.snapshot.Caller || block != nil {
			r.t.Fatal("validation changed exact send target/caller/latest payload")
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
func TestPlannerValidateRejectsDifferentDiscoveryAndSendExecutors(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(map[bool]string{false: "same", true: "different"}[different], func(t *testing.T) {
			header := &types.Header{Number: big.NewInt(12)}
			snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: common.HexToHash("0x04"), Root: common.HexToHash("0x05"), RecipeBlock: 11}
			payload, returned := validationWork(t, true)
			r := &validationReader{t: t, snapshot: snapshot, header: header, payload: payload, returned: returned}
			plan := Plan{Recipe: snapshot, Snapshot: provider.Snapshot{Executor: snapshot.Executor}, Calldata: r.payload}
			if different {
				plan.Snapshot.Executor = common.HexToAddress("0x99")
			}
			planner := Planner{Chain: r, Caller: snapshot.Caller}
			err := planner.Validate(t.Context(), plan)
			if different {
				if err == nil || !strings.Contains(err.Error(), "executor") || r.reads != 0 {
					t.Fatalf("inconsistent destination reached RPC: %v reads%d", err, r.reads)
				}
			} else if err != nil || r.simulations != 1 {
				t.Fatalf("legitimate plan rejected: %v simulations%d", err, r.simulations)
			}
		})
	}
}
