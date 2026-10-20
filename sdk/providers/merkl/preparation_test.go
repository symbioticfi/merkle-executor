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
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type stagedBatchReader struct{ reader }

func (r stagedBatchReader) BatchRead(ctx context.Context, qs []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	results := make([]protocolio.ReadResult, len(qs))
	for i, q := range qs {
		results[i].Data, results[i].Err = r.CallContract(ctx, q.Call, q.Block)
	}
	return results, nil
}
func TestMerklStagedPreparationPreservesAPIEligibilityAndLazyErrors(t *testing.T) {
	for _, batched := range []bool{false, true} {
		for _, mode := range []string{"valid", "absent", "absent skip", "member error", "malformed", "wrong count", "getter failure", "pin mismatch", "cancel"} {
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
						t.Fatal("staged read escaped pin")
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
				p, err := New(Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: 452, AmountInputs: []uint{0}, ClaimableInputs: []uint{1}, SkipIfEmpty: mode == "absent skip", DedicatedClaimGroup: mode == "absent skip"}, provider.Deps{Chain: chain, HTTP: srv.Client()})
				if err != nil {
					t.Fatal(err)
				}
				pin := protocolio.PreparationPin{ChainID: "1", BlockNumber: 12, BlockHash: common.HexToHash("0x12"), Timestamp: 144, Executor: addr(6), Account: addr(3), Caller: addr(7)}
				plan, err := p.PlanPreparation(pin)
				if err != nil || chainCalls != 0 || apiCalls != 0 || len(plan.Requests) != 1 || plan.Requests[0].MulticallSafe {
					t.Fatalf("planning performed I/O/nested aggregate: %v", err)
				}
				var backend any = chain
				if batched {
					backend = stagedBatchReader{chain}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				reads, err := protocolio.ReadGroup(ctx, backend, plan.Requests...)
				if err != nil {
					t.Fatal(err)
				}
				s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), BlockHash: pin.BlockHash, Timestamp: pin.Timestamp, Executor: pin.Executor, Account: pin.Account, Caller: pin.Caller}
				if mode == "pin mismatch" {
					s.BlockNumber = big.NewInt(13)
				}
				if mode == "cancel" {
					cancel()
				}
				out, err := plan.Resolve(ctx, s, reads.At)
				switch mode {
				case "valid":
					if err != nil || out.Inputs[0] != "100" || out.Inputs[1] != "20" || chainCalls != 1 || apiCalls != 1 {
						t.Fatalf("staged claim changed: %+v %v calls%d API%d", out, err, chainCalls, apiCalls)
					}
					direct, e := p.Prepare(t.Context(), s)
					if e != nil || !reflect.DeepEqual(out, direct) {
						t.Fatalf("direct and staged outputs differ: %v", e)
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
						t.Fatal("invalid staged read accepted")
					}
				}
				if (mode == "pin mismatch" || mode == "cancel") && apiCalls != 0 {
					t.Fatal("invalid identity/cancellation reached API")
				}
				if !batched && (mode == "absent" || mode == "absent skip" || mode == "pin mismatch" || mode == "cancel") && chainCalls != 0 {
					t.Fatal("single unused staged read executed")
				}
				if chainCalls > 1 && mode != "valid" {
					t.Fatal("staged result retried")
				}
			})
		}
	}
}
