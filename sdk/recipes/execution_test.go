package recipes

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

func admissionRecipe(t *testing.T) Recipe {
	t.Helper()
	return Recipe{Steps: [][]byte{step(Call, pack(t, callArgs, common.HexToAddress("0x8888"), []byte{1, 2, 3, 4}, []*big.Int{}))}}
}

func assembledCall(t *testing.T, member PreparedRecipe) ExecutorCall {
	t.Helper()
	data, _, err := Assemble(member.Recipe, &Tree{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ExecutorCall{Executor: member.Snapshot.Executor, Data: data}
}

func isExecutionCall(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], executorABI.GetABI().Methods["execute"].ID)
}

type executionReader struct {
	*fakeReader
	validating               bool
	headerReads, simulations int
	mode                     string
	roleSeen                 map[common.Hash]int
	afterCall                func([]byte)
	executed                 []byte
}

func newExecutionReader(t *testing.T) *executionReader {
	return &executionReader{fakeReader: fakeChain(t, admissionRecipe(t)), roleSeen: map[common.Hash]int{}}
}
func (r *executionReader) HeaderByNumber(ctx context.Context, block *big.Int) (*types.Header, error) {
	header, err := r.fakeReader.HeaderByNumber(ctx, block)
	if !r.validating {
		return header, err
	}
	r.headerReads++
	if block == nil {
		switch r.mode {
		case "stale latest":
			return r.headers[8], nil
		case "missing latest":
			return nil, ethereum.NotFound
		case "opening equal conflict":
			header = types.CopyHeader(header)
			header.Extra = []byte("conflicting current pin")
		}
	} else if (r.mode == "opening original" && block.Uint64() == 10 && r.simulations == 0) ||
		(r.mode == "closing original" && block.Uint64() == 10 && r.simulations != 0) ||
		(r.mode == "closing latest" && block.Uint64() == r.head && r.simulations != 0) {
		header = types.CopyHeader(header)
		header.Extra = []byte("reorg")
	}
	return header, err
}
func (r *executionReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if isExecutionCall(msg.Data) {
		r.simulations++
		r.executed = bytes.Clone(msg.Data)
		if msg.From != common.HexToAddress("0x3333") || msg.To == nil || *msg.To != r.executor || block == nil || block.Uint64() != r.head || msg.Gas != min(r.headers[r.head].GasLimit, uint64(16_777_216)) {
			r.t.Fatalf("simulation context changed: %+v block%v", msg, block)
		}
		if r.mode == "during simulation" {
			header := types.CopyHeader(r.headers[r.head])
			header.Extra = []byte("simulation reorg")
			r.headers[r.head] = header
		}
		if strings.Contains(r.mode, "simulation error") {
			return nil, errors.New("speculative simulation failure")
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
	if r.validating && len(msg.Data) >= 4 && bytes.Equal(msg.Data[:4], accountABI.PackHasRole(common.Hash{}, r.executor)[:4]) {
		if block == nil || block.Uint64() != r.head {
			r.t.Fatal("role escaped current admission pin")
		}
		method := accountABI.GetABI().Methods["hasRole"]
		args, err := method.Inputs.Unpack(msg.Data[4:])
		if err != nil {
			r.t.Fatal(err)
		}
		leaf := common.Hash(args[0].([32]byte))
		r.roleSeen[leaf]++
		if strings.Contains(r.mode, "role error") {
			return nil, errors.New("required role unavailable")
		}
		if r.mode == "false role" {
			return method.Outputs.Pack(false)
		}
	}
	if r.validating && bytes.Equal(msg.Data, accountABI.PackRoot()) && strings.Contains(r.mode, "metadata error") {
		return nil, errors.New("required root unavailable")
	}
	if msg.To != nil && *msg.To != r.executor && *msg.To != r.account {
		return nil, errors.New("unknown speculative target")
	}
	raw, err := r.fakeReader.CallContract(ctx, msg, block)
	if r.afterCall != nil {
		r.afterCall(msg.Data)
	}
	return raw, err
}

type executionBatchReader struct {
	*executionReader
	groups [][]protocolio.ReadRequest
}

func (r *executionBatchReader) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	if r.validating {
		r.groups = append(r.groups, append([]protocolio.ReadRequest(nil), requests...))
	}
	out := make([]protocolio.ReadResult, len(requests))
	for i, request := range requests {
		switch request.Kind {
		case protocolio.ReadCall:
			if isExecutionCall(request.Call.Data) {
				if request.MulticallSafe {
					r.t.Fatal("simulation marked aggregate-safe")
				}
			} else if !request.MulticallSafe {
				r.t.Fatal("verified getter lost aggregation eligibility")
			}
			out[i].Data, out[i].Err = r.CallContract(ctx, request.Call, request.Block)
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = r.HeaderByNumber(ctx, request.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = r.FilterLogs(ctx, request.Filter)
		default:
			r.t.Fatal("unexpected request")
		}
	}
	return out, nil
}

func TestPreparedAdmissionChecksEveryRoleAcrossChunks(t *testing.T) {
	for _, count := range []int{0, 1, 250, 251, 256, 257} {
		for _, advanced := range []bool{false, true} {
			t.Run(big.NewInt(int64(count)).String()+map[bool]string{false: "/same", true: "/advanced"}[advanced], func(t *testing.T) {
				reader := newExecutionReader(t)
				batch := &executionBatchReader{executionReader: reader}
				prepared, err := DiscoverBatch(t.Context(), batch, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if prepared[0].Snapshot.BlockNumber != 10 || prepared[0].Snapshot.Root != reader.root || len(prepared[0].Recipe.Steps) != 1 {
					t.Fatal("unverified assembly identity")
				}
				call := assembledCall(t, prepared[0])
				leaves := make([]common.Hash, 0, count+1)
				for i := range count {
					leaves = append(leaves, common.BigToHash(big.NewInt(int64(i+1))))
				}
				if count > 0 {
					leaves = append(leaves, leaves[0])
				}
				reader.validating = true
				if advanced {
					reader.head = 11
				}
				err = ValidatePreparedBatch(t.Context(), batch, prepared, []ExecutorCall{call}, [][]common.Hash{leaves}, []map[uint]string{nil}, reader.caller)
				if err != nil || reader.simulations != 1 || len(reader.roleSeen) != count || !bytes.Equal(reader.executed, call.Data) {
					t.Fatalf("lost role/exact simulation: %v sims%d roles%d", err, reader.simulations, len(reader.roleSeen))
				}
				for _, n := range reader.roleSeen {
					if n != 1 {
						t.Fatal("duplicate role checked twice")
					}
				}
				pins := 1
				if advanced {
					pins = 2
				}
				remaining := 5 + count + pins + 1
				wantChunks := (remaining + protocolio.MaxReadBatch - 1) / protocolio.MaxReadBatch
				if len(batch.groups) != wantChunks+1 {
					t.Fatalf("groups=%d want%d", len(batch.groups), wantChunks+1)
				}
				for _, group := range batch.groups[:wantChunks] {
					want := min(remaining, protocolio.MaxReadBatch)
					if len(group) != want {
						t.Fatalf("chunk %d want%d", len(group), want)
					}
					remaining -= want
				}
				closing := batch.groups[len(batch.groups)-1]
				if len(closing) != pins {
					t.Fatal("closing pins not deduplicated")
				}
				for _, request := range closing {
					if request.Kind != protocolio.ReadHeader || request.Block == nil {
						t.Fatal("closing fence not after all contract chunks")
					}
				}
			})
		}
	}
}

func TestPreparedAdmissionClosesBothPinsAndPreservesErrorOrder(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		advanced bool
		want     string
	}{
		{"opening original", true, "reorganized"}, {"opening equal conflict", false, "conflicting"}, {"stale latest", false, "invalid prepared block"}, {"missing latest", false, "not found"},
		{"closing original", true, "reorganized"}, {"closing latest", true, "reorganized"}, {"during simulation", true, "reorganized"},
		{"false role", true, "lacks leaf"}, {"role error simulation error", true, "role"}, {"simulation error metadata error", true, "root"}, {"metadata error", true, "root"},
	} {
		for _, batched := range []bool{false, true} {
			t.Run(tc.mode+map[bool]string{false: "/single", true: "/batch"}[batched], func(t *testing.T) {
				reader := newExecutionReader(t)
				var chain Reader = reader
				if batched {
					chain = &executionBatchReader{executionReader: reader}
				}
				prepared, err := DiscoverBatch(t.Context(), chain, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				call := assembledCall(t, prepared[0])
				reader.mode, reader.validating = tc.mode, true
				if tc.advanced {
					reader.head = 11
				}
				err = ValidatePreparedBatch(t.Context(), chain, prepared, []ExecutorCall{call}, [][]common.Hash{{{1}}}, []map[uint]string{nil}, reader.caller)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("wrong failure: %v", err)
				}
			})
		}
	}
}

func TestPreparedAdmissionRejectsMetadataDriftAtUnchangedHead(t *testing.T) {
	for _, field := range []string{"account", "caller", "root", "recipe", "recipe block"} {
		t.Run(field, func(t *testing.T) {
			reader := newExecutionReader(t)
			batch := &executionBatchReader{executionReader: reader}
			prepared, err := DiscoverBatch(t.Context(), batch, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			call, caller := assembledCall(t, prepared[0]), reader.caller
			reader.validating = true
			switch field {
			case "account":
				reader.account = common.HexToAddress("0x4444")
			case "caller":
				reader.caller = common.HexToAddress("0x4444")
			case "root":
				reader.root = common.HexToHash("0x4444")
			case "recipe":
				reader.recipeHash = common.HexToHash("0x4444")
			case "recipe block":
				reader.recipeBlock = 9
			}
			if err = ValidatePreparedBatch(t.Context(), batch, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, caller); err == nil {
				t.Fatal("unchanged header authenticated changed metadata")
			}
		})
	}
}

func TestPreparedAdmissionAuthenticatesSuppliedIdentity(t *testing.T) {
	for _, field := range []string{"account", "caller", "executor", "root", "recipe hash", "recipe bytes", "execution bytes"} {
		t.Run(field, func(t *testing.T) {
			reader := newExecutionReader(t)
			prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			call := assembledCall(t, prepared[0])
			switch field {
			case "account":
				prepared[0].Snapshot.Account = common.Address{44}
			case "caller":
				prepared[0].Snapshot.Caller = common.Address{44}
			case "executor":
				call.Executor = common.Address{44}
			case "root":
				prepared[0].Snapshot.Root = common.Hash{44}
			case "recipe hash":
				prepared[0].Snapshot.RecipeHash = common.Hash{44}
			case "recipe bytes":
				prepared[0].Recipe.Steps[0][0] = 255
			case "execution bytes":
				other := prepared[0]
				other.Recipe = Recipe{}
				call = assembledCall(t, other)
			}
			reader.validating = true
			if err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{nil}, []map[uint]string{nil}, reader.caller); err == nil {
				t.Fatal("forged prepared identity accepted")
			}
		})
	}
}

func TestBatchDiscoveryChecksIdentityBeforeAssembly(t *testing.T) {
	for _, who := range []string{"account", "caller"} {
		t.Run(who, func(t *testing.T) {
			reader := newExecutionReader(t)
			account, caller := reader.account, reader.caller
			if who == "account" {
				account = common.HexToAddress("0x4444")
			} else {
				caller = common.HexToAddress("0x4444")
			}
			prepared, err := DiscoverBatch(t.Context(), &executionBatchReader{executionReader: reader}, []common.Address{reader.executor}, []common.Address{account}, caller, PreparationOptions{})
			if err == nil || len(prepared) != 0 || reader.simulations != 0 {
				t.Fatalf("untrusted identity reached assembly: %v prepared%d", err, len(prepared))
			}
		})
	}
}

func TestBatchDiscoverySingleAndBatchReturnSamePreparation(t *testing.T) {
	var snapshots []Snapshot
	for _, batched := range []bool{false, true} {
		reader := newExecutionReader(t)
		var chain Reader = reader
		if batched {
			chain = &executionBatchReader{executionReader: reader}
		}
		prepared, err := DiscoverBatch(t.Context(), chain, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, prepared[0].Snapshot)
		call := assembledCall(t, prepared[0])
		reader.validating, reader.head = true, 11
		if err = ValidatePreparedBatch(t.Context(), chain, prepared, []ExecutorCall{call}, [][]common.Hash{{{1}}}, []map[uint]string{nil}, reader.caller); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
		t.Fatal("batching changed prepared snapshot")
	}
}

func TestSharedPathRetainsPostReadCanonicalGuard(t *testing.T) {
	for _, phase := range []string{"discovery", "metadata", "roles"} {
		t.Run(phase, func(t *testing.T) {
			reader := newExecutionReader(t)
			prepared, err := DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			call := assembledCall(t, prepared[0])
			reader.afterCall = func(data []byte) {
				if phase == "roles" && !bytes.Equal(data[:4], accountABI.PackHasRole(common.Hash{}, reader.executor)[:4]) {
					return
				}
				header := types.CopyHeader(reader.headers[10])
				header.Extra = []byte("post-read reorg")
				reader.headers[10] = header
			}
			if phase == "discovery" {
				_, err = DiscoverBatch(t.Context(), reader, []common.Address{reader.executor}, []common.Address{reader.account}, reader.caller, PreparationOptions{})
			} else {
				reader.validating = true
				err = ValidatePreparedBatch(t.Context(), reader, prepared, []ExecutorCall{call}, [][]common.Hash{{{1}}}, []map[uint]string{nil}, reader.caller)
			}
			if err == nil {
				t.Fatal("post-read canonical guard removed")
			}
		})
	}
}
