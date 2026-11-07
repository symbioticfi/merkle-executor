package redemptions

import (
	"context"
	"maps"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	runtimeSDK "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

func TestIntegrationCompilesStableRouteAndRestoresWithoutRecipe(t *testing.T) {
	reader := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	integration, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	cfg := automaticConfig(integration.(*automatic).assets[0], "request")
	snapshot, reader := issuerFixture(t, cfg)
	integration, err = NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
	program := provider.Program{Snapshot: snapshot, Recipe: recipe, Tree: tree}
	operations, err := integration.Compile(program)
	if err != nil || len(operations) != 1 || reader.reads != 0 {
		t.Fatalf("compile: %v ops=%d reads=%d", err, len(operations), reader.reads)
	}
	operation := operations[0]

	for _, field := range []string{"chain", "block", "hash", "time", "executor", "account", "caller", "recipe", "root"} {
		t.Run("compiled identity/"+field, func(t *testing.T) {
			changed := snapshot
			switch field {
			case "chain":
				changed.ChainID = big.NewInt(10)
			case "block":
				changed.BlockNumber = big.NewInt(43)
			case "hash":
				changed.BlockHash = common.HexToHash("0xbad")
			case "time":
				changed.Timestamp++
			case "executor":
				changed.Executor = issuerAddress(123)
			case "account":
				changed.Account = issuerAddress(123)
			case "caller":
				changed.Caller = issuerAddress(123)
			case "recipe":
				changed.RecipeHash = common.HexToHash("0xbad")
			case "root":
				changed.Root = common.HexToHash("0xbad")
			}
			before := reader.reads
			if _, err := operation.Prepare(t.Context(), provider.Preparation{Program: provider.Program{Snapshot: changed}, Results: map[uint]*big.Int{0: big.NewInt(100)}, Reader: reader}); err == nil || reader.reads != before {
				t.Fatalf("changed identity reached preparation: %v reads=%d", err, reader.reads)
			}
		})
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := operation.Prepare(canceled, provider.Preparation{Program: provider.Program{Snapshot: snapshot}, Results: map[uint]*big.Int{0: big.NewInt(100)}, Reader: reader}); err != context.Canceled || reader.reads != 0 {
		t.Fatalf("canceled preparation reached I/O: %v", err)
	}
	if operation.ID() != cfg.Action {
		t.Fatalf("unstable action identity %q", operation.ID())
	}
	// Compile owns its interpretation; later permission mutation cannot change preparation.
	program.Recipe.Steps = nil
	program.Tree.Leaves[0].Data[0] ^= 1
	output, err := operation.Prepare(t.Context(), provider.Preparation{Program: provider.Program{Snapshot: snapshot}, Results: map[uint]*big.Int{0: big.NewInt(100)}, Reader: reader})
	if err != nil {
		t.Fatal(err)
	}
	record := provider.Record{Integration: integration.ID(), Policy: integration.Policy(), ID: operation.ID(), Requirements: operation.Requirements(), Output: output, Progress: provider.Progress{Done: true}}
	before := reader.reads
	restored, err := integration.Restore(record)
	if err != nil || restored.ID() != operation.ID() || reader.reads != before {
		t.Fatalf("restore: %v reads=%d", err, reader.reads)
	}
	record.Requirements.ExclusiveTargets = nil
	if _, err = integration.Restore(record); err == nil || reader.reads != before {
		t.Fatal("completed operation lost receipt ownership")
	}
}

func TestAutomaticIndependentRoutesShareSchedulerAndRecoverStableIdentity(t *testing.T) {
	initial := &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}
	raw, err := NewAutomatic(registry.Deps{Chain: initial})
	if err != nil {
		t.Fatal(err)
	}
	var assets []issuerAsset
	for _, asset := range raw.(*automatic).assets {
		if (asset.Family == "midas" || asset.Family == "centrifuge") && len(assets) < 2 {
			if len(assets) == 0 || assets[0].Family != asset.Family {
				assets = append(assets, asset)
			}
		}
	}
	if len(assets) != 2 {
		t.Fatal("fixture routes absent")
	}
	var leaves []recipes.Leaf
	var queries [][]byte
	var snapshot chainio.Snapshot
	for _, asset := range assets {
		cfg := automaticConfig(asset, "request")
		s, reader := issuerFixture(t, cfg)
		snapshot = s
		maps.Copy(initial.results, reader.results)
		initial.headers = reader.headers
		r, tree := automaticRecipe(t, cfg, s, reader)
		queries = append(queries, r.Queries[0])
		leaves = append(leaves, tree.Leaves[0])
	}
	manage := abi.Arguments{{Type: argumentType("uint256")}, {Type: argumentType("bytes32[]")}, {Type: argumentType("uint256[][]")}}
	recipe := recipes.Recipe{Queries: queries, Steps: [][]byte{packedStep(t, recipes.Read, readArguments, big.NewInt(0)), packedStep(t, recipes.Read, readArguments, big.NewInt(1))}}
	for i, leaf := range leaves {
		offset := uint32(4)
		if assets[i].Family == "midas" {
			offset = 36
		}
		recipe.Steps = append(recipe.Steps, packedStep(t, recipes.Manage, manage, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{packedBinding(offset, recipes.Result, uint64(i))}}))
	}
	tree := &recipes.Tree{ChainID: snapshot.ChainID, Leaves: leaves}
	sealAutomaticTree(t, &recipe, tree)
	integration, err := NewAutomatic(registry.Deps{Chain: initial})
	if err != nil {
		t.Fatal(err)
	}
	program := provider.Program{Snapshot: snapshot, Recipe: recipe, Tree: tree}
	operations, err := integration.Compile(program)
	if err != nil || len(operations) != 2 || initial.reads != 0 {
		t.Fatalf("compile routes %d %v", len(operations), err)
	}
	requirements := []provider.Requirements{operations[0].Requirements(), operations[1].Requirements()}
	if err = runtimeSDK.ValidateComposition(recipe, tree, requirements); err != nil {
		t.Fatal(err)
	}
	backend := &batchIssuerReader{issuerReader: initial, failIndex: -1}
	scheduler, err := chainio.NewScheduler(backend, chainio.Limits{ReadTimeout: time.Second, MaxMembers: 256, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxCalldataBytes: 1 << 20, MaxGas: 10000000, MaxOutputBytes: 1 << 20, DefaultCallGas: 100000, DefaultOutputBytes: 4096, MaxConcurrent: 1, MaxQueuedRequests: 256, MaxQueuedBytes: 1 << 20, CollectionWindow: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- scheduler.Run(ctx) }()
	defer func() { cancel(); _ = scheduler.Close(); <-done }()
	outputs := make([]provider.Output, 2)
	errs := make(chan error, 2)
	for i, operation := range operations {
		go func() {
			var e error
			outputs[i], e = operation.Prepare(ctx, provider.Preparation{Program: program, Results: map[uint]*big.Int{0: big.NewInt(100), 1: big.NewInt(100)}, Reader: scheduler})
			errs <- e
		}()
	}
	for range operations {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if len(backend.groups) != 1 || backend.groups[0] != 6 {
		t.Fatalf("automatic routes were not coalesced: %v", backend.groups)
	}
	reordered := integration.(*automatic)
	slices.Reverse(reordered.assets)
	bundle := durableRoutes{Account: snapshot.Account}
	merged := provider.Output{Inputs: map[uint]string{}, Actions: map[string]hexutil.Bytes{}}
	for i, operation := range operations {
		record := provider.Record{Integration: integration.ID(), Policy: integration.Policy(), ID: operation.ID(), Requirements: operation.Requirements(), Output: outputs[i], Progress: provider.Progress{Done: true, Outcome: "requested"}}
		restored, e := reordered.Restore(record)
		if e != nil || restored.ID() != operation.ID() {
			t.Fatalf("reordered restore %v", e)
		}
		var envelope routeEnvelope
		inner, e := provider.DecodeOperation(integration.Policy(), outputs[i], &envelope)
		if e != nil {
			t.Fatal(e)
		}
		bundle.Routes = append(bundle.Routes, durableRoute{Config: envelope.Config, Output: inner})
		maps.Copy(merged.Inputs, inner.Inputs)
		maps.Copy(merged.Actions, inner.Actions)
	}
	legacy, err := derived.Encode(PolicyVersion, bundle, merged, true)
	if err != nil {
		t.Fatal(err)
	}
	before := initial.reads
	imported, err := ImportLegacy(registry.Deps{Chain: initial}, snapshot, legacy, provider.Progress{Done: true, Outcome: "requested"})
	if err != nil || len(imported) != 2 || initial.reads != before {
		t.Fatalf("offline import %+v %v", imported, err)
	}
	wrongChain := snapshot
	wrongChain.ChainID = big.NewInt(42161)
	if _, err = ImportLegacy(registry.Deps{Chain: initial}, wrongChain, legacy, provider.Progress{}); err == nil || initial.reads != before {
		t.Fatal("legacy chain mismatch accepted or reached I/O")
	}
	for i, record := range imported {
		if record.ID != operations[i].ID() || !record.Progress.Done || !maps.Equal(record.Requirements.ExclusiveTargets, operations[i].Requirements().ExclusiveTargets) {
			t.Fatalf("legacy route identity or claims changed: %+v", record)
		}
	}
	legacy.Actions[operations[0].ID()][0] ^= 1
	if _, err = ImportLegacy(registry.Deps{Chain: initial}, snapshot, legacy, provider.Progress{}); err == nil || initial.reads != before {
		t.Fatal("legacy aggregate corruption reached I/O")
	}
}

func TestAutomaticIssuerUnmatchedHasNoOperation(t *testing.T) {
	raw, err := NewAutomatic(registry.Deps{Chain: &issuerReader{t: t, block: big.NewInt(42), results: map[string][]byte{}}})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := raw.Compile(provider.Program{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), Account: issuerAddress(99)}, Tree: &recipes.Tree{}})
	if err != nil || len(operations) != 0 {
		t.Fatalf("unmatched compile %+v %v", operations, err)
	}
}
