package liquidlanediscount

import (
	"bytes"
	"context"
	"encoding/json"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidlaneconnector"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func frontendDiscount(t *testing.T) (recipes.Recipe, *recipes.Tree) {
	t.Helper()
	raw, err := os.ReadFile("testdata/frontend-ll.json")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Recipe hexutil.Bytes }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	r, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	return r, tree
}

type discountReader struct {
	header       *types.Header
	calls        int
	used         bool
	wrongAccount bool
}

func (r *discountReader) CallContractWithCode(_ context.Context, msg ethereum.CallMsg, _ *big.Int, _ []byte) ([]byte, error) {
	args := arguments("address", "bytes[]", "bytes[]", "uint256[]", "bool[]", "bytes[]", "bytes[]", "uint256[]")
	values, err := args.Unpack(msg.Data[4:])
	if err != nil {
		return nil, err
	}
	results := make([]*big.Int, len(values[2].([][]byte)))
	for i := range results {
		results[i] = new(big.Int)
	}
	results[8].SetInt64(200)
	typ, _ := abi.NewType("uint256[]", "", nil)
	return (abi.Arguments{{Type: typ}}).Pack(results)
}
func (r *discountReader) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return r.header, nil
}
func (r *discountReader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.calls++
	if block.Cmp(r.header.Number) != 0 {
		return nil, ethereum.NotFound
	}
	a := binding.NewLiquidLaneConnector()
	selector := msg.Data[:4]
	pack := func(n int64) []byte { return protocolio.Word(big.NewInt(n)) }
	switch {
	case bytes.Equal(selector, a.PackAccount()):
		if r.wrongAccount {
			return protocolio.AddressWord(common.HexToAddress("0x109")), nil
		}
		return protocolio.AddressWord(common.HexToAddress("0x104")), nil
	case bytes.Equal(selector, a.PackOutputAsset()):
		return protocolio.AddressWord(common.HexToAddress("0x201")), nil
	case bytes.Equal(selector, a.PackRemainingAssets(common.HexToAddress("0x202"))[:4]):
		return pack(150), nil
	case bytes.Equal(selector, a.PackIsUsedNonce(common.HexToAddress("0x202"), big.NewInt(7))[:4]):
		if r.used {
			return pack(1), nil
		}
		return pack(0), nil
	case bytes.Equal(selector, a.PackGetAmountOut(common.HexToAddress("0x202"), big.NewInt(1))[:4]):
		amount := new(big.Int).SetBytes(msg.Data[36:68])
		return protocolio.Word(amount.Mul(amount, big.NewInt(2))), nil
	case bytes.Equal(selector, crypto.Keccak256([]byte("balanceOf(address)"))[:4]):
		if *msg.To == common.HexToAddress("0x202") {
			return pack(100), nil
		}
		return pack(200), nil
	}
	return nil, ethereum.NotFound
}

type reroute struct {
	base      *url.URL
	transport http.RoundTripper
}

func (r reroute) RoundTrip(in *http.Request) (*http.Response, error) {
	copy := in.Clone(in.Context())
	u := *in.URL
	u.Scheme = r.base.Scheme
	u.Host = r.base.Host
	copy.URL = &u
	return r.transport.RoundTrip(copy)
}
func TestAutomaticFrontendDiscountFlexibleAmounts(t *testing.T) {
	s, _, w := witnessFixture(t, "LiquidLaneConnector")
	r, tree := frontendDiscount(t)
	header := &types.Header{Number: s.BlockNumber, Time: s.Timestamp}
	s.BlockHash = header.Hash()
	chain := &discountReader{header: header}
	server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) { _ = json.NewEncoder(out).Encode(w) }))
	defer server.Close()
	base, _ := url.Parse(server.URL)
	p := &Automatic{deps: registry.Deps{Chain: chain, HTTP: &http.Client{Transport: reroute{base, http.DefaultTransport}}}}
	req, err := p.RecipeRequirements(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Actions) != 1 || len(req.Inputs) != 2 {
		t.Fatalf("wrong ownership %+v", req)
	}
	out, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inputs[0] != "76" || out.Inputs[1] != "150" {
		t.Fatalf("wrong amounts %v", out.Inputs)
	}
	if chain.calls > 264 {
		t.Fatalf("amount selection unbounded: %d", chain.calls)
	}
	for _, test := range []struct {
		name, want string
		mutate     func()
	}{
		{"used nonce", "nonce", func() { chain.used = true }},
		{"other account", "account", func() { chain.used = false; chain.wrongAccount = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.mutate()
			_, err := p.PrepareRecipe(context.Background(), s, r, tree, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %s got %v", test.want, err)
			}
		})
	}
}
func TestAutomaticRejectsChangedGuardsAndMaskedIdentities(t *testing.T) {
	s, _, _ := witnessFixture(t, "LiquidLaneConnector")
	r, tree := frontendDiscount(t)
	p := &Automatic{}
	request, err := p.derive(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{4, 68, 132, 196, 228, 324, 388, 485, 613} {
		t.Run(big.NewInt(int64(at)).String(), func(t *testing.T) {
			action := request.action
			action.Leaf.DataMask = bytes.Clone(action.Leaf.DataMask)
			action.Leaf.DataMask[at] = 255
			if _, err := discountAction(action); err == nil {
				t.Fatal("masked identity or framing must fail")
			}
		})
	}
	for _, offset := range []uint{3, 6, 9} {
		copy := recipes.CloneRecipe(r)
		copy.Steps[request.action.Step+offset][0] = recipes.LessOrEqual
		if _, err := p.derive(s, copy, tree); err == nil || !strings.Contains(err.Error(), "balance guards") {
			t.Fatalf("loosened balance guard accepted %v", err)
		}
	}
}

func TestAutomaticRespectsCommittedInputCeilings(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		input                 int64
		cap                   *big.Int
		amount, payment, want string
	}{
		{name: "RWA cap", input: 0, cap: big.NewInt(50), amount: "50", payment: "98"},
		{name: "payment cap", input: 1, cap: big.NewInt(100), amount: "51", payment: "100"},
		{name: "future payment cap", input: 1, cap: result(16), want: "result must reference an earlier numerical instruction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, w := witnessFixture(t, "LiquidLaneConnector")
			r, tree := frontendDiscount(t)
			r.Steps[0] = binary(recipes.LessOrEqual, encoded(recipes.Value{Source: recipes.Input, Value: big.NewInt(tc.input)}), tc.cap)
			header := &types.Header{Number: s.BlockNumber, Time: s.Timestamp}
			s.BlockHash = header.Hash()
			chain := &discountReader{header: header}
			server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(out).Encode(w) }))
			defer server.Close()
			base, _ := url.Parse(server.URL)
			p := &Automatic{deps: registry.Deps{Chain: chain, HTTP: &http.Client{Transport: reroute{base, http.DefaultTransport}}}}
			out, err := p.PrepareRecipe(t.Context(), s, r, tree, nil)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("want %s got %v", tc.want, err)
				}
				return
			}
			if err != nil || out.Inputs[0] != tc.amount || out.Inputs[1] != tc.payment {
				t.Fatalf("ceiling escaped %+v %v", out.Inputs, err)
			}
		})
	}
}

func TestSelectAmountPreservesAuthorizedZeroPayment(t *testing.T) {
	s, _, w := witnessFixture(t, "LiquidLaneConnector")
	r, tree := frontendDiscount(t)
	header := &types.Header{Number: s.BlockNumber, Time: s.Timestamp}
	chain := &discountReader{header: header}
	p := &Automatic{deps: registry.Deps{Chain: chain}}
	request, err := p.derive(s, r, tree)
	if err != nil {
		t.Fatal(err)
	}
	request.amountLimit = big.NewInt(100)
	swap, err := validateWitness(s, request.scope, w)
	if err != nil {
		t.Fatal(err)
	}
	swap.Discount.Discount = big.NewInt(100000000)
	amount, payment, err := p.selectAmount(t.Context(), s, request, swap.Discount, new(big.Int))
	if err != nil || amount.Int64() != 100 || payment.Sign() != 0 {
		t.Fatalf("valid signed zero-payment fill rejected: amount=%v payment=%v err=%v", amount, payment, err)
	}
}
