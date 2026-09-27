package liquidlane

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
	"github.com/ethereum/go-ethereum/crypto"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidconnector"
	accountbinding "github.com/symbioticfi/merkle-executor/api/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/api/protocolio"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/provider"
	"gopkg.in/yaml.v3"
)

type reader func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f reader) CallContract(c context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	return f(c, m, b)
}
func addr(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }
func node(t *testing.T, v any) yaml.Node {
	t.Helper()
	b, e := yaml.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	var n yaml.Node
	if e = yaml.Unmarshal(b, &n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestConnectorHashesViemGolden(t *testing.T) {
	d := binding.ILiquidLaneConnectorDiscount{TokenToRedeem: addr(5), Discount: big.NewInt(10000), Signer: addr(6), Protocol: addr(7), Nonce: big.NewInt(42), Deadline: big.NewInt(2000)}
	a, b := Hashes(protocolio.Domain("LiquidLaneConnector", "1", big.NewInt(1), addr(4)), d, []byte{0x12, 0x34}, big.NewInt(1900))
	if a.Hex() != "0x013ccaf5b90414d2b2e3e85792dd1c878dd7df9f0eb51a1ba3ee57c50d8ba08b" || b.Hex() != "0x0449a0d529064bb4829a9c067742d4c01b8e89772549afb942a52a745f99f161" {
		t.Fatalf("connector EIP712 golden mismatch: %s %s", a, b)
	}
}
func TestPrepareConnectorAndRejectLegacyDomain(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "connector", true: "legacy-rejected"}[legacy], func(t *testing.T) {
			key, _ := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
			protocolKey, _ := crypto.HexToECDSA(strings.Repeat("0", 63) + "2")
			signer := crypto.PubkeyToAddress(key.PublicKey)
			protocol := crypto.PubkeyToAddress(protocolKey.PublicKey)
			d := binding.ILiquidLaneConnectorDiscount{TokenToRedeem: addr(5), Discount: big.NewInt(10000), Signer: signer, Protocol: protocol, Nonce: big.NewInt(42), Deadline: big.NewInt(2000)}
			name := "LiquidLaneConnector"
			if legacy {
				name = "LiquidLaneAdapter"
			}
			domain := protocolio.Domain(name, "1", big.NewInt(1), addr(4))
			dh, _ := Hashes(domain, d, nil, big.NewInt(1900))
			ss, _ := crypto.Sign(dh.Bytes(), key)
			ss[64] += 27
			_, wh := Hashes(domain, d, ss, big.NewInt(1900))
			ps, _ := crypto.Sign(wh.Bytes(), protocolKey)
			ps[64] += 27
			resolved := wire.LLResolved{DiscountID: dh.Hex(), Discount: wire.LLDiscount{Adapter: addr(4).Hex(), TokenToRedeem: addr(5).Hex(), Discount: "10000", Signer: signer.Hex(), Protocol: protocol.Hex(), Nonce: "42", Deadline: 2000}, SignerSignature: hexutil.Encode(ss), ProtocolDeadline: 1900, ProtocolSignature: hexutil.Encode(ps)}
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/api-internal/v1/discounts" || r.Method != http.MethodPost {
					t.Errorf("unexpected endpoint")
				}
				var req map[string]string
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req["discountId"] != dh.Hex() {
					t.Error("wrong selector")
				}
				_ = json.NewEncoder(w).Encode(resolved)
			}))
			defer srv.Close()
			abi := binding.NewLiquidConnector().GetABI()
			chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				if b.Cmp(big.NewInt(12)) != 0 || *m.To != addr(4) {
					t.Error("unpinned or wrong target read")
				}
				method, e := abi.MethodById(m.Data[:4])
				if e != nil {
					return nil, e
				}
				var values []any
				switch method.Name {
				case "account":
					values = []any{addr(3)}
				case "inputAsset":
					values = []any{addr(1)}
				case "eip712Domain":
					values = []any{[1]byte{15}, "LiquidLaneConnector", "1", big.NewInt(1), addr(4), [32]byte{}, []*big.Int{}}
				case "owner":
					values = []any{signer}
				case "signer":
					values = []any{common.Address{}}
				case "isUsedNonce":
					values = []any{false}
				case "minDiscount":
					values = []any{big.NewInt(0)}
				case "getAmountOut":
					values = []any{big.NewInt(1000)}
				case "remainingAssets":
					values = []any{big.NewInt(1000)}
				default:
					t.Fatalf("unexpected read %s", method.Name)
				}
				return method.Outputs.Pack(values...)
			})
			payload, e := binding.NewLiquidConnector().TryPackSwap0(binding.ILiquidLaneConnectorDiscountSwap{Discount: d, SignerSignature: ss, ProtocolDeadline: big.NewInt(1900)}, ps, addr(8), big.NewInt(100))
			if e != nil {
				t.Fatal(e)
			}
			if len(payload) != 644 {
				t.Fatalf("connector ABI layout changed: %d", len(payload))
			}
			input := uint(0)
			output := uint(1)
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Format: "connector-v1", Connector: addr(4).Hex(), Token: addr(5).Hex(), InputAsset: addr(1).Hex(), Recipient: addr(8).Hex(), Protocol: protocol.Hex(), DiscountID: dh.Hex(), AmountIn: "100", MaxAmountOut: "1000", Action: "0:0", CalldataBytes: len(payload), AmountInInput: &input, AmountOutInput: &output}
			p, e := Factory(node(t, cfg), provider.Deps{Chain: chain, HTTP: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Timestamp: 1000, Account: addr(3)}
			result, e := p.Prepare(context.Background(), s)
			if legacy {
				if e == nil || !strings.Contains(e.Error(), "domain mismatch") {
					t.Fatalf("old domain not rejected explicitly: %v", e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if result.Inputs[0] != "100" || result.Inputs[1] != "990" || hexutil.Encode(result.Actions["0:0"]) != hexutil.Encode(payload) || requests != 1 {
				t.Fatalf("wrong prepared output %+v", result)
			}
			cfg.CalldataBytes++
			p, e = Factory(node(t, cfg), provider.Deps{Chain: chain, HTTP: srv.Client()})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = p.Prepare(context.Background(), s); e == nil {
				t.Fatal("wrong approved layout accepted")
			}
		})
	}
}
func TestResolveUnionRejectsAmbiguousBatch(t *testing.T) {
	for _, body := range []string{`{"discounts":[]}`, `{"discounts":[{},{}]}`, `{"discountId":"x","discounts":[{}]}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		c, e := NewClient(protocolio.HTTPConfig{URL: s.URL}, s.Client())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Resolve(context.Background(), "id"); e == nil {
			t.Errorf("accepted %s", body)
		}
		s.Close()
	}
}

func TestContractSignatureUsesPinnedERC1271(t *testing.T) {
	a := accountbinding.NewMerkleAccount().GetABI()
	approved := true
	calls := 0
	digest := common.HexToHash("0x1234")
	chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
		calls++
		if b.Int64() != 12 || *m.To != addr(6) {
			t.Error("ERC1271 read not pinned to signer")
		}
		method, e := a.MethodById(m.Data[:4])
		if e != nil {
			return nil, e
		}
		if method.Name != "isValidSignature" {
			t.Fatal("wrong signature method")
		}
		args, e := method.Inputs.Unpack(m.Data[4:])
		if e != nil {
			return nil, e
		}
		if common.Hash(args[0].([32]byte)) != digest {
			t.Fatal("wrong digest")
		}
		magic := [4]byte{}
		if approved {
			magic = [4]byte{0x16, 0x26, 0xba, 0x7e}
		}
		return method.Outputs.Pack(magic)
	})
	snap := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: addr(3)}
	if !validSignature(context.Background(), chain, snap, addr(6), digest, []byte{0x42}) {
		t.Fatal("contract approval rejected")
	}
	approved = false
	if validSignature(context.Background(), chain, snap, addr(6), digest, []byte{0x42}) || calls != 2 {
		t.Fatal("wrong ERC1271 magic accepted")
	}
}
