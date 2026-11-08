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
)

type fakeReader struct {
	t                         *testing.T
	executor, account, caller common.Address
	recipeHash, root          common.Hash
	recipeBlock               uint64
	headers                   map[uint64]*types.Header
	head                      uint64
	logs                      []types.Log
	allowed                   bool
	roleCalls                 int
	roleBlock                 uint64
}

func fakeChain(t *testing.T, recipe Recipe) *fakeReader {
	t.Helper()
	hash, err := HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeReader{t: t, executor: common.HexToAddress("0x1111"), account: common.HexToAddress("0x2222"), caller: common.HexToAddress("0x3333"), recipeHash: hash, root: common.HexToHash("0x1234"), recipeBlock: 8, head: 10, allowed: true, headers: map[uint64]*types.Header{}}
	for _, number := range []uint64{8, 10, 11} {
		reader.headers[number] = &types.Header{Number: new(big.Int).SetUint64(number), Time: number * 12, GasLimit: 30_000_000}
	}
	reader.logs = []types.Log{reader.recipeLog(t, recipe, 4)}
	return reader
}

func (reader *fakeReader) recipeLog(t *testing.T, recipe Recipe, index uint) types.Log {
	t.Helper()
	event := executorABI.GetABI().Events["SetRecipe"]
	hash, err := HashRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	data := pack(t, event.Inputs.NonIndexed(), recipe.Queries, recipe.Steps)
	return types.Log{Address: reader.executor, Topics: []common.Hash{event.ID, hash}, Data: data, BlockNumber: reader.recipeBlock, BlockHash: reader.headers[reader.recipeBlock].Hash(), Index: index}
}

func (reader *fakeReader) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	index := reader.head
	if number != nil {
		index = number.Uint64()
	}
	return reader.headers[index], nil
}

func (reader *fakeReader) FilterLogs(_ context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	reader.t.Helper()
	if query.FromBlock.Uint64() != reader.recipeBlock || query.ToBlock.Uint64() != reader.recipeBlock || len(query.Addresses) != 1 || query.Addresses[0] != reader.executor {
		reader.t.Fatal("log query is not narrowly pinned")
	}
	if len(query.Topics) != 1 || len(query.Topics[0]) != 1 || query.Topics[0][0] != executorABI.GetABI().Events["SetRecipe"].ID {
		reader.t.Fatal("query must inspect every SetRecipe at recipeBlock")
	}
	return append([]types.Log(nil), reader.logs...), nil
}

func (reader *fakeReader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	reader.t.Helper()
	if block == nil || (block.Uint64() != 10 && block.Uint64() != 11) {
		reader.t.Fatal("contract read is not pinned")
	}
	if isExecutionCall(msg.Data) {
		if msg.To == nil || *msg.To != reader.executor || msg.From != common.HexToAddress("0x3333") || block.Uint64() != reader.head {
			reader.t.Fatal("full execution lost current pin or caller")
		}
		recipe, _, err := decodeExecution(msg.Data)
		if err != nil {
			return nil, err
		}
		values := make([]*big.Int, len(recipe.Steps))
		for i := range values {
			values[i] = new(big.Int)
		}
		return executorABI.GetABI().Methods["execute"].Outputs.Pack(values)
	}
	var method *abi.Method
	var value any
	switch {
	case msg.To != nil && *msg.To == reader.executor:
		contractABI := executorABI.GetABI()
		candidate, err := contractABI.MethodById(msg.Data)
		if err != nil {
			reader.t.Fatal(err)
		}
		method = candidate
		switch method.Name {
		case "account":
			value = reader.account
		case "caller":
			value = reader.caller
		case "recipeHash":
			value = reader.recipeHash
		case "recipeBlock":
			value = new(big.Int).SetUint64(reader.recipeBlock)
		default:
			reader.t.Fatalf("unexpected executor method %s", method.Name)
		}
	case msg.To != nil && *msg.To == reader.account:
		contractABI := accountABI.GetABI()
		candidate, err := contractABI.MethodById(msg.Data)
		if err != nil {
			reader.t.Fatal(err)
		}
		method = candidate
		switch method.Name {
		case "root":
			value = reader.root
		case "hasRole":
			args, err := method.Inputs.Unpack(msg.Data[4:])
			if err != nil {
				reader.t.Fatal(err)
			}
			if args[1].(common.Address) != reader.executor {
				reader.t.Fatal("keeper EOA used as role holder")
			}
			reader.roleCalls++
			reader.roleBlock = block.Uint64()
			value = reader.allowed
		default:
			reader.t.Fatalf("unexpected account method %s", method.Name)
		}
	default:
		return nil, errors.New("unknown speculative call target")
	}
	return method.Outputs.Pack(value)
}

func TestDiscoverPinsAndChoosesLatestEvent(t *testing.T) {
	current := Recipe{Steps: [][]byte{step(Call, pack(t, callArgs, common.HexToAddress("0x9999"), []byte{1, 2, 3, 4}, []*big.Int{}))}}
	reader := fakeChain(t, current)
	// RPC delivery order is deliberately reversed; ordering is by global log index.
	reader.logs = append(reader.logs, reader.recipeLog(t, Recipe{}, 2))
	prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, recipe := prepared[0].Snapshot, prepared[0].Recipe
	if snapshot.BlockNumber != 10 || snapshot.BlockHash != reader.headers[10].Hash() || snapshot.RecipeBlock != 8 || snapshot.Caller != reader.caller || snapshot.Root != reader.root {
		t.Fatalf("wrong snapshot: %+v", snapshot)
	}
	if len(recipe.Steps) != 1 || !bytes.Equal(recipe.Steps[0], current.Steps[0]) {
		t.Fatal("wrong committed event body")
	}
	call := assembledCall(t, prepared[0])
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err != nil {
		t.Fatal(err)
	}
	reader.head = 11
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverRejectsWrongEventEvidence(t *testing.T) {
	cases := map[string]func(*fakeReader){
		"missing":    func(r *fakeReader) { r.logs = nil },
		"removed":    func(r *fakeReader) { r.logs[0].Removed = true },
		"block hash": func(r *fakeReader) { r.logs[0].BlockHash = common.Hash{} },
		"duplicate":  func(r *fakeReader) { r.logs = append(r.logs, r.logs[0]) },
		"later distinct commitment": func(r *fakeReader) {
			log := r.logs[0]
			log.Index++
			log.Topics = append([]common.Hash(nil), log.Topics...)
			log.Topics[1] = common.HexToHash("0xdead")
			r.logs = append(r.logs, log)
		},
		"forged body": func(r *fakeReader) {
			r.logs[0].Data = pack(t, executorABI.GetABI().Events["SetRecipe"].Inputs.NonIndexed(), [][]byte{{1}}, [][]byte{})
		},
		"unconfigured":  func(r *fakeReader) { r.recipeHash = common.Hash{} },
		"future recipe": func(r *fakeReader) { r.recipeBlock = 11 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			reader := fakeChain(t, Recipe{})
			mutate(reader)
			if _, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{}); err == nil {
				t.Fatal("invalid evidence accepted")
			}
		})
	}
}

func TestSnapshotDriftAndRoleChecks(t *testing.T) {
	cases := map[string]func(*fakeReader){
		"account":      func(r *fakeReader) { r.account = common.HexToAddress("0x4444") },
		"caller":       func(r *fakeReader) { r.caller = common.HexToAddress("0x4444") },
		"recipe":       func(r *fakeReader) { r.recipeHash = common.HexToHash("0x4444") },
		"recipe block": func(r *fakeReader) { r.recipeBlock = 9 },
		"root":         func(r *fakeReader) { r.root = common.HexToHash("0x4444") },
		"reorg":        func(r *fakeReader) { copy := *r.headers[10]; copy.Extra = []byte{1}; r.headers[10] = &copy },
		"stale head":   func(r *fakeReader) { r.head = 8 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			reader := fakeChain(t, admissionRecipe(t))
			prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			call, caller := assembledCall(t, prepared[0]), reader.caller
			mutate(reader)
			if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, caller); err == nil {
				t.Fatal("changed configuration accepted")
			}
		})
	}
	reader := fakeChain(t, admissionRecipe(t))
	prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	call := assembledCall(t, prepared[0])
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, common.HexToAddress("0x9999")); err == nil {
		t.Fatal("foreign sender accepted")
	}
	leaf := common.HexToHash("0xabcd")
	reader.head = 11
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{leaf, leaf}}, []map[uint]string{nil}, reader.caller); err != nil {
		t.Fatal(err)
	}
	if reader.roleCalls != 1 || reader.roleBlock != reader.head {
		t.Fatal("role reads are not pinned and deduplicated")
	}
	reader.allowed = false
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{leaf}}, []map[uint]string{nil}, reader.caller); err == nil {
		t.Fatal("missing role accepted")
	}
	checked := reader.roleCalls
	if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err != nil || reader.roleCalls != checked {
		t.Fatalf("empty role set performed a role check: %v", err)
	}
}

type headerFaultReader struct {
	Reader
	fault func(*big.Int, *types.Header) *types.Header
}

func (r headerFaultReader) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	header, err := r.Reader.HeaderByNumber(ctx, block)
	if err == nil {
		header = r.fault(block, header)
	}
	return header, err
}

func TestSharedDiscoveryRejectsMalformedHeaders(t *testing.T) {
	for _, phase := range []string{"head", "event", "closing"} {
		for _, fault := range []string{"null", "missing number", "negative", "overflow", "wrong height"} {
			if phase == "head" && fault == "wrong height" {
				continue // A latest head has no externally requested height.
			}
			t.Run(phase+"/"+fault, func(t *testing.T) {
				base := fakeChain(t, admissionRecipe(t))
				reader := headerFaultReader{Reader: base, fault: func(block *big.Int, header *types.Header) *types.Header {
					selected := phase == "head" && block == nil || phase == "event" && block != nil && block.Uint64() == 8 || phase == "closing" && block != nil && block.Uint64() == 10
					if !selected {
						return header
					}
					if fault == "null" {
						return nil
					}
					header = types.CopyHeader(header)
					switch fault {
					case "missing number":
						header.Number = nil
					case "negative":
						header.Number = big.NewInt(-1)
					case "overflow":
						header.Number = new(big.Int).Lsh(big.NewInt(1), 65)
					case "wrong height":
						header.Number = new(big.Int).Add(header.Number, big.NewInt(1))
					}
					return header
				}}
				if _, err := DiscoverBatch(t.Context(), reader, []common.Address{base.executor}, []common.Address{base.account}, base.caller, PreparationOptions{}); err == nil {
					t.Fatal("malformed shared discovery header accepted")
				}
			})
		}
	}
}
