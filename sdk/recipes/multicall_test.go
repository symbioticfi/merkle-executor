package recipes

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
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

type repeatedProbeError struct{}

func (repeatedProbeError) Error() string  { return "execution reverted" }
func (repeatedProbeError) ErrorCode() int { return 3 }

type repeatedProbeReader struct {
	*fakeReader
	limit, idleAt, duplicateAt int
	probes, exact, codeReads   int
	transportFailure           bool
}

func (r *repeatedProbeReader) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	r.codeReads++
	return []byte{0}, nil
}
func (r *repeatedProbeReader) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	r.exact++
	return nil, nil
}
func (r *repeatedProbeReader) CallContractWithCode(_ context.Context, call ethereum.CallMsg, _ *big.Int, _ []byte) ([]byte, error) {
	r.probes++
	if r.transportFailure {
		return nil, errors.New("transport unavailable")
	}
	args, err := abiArguments("bytes", "bytes[]").Unpack(call.Data[4:])
	if err != nil {
		return nil, err
	}
	calls := args[1].([][]byte)
	if len(calls) > r.limit {
		return nil, repeatedProbeError{}
	}
	results := make([][]byte, len(calls))
	for i := range calls {
		value := int64(i + 1)
		if r.idleAt > 0 && i >= r.idleAt {
			value = 0
		}
		if r.duplicateAt > 0 && i >= r.duplicateAt {
			value = int64(r.duplicateAt)
		}
		results[i], err = executorABI.GetABI().Methods["execute"].Outputs.Pack([]*big.Int{big.NewInt(value), new(big.Int)})
		if err != nil {
			return nil, err
		}
	}
	return batchArguments.Pack(results)
}

func TestRepeatedBatchUsesBoundedProbesAndKeepsFirstBoundary(t *testing.T) {
	r := Recipe{Queries: [][]byte{pack(t, queryArgs, common.Address{9}, []byte{1, 2, 3, 4}, []*big.Int{}, new(big.Int))}, Steps: [][]byte{step(Read, pack(t, readArgs, literal(0))), manageStep(t, common.Hash{1}, result(0), nil)}}
	call, err := executorABI.TryPackExecute(r.Queries, r.Steps, nil, [][]byte{{}})
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := HashRecipe(r)
	for _, tc := range []struct {
		name                  string
		idle, duplicate, want int
		transport             bool
	}{
		{"gas boundary", 0, 0, 64, false}, {"idle", 5, 0, 5, false}, {"duplicate", 0, 5, 5, false}, {"transport", 0, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &repeatedProbeReader{fakeReader: fakeChain(t, r), limit: 64, idleAt: tc.idle, duplicateAt: tc.duplicate, transportFailure: tc.transport}
			snapshot := Snapshot{Executor: reader.executor, Account: reader.account, Caller: reader.caller, BlockNumber: 10, RecipeHash: hash}
			batch, err := BatchExecution(t.Context(), reader, snapshot, call, 1_000_000)
			if tc.transport {
				if err == nil {
					t.Fatal("transport failure became a batch boundary")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			entries, err := Executions(batch)
			if err != nil || len(entries) != tc.want {
				t.Fatalf("entries=%d error=%v", len(entries), err)
			}
			if reader.probes > 15 {
				t.Fatalf("quadratic prefix search: %d probes", reader.probes)
			}
			if reader.codeReads != 1 {
				t.Fatalf("read immutable runtime %d times", reader.codeReads)
			}
			if reader.exact == 0 {
				t.Fatal("no exact unchanged-code simulation")
			}
		})
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
	prepared, err := DiscoverBatch(ctx, reader, []common.Address{executor}, []common.Address{account}, sender, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidatePreparedBatch(ctx, reader, prepared, []ExecutorCall{{Executor: executor, Data: batch}}, [][]common.Hash{{leaf.Hash}}, []map[uint]string{nil}, sender); err != nil {
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

func TestMulticallChecksEveryNumericalWitness(t *testing.T) {
	steps := [][]byte{step(Add, pack(t, binaryArgs, input(0), literal(1)))}
	first, _ := executorABI.TryPackExecute(nil, steps, []*big.Int{big.NewInt(2)}, nil)
	second, _ := executorABI.TryPackExecute(nil, steps, []*big.Int{big.NewInt(3)}, nil)
	batch, err := Multicall([][]byte{first, second})
	if err != nil {
		t.Fatal(err)
	}
	reply := func(value int64) []byte {
		return pack(t, executorABI.GetABI().Methods["execute"].Outputs, []*big.Int{big.NewInt(value)})
	}
	for _, tc := range []struct {
		name   string
		values [][]byte
		want   string
	}{
		{"matching", [][]byte{reply(3), reply(3)}, ""},
		{"later witness changed", [][]byte{reply(3), reply(4)}, "differs from signed"},
		{"missing reply", [][]byte{reply(3)}, "result count mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyResults(batch, pack(t, batchArguments, tc.values), map[uint]string{0: "3"})
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatal(err)
			}
		})
	}
}
