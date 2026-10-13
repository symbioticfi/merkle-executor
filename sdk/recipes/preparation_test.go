package recipes

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"math/big"
	"strings"
	"testing"
)

type initialBatch struct {
	*cachedReader
	groups  [][]protocolio.ReadRequest
	failure string
}

func (r *initialBatch) BatchRead(ctx context.Context, qs []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
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
				var pin protocolio.PreparationPin
				options := PreparationOptions{ChainID: big.NewInt(1), Stage: func(p protocolio.PreparationPin) []protocolio.ReadRequest {
					pin = p
					qs := make([]protocolio.ReadRequest, count)
					for i := range qs {
						qs[i] = protocolio.ContractRead(p.Account, accountABI.PackRoot(), big.NewInt(10), true)
					}
					return qs
				}}
				err := PrepareExecutionWithOptions(t.Context(), reader, r.executor, r.account, r.caller, options, func(s Snapshot, _ Recipe, read protocolio.ReadAccessor) ([]byte, []common.Hash, error) {
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
			opts := PreparationOptions{ChainID: big.NewInt(1), Stage: func(p protocolio.PreparationPin) []protocolio.ReadRequest {
				return []protocolio.ReadRequest{protocolio.ContractRead(p.Account, accountABI.PackRoot(), big.NewInt(10), true)}
			}}
			err := PrepareExecutionWithOptions(t.Context(), batch, r.executor, account, caller, opts, func(Snapshot, Recipe, protocolio.ReadAccessor) ([]byte, []common.Hash, error) {
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
