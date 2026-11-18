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
	"github.com/symbioticfi/merkle-executor/sdk/bindings/accessmanager"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type forwardingReader struct {
	*validationReader
	sender     common.Address
	immediate  bool
	delay      uint32
	outerCalls int
	reorg      bool
	reject     bool
}

func (r *forwardingReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	manager := accessmanager.NewAccessManager()
	if msg.To != nil && *msg.To == r.snapshot.Caller {
		if block == nil || block.Cmp(r.header.Number) != 0 {
			r.t.Fatal("forwarding not pinned to admission block")
		}
		if bytes.Equal(msg.Data, manager.PackCanCall(r.sender, r.snapshot.Executor, [4]byte(r.payload[:4]))) {
			return manager.GetABI().Methods["canCall"].Outputs.Pack(r.immediate, r.delay)
		}
		if msg.From != r.sender || !bytes.Equal(msg.Data, manager.PackExecute(r.snapshot.Executor, r.payload)) || msg.Gas != 16_777_216 {
			r.t.Fatal("changed forwarding envelope")
		}
		r.outerCalls++
		if r.reorg {
			r.header = types.CopyHeader(r.header)
			r.header.Extra = []byte("reorg")
		}
		if r.reject {
			return nil, errors.New("outer transport failed")
		}
		return nil, nil
	}
	return r.validationReader.CallContract(ctx, msg, block)
}

func (r *forwardingReader) CodeAt(_ context.Context, address common.Address, block *big.Int) ([]byte, error) {
	if address != r.snapshot.Caller || block == nil || block.Cmp(r.header.Number) != 0 {
		r.t.Fatal("forwarded result probe read code outside its pinned caller")
	}
	return []byte{0}, nil
}

func (r *forwardingReader) CallContractWithCode(_ context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	if msg.From != r.sender || msg.To == nil || *msg.To != r.snapshot.Caller || block == nil || block.Cmp(r.header.Number) != 0 || len(code) == 0 {
		r.t.Fatal("forwarded result probe changed sender, caller or block")
	}
	r.simulations++
	return recipes.Arguments("bytes[]").Pack([][]byte{r.returned})
}

func TestForwardingAdmissionRetainsWitnessAndCanonicalChecks(t *testing.T) {
	for _, mode := range []string{"valid", "denied", "delayed", "changed witness", "outer failure", "reorg"} {
		t.Run(mode, func(t *testing.T) {
			recipe, payload, returned := validationWork(t, true)
			hash, _ := recipes.HashRecipe(recipe)
			header := &types.Header{Number: big.NewInt(12), GasLimit: 30_000_000}
			snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: header.Hash(), Executor: common.Address{1}, Account: common.Address{2}, Caller: common.Address{3}, RecipeHash: hash, RecipeBlock: 12}
			r := &forwardingReader{validationReader: &validationReader{t: t, snapshot: snapshot, header: header, payload: payload, returned: returned}, sender: common.Address{4}, immediate: mode != "denied"}
			if mode == "delayed" {
				r.delay = 60
			}
			r.reorg = mode == "reorg"
			r.reject = mode == "outer failure"
			witness := map[uint]string{}
			if mode == "changed witness" {
				witness[0] = "99999"
			}
			err := recipes.ValidatePreparedBatchForSender(t.Context(), r, []recipes.PreparedRecipe{{Snapshot: snapshot, Recipe: recipe}}, []recipes.ExecutorCall{{Executor: snapshot.Executor, Data: payload}}, [][]common.Hash{nil}, []map[uint]string{witness}, snapshot.Caller, r.sender)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("admission: %v", err)
			}
			if mode == "valid" && (r.simulations != 1 || r.outerCalls != 1) {
				t.Fatal("missing inner or outer simulation")
			}
			if (mode == "denied" || mode == "delayed") && r.outerCalls != 0 {
				t.Fatal("invalid candidate reached outer simulation")
			}
		})
	}
}

func TestForwardingTransactionPreservesRecipe(t *testing.T) {
	_, data, _ := validationWork(t, true)
	caller, executor := common.Address{1}, common.Address{2}
	plan := ExecutionPlan{To: caller, Sender: common.Address{3}, Members: []PlanMember{{Name: "claim", Plan: Plan{Snapshot: provider.Snapshot{Caller: caller, Executor: executor}, Calldata: data}}}}
	to, encoded, err := plan.Transaction()
	if err != nil || to != caller || !bytes.Equal(encoded, accessmanager.NewAccessManager().PackExecute(executor, data)) {
		t.Fatalf("wrong forwarding transaction: %v", err)
	}
	plan.To = executor
	if _, _, err = plan.Transaction(); err == nil || !strings.Contains(err.Error(), "AccessManager") {
		t.Fatal("accepted wrong forwarding target")
	}
}
