package merkl

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type preparationBatchReader struct{ reader }

func (r preparationBatchReader) BatchRead(ctx context.Context, qs []chainio.ReadRequest) ([]chainio.ReadResult, error) {
	results := make([]chainio.ReadResult, len(qs))
	for i, q := range qs {
		results[i].Data, results[i].Err = r.CallContract(ctx, q.Call, q.Block)
	}
	return results, nil
}
func TestMerklPreparePreservesAPIEligibilityAndLazyReadErrors(t *testing.T) {
	for _, batched := range []bool{false, true} {
		for _, mode := range []string{"valid", "absent", "absent skip", "member error", "malformed", "wrong count", "getter failure", "invalid snapshot", "cancel"} {
			t.Run(mode+map[bool]string{false: "/single", true: "/batch"}[batched], func(t *testing.T) {
				apiCalls, chainCalls := 0, 0
				root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
				reward := wire.MerklReward{Token: wire.MerklToken{Address: addr(1).Hex(), Decimals: 6}, Amount: "100", Claimed: "70", Pending: "0", Proofs: []string{common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111").Hex()}}
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					apiCalls++
					rewards := []wire.MerklReward{reward}
					if mode == "absent" || mode == "absent skip" {
						rewards = nil
					}
					if rewards == nil {
						rewards = []wire.MerklReward{}
					}
					_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rewards}})
				}))
				defer srv.Close()
				chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
					chainCalls++
					if b == nil || b.Uint64() != 12 || m.To == nil || *m.To != addr(5) {
						t.Fatal("preparation read escaped pin")
					}
					if mode == "member error" || mode == "absent" || mode == "absent skip" {
						return nil, errors.New("unused speculative getter failure")
					}
					if mode == "malformed" {
						return []byte{1}, nil
					}
					a := binding.NewMerklDistributor().GetABI()
					pack := func(method string, v ...any) []byte {
						raw, err := a.Methods[method].Outputs.Pack(v...)
						if err != nil {
							t.Fatal(err)
						}
						return raw
					}
					values := []multicall.Multicall3Result{{Success: true, ReturnData: pack("getMerkleRoot", root)}, {Success: true, ReturnData: pack("claimRecipient", common.Address{})}, {Success: true, ReturnData: pack("claimed", big.NewInt(80), big.NewInt(500), root)}, {Success: true, ReturnData: pack("claimRecipient", addr(3))}}
					if mode == "wrong count" {
						values = values[:3]
					}
					if mode == "getter failure" {
						values[2].Success = false
					}
					return multicall.NewMulticall3().GetABI().Methods["aggregate3"].Outputs.Pack(values)
				})
				var backend chainio.Reader = chain
				if batched {
					backend = preparationBatchReader{chain}
				}
				p, err := New(Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: 452, AmountInputs: []uint{0}, ClaimableInputs: []uint{1}, SkipIfEmpty: mode == "absent skip", DedicatedClaimGroup: mode == "absent skip"}, registry.Deps{Chain: backend, HTTP: srv.Client()})
				if err != nil {
					t.Fatal(err)
				}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				s := chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), BlockHash: common.HexToHash("0x12"), Timestamp: 144, Executor: addr(6), Account: addr(3), Caller: addr(7)}
				if mode == "invalid snapshot" {
					s.BlockNumber = nil
				}

				if mode == "cancel" {
					cancel()
				}
				out, err := p.Prepare(ctx, s)
				switch mode {
				case "valid":
					if err != nil || out.Inputs[0] != "100" || out.Inputs[1] != "20" || chainCalls != 1 || apiCalls != 1 {
						t.Fatalf("prepared claim changed: %+v %v calls%d API%d", out, err, chainCalls, apiCalls)
					}
					direct, e := p.Prepare(t.Context(), s)
					if e != nil || !reflect.DeepEqual(out, direct) {
						t.Fatalf("direct and repeated outputs differ: %v", e)
					}
				case "absent":
					if !errors.Is(err, provider.ErrNotReady) {
						t.Fatalf("unused error displaced idle: %v", err)
					}
				case "absent skip":
					if err != nil || len(out.SkipSteps) != 1 || out.Inputs[0] != "0" {
						t.Fatalf("skip changed: %+v %v", out, err)
					}
				default:
					if err == nil {
						t.Fatal("invalid preparation read accepted")
					}
				}
				if (mode == "invalid snapshot" || mode == "cancel") && apiCalls != 0 {
					t.Fatal("invalid identity/cancellation reached API")
				}
				if !batched && (mode == "absent" || mode == "absent skip" || mode == "invalid snapshot" || mode == "cancel") && chainCalls != 0 {
					t.Fatal("unused preparation read executed")
				}
				if chainCalls > 1 && mode != "valid" {
					t.Fatal("preparation result retried")
				}
			})
		}
	}
}
