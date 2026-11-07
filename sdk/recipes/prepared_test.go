package recipes

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type preparedReader struct {
	members            []*fakeReader
	groups             [][]protocolio.ReadRequest
	heads, simulations int
	afterSimulation    func()
	afterBatch         func(int)
}

type finalMemberFailureReader struct {
	*preparedReader
	bad      common.Address
	metadata bool
}

func (r *finalMemberFailureReader) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if call.To != nil && *call.To == r.bad && r.metadata {
		return nil, errors.New("executor metadata unavailable")
	}
	method := accountABI.GetABI().Methods["hasRole"]
	if len(call.Data) >= 4 && bytes.Equal(call.Data[:4], method.ID) {
		args, err := method.Inputs.Unpack(call.Data[4:])
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(args[1].(common.Address) != r.bad)
	}
	return r.preparedReader.CallContract(ctx, call, block)
}

// Override the grouped adapter too: embedded methods otherwise call the base reader.
func (r *finalMemberFailureReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	out := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		switch request.Kind {
		case protocolio.ReadCall:
			out[i].Data, out[i].Err = r.CallContract(ctx, request.Call, request.Block)
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = r.HeaderByNumber(ctx, request.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = r.FilterLogs(ctx, request.Filter)
		}
	}
	return out, nil
}

func TestPreparedBatchAttributesFinalMetadataAndRoleFailures(t *testing.T) {
	for _, metadata := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoked role", true: "metadata read"}[metadata], func(t *testing.T) {
			base, executors, accounts := preparedFixture(t, 2)
			prepared, err := DiscoverBatch(t.Context(), base, executors, accounts, base.members[0].caller, PreparationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			reader := &finalMemberFailureReader{preparedReader: base, bad: executors[1], metadata: metadata}
			calls := make([]ExecutorCall, len(prepared))
			for i, p := range prepared {
				data, _, e := Assemble(p.Recipe, &Tree{}, nil, nil, nil)
				if e != nil {
					t.Fatal(e)
				}
				calls[i] = ExecutorCall{Executor: executors[i], Data: data}
			}
			err = ValidatePreparedBatch(t.Context(), reader, prepared, calls, [][]common.Hash{{common.Hash{1}}, {common.Hash{2}}}, make([]map[uint]string, len(calls)), base.members[0].caller)
			var member *MemberError
			if !errors.As(err, &member) || member.Index != 1 {
				t.Fatalf("member failure became global or misattributed: %v", err)
			}
		})
	}
}

type idleEnvelopeReader struct {
	*preparedReader
	envelopes, overrides int
	reorg                bool
}

func (r *idleEnvelopeReader) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	return types.AddressToDelegation(common.Address{77}), nil
}

func (r *idleEnvelopeReader) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if call.To != nil && *call.To == r.members[0].caller {
		method := delegatedABI.GetABI().Methods["supportsExecutionMode"]
		if bytes.Equal(call.Data[:4], method.ID) {
			return method.Outputs.Pack(true)
		}
		r.envelopes++
		return nil, nil
	}
	return r.preparedReader.CallContract(ctx, call, block)
}

func (r *idleEnvelopeReader) CallContractWithCode(_ context.Context, call ethereum.CallMsg, _ *big.Int, _ []byte) ([]byte, error) {
	r.overrides++
	values, err := executorCalls.Unpack(call.Data[4:])
	if err != nil {
		return nil, err
	}
	entries := *abi.ConvertType(values[0], new([]struct {
		Executor common.Address
		Data     []byte
	})).(*[]struct {
		Executor common.Address
		Data     []byte
	})
	results := make([][]byte, len(entries))
	for i, entry := range entries {
		recipe, _, err := decodeExecution(entry.Data)
		if err != nil {
			return nil, err
		}
		values := make([]*big.Int, len(recipe.Steps))
		for i := range values {
			values[i] = new(big.Int)
		}
		results[i], err = executorABI.GetABI().Methods["execute"].Outputs.Pack(values)
		if err != nil {
			return nil, err
		}
	}
	if r.reorg {
		h := types.CopyHeader(r.members[0].headers[10])
		h.Extra = []byte{1}
		r.members[0].headers[10] = h
	}
	return batchArguments.Pack(results)
}

func TestPreparedBatchCollectsEveryIdleMemberAfterCanonicalClosing(t *testing.T) {
	for _, reorg := range []bool{false, true} {
		t.Run(map[bool]string{false: "all idle", true: "idle reorg"}[reorg], func(t *testing.T) {
			base, executors, accounts := preparedFixture(t, 50)
			reader := &idleEnvelopeReader{preparedReader: base, reorg: reorg}
			calls := make([]ExecutorCall, len(executors))
			recipe := Recipe{Steps: [][]byte{manageStep(t, common.Hash{1}, literal(0), nil)}}
			hash, _ := HashRecipe(recipe)
			for i, member := range reader.members {
				member.recipeHash = hash
				member.logs = []types.Log{member.recipeLog(t, recipe, 4)}
				data, err := executorABI.TryPackExecute(recipe.Queries, recipe.Steps, nil, [][]byte{{}})
				if err != nil {
					t.Fatal(err)
				}
				calls[i] = ExecutorCall{Executor: member.executor, Data: data}
			}
			cache := NewRecipeCache(len(executors), 1<<20)
			prepared, err := DiscoverBatch(t.Context(), reader, executors, accounts, reader.members[0].caller, PreparationOptions{ChainID: big.NewInt(1), Cache: cache})
			if err != nil {
				t.Fatal(err)
			}
			err = ValidatePreparedBatch(t.Context(), reader, prepared, calls, make([][]common.Hash, len(calls)), make([]map[uint]string, len(calls)), reader.members[0].caller)
			for _, executor := range executors {
				if (cache.load(evidenceKey{chainID: "1", executor: executor}) != nil) != !reorg {
					t.Fatal("validated idle evidence cache did not follow canonical guards")
				}
			}
			if reorg {
				if err == nil || errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("reorg became idle: %v", err)
				}
				return
			}
			var member *MemberError
			if !errors.As(err, &member) || !errors.Is(err, provider.ErrNotReady) || member.Index != 0 || len(member.Indices) != len(calls) {
				t.Fatalf("idle members lost: %v", err)
			}
			for i, index := range member.Indices {
				if index != i {
					t.Fatal("idle indices lost execution order")
				}
			}
			if reader.envelopes != 1 || reader.overrides != 1 {
				t.Fatalf("quadratic idle probing: exact=%d override=%d", reader.envelopes, reader.overrides)
			}
		})
	}
}

func preparedFixture(t *testing.T, count int) (*preparedReader, []common.Address, []common.Address) {
	r := &preparedReader{}
	var executors, accounts []common.Address
	for i := range count {
		recipe := Recipe{Steps: [][]byte{step(Call, pack(t, callArgs, common.Address{9}, []byte{1, 2, 3, 4}, []*big.Int{}))}}
		member := fakeChain(t, recipe)
		member.executor = common.BigToAddress(big.NewInt(int64(i + 1)))
		member.headers[10].GasLimit = 30_000_000
		member.logs = []types.Log{member.recipeLog(t, recipe, 4)}
		r.members = append(r.members, member)
		executors, accounts = append(executors, member.executor), append(accounts, member.account)
	}
	return r, executors, accounts
}
func (r *preparedReader) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	if block == nil {
		r.heads++
	}
	return r.members[0].HeaderByNumber(ctx, block)
}
func (r *preparedReader) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	for _, member := range r.members {
		if query.Addresses[0] == member.executor {
			return member.FilterLogs(ctx, query)
		}
	}
	panic("unknown log executor")
}
func (r *preparedReader) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	for _, member := range r.members {
		if call.To != nil && *call.To == member.executor {
			if bytes.Equal(call.Data[:4], executorABI.GetABI().Methods["execute"].ID) {
				r.simulations++
				if block == nil || call.From != member.caller {
					member.t.Fatal("simulation lost pinned caller")
				}
				if r.afterSimulation != nil {
					r.afterSimulation()
				}
				return executorABI.GetABI().Methods["execute"].Outputs.Pack([]*big.Int{new(big.Int)})
			}
			return member.CallContract(ctx, call, block)
		}
	}
	if call.To != nil && *call.To == r.members[0].account {
		return r.members[0].CallContract(ctx, call, block)
	}
	return nil, errors.New("unknown speculative account")
}
func (r *preparedReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.groups = append(r.groups, append([]protocolio.ReadRequest(nil), requests...))
	out := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		switch request.Kind {
		case protocolio.ReadCall:
			out[i].Data, out[i].Err = r.CallContract(ctx, request.Call, request.Block)
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = r.HeaderByNumber(ctx, request.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = r.FilterLogs(ctx, request.Filter)
		}
	}
	if r.afterBatch != nil {
		r.afterBatch(len(r.groups))
	}
	return out, nil
}

func TestDiscoverBatchSharesHeadRootAndEventHeader(t *testing.T) {
	r, executors, accounts := preparedFixture(t, 3)
	prepared, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.heads != 1 || len(r.groups) != 3 || len(r.groups[0]) != 13 || len(r.groups[1]) != 4 || len(r.groups[2]) != 1 {
		t.Fatalf("heads=%d groups=%v", r.heads, r.groups)
	}
	for i, member := range prepared {
		if member.Snapshot.Executor != executors[i] || member.Snapshot.BlockHash != prepared[0].Snapshot.BlockHash {
			t.Fatal("batch lost order or common pin")
		}
	}
}

func TestDiscoverCandidatesIsolatesInvalidMembersAtSharedPin(t *testing.T) {
	for _, failure := range []string{"first caller", "last caller", "first recipe", "last recipe", "missing events", "reorganized shared pin"} {
		t.Run(failure, func(t *testing.T) {
			r, executors, accounts := preparedFixture(t, 3)
			caller := r.members[0].caller
			bad := 0
			if failure == "last caller" || failure == "last recipe" {
				bad = 2
			}
			switch failure {
			case "first caller", "last caller":
				r.members[bad].caller = common.Address{99}
			case "first recipe", "last recipe":
				r.members[bad].recipeHash = common.Hash{}
			case "missing events":
				r.members[bad].logs = nil
			case "reorganized shared pin":
				r.afterBatch = func(stage int) {
					if stage == 2 {
						h := types.CopyHeader(r.members[0].headers[10])
						h.Extra = []byte{1}
						r.members[0].headers[10] = h
					}
				}
			}
			results, err := DiscoverCandidates(t.Context(), r, executors, accounts, caller, PreparationOptions{})
			if failure == "reorganized shared pin" {
				if err == nil || len(results) != 0 {
					t.Fatal("partial discovery bypassed common canonical guard")
				}
				return
			}
			if err != nil || len(results) != 3 {
				t.Fatalf("shared discovery failed: %v", err)
			}
			for i, result := range results {
				if (result.Err != nil) != (i == bad) {
					t.Fatalf("candidate%d error=%v", i, result.Err)
				}
				if i != bad && (result.Prepared.Snapshot.Executor != executors[i] || result.Prepared.Snapshot.BlockHash != r.members[0].headers[10].Hash()) {
					t.Fatal("healthy candidate lost identity or shared pin")
				}
			}
			if r.heads != 1 || len(r.groups) != 3 || len(r.groups[0]) != 13 || len(r.groups[2]) != 1 {
				t.Fatalf("partial discovery retried independent RPCs: heads=%d groups=%d", r.heads, len(r.groups))
			}
			if _, err := DiscoverBatch(t.Context(), r, executors, accounts, caller, PreparationOptions{}); err == nil {
				t.Fatal("strict discovery accepted invalid startup member")
			}
		})
	}
}

func TestDiscoverBatchClosesSharedPinBeforeReturning(t *testing.T) {
	r, executors, accounts := preparedFixture(t, 2)
	r.afterBatch = func(stage int) {
		if stage == 2 {
			h := types.CopyHeader(r.members[0].headers[10])
			h.Extra = []byte{1}
			r.members[0].headers[10] = h
		}
	}
	if _, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{}); err == nil {
		t.Fatal("discovery returned reorganized metadata")
	}
}

func TestDiscoverBatchChunksLargeGroupsAndRejectsInvalidIdentity(t *testing.T) {
	r, executors, accounts := preparedFixture(t, 70)
	if _, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{}); err != nil {
		t.Fatal(err)
	}
	if r.heads != 1 || len(r.groups) != 4 || len(r.groups[0]) != protocolio.MaxReadBatch {
		t.Fatalf("large discovery heads=%d groups=%d", r.heads, len(r.groups))
	}
	for _, group := range r.groups {
		if len(group) > protocolio.MaxReadBatch {
			t.Fatal("unbounded discovery stage")
		}
	}
	for _, tc := range []struct {
		name                string
		executors, accounts []common.Address
		caller              common.Address
	}{
		{"duplicates", []common.Address{executors[0], executors[0]}, accounts[:2], r.members[0].caller},
		{"account mismatch", executors[:1], []common.Address{{99}}, r.members[0].caller},
		{"caller mismatch", executors[:1], accounts[:1], common.Address{99}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DiscoverBatch(t.Context(), r, tc.executors, tc.accounts, tc.caller, PreparationOptions{}); err == nil {
				t.Fatal("invalid batch identity accepted")
			}
		})
	}
}

func TestDiscoverBatchCacheReauthenticatesEventAndIsolatesBodies(t *testing.T) {
	r, executors, accounts := preparedFixture(t, 1)
	options := PreparationOptions{ChainID: big.NewInt(1), Cache: NewRecipeCache(1, 1<<20)}
	prepared, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, options)
	if err != nil {
		t.Fatal(err)
	}
	call, _, err := Assemble(prepared[0].Recipe, &Tree{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidatePreparedBatch(t.Context(), r, prepared, []ExecutorCall{{Executor: executors[0], Data: call}}, [][]common.Hash{nil}, []map[uint]string{nil}, r.members[0].caller); err != nil {
		t.Fatal(err)
	}
	r.groups = nil
	prepared, err = DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range r.groups {
		for _, request := range group {
			if request.Kind == protocolio.ReadLogs {
				t.Fatal("warm cache rediscovered logs")
			}
		}
	}
	prepared[0].Recipe.Steps[0][0] = 255
	prepared, err = DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, options)
	if err != nil || prepared[0].Recipe.Steps[0][0] != Call {
		t.Fatalf("cached body mutated: %v", err)
	}
	r.members[0].headers[8] = nil
	if _, err = DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, options); err == nil {
		t.Fatal("matched cache hint bypassed missing canonical event header")
	}
}

func TestPreparedBatchPublishesCacheOnlyAfterFinalGuards(t *testing.T) {
	for _, failure := range []string{"", "reorg", "role", "witness", "mutated recipe"} {
		t.Run(failure, func(t *testing.T) {
			r, executors, accounts := preparedFixture(t, 1)
			cache := NewRecipeCache(2, 1<<20)
			prepared, err := DiscoverBatch(t.Context(), r, executors, accounts, r.members[0].caller, PreparationOptions{ChainID: big.NewInt(1), Cache: cache})
			if err != nil {
				t.Fatal(err)
			}
			key := evidenceKey{chainID: "1", executor: executors[0]}
			if cache.load(key) != nil {
				t.Fatal("provisional discovery populated cache")
			}
			call, _, err := Assemble(prepared[0].Recipe, &Tree{}, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			witness := map[uint]string(nil)
			switch failure {
			case "reorg":
				r.afterSimulation = func() {
					h := types.CopyHeader(r.members[0].headers[10])
					h.Extra = []byte{1}
					r.members[0].headers[10] = h
				}
			case "role":
				r.members[0].allowed = false
			case "witness":
				witness = map[uint]string{0: "1"}
			case "mutated recipe":
				prepared[0].Recipe.Steps[0][0] = 255
			}
			err = ValidatePreparedBatch(t.Context(), r, prepared, []ExecutorCall{{Executor: executors[0], Data: call}}, [][]common.Hash{{common.Hash{1}, common.Hash{1}}}, []map[uint]string{witness}, r.members[0].caller)
			if (err != nil) != (failure != "") {
				t.Fatalf("error=%v failure=%q", err, failure)
			}
			if failure == "witness" {
				var member *MemberError
				if !errors.As(err, &member) || member.Index != 0 {
					t.Fatalf("missing member identity: %v", err)
				}
			}
			if (cache.load(key) != nil) != (failure == "") {
				t.Fatal("cache publication did not follow final admission")
			}
			if failure == "" && (r.simulations != 1 || r.members[0].roleCalls != 1) {
				t.Fatalf("simulations=%d roles=%d", r.simulations, r.members[0].roleCalls)
			}
		})
	}
}
