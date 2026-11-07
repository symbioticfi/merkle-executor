package provider

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Program is the authenticated immutable program from which integrations compile.
type Program struct {
	Snapshot chainio.Snapshot `json:"snapshot"`
	Recipe   recipes.Recipe   `json:"recipe"`
	Tree     *recipes.Tree    `json:"tree"`
}

// Requirements declares the complete numerical and assignment ownership of one operation.
type Requirements struct {
	Values           []recipes.Value           `json:"values,omitempty"`
	Inputs           []uint                    `json:"inputs,omitempty"`
	Actions          []string                  `json:"actions,omitempty"`
	ExclusiveTargets map[string]common.Address `json:"exclusiveTargets,omitempty"`
}

// Integration compiles once per authenticated program. IDs and policies identify durable records.
type Integration interface {
	ID() string
	Policy() string
	Compile(Program) ([]Operation, error)
	Restore(Record) (Operation, error)
}

// Operation owns one protocol route and its settlement evidence.
type Operation interface {
	ID() string
	Requirements() Requirements
	Prepare(context.Context, Preparation) (Output, error)
	Advance(context.Context, chainio.Snapshot, Output, json.RawMessage) (Progress, error)
}

// Preparation supplies resolved dependencies and an attempt-scoped shared chain reader.
type Preparation struct {
	Program Program
	Results map[uint]*big.Int
	Reader  chainio.Reader
}

func (p Program) Clone() Program {
	p.Snapshot.ChainID = cloneInt(p.Snapshot.ChainID)
	p.Snapshot.BlockNumber = cloneInt(p.Snapshot.BlockNumber)
	p.Snapshot.Execution = p.Snapshot.Execution.Clone()
	p.Recipe.Queries = cloneBytes(p.Recipe.Queries)
	p.Recipe.Steps = cloneBytes(p.Recipe.Steps)
	if p.Tree != nil {
		tree := *p.Tree
		tree.ChainID = cloneInt(tree.ChainID)
		if tree.Account != nil {
			account := *tree.Account
			tree.Account = &account
		}
		tree.Leaves = append([]recipes.Leaf(nil), tree.Leaves...)
		for i := range tree.Leaves {
			leaf := &tree.Leaves[i]
			leaf.Value = cloneInt(leaf.Value)
			leaf.Data = append([]byte(nil), leaf.Data...)
			leaf.DataMask = append([]byte(nil), leaf.DataMask...)
			leaf.Proof = append([]common.Hash(nil), leaf.Proof...)
		}
		p.Tree = &tree
	}
	return p
}
func (r Requirements) Clone() Requirements {
	r.Values = append([]recipes.Value(nil), r.Values...)
	for i := range r.Values {
		r.Values[i].Value = cloneInt(r.Values[i].Value)
	}
	r.Inputs = append([]uint(nil), r.Inputs...)
	r.Actions = append([]string(nil), r.Actions...)
	if r.ExclusiveTargets != nil {
		targets := make(map[string]common.Address, len(r.ExclusiveTargets))
		for key, value := range r.ExclusiveTargets {
			targets[key] = value
		}
		r.ExclusiveTargets = targets
	}
	return r
}
func cloneInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}
func cloneBytes(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	result := make([][]byte, len(values))
	for i, value := range values {
		result[i] = append([]byte(nil), value...)
	}
	return result
}
