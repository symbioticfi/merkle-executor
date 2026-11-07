package recipes

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"math/big"
	"strings"
	"testing"
)

type initialBatch struct {
	*cachedReader
	groups  [][]chainio.ReadRequest
	failure string
}

func (r *initialBatch) BatchRead(ctx context.Context, qs []chainio.ReadRequest) ([]chainio.ReadResult, error) {
	r.groups = append(r.groups, qs)
	if len(r.groups) == 1 {
		switch r.failure {
		case "transport":
			return nil, errors.New("batch unavailable")
		case "shape":
			return nil, nil
		}
	}
	return cachedBatch{r.cachedReader}.BatchRead(ctx, qs)
}
func TestInitialStagedReadsAreLazyBoundedAndOnlyConsumedAfterIdentity(t *testing.T) {
	for _, batched := range []bool{false, true} {
		for _, count := range []int{0, 1, 251, 252, 257} {
			t.Run(big.NewInt(int64(count)).String()+map[bool]string{false: "/single", true: "/batch"}[batched], func(t *testing.T) {
				r, _ := cacheFixture(t, Recipe{})
				batch := &initialBatch{cachedReader: r}
				var reader Reader = r
				if batched {
					reader = batch
				}
				var pin chainio.PreparationPin
				options := PreparationOptions{ChainID: big.NewInt(1), Stage: func(p chainio.PreparationPin) []chainio.ReadRequest {
					pin = p
					qs := make([]chainio.ReadRequest, count)
					for i := range qs {
						qs[i] = chainio.ContractRead(p.Account, accountABI.PackRoot(), big.NewInt(10), true)
					}
					return qs
				}}
				err := PrepareExecutionWithOptions(t.Context(), reader, r.executor, r.account, r.caller, options, func(s Snapshot, _ Recipe, read chainio.ReadAccessor) ([]byte, []common.Hash, error) {
					if s.BlockHash != pin.BlockHash || s.Root != r.root {
						t.Fatal("unverified build")
					}
					for i := range count {
						got := read(i)
						root, err := accountABI.UnpackRoot(got.Data)
						if err != nil || root != r.root {
							t.Fatal("staged read lost")
						}
					}
					if read(-1).Err == nil || read(count).Err == nil {
						t.Fatal("unbounded initial accessor")
					}
					r.validating = true
					return executionPayload, nil, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if batched {
					chunks := (count + 5 + 255) / 256
					if len(batch.groups) != chunks+4 {
						t.Fatalf("groups=%d chunks%d", len(batch.groups), chunks)
					}
					for _, qs := range batch.groups {
						if len(qs) > 256 {
							t.Fatal("unbounded initial batch")
						}
					}
				}
			})
		}
	}
}
func TestInitialStagedFailureNeverFallsBackOrReachesUnauthenticatedBuild(t *testing.T) {
	for _, mode := range []string{"transport", "shape", "account", "caller"} {
		t.Run(mode, func(t *testing.T) {
			r, _ := cacheFixture(t, Recipe{})
			account, caller := r.account, r.caller
			if mode == "account" {
				r.account = common.HexToAddress("0x8888")
			}
			if mode == "caller" {
				r.caller = common.HexToAddress("0x8888")
			}
			batch := &initialBatch{cachedReader: r, failure: mode}
			opts := PreparationOptions{ChainID: big.NewInt(1), Stage: func(p chainio.PreparationPin) []chainio.ReadRequest {
				return []chainio.ReadRequest{chainio.ContractRead(p.Account, accountABI.PackRoot(), big.NewInt(10), true)}
			}}
			err := PrepareExecutionWithOptions(t.Context(), batch, r.executor, account, caller, opts, func(Snapshot, Recipe, chainio.ReadAccessor) ([]byte, []common.Hash, error) {
				t.Fatal("unauthenticated build/API reached")
				return nil, nil, nil
			})
			if err == nil {
				t.Fatal("invalid stage accepted")
			}
			if (mode == "transport" || mode == "shape") && (r.allReads != 1 || len(batch.groups) != 1) {
				t.Fatal("failed batch retried as singles")
			}
			if mode == "caller" && !strings.Contains(err.Error(), "mismatch") {
				t.Fatal(err)
			}
		})
	}
}

type sharedHeadReader struct {
	Reader
	built     *bool
	earlyHead bool
}

func (r *sharedHeadReader) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if n == nil && !*r.built {
		r.earlyHead = true
	}
	return r.Reader.HeaderByNumber(ctx, n)
}
func TestSharedPreparationPinSkipsHeadAcquisitionAndKeepsClosures(t *testing.T) {
	r, c := cacheFixture(t, Recipe{})
	built := false
	reader := &sharedHeadReader{Reader: r, built: &built}
	pin := types.CopyHeader(r.headers[10])
	err := PrepareExecutionWithOptions(t.Context(), reader, r.executor, r.account, r.caller, PreparationOptions{ChainID: big.NewInt(1), Cache: c, Pin: pin}, func(s Snapshot, _ Recipe, _ chainio.ReadAccessor) ([]byte, []common.Hash, error) {
		built = true
		r.validating = true
		if s.BlockNumber != 10 {
			t.Fatal("shared head lost")
		}
		return executionPayload, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if reader.earlyHead {
		t.Fatal("reacquired opening head")
	}
	if r.headerReads == 0 {
		t.Fatal("omitted fresh canonical closures")
	}
}
