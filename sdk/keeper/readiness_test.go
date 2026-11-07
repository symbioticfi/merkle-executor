package keeper

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func validationWork(t *testing.T, ready bool) ([]byte, []byte) {
	t.Helper()
	var args abi.Arguments
	for _, name := range []string{"uint256", "bytes32[]", "uint256[][]"} {
		typ, err := abi.NewType(name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args = append(args, abi.Argument{Type: typ})
	}
	gate := new(big.Int)
	if ready {
		gate.SetInt64(1)
	}
	step, err := args.Pack(gate, [][32]byte{common.HexToHash("0x1234")}, [][]*big.Int{{}})
	if err != nil {
		t.Fatal(err)
	}
	a := executor.NewRecipeExecutor()
	calldata := a.PackExecute(nil, [][]byte{append([]byte{recipes.Manage}, step...)}, nil, [][]byte{{}})
	returned, err := a.GetABI().Methods["execute"].Outputs.Pack([]*big.Int{new(big.Int)})
	if err != nil {
		t.Fatal(err)
	}
	return calldata, returned
}

func TestPlannerValidateWorkAfterCanonicalGuards(t *testing.T) {
	for _, mode := range []string{"ready", "idle", "idle reorg", "idle simulation failure", "malformed results"} {
		t.Run(mode, func(t *testing.T) {
			header := &types.Header{Number: big.NewInt(12)}
			snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.HexToAddress("0x01"), Account: common.HexToAddress("0x02"), Caller: common.HexToAddress("0x03"), RecipeHash: common.HexToHash("0x04"), Root: common.HexToHash("0x05"), RecipeBlock: 11}
			payload, returned := validationWork(t, mode == "ready")
			r := &validationReader{t: t, snapshot: snapshot, header: header, payload: payload, returned: returned, reorgOnSimulation: mode == "idle reorg"}
			if mode == "idle simulation failure" {
				r.simulationErr = errors.New("assertion failed")
			}
			if mode == "malformed results" {
				r.returned = nil
			}
			planner := Planner{Chain: r, Caller: snapshot.Caller}
			err := planner.Validate(t.Context(), Plan{Recipe: snapshot, Snapshot: provider.Snapshot{Executor: snapshot.Executor}, Calldata: payload})
			if mode == "ready" {
				if err != nil {
					t.Fatal(err)
				}
			} else if mode == "idle" {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("expected idle: %v", err)
				}
			} else if err == nil || errors.Is(err, provider.ErrNotReady) {
				t.Fatalf("failed execution became idle: %v", err)
			}
			if r.simulations != 1 {
				t.Fatalf("full original execution count: %d", r.simulations)
			}
		})
	}
}
