package runtime

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"testing"
)

type testIntegration struct {
	name  string
	calls int
	ops   []provider.Operation
}

func (p *testIntegration) ID() string   { return p.name }
func (*testIntegration) Policy() string { return "test/v1" }
func (p *testIntegration) Compile(provider.Program) ([]provider.Operation, error) {
	p.calls++
	return p.ops, nil
}
func (p *testIntegration) Restore(r provider.Record) (provider.Operation, error) {
	for _, op := range p.ops {
		if op.ID() == r.ID {
			return op, nil
		}
	}
	return nil, nil
}

type testOperation struct {
	id    string
	req   provider.Requirements
	calls int
}

func (p *testOperation) ID() string                          { return p.id }
func (p *testOperation) Requirements() provider.Requirements { return p.req }
func (p *testOperation) Prepare(context.Context, provider.Preparation) (provider.Output, error) {
	p.calls++
	return provider.Output{}, nil
}
func (*testOperation) Advance(context.Context, chainio.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}

func TestCompileStableIdentityAndUnmatchedOperations(t *testing.T) {
	op := &testOperation{id: "3:0", req: provider.Requirements{Inputs: []uint{2}}}
	a, b := &testIntegration{name: "alpha", ops: []provider.Operation{op}}, &testIntegration{name: "beta"}
	for _, list := range [][]provider.Integration{{a, b}, {b, a}} {
		compiled, err := compileOperations(provider.Program{}, list)
		if err != nil || len(compiled) != 1 || compiled[0].record.Integration != "alpha" || compiled[0].record.ID != "3:0" {
			t.Fatalf("compile: %#v %v", compiled, err)
		}
	}
	if a.calls != 2 || b.calls != 2 || op.calls != 0 {
		t.Fatal("compile repeated or performed protocol I/O")
	}
}
func TestCompileRejectsDuplicateIdentityAndInputBeforePrepare(t *testing.T) {
	for _, tc := range []struct {
		name string
		list []provider.Integration
	}{
		{"integration", []provider.Integration{&testIntegration{name: "a"}, &testIntegration{name: "a"}}},
		{"operation", []provider.Integration{&testIntegration{name: "a", ops: []provider.Operation{&testOperation{id: "0:0"}, &testOperation{id: "0:0"}}}}},
		{"input", []provider.Integration{&testIntegration{name: "a", ops: []provider.Operation{&testOperation{id: "0:0", req: provider.Requirements{Inputs: []uint{1}}}, &testOperation{id: "1:0", req: provider.Requirements{Inputs: []uint{1}}}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := compileOperations(provider.Program{}, tc.list); err == nil {
				t.Fatal("conflicting operation accepted")
			}
		})
	}
}
func TestPreparedOutputCannotExceedDeclaredOwnership(t *testing.T) {
	if err := validateOutput(provider.Requirements{}, provider.Output{Inputs: map[uint]string{0: "1"}}); err == nil {
		t.Fatal("undeclared input accepted")
	}
	if err := validateOutput(provider.Requirements{Inputs: []uint{0}}, provider.Output{Inputs: map[uint]string{0: new(big.Int).Lsh(big.NewInt(1), 256).String()}}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestSkipOwnershipIncludesEveryManagedAction(t *testing.T) {
	word, _ := abi.NewType("uint256", "", nil)
	hashes, _ := abi.NewType("bytes32[]", "", nil)
	bindings, _ := abi.NewType("uint256[][]", "", nil)
	data, err := (abi.Arguments{{Type: word}, {Type: hashes}, {Type: bindings}}).Pack(big.NewInt(0), [][32]byte{{1}, {2}}, [][]*big.Int{{}, {}})
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Manage}, data...)}}
	for _, tc := range []struct {
		name    string
		actions []string
		ok      bool
	}{
		{"partial", []string{"0:0"}, false},
		{"complete", []string{"0:0", "0:1"}, true},
		{"unowned", []string{"1:0"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSkipOwnership(recipe, provider.Requirements{Actions: tc.actions}, provider.Output{SkipSteps: []uint{0}})
			if (err == nil) != tc.ok {
				t.Fatalf("valid=%v error=%v", tc.ok, err)
			}
		})
	}
}
