package merkl

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"gopkg.in/yaml.v3"
)

type reader func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f reader) CallContract(c context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	return f(c, m, b)
}
func addr(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }
func node(t *testing.T, c any) yaml.Node {
	t.Helper()
	b, e := yaml.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	var n yaml.Node
	if e = yaml.Unmarshal(b, &n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestMerklProofViemGolden(t *testing.T) {
	sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
	root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
	if !VerifyProof(addr(3), addr(1), big.NewInt(100), [][32]byte{sibling}, root) {
		t.Fatal("independent viem proof vector rejected")
	}
	if VerifyProof(addr(3), addr(1), big.NewInt(99), [][32]byte{sibling}, root) {
		t.Fatal("incorrect cumulative amount accepted")
	}
}
func TestPrepareCumulativeSelfClaim(t *testing.T) {
	for _, test := range []string{"valid", "wrong-root", "redirected", "duplicate-token", "proof-layout", "already-claimed"} {
		t.Run(test, func(t *testing.T) {
			sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
			root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
			reward := wire.MerklReward{Token: wire.MerklToken{Address: addr(1).Hex(), Decimals: 6}, Amount: "100", Claimed: "70", Pending: "500", Proofs: []string{sibling.Hex()}}
			rows := []wire.MerklReward{reward}
			if test == "duplicate-token" {
				rows = append(rows, reward)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v4/users/"+strings.ToLower(addr(3).Hex())+"/rewards" || r.URL.Query().Get("chainId") != "1" {
					t.Error("wrong rewards path")
				}
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
			}))
			defer srv.Close()
			a := binding.NewMerklDistributor()
			abi := a.GetABI()
			chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				if b.Cmp(big.NewInt(12)) != 0 || *m.To != addr(4) {
					t.Error("unbound read")
				}
				method, e := abi.MethodById(m.Data[:4])
				if e != nil {
					return nil, e
				}
				switch method.Name {
				case "getMerkleRoot":
					if test == "wrong-root" {
						return method.Outputs.Pack([32]byte{})
					}
					return method.Outputs.Pack(root)
				case "claimed":
					n := int64(80)
					if test == "already-claimed" {
						n = 100
					}
					return method.Outputs.Pack(big.NewInt(n), big.NewInt(500), root)
				case "claimRecipient":
					if test == "redirected" {
						return method.Outputs.Pack(addr(9))
					}
					return method.Outputs.Pack(common.Address{})
				default:
					t.Fatalf("unexpected %s", method.Name)
				}
				return nil, nil
			})
			payload, e := a.TryPackClaim([]common.Address{addr(3)}, []common.Address{addr(1)}, []*big.Int{big.NewInt(100)}, [][][32]byte{{sibling}})
			if e != nil {
				t.Fatal(e)
			}
			if len(payload) != 452 {
				t.Fatalf("claim ABI layout changed: %d", len(payload))
			}
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: len(payload), AmountInputs: []uint{0}, ClaimableInputs: []uint{1}}
			if test == "proof-layout" {
				cfg.ProofLengths[0] = 2
			}
			p, e := Factory(node(t, cfg), providerRegistry.Deps{Chain: chain, HTTP: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			out, e := p.Prepare(context.Background(), provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000})
			if test != "valid" {
				if e == nil {
					t.Fatalf("invalid %s accepted", test)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if out.Inputs[0] != "100" || out.Inputs[1] != "20" || hexutil.Encode(out.Actions["0:0"]) != hexutil.Encode(payload) {
				t.Fatal("claim did not use cumulative amount/current claimed balance")
			}
		})
	}
}

func TestOptionalClaimSkipsOnlyValidEmptyWork(t *testing.T) {
	for _, test := range []string{"no-chain", "no-token", "below-minimum", "bad-proof", "redirected", "rpc-error", "malformed-response", "idle-no-skip"} {
		t.Run(test, func(t *testing.T) {
			sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
			root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
			reward := wire.MerklReward{Token: wire.MerklToken{Address: addr(1).Hex(), Decimals: 6}, Amount: "100", Claimed: "80", Pending: "0", Proofs: []string{sibling.Hex()}}
			entries := []wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: []wire.MerklReward{reward}}}
			switch test {
			case "no-chain":
				entries = []wire.MerklEntry{}
			case "no-token":
				entries[0].Rewards = []wire.MerklReward{}
			case "malformed-response":
				entries[0].Rewards = nil
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(entries) }))
			defer srv.Close()
			rpcCalls := 0
			a := binding.NewMerklDistributor()
			abi := a.GetABI()
			chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				rpcCalls++
				if test == "rpc-error" {
					return nil, errors.New("local RPC unavailable")
				}
				if b.Int64() != 12 || *m.To != addr(4) {
					t.Error("optional claim read not pinned")
				}
				method, e := abi.MethodById(m.Data[:4])
				if e != nil {
					return nil, e
				}
				switch method.Name {
				case "getMerkleRoot":
					if test == "bad-proof" {
						return method.Outputs.Pack(common.HexToHash("0x1234"))
					}
					return method.Outputs.Pack(root)
				case "claimed":
					return method.Outputs.Pack(big.NewInt(95), big.NewInt(900), root)
				case "claimRecipient":
					if test == "redirected" {
						return method.Outputs.Pack(addr(9))
					}
					return method.Outputs.Pack(common.Address{})
				}
				t.Fatalf("unexpected method %s", method.Name)
				return nil, nil
			})
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "7:0", CalldataBytes: 452, MinClaimable: "10", AmountInputs: []uint{2}, ClaimableInputs: []uint{4}, SkipIfEmpty: true, DedicatedClaimGroup: true}
			if test == "idle-no-skip" {
				cfg.SkipIfEmpty = false
				cfg.DedicatedClaimGroup = false
			}
			p, e := Factory(node(t, cfg), providerRegistry.Deps{Chain: chain, HTTP: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			out, e := p.Prepare(context.Background(), provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000})
			switch test {
			case "bad-proof", "redirected", "rpc-error", "malformed-response":
				if e == nil || errors.Is(e, provider.ErrNotReady) {
					t.Fatalf("invalid response became skip/idle: %+v %v", out, e)
				}
				return
			case "idle-no-skip":
				if !errors.Is(e, provider.ErrNotReady) {
					t.Fatalf("below minimum not normal idle: %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if len(out.SkipSteps) != 1 || out.SkipSteps[0] != 7 || len(out.Actions) != 0 || out.Inputs[2] != "0" || out.Inputs[4] != "0" {
				t.Fatalf("optional claim did not zero gate and skip dedicated step: %+v", out)
			}
			var data struct {
				Skipped bool `json:"skipped"`
			}
			if json.Unmarshal(out.Data, &data) != nil || !data.Skipped {
				t.Fatal("skip state not persisted")
			}
			if (test == "no-chain" || test == "no-token") && rpcCalls != 0 {
				t.Fatal("empty rewards performed unnecessary claim reads")
			}
		})
	}
}

func TestOptionalClaimRequiresDedicatedSingleActionLayout(t *testing.T) {
	for _, test := range []string{"missing-ack", "batch", "nonzero-action", "wrong-layout", "step-overflow"} {
		t.Run(test, func(t *testing.T) {
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: "https://example.invalid"}, Distributor: addr(4).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "7:0", CalldataBytes: 452, SkipIfEmpty: true, DedicatedClaimGroup: true}
			switch test {
			case "missing-ack":
				cfg.DedicatedClaimGroup = false
			case "batch":
				cfg.Tokens = append(cfg.Tokens, addr(2).Hex())
				cfg.ProofLengths = append(cfg.ProofLengths, 1)
			case "nonzero-action":
				cfg.Action = "7:1"
			case "wrong-layout":
				cfg.CalldataBytes = 420
			case "step-overflow":
				cfg.Action = "999999999999999999999999999999:0"
			}
			_, e := Factory(node(t, cfg), providerRegistry.Deps{Chain: reader(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				t.Fatal("factory made RPC call")
				return nil, nil
			})})
			if e == nil {
				t.Fatal("unsafe optional claim configuration accepted")
			}
		})
	}
}
