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

var executionPayload = []byte{0xde, 0xad, 0xbe, 0xef}

type executionReader struct {
	*fakeReader
	validating, latestSimulation bool
	headerReads, simulations     int
	mode                         string
	roleSeen                     map[common.Hash]int
	afterCall                    func([]byte)
}

func newExecutionReader(t *testing.T) *executionReader {
	return &executionReader{fakeReader: fakeChain(t, Recipe{}), roleSeen: map[common.Hash]int{}}
}
func (r *executionReader) HeaderByNumber(ctx context.Context, b *big.Int) (*types.Header, error) {
	h, err := r.fakeReader.HeaderByNumber(ctx, b)
	if !r.validating {
		return h, err
	}
	r.headerReads++
	switch {
	case r.mode == "opening original" && r.headerReads == 1, r.mode == "opening equal conflict" && r.headerReads == 2, r.mode == "closing original" && r.headerReads == 3, r.mode == "closing latest" && r.headerReads == 4:
		h = types.CopyHeader(h)
		h.Extra = []byte("reorg")
	case r.mode == "stale latest" && r.headerReads == 2:
		h = r.headers[8]
	case r.mode == "missing latest" && r.headerReads == 2:
		return nil, ethereum.NotFound
	}
	return h, err
}
func (r *executionReader) CallContract(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	if bytes.Equal(m.Data, executionPayload) {
		r.simulations++
		if m.From != common.HexToAddress("0x3333") || m.To == nil || *m.To != r.executor || r.latestSimulation != (b == nil) || b != nil && b.Uint64() != 10 {
			r.t.Fatalf("simulation context changed: %+v block%v", m, b)
		}
		if r.mode == "during simulation" {
			h := types.CopyHeader(r.headers[10])
			h.Extra = []byte("simulation reorg")
			r.headers[10] = h
		}
		if strings.Contains(r.mode, "simulation error") {
			return nil, errors.New("speculative simulation failure")
		}
		return nil, nil
	}
	if r.validating && bytes.Equal(m.Data[:4], accountABI.PackHasRole(common.Hash{}, r.executor)[:4]) {
		if b == nil || b.Uint64() != 10 {
			r.t.Fatal("role escaped original pin")
		}
		method := accountABI.GetABI().Methods["hasRole"]
		args, err := method.Inputs.Unpack(m.Data[4:])
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
	if r.validating && bytes.Equal(m.Data, accountABI.PackRoot()) && strings.Contains(r.mode, "metadata error") {
		return nil, errors.New("required root unavailable")
	}
	if m.To != nil && *m.To != r.executor && *m.To != r.account {
		return nil, errors.New("unknown speculative target")
	}
	raw, err := r.fakeReader.CallContract(ctx, m, b)
	if r.afterCall != nil {
		r.afterCall(m.Data)
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
	for i, q := range requests {
		switch q.Kind {
		case protocolio.ReadCall:
			if bytes.Equal(q.Call.Data, executionPayload) {
				if q.MulticallSafe {
					r.t.Fatal("simulation marked aggregate-safe")
				}
			} else if !q.MulticallSafe {
				r.t.Fatal("verified getter lost aggregation eligibility")
			}
			out[i].Data, out[i].Err = r.CallContract(ctx, q.Call, q.Block)
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = r.HeaderByNumber(ctx, q.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = r.FilterLogs(ctx, q.Filter)
		default:
			r.t.Fatal("unexpected request")
		}
	}
	return out, nil
}
func TestPrepareExecutionChecksEveryRoleAcrossChunks(t *testing.T) {
	for _, count := range []int{0, 1, 250, 251, 256, 257} {
		for _, advanced := range []bool{false, true} {
			t.Run(big.NewInt(int64(count)).String()+map[bool]string{false: "/same", true: "/advanced"}[advanced], func(t *testing.T) {
				r := newExecutionReader(t)
				batch := &executionBatchReader{executionReader: r}
				leaves := make([]common.Hash, 0, count+1)
				for i := range count {
					leaves = append(leaves, common.BigToHash(big.NewInt(int64(i+1))))
				}
				if count > 0 {
					leaves = append(leaves, leaves[0])
				}
				builds := 0
				err := PrepareExecution(t.Context(), batch, r.executor, r.account, r.caller, func(s Snapshot, recipe Recipe) ([]byte, []common.Hash, error) {
					builds++
					if s.BlockNumber != 10 || s.Account != r.account || s.Root != r.root || len(recipe.Steps) != 0 {
						t.Fatal("unverified build identity")
					}
					r.validating = true
					if advanced {
						r.head = 11
					}
					return executionPayload, leaves, nil
				})
				if err != nil || builds != 1 || r.simulations != 1 || len(r.roleSeen) != count {
					t.Fatalf("lost role/simulation: %v builds%d sims%d roles%d", err, builds, r.simulations, len(r.roleSeen))
				}
				for _, n := range r.roleSeen {
					if n != 1 {
						t.Fatal("duplicate role checked twice")
					}
				}
				wantChunks := (count + 6 + 255) / 256
				if len(batch.groups) != wantChunks+2 {
					t.Fatalf("groups=%d want%d", len(batch.groups), wantChunks+2)
				}
				remaining := count + 6
				for _, g := range batch.groups[1 : len(batch.groups)-1] {
					want := min(remaining, 256)
					if len(g) != want {
						t.Fatalf("chunk %d want%d", len(g), want)
					}
					remaining -= want
				}
				closing := batch.groups[len(batch.groups)-1]
				wantClose := 1
				if advanced {
					wantClose = 2
				}
				if len(closing) != wantClose {
					t.Fatal("closing pins not deduplicated")
				}
				for _, q := range closing {
					if q.Kind != protocolio.ReadHeader || q.Block == nil {
						t.Fatal("closing fence not after all contract chunks")
					}
				}
			})
		}
	}
}
func TestPrepareExecutionClosesBothPinsAndPreservesErrorOrder(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		advanced bool
		want     string
	}{
		{"opening original", true, "reorganized"}, {"opening equal conflict", false, "conflict"}, {"stale latest", false, "precedes"}, {"missing latest", false, "head"},
		{"closing original", true, "reorganized"}, {"closing latest", true, "reorganized"}, {"during simulation", true, "reorganized"},
		{"false role", true, "lacks leaf"}, {"role error simulation error", true, "role"}, {"simulation error metadata error", true, "simulation"}, {"metadata error", true, "root"},
	} {
		for _, batched := range []bool{false, true} {
			t.Run(tc.mode+map[bool]string{false: "/single", true: "/batch"}[batched], func(t *testing.T) {
				r := newExecutionReader(t)
				r.mode = tc.mode
				var reader Reader = r
				if batched {
					reader = &executionBatchReader{executionReader: r}
				}
				err := PrepareExecution(t.Context(), reader, r.executor, r.account, r.caller, func(Snapshot, Recipe) ([]byte, []common.Hash, error) {
					r.validating = true
					if tc.advanced {
						r.head = 11
					}
					return executionPayload, []common.Hash{{1}}, nil
				})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("wrong failure: %v", err)
				}
			})
		}
	}
}
func TestPrepareExecutionRejectsMetadataDriftAtUnchangedHead(t *testing.T) {
	for _, field := range []string{"account", "caller", "root", "recipe", "recipe block"} {
		t.Run(field, func(t *testing.T) {
			r := newExecutionReader(t)
			batch := &executionBatchReader{executionReader: r}
			err := PrepareExecution(t.Context(), batch, r.executor, r.account, r.caller, func(Snapshot, Recipe) ([]byte, []common.Hash, error) {
				r.validating = true
				switch field {
				case "account":
					r.account = common.HexToAddress("0x4444")
				case "caller":
					r.caller = common.HexToAddress("0x4444")
				case "root":
					r.root = common.HexToHash("0x4444")
				case "recipe":
					r.recipeHash = common.HexToHash("0x4444")
				case "recipe block":
					r.recipeBlock = 9
				}
				return executionPayload, nil, nil
			})
			if err == nil {
				t.Fatal("unchanged header authenticated changed metadata")
			}
		})
	}
}
func TestValidateExecutionAuthenticatesFieldsAndClosesAfterLatestSimulation(t *testing.T) {
	for _, mode := range []string{"same", "advanced", "forged account", "forged root", "during simulation", "closing original", "closing latest", "metadata error simulation error"} {
		t.Run(mode, func(t *testing.T) {
			r := newExecutionReader(t)
			snapshot, _, err := DiscoverForAccount(t.Context(), r, r.executor, r.account, nil)
			if err != nil {
				t.Fatal(err)
			}
			r.mode = mode
			r.validating = true
			r.latestSimulation = true
			if mode == "advanced" || mode == "closing latest" {
				r.head = 11
			}
			if mode == "forged account" {
				snapshot.Account = common.HexToAddress("0x4444")
			}
			if mode == "forged root" {
				snapshot.Root = common.HexToHash("0x4444")
			}
			batch := &executionBatchReader{executionReader: r}
			err = ValidateExecution(t.Context(), batch, snapshot, r.caller, executionPayload)
			if mode == "same" || mode == "advanced" {
				if err != nil || r.simulations != 1 || len(batch.groups) != 3 {
					t.Fatalf("validation: %v sims%d groups%d", err, r.simulations, len(batch.groups))
				}
			} else if err == nil {
				t.Fatal("invalid snapshot accepted")
			}
			if mode == "metadata error simulation error" && !strings.Contains(err.Error(), "root") {
				t.Fatalf("metadata error masked: %v", err)
			}
		})
	}
}
func TestPrepareExecutionChecksIdentityBeforeBuild(t *testing.T) {
	for _, who := range []string{"account", "caller"} {
		t.Run(who, func(t *testing.T) {
			r := newExecutionReader(t)
			account, caller := r.account, r.caller
			if who == "account" {
				account = common.HexToAddress("0x4444")
			} else {
				caller = common.HexToAddress("0x4444")
			}
			builds := 0
			err := PrepareExecution(t.Context(), &executionBatchReader{executionReader: r}, r.executor, account, caller, func(Snapshot, Recipe) ([]byte, []common.Hash, error) { builds++; return nil, nil, nil })
			if err == nil || builds != 0 {
				t.Fatalf("untrusted identity reached build: %v builds%d", err, builds)
			}
		})
	}
}
func TestPrepareExecutionLegacyAndBatchReturnSameBuild(t *testing.T) {
	var snapshots []Snapshot
	for _, batched := range []bool{false, true} {
		r := newExecutionReader(t)
		var reader Reader = r
		if batched {
			reader = &executionBatchReader{executionReader: r}
		}
		err := PrepareExecution(t.Context(), reader, r.executor, r.account, r.caller, func(s Snapshot, _ Recipe) ([]byte, []common.Hash, error) {
			snapshots = append(snapshots, s)
			r.validating = true
			r.head = 11
			return executionPayload, []common.Hash{{1}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
		t.Fatal("batching changed prepared snapshot")
	}
}
func TestStandaloneAPIsRetainPostReadCanonicalGuard(t *testing.T) {
	for _, op := range []string{"discover", "roles", "snapshot"} {
		t.Run(op, func(t *testing.T) {
			r := newExecutionReader(t)
			snapshot, _, err := DiscoverForAccount(t.Context(), r, r.executor, r.account, nil)
			if err != nil {
				t.Fatal(err)
			}
			r.afterCall = func([]byte) {
				h := types.CopyHeader(r.headers[10])
				h.Extra = []byte("post-read reorg")
				r.headers[10] = h
			}
			switch op {
			case "discover":
				_, _, err = Discover(t.Context(), r, r.executor, nil)
			case "roles":
				err = VerifyRoles(t.Context(), r, snapshot, []common.Hash{{1}})
			case "snapshot":
				err = ValidateSnapshot(t.Context(), r, snapshot, r.caller)
			}
			if err == nil {
				t.Fatal("standalone guard removed")
			}
		})
	}
}
