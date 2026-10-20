package liquidlanediscount

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func witnessFixture(t *testing.T, domainName string) (provider.Snapshot, scope, Resolved) {
	t.Helper()
	signer, err := crypto.HexToECDSA(strings.Repeat("1", 64))
	if err != nil {
		t.Fatal(err)
	}
	protocol, err := crypto.HexToECDSA(strings.Repeat("2", 64))
	if err != nil {
		t.Fatal(err)
	}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: common.HexToAddress("0x104"), Timestamp: 1000}
	sc := scope{Connector: common.HexToAddress("0x106"), Token: common.HexToAddress("0x202"), Signer: crypto.PubkeyToAddress(signer.PublicKey), Protocol: crypto.PubkeyToAddress(protocol.PublicKey), Recipient: common.HexToAddress("0x107")}
	w := Resolved{Discount: Terms{Adapter: sc.Connector.Hex(), TokenToRedeem: sc.Token.Hex(), Discount: "1250000", Signer: sc.Signer.Hex(), Protocol: sc.Protocol.Hex(), Nonce: "7", Deadline: 1200}, ProtocolDeadline: 1100}
	d := protocolio.Domain(domainName, "1", s.ChainID, sc.Connector)
	h := crypto.Keccak256Hash(discountTypeHash.Bytes(), protocolio.AddressWord(sc.Token), protocolio.Word(big.NewInt(1250000)), protocolio.AddressWord(sc.Signer), protocolio.AddressWord(sc.Protocol), protocolio.Word(big.NewInt(7)), protocolio.Word(big.NewInt(1200)))
	digest := protocolio.Digest(d, h)
	w.DiscountID = digest.Hex()
	sign := func(k *ecdsa.PrivateKey, h common.Hash) string {
		b, e := crypto.Sign(h.Bytes(), k)
		if e != nil {
			t.Fatal(e)
		}
		b[64] += 27
		return hexutil.Encode(b)
	}
	w.SignerSignature = sign(signer, digest)
	sig, _ := hexutil.Decode(w.SignerSignature)
	swap := crypto.Keccak256Hash(discountSwapTypeHash.Bytes(), h.Bytes(), crypto.Keccak256(sig), protocolio.Word(big.NewInt(1100)))
	w.ProtocolSignature = sign(protocol, protocolio.Digest(d, swap))
	return s, sc, w
}
func TestWitnessRequiresConnectorDomainAndExactScope(t *testing.T) {
	s, sc, valid := witnessFixture(t, "LiquidLaneConnector")
	if _, err := validateWitness(s, sc, valid); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, want string
		mutate     func(*Resolved)
	}{
		{"other connector", "identity", func(w *Resolved) { w.Discount.Adapter = common.HexToAddress("0x108").Hex() }},
		{"other token", "identity", func(w *Resolved) { w.Discount.TokenToRedeem = common.HexToAddress("0x108").Hex() }},
		{"other signer", "identity", func(w *Resolved) { w.Discount.Signer = common.HexToAddress("0x108").Hex() }},
		{"other protocol", "identity", func(w *Resolved) { w.Discount.Protocol = common.HexToAddress("0x108").Hex() }},
		{"expired", "deadline", func(w *Resolved) { w.ProtocolDeadline = 999 }},
		{"uint48 overflow", "deadline", func(w *Resolved) { w.Discount.Deadline = 1 << 48 }},
		{"precision", "precision", func(w *Resolved) { w.Discount.Discount = "100000001" }},
		{"fractional discount", "uint256", func(w *Resolved) { w.Discount.Discount = "1.25" }},
		{"bad signer signature", "signer signature", func(w *Resolved) { w.SignerSignature = valid.ProtocolSignature }},
		{"bad protocol signature", "protocol signature", func(w *Resolved) { w.ProtocolSignature = valid.SignerSignature }},
		{"bad id", "discount ID", func(w *Resolved) { w.DiscountID = common.Hash{}.Hex() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := valid
			tc.mutate(&w)
			_, err := validateWitness(s, sc, w)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
		})
	}
	_, _, badDomain := witnessFixture(t, "LiquidLaneAdapter")
	if _, err := validateWitness(s, sc, badDomain); err == nil || !strings.Contains(err.Error(), "signer signature") {
		t.Fatalf("adapter-domain backend must fail closed: %v", err)
	}
}
func TestResolveUsesDocumentedPairAndRejectsUnknownWire(t *testing.T) {
	_, sc, w := witnessFixture(t, "LiquidLaneConnector")
	extra := false
	server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) {
		if in.Method != "POST" || in.URL.Path != "/api-internal/v1/discounts" {
			t.Errorf("unexpected route %s %s", in.Method, in.URL.Path)
		}
		var request map[string]string
		if e := json.NewDecoder(in.Body).Decode(&request); e != nil {
			t.Error(e)
		}
		if len(request) != 2 || request["adapter"] != sc.Connector.Hex() || request["tokenToRedeem"] != sc.Token.Hex() {
			t.Errorf("bad selector %v", request)
		}
		raw, _ := json.Marshal(w)
		if extra {
			raw = append(raw[:len(raw)-1], []byte(`,"inputs":{"0":"999"}}`)...)
		}
		_, _ = out.Write(raw)
	}))
	defer server.Close()
	c, err := protocolio.NewHTTP(protocolio.HTTPConfig{URL: server.URL}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolve(context.Background(), c, sc)
	if err != nil || got.DiscountID != w.DiscountID {
		t.Fatalf("resolve %v %v", got, err)
	}
	extra = true
	if _, err = resolve(context.Background(), c, sc); err == nil {
		t.Fatal("unknown fields must fail closed")
	}
}

type contractSignatureReader struct {
	response []byte
	digest   common.Hash
	calls    int
}

func (r *contractSignatureReader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.calls++
	if block.Int64() != 12 || !bytes.Equal(msg.Data[4:36], r.digest.Bytes()) {
		return nil, ethereum.NotFound
	}
	return r.response, nil
}
func TestFixedSignatureSupportsPinnedERC1271(t *testing.T) {
	s, _, _ := witnessFixture(t, "LiquidLaneConnector")
	digest := crypto.Keccak256Hash([]byte("contract-witness"))
	raw := hexutil.Encode(make([]byte, 65))
	chain := &contractSignatureReader{digest: digest, response: append(common.FromHex("0x1626ba7e"), make([]byte, 28)...)}
	if _, err := signatureAt(context.Background(), chain, s, raw, digest, common.HexToAddress("0x109")); err != nil {
		t.Fatal(err)
	}
	for _, response := range [][]byte{nil, common.FromHex("0x1626ba7e"), make([]byte, 32), append(common.FromHex("0x1626ba7e"), bytes.Repeat([]byte{1}, 28)...)} {
		chain.response = response
		if _, err := signatureAt(context.Background(), chain, s, raw, digest, common.HexToAddress("0x109")); err == nil {
			t.Fatal("noncanonical ERC1271 response accepted")
		}
	}
}
