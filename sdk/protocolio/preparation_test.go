package protocolio

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"math/big"
	"testing"
)

type ordinaryPreparation struct{ run func() }

func (p ordinaryPreparation) Prepare(context.Context, provider.Snapshot) (provider.Output, error) {
	p.run()
	return provider.Output{}, nil
}
func (ordinaryPreparation) Advance(context.Context, provider.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{}, nil
}

type stagedPreparation struct {
	ordinaryPreparation
	stage func(PreparationPin) (PreparationPlan, error)
}

func (p stagedPreparation) PlanPreparation(pin PreparationPin) (PreparationPlan, error) {
	return p.stage(pin)
}
func preparationFixture() (PreparationPin, provider.Snapshot) {
	pin := PreparationPin{ChainID: "18446744073709551617", BlockNumber: 7, BlockHash: common.HexToHash("0x01"), Timestamp: 9, Executor: common.HexToAddress("0x02"), Account: common.HexToAddress("0x03"), Caller: common.HexToAddress("0x04")}
	chain, _ := new(big.Int).SetString(pin.ChainID, 10)
	return pin, provider.Snapshot{ChainID: chain, BlockNumber: big.NewInt(7), BlockHash: pin.BlockHash, Timestamp: pin.Timestamp, Executor: pin.Executor, Account: pin.Account, Caller: pin.Caller}
}
func TestPreparationRoutesAssignedResultsAndPreservesOrder(t *testing.T) {
	pin, s := preparationFixture()
	var order []int
	staged := func(index, n int) provider.Provider {
		return stagedPreparation{stage: func(got PreparationPin) (PreparationPlan, error) {
			if got != pin {
				t.Fatal("provisional pin changed")
			}
			plan := PreparationPlan{Resolve: func(_ context.Context, got provider.Snapshot, read ReadAccessor) (provider.Output, error) {
				order = append(order, index)
				if !pin.Matches(got) {
					t.Fatal("unauthenticated snapshot")
				}
				for i := range n {
					if read(i).Data[0] != byte(index+i) {
						t.Fatal("wrong provider result")
					}
				}
				if read(n).Err == nil || read(-1).Err == nil {
					t.Fatal("unbounded provider accessor")
				}
				return provider.Output{}, nil
			}}
			for range n {
				plan.Requests = append(plan.Requests, ContractRead(pin.Account, []byte{byte(index)}, big.NewInt(7), false))
			}
			return plan, nil
		}}
	}
	p := PlanPreparation([]provider.Provider{staged(1, 2), ordinaryPreparation{func() { order = append(order, 8) }}, staged(3, 1)}, pin)
	if len(p.Requests()) != 3 || len(order) != 0 {
		t.Fatal("planning performed provider work")
	}
	for i := range 3 {
		if _, err := p.Resolve(t.Context(), i, s, func(i int) ReadResult { return ReadResult{Data: []byte{byte(i + 1)}} }); err != nil {
			t.Fatal(err)
		}
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 8 || order[2] != 3 {
		t.Fatal(order)
	}
}
func TestPreparationRejectsIdentityBeforeResolver(t *testing.T) {
	pin, s := preparationFixture()
	for _, field := range []string{"chain", "block", "hash", "time", "executor", "account", "caller"} {
		t.Run(field, func(t *testing.T) {
			bad := s
			switch field {
			case "chain":
				bad.ChainID = big.NewInt(1)
			case "block":
				bad.BlockNumber = big.NewInt(8)
			case "hash":
				bad.BlockHash = common.Hash{}
			case "time":
				bad.Timestamp++
			case "executor":
				bad.Executor = common.Address{}
			case "account":
				bad.Account = common.Address{}
			case "caller":
				bad.Caller = common.Address{}
			}
			p := PlanPreparation([]provider.Provider{stagedPreparation{stage: func(PreparationPin) (PreparationPlan, error) {
				return PreparationPlan{Resolve: func(context.Context, provider.Snapshot, ReadAccessor) (provider.Output, error) {
					t.Fatal("resolver reached")
					return provider.Output{}, nil
				}}, nil
			}}}, pin)
			if _, err := p.Resolve(t.Context(), 0, bad, nil); err == nil {
				t.Fatal("accepted changed identity")
			}
		})
	}
}
func TestPreparationDefersPlanErrorsAndRequiresPinnedCalls(t *testing.T) {
	pin, s := preparationFixture()
	for _, mode := range []string{"plan error", "nil resolver", "latest", "other pin", "noncall"} {
		t.Run(mode, func(t *testing.T) {
			p := PlanPreparation([]provider.Provider{stagedPreparation{stage: func(PreparationPin) (PreparationPlan, error) {
				if mode == "plan error" {
					return PreparationPlan{}, errors.New("local plan failed")
				}
				plan := PreparationPlan{Requests: []ReadRequest{ContractRead(pin.Account, nil, big.NewInt(7), false)}, Resolve: func(context.Context, provider.Snapshot, ReadAccessor) (provider.Output, error) {
					t.Fatal("invalid plan resolved")
					return provider.Output{}, nil
				}}
				switch mode {
				case "nil resolver":
					plan.Resolve = nil
				case "latest":
					plan.Requests[0].Block = nil
				case "other pin":
					plan.Requests[0].Block = big.NewInt(8)
				case "noncall":
					plan.Requests[0].Kind = ReadHeader
				}
				return plan, nil
			}}}, pin)
			if len(p.Requests()) != 0 {
				t.Fatal("invalid plan submitted reads")
			}
			if _, err := p.Resolve(t.Context(), 0, s, nil); err == nil {
				t.Fatal("lost plan error")
			}
		})
	}
}
