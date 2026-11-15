package keeper

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"slices"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	delegated "github.com/symbioticfi/merkle-executor/sdk/bindings/delegatedaccount"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type plannerScaleReader struct {
	t                 *testing.T
	header            *types.Header
	members           []recipes.PreparedRecipe
	logs              map[common.Address]types.Log
	bad               common.Address
	deferredError     error
	badAfterDiscovery bool
	badReads          int
	emptyResults      bool
	accepted          map[int]bool
	probes            []int
	gasBudgets        []uint64
	gasPerCall        uint64
}

func scaleArgs(t *testing.T, names ...string) abi.Arguments {
	t.Helper()
	var args abi.Arguments
	for _, name := range names {
		typ, err := abi.NewType(name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args = append(args, abi.Argument{Type: typ})
	}
	return args
}

func scaleFixture(t *testing.T, count int) (*Planner, *plannerScaleReader, []Strategy, ExecutionPlan) {
	t.Helper()
	r := &plannerScaleReader{t: t, header: &types.Header{Number: big.NewInt(12), GasLimit: 30_000_000}, logs: map[common.Address]types.Log{}}
	args, err := scaleArgs(t, "address", "bytes", "uint256[]").Pack(common.Address{99}, []byte{1, 2, 3, 4}, []*big.Int{})
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipes.Recipe{Steps: [][]byte{append([]byte{recipes.Call}, args...)}}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	a := executor.NewRecipeExecutor()
	data := a.PackExecute(nil, recipe.Steps, nil, nil)
	var strategies []Strategy
	batch := ExecutionPlan{To: common.Address{3}}
	for i := range count {
		snapshot := recipes.Snapshot{BlockNumber: 12, BlockHash: r.header.Hash(), Timestamp: r.header.Time, Executor: common.BigToAddress(big.NewInt(int64(10 + i))), Account: common.Address{2}, Caller: common.Address{3}, RecipeHash: hash, RecipeBlock: 12, Root: common.Hash{4}}
		prepared := recipes.PreparedRecipe{Snapshot: snapshot, Recipe: recipe}
		r.members = append(r.members, prepared)
		event := a.GetABI().Events["SetRecipe"]
		eventData, e := event.Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
		if e != nil {
			t.Fatal(e)
		}
		r.logs[snapshot.Executor] = types.Log{Address: snapshot.Executor, BlockNumber: 12, BlockHash: r.header.Hash(), Topics: []common.Hash{event.ID, hash}, Data: eventData}
		strategy := Strategy{Config: Config{Name: snapshot.Executor.Hex(), Executor: snapshot.Executor.Hex(), Account: snapshot.Account.Hex()}, Providers: []Provider{&permissionProvider{}}}
		strategies = append(strategies, strategy)
		batch.Members = append(batch.Members, PlanMember{Name: strategy.Config.Name, Plan: Plan{Snapshot: provider.Snapshot{Executor: snapshot.Executor, Account: snapshot.Account, Caller: snapshot.Caller}, Recipe: snapshot, Calldata: bytes.Clone(data), Outputs: [][]provider.Output{{{}}}}})
	}
	permission, err := json.Marshal(map[string]any{"chainId": 1, "root": r.members[0].Snapshot.Root.Hex(), "leaves": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	p := &Planner{Chain: r, ChainID: big.NewInt(1), Caller: batch.To, PermissionSource: permissionSourceFunc(func(context.Context, *big.Int, recipes.Snapshot) ([]byte, error) { return permission, nil })}
	return p, r, strategies, batch
}

func (r *plannerScaleReader) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return r.header, nil
}
func (r *plannerScaleReader) HeaderByHash(context.Context, common.Hash) (*types.Header, error) {
	return r.header, nil
}
func (r *plannerScaleReader) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return []types.Log{r.logs[q.Addresses[0]]}, nil
}
func (r *plannerScaleReader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if msg.Gas != 0 {
		r.gasBudgets = append(r.gasBudgets, msg.Gas)
	}
	if block == nil || block.Cmp(r.header.Number) != 0 {
		r.t.Fatal("lost pinned call")
	}
	a := executor.NewRecipeExecutor().GetABI()
	if msg.To == nil {
		return nil, errors.New("missing target")
	}
	for _, member := range r.members {
		s := member.Snapshot
		if *msg.To != s.Executor {
			continue
		}
		if s.Executor == r.bad {
			r.badReads++
			if !r.badAfterDiscovery || r.badReads > 4 {
				return nil, r.deferredError
			}
		}
		method, e := a.MethodById(msg.Data[:4])
		if e != nil {
			return nil, e
		}
		var value any
		switch method.Name {
		case "account":
			value = s.Account
		case "caller":
			value = s.Caller
		case "recipeHash":
			value = s.RecipeHash
		case "recipeBlock":
			value = new(big.Int).SetUint64(s.RecipeBlock)
		case "execute":
			r.probes = append(r.probes, 1)
			values := make([]*big.Int, len(member.Recipe.Steps))
			for i := range values {
				values[i] = new(big.Int)
			}
			return method.Outputs.Pack(values)
		case "multicall":
			calls, e := recipes.Executions(msg.Data)
			if e != nil {
				return nil, e
			}
			return r.accept(len(calls), msg.Gas)
		default:
			return nil, errors.New("unknown executor method")
		}
		return method.Outputs.Pack(value)
	}
	if *msg.To == r.members[0].Snapshot.Account {
		return account.NewMerkleAccount().GetABI().Methods["root"].Outputs.Pack(r.members[0].Snapshot.Root)
	}
	if *msg.To == r.members[0].Snapshot.Caller {
		d := delegated.NewDelegatedAccount().GetABI()
		method, e := d.MethodById(msg.Data[:4])
		if e != nil {
			return nil, e
		}
		if method.Name == "supportsExecutionMode" {
			return method.Outputs.Pack(true)
		}
		values, e := method.Inputs.Unpack(msg.Data[4:])
		if e != nil {
			return nil, e
		}
		tupleType, e := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{{Name: "target", Type: "address"}, {Name: "value", Type: "uint256"}, {Name: "callData", Type: "bytes"}})
		if e != nil {
			return nil, e
		}
		calls, e := (abi.Arguments{{Type: tupleType}}).Unpack(values[1].([]byte))
		if e != nil {
			return nil, e
		}
		return r.accept(len(*abi.ConvertType(calls[0], new([]struct {
			Target   common.Address
			Value    *big.Int
			CallData []byte
		})).(*[]struct {
			Target   common.Address
			Value    *big.Int
			CallData []byte
		})), msg.Gas)
	}
	return nil, errors.New("unknown target")
}
func (r *plannerScaleReader) accept(count int, gas uint64) ([]byte, error) {
	r.probes = append(r.probes, count)
	if !r.accepted[count] || r.gasPerCall != 0 && uint64(count)*r.gasPerCall > gas {
		return nil, errors.New("out of gas")
	}
	return nil, nil
}
func (r *plannerScaleReader) CodeAt(_ context.Context, address common.Address, _ *big.Int) ([]byte, error) {
	if address == r.members[0].Snapshot.Caller {
		return types.AddressToDelegation(common.Address{77}), nil
	}
	return []byte{0}, nil
}
func (r *plannerScaleReader) CallContractWithCode(_ context.Context, msg ethereum.CallMsg, _ *big.Int, _ []byte) ([]byte, error) {
	var count int
	if *msg.To == r.members[0].Snapshot.Caller {
		typ, err := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{{Name: "executor", Type: "address"}, {Name: "data", Type: "bytes"}})
		if err != nil {
			return nil, err
		}
		values, err := (abi.Arguments{{Type: typ}}).Unpack(msg.Data[4:])
		if err != nil {
			return nil, err
		}
		count = len(*abi.ConvertType(values[0], new([]struct {
			Executor common.Address
			Data     []byte
		})).(*[]struct {
			Executor common.Address
			Data     []byte
		}))
	} else {
		values, err := scaleArgs(r.t, "bytes", "bytes[]").Unpack(msg.Data[4:])
		if err != nil {
			return nil, err
		}
		count = len(values[1].([][]byte))
	}
	values := []*big.Int{new(big.Int)}
	if r.emptyResults {
		values = nil
	}
	result, err := executor.NewRecipeExecutor().GetABI().Methods["execute"].Outputs.Pack(values)
	if err != nil {
		return nil, err
	}
	results := make([][]byte, count)
	for i := range results {
		results[i] = result
	}
	return scaleArgs(r.t, "bytes[]").Pack(results)
}

func TestPlannerFitKeepsLargestPassingPrefix(t *testing.T) {
	for _, accepted := range []map[int]bool{{1: true, 2: true, 3: true}, {1: true, 3: true}} {
		t.Run(map[bool]string{true: "nonmonotone", false: "gas prefix"}[!accepted[2]], func(t *testing.T) {
			p, r, strategies, batch := scaleFixture(t, 4)
			r.accepted = accepted
			plan, err := p.fit(t.Context(), batch, r.members, strategies, nil)
			if err != nil || len(plan.Members) != 3 || !slices.Equal(r.probes, []int{4, 3}) {
				t.Fatalf("members=%d probes=%v error=%v", len(plan.Members), r.probes, err)
			}
		})
	}
}

func TestPlannerFitKeepsLargestPassingRepeatPrefix(t *testing.T) {
	p, r, strategies, batch := scaleFixture(t, 1)
	r.accepted = map[int]bool{1: true, 2: true, 3: true}
	entry := batch.Members[0].Plan.Calldata
	var err error
	batch.Members[0].Plan.Calldata, err = recipes.Multicall([][]byte{entry, entry, entry, entry})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := p.fit(t.Context(), batch, r.members, strategies, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := recipes.Executions(plan.Members[0].Plan.Calldata)
	if err != nil || len(entries) != 3 || !slices.Equal(r.probes, []int{4, 3}) {
		t.Fatalf("entries=%d probes=%v error=%v", len(entries), r.probes, err)
	}
}

func TestPlannerIsolatesUnavailableCandidateAndReportsFailure(t *testing.T) {
	for _, bad := range []int{0, 2} {
		t.Run(map[int]string{0: "first", 2: "last"}[bad], func(t *testing.T) {
			p, r, strategies, _ := scaleFixture(t, 3)
			// Keep the production metadata path while exercising ordinary direct sends.
			p.Chain = struct{ Backend }{r}
			r.bad = r.members[bad].Snapshot.Executor
			r.deferredError = errors.New("executor metadata unavailable")
			plan, err := p.PrepareExecution(t.Context(), strategies)
			want := 0
			if bad == 0 {
				want = 1
			}
			if err != nil || len(plan.Members) != 1 || plan.Members[0].Name != strategies[want].Config.Name {
				t.Fatalf("healthy candidate blocked: plan=%+v error=%v", plan, err)
			}
			if len(plan.Failures) != 1 || !errors.Is(plan.Failures[strategies[bad].Config.Name], r.deferredError) {
				t.Fatalf("candidate failure lost: %v", plan.Failures)
			}
		})
	}
}

func TestPlannerAllCandidatesUnavailableRemainErrors(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 1)
	p.Chain = struct{ Backend }{r}
	r.bad = r.members[0].Snapshot.Executor
	r.deferredError = errors.New("executor metadata unavailable")
	if _, err := p.PrepareExecution(t.Context(), strategies); !errors.Is(err, r.deferredError) || errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("failed candidate became idle: %v", err)
	}
}

func TestPlannerIsolatesFreshMemberFailureBeforeDirectAdmission(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 2)
	p.Chain = struct{ Backend }{r}
	r.bad = r.members[0].Snapshot.Executor
	r.badAfterDiscovery = true
	r.deferredError = errors.New("metadata became unavailable at admission")
	plan, err := p.PrepareExecution(t.Context(), strategies)
	if err != nil || len(plan.Members) != 1 || plan.Members[0].Name != strategies[1].Config.Name || !errors.Is(plan.Failures[strategies[0].Config.Name], r.deferredError) {
		t.Fatalf("fresh member failure blocked independent execution: members=%d failures=%v error=%v", len(plan.Members), plan.Failures, err)
	}
}

func TestPlannerFitPreservesEarlierFailureWhenRemainingMemberTurnsIdle(t *testing.T) {
	p, r, strategies, batch := scaleFixture(t, 1)
	failure := errors.New("another candidate failed discovery")
	batch.Failures = map[string]error{"failed": failure}
	_, err := p.fit(t.Context(), batch, r.members, strategies, &recipes.MemberError{Index: 0, Err: provider.ErrNotReady})
	if !errors.Is(err, failure) || errors.Is(err, provider.ErrNotReady) {
		t.Fatalf("actual failure became idle: %v", err)
	}
}

func TestPlannerDelegatedIdleSurvivorsPreserveDiscoveryFailure(t *testing.T) {
	p, r, strategies, _ := scaleFixture(t, 3)
	r.bad = r.members[0].Snapshot.Executor
	r.deferredError = errors.New("first executor metadata unavailable")
	r.accepted = map[int]bool{2: true}
	r.emptyResults = true
	empty := recipes.Recipe{}
	hash, err := recipes.HashRecipe(empty)
	if err != nil {
		t.Fatal(err)
	}
	event := executor.NewRecipeExecutor().GetABI().Events["SetRecipe"]
	data, err := event.Inputs.NonIndexed().Pack(empty.Queries, empty.Steps)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(r.members); i++ {
		r.members[i].Recipe = empty
		r.members[i].Snapshot.RecipeHash = hash
		log := r.logs[r.members[i].Snapshot.Executor]
		log.Topics[1], log.Data = hash, data
		r.logs[log.Address] = log
	}
	_, err = p.PrepareExecution(t.Context(), strategies)
	if !errors.Is(err, r.deferredError) || errors.Is(err, provider.ErrNotReady) || !slices.Equal(r.probes, []int{2}) {
		t.Fatalf("idle survivors hid candidate failure: probes=%v error=%v", r.probes, err)
	}
}
