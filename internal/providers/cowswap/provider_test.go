package cowswap

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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	sb "github.com/symbioticfi/merkle-executor/api/bindings/cowsettlement"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	ab "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"gopkg.in/yaml.v3"
)

type reader func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f reader) CallContract(c context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	return f(c, m, b)
}
func (f reader) HeaderByNumber(_ context.Context, b *big.Int) (*types.Header, error) {
	return testHeader(b), nil
}
func testHeader(b *big.Int) *types.Header {
	timestamp := uint64(1000)
	if b.Int64() >= 14 {
		timestamp = 2000
	}
	return &types.Header{Number: new(big.Int).Set(b), Time: timestamp}
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
func sampleOrder() wire.CowOrder {
	return wire.CowOrder{SellToken: addr(1).Hex(), BuyToken: addr(2).Hex(), Receiver: addr(3).Hex(), From: addr(3).Hex(), SellAmount: "990", BuyAmount: "1980", FeeAmount: "0", ValidTo: 1600, AppData: (common.Hash{}).Hex(), Kind: "sell", PartiallyFillable: false, SellTokenBalance: "erc20", BuyTokenBalance: "erc20", SigningScheme: "eip1271", Signature: "0x"}
}
func TestCoWViemDigestAndUIDGolden(t *testing.T) {
	o := sampleOrder()
	buffer, e := EncodeOrder(o)
	if e != nil {
		t.Fatal(e)
	}
	if len(buffer) != 416 {
		t.Fatal("wrong account hash preimage length")
	}
	digest := protocolio.Digest(protocolio.Domain("Gnosis Protocol", "v2", big.NewInt(1), addr(4)), crypto.Keccak256Hash(buffer))
	if digest.Hex() != "0xbd75e6e37218661e5ace0e957199172a6b42ef2a8783a36afa4ca94d62b3d895" {
		t.Fatalf("wrong typed digest %s", digest)
	}
	if UID(digest, addr(3), 1600) != "0xbd75e6e37218661e5ace0e957199172a6b42ef2a8783a36afa4ca94d62b3d895000000000000000000000000000000000000000300000640" {
		t.Fatal("UID is not digest/owner/big-endian expiry")
	}
}
func TestRestartReconcilesAmbiguousPostAndRequiresConfirmedFill(t *testing.T) {
	account := addr(3)
	domain := protocolio.Domain("Gnosis Protocol", "v2", big.NewInt(1), addr(4))
	posts, gets := 0, 0
	exists := false
	status := "open"
	approved := false
	filled := big.NewInt(0)
	var submitted wire.CowOrder
	uid := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/quote":
			if r.Method != http.MethodPost {
				t.Error("quote method")
			}
			var request wire.CowQuoteRequest
			_ = json.NewDecoder(r.Body).Decode(&request)
			if request.SigningScheme != "eip1271" || request.From != account.Hex() || request.SellAmountBeforeFee != "1000" || request.ValidTo != 1600 {
				t.Error("wrong quote request")
			}
			q := sampleOrder()
			q.BuyAmount = "2000"
			q.FeeAmount = "10"
			_ = json.NewEncoder(w).Encode(wire.CowQuoteResponse{Quote: q, ID: 1})
		case r.URL.Path == "/api/v1/orders":
			posts++
			if r.Method != http.MethodPost {
				t.Error("post method")
			}
			if e := json.NewDecoder(r.Body).Decode(&submitted); e != nil {
				t.Error(e)
			}
			if submitted.SigningScheme != "eip1271" || submitted.Signature != "0x" || submitted.FeeAmount != "0" || submitted.PartiallyFillable || submitted.SellAmount != "990" {
				t.Error("wrong account order")
			}
			exists = true
			w.WriteHeader(502)
			_, _ = w.Write([]byte("private backend response"))
		case strings.HasPrefix(r.URL.Path, "/api/v1/orders/"):
			gets++
			if !exists {
				http.NotFound(w, r)
				return
			}
			if r.URL.Path != "/api/v1/orders/"+uid {
				t.Error("UID changed after restart")
			}
			_ = json.NewEncoder(w).Encode(wire.CowOrderStatus{UID: uid, Owner: account.Hex(), Status: status, SellAmount: "990", BuyAmount: "1980", ValidTo: 1600})
		default:
			t.Errorf("unexpected HTTP path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
		if b == nil || b.Int64() != 12 && b.Int64() != 13 && b.Int64() != 14 {
			t.Error("read not pinned")
		}
		if *m.To == addr(4) {
			a := sb.NewCowSettlement().GetABI()
			method, e := a.MethodById(m.Data[:4])
			if e != nil {
				return nil, e
			}
			switch method.Name {
			case "domainSeparator":
				return method.Outputs.Pack(domain)
			case "filledAmount":
				return method.Outputs.Pack(filled)
			}
		}
		if *m.To == account {
			a := ab.NewMerkleAccount().GetABI()
			method, e := a.MethodById(m.Data[:4])
			if e != nil {
				return nil, e
			}
			if method.Name != "signatures" {
				t.Error("expected signatures read")
			}
			return method.Outputs.Pack(approved)
		}
		t.Fatal("wrong target")
		return nil, nil
	})
	sellInput, buyInput := uint(0), uint(1)
	cfg := Config{SellAmountInput: &sellInput, BuyAmountInput: &buyInput, HTTP: protocolio.HTTPConfig{URL: srv.URL}, Settlement: addr(4).Hex(), SellToken: addr(1).Hex(), BuyToken: addr(2).Hex(), SellAmount: "1000", MinBuyAmount: "1800", SlippageBPS: 100, ValiditySeconds: 600, AppData: (common.Hash{}).Hex(), Action: "1:0"}
	deps := providerRegistry.Deps{Chain: chain, HTTP: srv.Client()}
	p, e := Factory(node(t, cfg), deps)
	if e != nil {
		t.Fatal(e)
	}
	snap := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(12), Account: account, Timestamp: 1000, BlockHash: testHeader(big.NewInt(12)).Hash()}
	out, e := p.Prepare(context.Background(), snap)
	if e != nil {
		t.Fatal(e)
	}
	if out.Inputs[0] != "990" || out.Inputs[1] != "1980" {
		t.Fatal("recipe inputs differ from net authorized order")
	}
	if posts != 0 || gets != 0 {
		t.Fatal("Prepare submitted or polled external order")
	}
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	uid = d.UID
	if _, e = p.Advance(context.Background(), snap, out, nil); e == nil {
		t.Fatal("pre-authorization snapshot accepted")
	}
	snap.BlockNumber = big.NewInt(13)
	snap.BlockHash = testHeader(snap.BlockNumber).Hash()
	if _, e = p.Advance(context.Background(), snap, out, nil); e == nil || posts != 0 {
		t.Fatal("unapproved digest posted")
	}
	approved = true
	snap.BlockHash = common.HexToHash("0xdead")
	if _, e = p.Advance(context.Background(), snap, out, nil); e == nil || posts != 0 {
		t.Fatal("noncanonical snapshot posted an order")
	}
	snap.BlockHash = testHeader(snap.BlockNumber).Hash()
	gets = 0
	result, e := p.Advance(context.Background(), snap, out, nil)
	if e != nil || result.Done || posts != 1 {
		t.Fatalf("ambiguous post not retained: %+v %v", result, e)
	}
	// Reconstruct provider and persisted Output exactly as a process restart does.
	encoded, _ := json.Marshal(out)
	var restored provider.Output
	_ = json.Unmarshal(encoded, &restored)
	p, e = Factory(node(t, cfg), deps)
	if e != nil {
		t.Fatal(e)
	}
	result, e = p.Advance(context.Background(), snap, restored, result.State)
	if e != nil || result.Done || posts != 1 || gets != 2 {
		t.Fatalf("restart duplicated or lost order: %+v %v, posts=%d gets=%d", result, e, posts, gets)
	}
	status = "fulfilled"
	result, e = p.Advance(context.Background(), snap, restored, result.State)
	if e != nil || result.Done {
		t.Fatal("API fulfilled accepted without confirmed on-chain fill")
	}
	filled = big.NewInt(990)
	result, e = p.Advance(context.Background(), snap, restored, result.State)
	if e != nil || !result.Done || result.Outcome != "settled" {
		t.Fatalf("confirmed full fill not complete: %+v %v", result, e)
	}
	filled = big.NewInt(0)
	snap.BlockNumber = big.NewInt(14)
	snap.Timestamp = 2000
	snap.BlockHash = testHeader(snap.BlockNumber).Hash()
	result, e = p.Advance(context.Background(), snap, restored, result.State)
	if e != nil || !result.Done || result.Outcome != "expired" {
		t.Fatalf("confirmed expiry did not finish as expired: %+v %v", result, e)
	}
	snap.BlockNumber = big.NewInt(13)
	snap.Timestamp = 1000
	snap.BlockHash = testHeader(snap.BlockNumber).Hash()
	filled = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	result, e = p.Advance(context.Background(), snap, restored, result.State)
	if e == nil || result.Done {
		t.Fatal("invalidation sentinel marked fulfilled")
	}
	savedPayload := restored.Actions[cfg.Action]
	restored.Actions[cfg.Action] = append([]byte(nil), savedPayload...)
	restored.Actions[cfg.Action][len(savedPayload)-1] ^= 1
	if _, e = p.Advance(context.Background(), snap, restored, nil); e == nil {
		t.Fatal("calldata drift accepted")
	}
	restored.Actions[cfg.Action] = savedPayload
	restored.Inputs[0] = "1000"
	if _, e = p.Advance(context.Background(), snap, restored, nil); e == nil {
		t.Fatal("gross/net numerical input drift accepted")
	}
	restored.Inputs[0] = "990"
	d.UID = "0x00"
	restored.Data, _ = json.Marshal(d)
	if _, e = p.Advance(context.Background(), snap, restored, nil); e == nil {
		t.Fatal("tampered persisted UID accepted")
	}
}
