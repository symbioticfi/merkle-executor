package keeper

import (
	"bytes"
	"maps"
	"math/big"
	"slices"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func cloneSnapshot(s provider.Snapshot) provider.Snapshot {
	if s.ChainID != nil {
		s.ChainID = new(big.Int).Set(s.ChainID)
	}
	if s.BlockNumber != nil {
		s.BlockNumber = new(big.Int).Set(s.BlockNumber)
	}
	s.Execution = s.Execution.Clone()
	return s
}

func freezeExecution(plan ExecutionPlan) ExecutionPlan {
	plan.Members = slices.Clone(plan.Members)
	for i := range plan.Members {
		p := &plan.Members[i].Plan
		p.Snapshot = cloneSnapshot(p.Snapshot)
		p.Calldata = bytes.Clone(p.Calldata)
		p.Leaves = slices.Clone(p.Leaves)
		p.Results = maps.Clone(p.Results)
		p.Outputs = slices.Clone(p.Outputs)
		for j, group := range p.Outputs {
			p.Outputs[j] = make([]provider.Output, len(group))
			for k, out := range group {
				p.Outputs[j][k] = out.Clone()
			}
		}
	}
	return plan
}
