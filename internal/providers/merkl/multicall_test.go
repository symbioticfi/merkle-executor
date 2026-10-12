package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// This fixture validates the emitted aggregate request, then supplies independent
// distributor getter responses. The Anvil test also executes the official code.
func multicallReader(t *testing.T, tokens []common.Address, inner reader) reader {
	t.Helper()
	return reader(func(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
		t.Helper()
		if msg.To == nil || *msg.To != addr(5) || msg.From != (common.Address{}) || msg.Value != nil && msg.Value.Sign() != 0 {
			t.Fatal("unexpected aggregate target, sender or value")
		}
		a := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
		if len(msg.Data) < 4 || !bytes.Equal(msg.Data[:4], a.ID) {
			t.Fatal("expected one aggregate3 call")
		}
		decoded, err := a.Inputs.Unpack(msg.Data[4:])
		if err != nil {
			t.Fatal(err)
		}
		calls := *abi.ConvertType(decoded[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
		if len(calls) != 2+2*len(tokens) {
			t.Fatalf("batch calls=%d, want %d", len(calls), 2+2*len(tokens))
		}
		distributor := binding.NewMerklDistributor()
		expected := [][]byte{distributor.PackGetMerkleRoot(), distributor.PackClaimRecipient(addr(3), common.Address{})}
		for _, token := range tokens {
			expected = append(expected, distributor.PackClaimed(addr(3), token), distributor.PackClaimRecipient(addr(3), token))
		}
		results := make([]multicall.Multicall3Result, len(calls))
		for i, call := range calls {
			if call.Target != addr(4) || call.AllowFailure || !bytes.Equal(call.CallData, expected[i]) {
				t.Fatalf("batch call %d changed target/failure policy/account/token", i)
			}
			raw, err := inner(ctx, ethereum.CallMsg{From: addr(5), To: &call.Target, Data: call.CallData}, block)
			if err != nil {
				return nil, err
			}
			results[i] = multicall.Multicall3Result{Success: true, ReturnData: raw}
		}
		return a.Outputs.Pack(results)
	})
}

func TestPrepareRejectsInvalidAggregateWithoutIndividualFallback(t *testing.T) {
	sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
	root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
	reward := wire.MerklReward{Token: wire.MerklToken{Address: addr(1).Hex(), Decimals: 6}, Amount: "100", Claimed: "70", Pending: "0", Proofs: []string{sibling.Hex()}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: []wire.MerklReward{reward}}})
	}))
	defer srv.Close()
	for _, tc := range []struct{ name, want string }{
		{"outer RPC error", "chain read failed"},
		{"canceled RPC", "chain read failed"},
		{"empty aggregate", "invalid Merkl aggregate response"},
		{"malformed aggregate", "invalid Merkl aggregate response"},
		{"short result count", "result count mismatch"},
		{"long result count", "result count mismatch"},
		{"failed root", "aggregate getter failed"},
		{"failed unused default", "aggregate getter failed"},
		{"failed claimed", "aggregate getter failed"},
		{"failed recipient", "aggregate getter failed"},
		{"malformed root", "invalid Merkl effective root"},
		{"malformed claimed", "invalid Merkl claimed amount"},
		{"malformed recipient", "invalid Merkl claim recipient"},
		{"malformed required default", "invalid Merkl default claim recipient"},
		{"malformed unused default", ""},
		{"redirected unused default", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			a := binding.NewMerklDistributor()
			pack := func(method string, args ...any) []byte {
				t.Helper()
				raw, err := a.GetABI().Methods[method].Outputs.Pack(args...)
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			results := []multicall.Multicall3Result{
				{Success: true, ReturnData: pack("getMerkleRoot", root)},
				{Success: true, ReturnData: pack("claimRecipient", common.Address{})},
				{Success: true, ReturnData: pack("claimed", big.NewInt(80), big.NewInt(500), root)},
				{Success: true, ReturnData: pack("claimRecipient", addr(3))},
			}
			switch tc.name {
			case "short result count":
				results = results[:3]
			case "long result count":
				results = append(results, results[0])
			case "failed root":
				results[0].Success = false
			case "failed unused default":
				results[1].Success = false
			case "failed claimed":
				results[2].Success = false
			case "failed recipient":
				results[3].Success = false
			case "malformed root":
				results[0].ReturnData = []byte{1}
			case "malformed claimed":
				results[2].ReturnData = []byte{1}
			case "malformed recipient":
				results[3].ReturnData = []byte{1}
			case "malformed required default":
				results[1].ReturnData = []byte{1}
				results[3].ReturnData = pack("claimRecipient", common.Address{})
			case "malformed unused default":
				results[1].ReturnData = []byte{1}
			case "redirected unused default":
				results[1].ReturnData = pack("claimRecipient", addr(9))
			}
			calls := 0
			chain := reader(func(received context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
				calls++
				if received != ctx || block == nil || block.Int64() != 12 || msg.To == nil || *msg.To != addr(5) {
					t.Fatal("aggregate escaped caller context, snapshot block or configured target")
				}
				switch tc.name {
				case "outer RPC error":
					return nil, errors.New("local RPC unavailable")
				case "canceled RPC":
					cancel()
					<-received.Done()
					return nil, received.Err()
				case "empty aggregate":
					return nil, nil
				case "malformed aggregate":
					return []byte{1}, nil
				}
				return multicall.NewMulticall3().GetABI().Methods["aggregate3"].Outputs.Pack(results)
			})
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: 452, AmountInputs: []uint{0}, ClaimableInputs: []uint{1}, SkipIfEmpty: true, DedicatedClaimGroup: true}
			p, err := Factory(node(t, cfg), providerRegistry.Deps{Chain: chain, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.Prepare(ctx, provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000})
			if calls != 1 {
				t.Fatalf("aggregate failure used %d RPC calls; individual fallback is unsupported", calls)
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || errors.Is(err, provider.ErrNotReady) || len(out.Actions) != 0 || len(out.SkipSteps) != 0 {
					t.Fatalf("invalid aggregate became usable claim or skip: output=%+v error=%v", out, err)
				}
				return
			}
			expected := a.PackClaim([]common.Address{addr(3)}, []common.Address{addr(1)}, []*big.Int{big.NewInt(100)}, [][][32]byte{{sibling}})
			if err != nil || out.Inputs[0] != "100" || out.Inputs[1] != "20" || !bytes.Equal(out.Actions["0:0"], expected) {
				t.Fatalf("unused default changed account claim: output=%+v error=%v", out, err)
			}
		})
	}
}

func TestFactoryRequiresExplicitMulticallDeployment(t *testing.T) {
	for _, address := range []string{"", "0x0000000000000000000000000000000000000000", "not-an-address"} {
		t.Run(address, func(t *testing.T) {
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: "https://example.invalid"}, Distributor: addr(4).Hex(), Multicall3: address, Tokens: []string{addr(1).Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: 452}
			_, err := Factory(node(t, cfg), providerRegistry.Deps{})
			if err == nil || !strings.Contains(err.Error(), "multicall3 must be a nonzero address") {
				t.Fatalf("missing or invalid multicall deployment not clearly rejected: %v", err)
			}
		})
	}
}

func TestPrepareValidatesWholeBatchBeforeReturningIdle(t *testing.T) {
	for _, test := range []string{"valid below minimum", "later malformed claimed", "later malformed recipient", "later redirected recipient", "later stale claimed", "later invalid proof"} {
		t.Run(test, func(t *testing.T) {
			rows, root, proofs := rewardBatch(t)
			if test == "later invalid proof" {
				rows[1].Amount = "101"
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
			}))
			defer srv.Close()
			a := binding.NewMerklDistributor()
			contractABI := a.GetABI()
			chain := reader(func(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
				method, err := contractABI.MethodById(msg.Data[:4])
				if err != nil {
					return nil, err
				}
				if method.Name == "getMerkleRoot" {
					return method.Outputs.Pack(root)
				}
				args, err := method.Inputs.Unpack(msg.Data[4:])
				if err != nil {
					return nil, err
				}
				token := args[1].(common.Address)
				if method.Name == "claimed" {
					if token == addr(101) && test == "later malformed claimed" {
						return []byte{1}, nil
					}
					amount := int64(0)
					if token == addr(100) {
						amount = 100 // First token has nothing left to claim.
					} else if token == addr(101) && test == "later stale claimed" {
						amount = 101
					}
					return method.Outputs.Pack(big.NewInt(amount), big.NewInt(500), root)
				}
				if token == addr(101) {
					if test == "later malformed recipient" {
						return []byte{1}, nil
					}
					if test == "later redirected recipient" {
						return method.Outputs.Pack(addr(9))
					}
				}
				return method.Outputs.Pack(common.Address{})
			})
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Action: "0:0"}
			tokens, users, amounts := make([]common.Address, 32), make([]common.Address, 32), make([]*big.Int, 32)
			for i, row := range rows {
				tokens[i], users[i], amounts[i] = common.HexToAddress(row.Token.Address), addr(3), big.NewInt(100)
				cfg.Tokens = append(cfg.Tokens, row.Token.Address)
				cfg.ProofLengths = append(cfg.ProofLengths, 5)
			}
			cfg.CalldataBytes = len(a.PackClaim(users, tokens, amounts, proofs))
			p, err := Factory(node(t, cfg), providerRegistry.Deps{Chain: multicallReader(t, tokens, chain), HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			out, err := p.Prepare(context.Background(), provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3), Timestamp: 1000})
			if err == nil || len(out.Actions) != 0 || len(out.SkipSteps) != 0 {
				t.Fatalf("incomplete batch became usable claim/skip: %+v %v", out, err)
			}
			if (test == "valid below minimum") != errors.Is(err, provider.ErrNotReady) {
				t.Fatalf("early below-minimum token hid later invalid data: %v", err)
			}
		})
	}
}
