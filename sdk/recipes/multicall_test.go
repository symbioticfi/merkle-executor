package recipes

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestMulticallRejectsOtherMethodsAndRecipes(t *testing.T) {
	call, err := executorABI.TryPackExecute(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := executorABI.TryPackExecute(nil, [][]byte{step(Add, pack(t, binaryArgs, literal(0), literal(0)))}, nil, nil)
	for _, calls := range [][][]byte{nil, {call, executorABI.PackSetCaller(common.Address{1})}, {call, other}} {
		if _, err := Multicall(calls); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	batch, err := Multicall([][]byte{call, call})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Multicall([][]byte{batch}); err == nil {
		t.Fatal("nested batch accepted")
	}
	returned := pack(t, executorABI.GetABI().Methods["execute"].Outputs, []*big.Int{})
	work, err := HasWork(batch, pack(t, batchArguments, [][]byte{returned, returned}))
	if err != nil || work {
		t.Fatalf("work=%v err=%v", work, err)
	}
	if _, err := HasWork(batch, pack(t, batchArguments, [][]byte{returned})); err == nil {
		t.Fatal("short result accepted")
	}
}

func TestRepeatReadinessIgnoresOnlyApprovalMaintenance(t *testing.T) {
	r := Recipe{Steps: [][]byte{
		step(Approve, pack(t, approveArgs, literal(0), common.Address{1}, common.Address{2}, common.Hash{3})),
		manageStep(t, common.Hash{4}, literal(0), nil),
	}}
	call, err := executorABI.TryPackExecute(r.Queries, r.Steps, nil, make([][]byte, 2))
	if err != nil {
		t.Fatal(err)
	}
	returned := pack(t, executorABI.GetABI().Methods["execute"].Outputs, []*big.Int{new(big.Int), new(big.Int)})
	if work, err := HasWork(call, returned); err != nil || !work {
		t.Fatalf("ordinary cleanup work=%v error=%v", work, err)
	}
	if work, err := hasWork(call, returned, false); err != nil || work {
		t.Fatalf("repeat cleanup work=%v error=%v", work, err)
	}
	inputCall, _ := executorABI.TryPackExecute(nil, [][]byte{step(Add, pack(t, binaryArgs, input(0), literal(1)))}, []*big.Int{literal(1)}, nil)
	if got, err := BatchExecution(t.Context(), nil, Snapshot{}, inputCall, 1); err != nil || !bytes.Equal(got, inputCall) {
		t.Fatalf("external input must remain a single execution: %v", err)
	}
}

func TestAnvilRepeatedRecipeBatch(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	client := startAnvil(t, ctx)
	reader := &lensAnvilReader{Client: client}
	var accounts []common.Address
	if err := client.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	sender, recipient := accounts[0], accounts[1]
	a, token, e := loadAnvilFixture(t, "FixtureAccount"), loadAnvilFixture(t, "FixtureToken"), loadAnvilFixture(t, "RecipeExecutor")
	account := localTransaction(t, ctx, client, sender, nil, a.Bytecode).ContractAddress
	target := localTransaction(t, ctx, client, sender, nil, token.Bytecode).ContractAddress
	executor := localTransaction(t, ctx, client, sender, nil, append(bytes.Clone(e.Bytecode), fixtureCalldata(t, e, "", sender, account)...)).ContractAddress
	data := fixtureCalldata(t, token, "transfer", recipient, new(big.Int))
	mask := make([]byte, len(data))
	for i := 36; i < len(mask); i++ {
		mask[i] = 0xff
	}
	leaf := Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
	leaf.Hash, _ = HashLeaf(leaf)
	tree := &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}
	localTransaction(t, ctx, client, sender, &account, fixtureCalldata(t, a, "configure", leaf.Hash, [][32]byte{leaf.Hash}, executor))
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetCaller(sender))
	localTransaction(t, ctx, client, sender, &target, fixtureCalldata(t, token, "mint", account, big.NewInt(25)))
	r := Recipe{Queries: [][]byte{pack(t, queryArgs, target, fixtureCalldata(t, token, "balanceOf", account), []*big.Int{}, new(big.Int))}, Steps: [][]byte{
		step(Read, pack(t, readArgs, literal(0))),
		step(Min, pack(t, binaryArgs, result(0), literal(10))),
		manageStep(t, leaf.Hash, result(1), []*big.Int{binding(36, result(1))}),
	}}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	snapshot, _, err := Discover(ctx, reader, executor, nil)
	if err != nil {
		t.Fatal(err)
	}
	call, _, err := Assemble(r, tree, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := BatchExecution(ctx, reader, snapshot, call, 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := batchCalls(batch)
	if err != nil || len(calls) != 3 {
		t.Fatalf("calls=%d err=%v", len(calls), err)
	}
	if balance := fixtureRead(t, ctx, client, token, target, "balanceOf", account)[0].(*big.Int); balance.Int64() != 25 {
		t.Fatal("simulation persisted state")
	}
	if err := ValidateExecution(ctx, reader, snapshot, sender, batch); err != nil {
		t.Fatal(err)
	}
	if work, err := ExecutionHasWork(ctx, reader, snapshot, sender, batch, nil); err != nil || !work {
		t.Fatalf("batch readiness: %v %v", work, err)
	}
	if _, err := BatchExecution(ctx, reader, snapshot, call, 21_000); err == nil {
		t.Fatal("insufficient gas admitted")
	}
	wrong := snapshot
	wrong.Caller = recipient
	if _, err := SimulateBatch(ctx, reader, wrong, calls, 5_000_000); err == nil {
		t.Fatal("wrong caller admitted")
	}
	localTransaction(t, ctx, client, sender, &executor, batch)
	if balance := fixtureRead(t, ctx, client, token, target, "balanceOf", recipient)[0].(*big.Int); balance.Int64() != 25 {
		t.Fatalf("recipient balance %s", balance)
	}
	latest, _, err := Discover(ctx, reader, executor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BatchExecution(ctx, reader, latest, call, 5_000_000); err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("idle error %v", err)
	}
	// A later failing cycle is omitted; the preceding full batch remains valid.
	localTransaction(t, ctx, client, sender, &target, fixtureCalldata(t, token, "mint", account, big.NewInt(25)))
	r.Steps[2] = manageStep(t, leaf.Hash, result(0), []*big.Int{binding(36, literal(10))})
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	latest, _, err = Discover(ctx, reader, executor, nil)
	if err != nil {
		t.Fatal(err)
	}
	call, _, err = Assemble(r, tree, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := BatchExecution(ctx, reader, latest, call, 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	prefixCalls, err := batchCalls(prefix)
	if err != nil || len(prefixCalls) != 2 {
		t.Fatalf("executable prefix: %d, %v", len(prefixCalls), err)
	}
	if err := ValidateExecution(ctx, reader, latest, sender, prefix); err != nil {
		t.Fatal(err)
	}
	if balance := fixtureRead(t, ctx, client, token, target, "balanceOf", account)[0].(*big.Int); balance.Int64() != 25 {
		t.Fatal("failed simulation persisted prefix")
	}
	r.Steps = [][]byte{manageStep(t, leaf.Hash, literal(1), []*big.Int{binding(36, literal(0))})}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	latest, _, err = Discover(ctx, reader, executor, nil)
	if err != nil {
		t.Fatal(err)
	}
	call, _, err = Assemble(r, tree, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := BatchExecution(ctx, reader, latest, call, 5_000_000); err != nil || !bytes.Equal(got, call) {
		t.Fatalf("non-progressing recipe must execute once: %v", err)
	}
}
