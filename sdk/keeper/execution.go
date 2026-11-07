package keeper

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type PlanMember struct {
	Name string `json:"name"`
	Plan Plan   `json:"plan"`
}
type ExecutionPlan struct {
	To      common.Address `json:"to"`
	Members []PlanMember   `json:"members"`
}

// Transaction encodes a direct call or an ordered delegated self-call.
func (b ExecutionPlan) Transaction() (common.Address, []byte, error) {
	if len(b.Members) == 0 {
		return common.Address{}, nil, errors.New("empty executor batch")
	}
	caller := b.Members[0].Plan.Snapshot.Caller
	if caller == (common.Address{}) {
		return caller, nil, errors.New("executor batch caller is zero")
	}
	calls := make([]recipes.ExecutorCall, len(b.Members))
	names := make(map[string]bool, len(b.Members))
	for i, member := range b.Members {
		if member.Name == "" || names[member.Name] {
			return caller, nil, errors.New("executor batch has an empty or duplicate member")
		}
		names[member.Name] = true
		if member.Plan.Snapshot.Executor == (common.Address{}) {
			return b.To, nil, errors.New("execution plan executor is zero")
		}
		if member.Plan.Snapshot.Caller != caller {
			return caller, nil, errors.New("executor batch mixes callers")
		}
		if _, err := recipes.Executions(member.Plan.Calldata); err != nil {
			return b.To, nil, err
		}
		calls[i] = recipes.ExecutorCall{Executor: member.Plan.Snapshot.Executor, Data: member.Plan.Calldata}
	}
	if len(calls) == 1 && b.To == calls[0].Executor {
		return b.To, bytes.Clone(calls[0].Data), nil
	}
	if b.To != caller {
		return b.To, nil, errors.New("execution plan target does not match its caller")
	}
	data, err := recipes.DelegatedMulticall(calls)
	return b.To, data, err
}

func validatePlanMember(plan Plan, strategy Strategy) error {
	if plan.Snapshot.Account != common.HexToAddress(strategy.Config.Account) || plan.Snapshot.Executor != common.HexToAddress(strategy.Config.Executor) || plan.Recipe.Account != plan.Snapshot.Account || plan.Recipe.Executor != plan.Snapshot.Executor || plan.Recipe.Caller != plan.Snapshot.Caller {
		return errors.New("executor batch member identity mismatch")
	}
	return nil
}
