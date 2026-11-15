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
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type reader func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f reader) CallContract(c context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	return f(c, m, b)
}
func addr(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }
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
					if test == "redirected" || test == "recipient override" {
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
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Tokens: []string{addr(1).Hex()}, ProofCapacities: []int{1}, Action: "0:0", CalldataBytes: len(payload), ClaimableInputs: []uint{1}}
			if test == "proof-layout" {
				cfg.ProofCapacities[0] = 2
			}
			p, e := New(cfg, provider.Deps{Chain: multicallReader(t, []common.Address{addr(1)}, chain), HTTP: srv.Client()})
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
			if out.Inputs[1] != "20" || hexutil.Encode(out.Actions["0:0"]) != hexutil.Encode(payload) {
				t.Fatal("claim did not use cumulative amount/current claimed balance")
			}
		})
	}
}

func TestPrepareIdlesOnlyAfterValidEmptyWork(t *testing.T) {
	for _, test := range []string{"no-chain", "no-token", "fully-claimed", "bad-proof", "redirected", "rpc-error", "malformed-response"} {
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
			abi := binding.NewMerklDistributor().GetABI()
			chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				rpcCalls++
				if test == "rpc-error" {
					return nil, errors.New("local RPC unavailable")
				}
				if b.Int64() != 12 || *m.To != addr(4) {
					t.Error("claim read not pinned")
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
					claimed := int64(95)
					if test == "fully-claimed" {
						claimed = 100
					}
					return method.Outputs.Pack(big.NewInt(claimed), big.NewInt(900), root)
				case "claimRecipient":
					if test == "redirected" {
						return method.Outputs.Pack(addr(9))
					}
					return method.Outputs.Pack(common.Address{})
				}
				t.Fatalf("unexpected method %s", method.Name)
				return nil, nil
			})
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Tokens: []string{addr(1).Hex()}, ProofCapacities: []int{1}, Action: "7:0", CalldataBytes: 452, ClaimableInputs: []uint{4}}
			p, e := New(cfg, provider.Deps{Chain: multicallReader(t, []common.Address{addr(1)}, chain), HTTP: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			out, e := p.Prepare(context.Background(), provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000})
			switch test {
			case "bad-proof", "redirected", "rpc-error", "malformed-response":
				if e == nil || errors.Is(e, provider.ErrNotReady) {
					t.Fatalf("invalid response became idle: %+v %v", out, e)
				}
				return
			}
			if !errors.Is(e, provider.ErrNotReady) || len(out.Actions) != 0 {
				t.Fatalf("valid empty work was not idle: %+v %v", out, e)
			}
			if (test == "no-chain" || test == "no-token") && rpcCalls != 0 {
				t.Fatal("empty rewards performed unnecessary claim reads")
			}
		})
	}
}
