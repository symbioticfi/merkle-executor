package liquidlanerfq

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	reactor "github.com/symbioticfi/merkle-executor/sdk/bindings/liquidlanereactor"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

var addr = func(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }

const quoteID = "11111111-1111-4111-8111-111111111111"
const orderID = "22222222-2222-4222-8222-222222222222"

type testChain struct {
	times                         map[uint64]uint64
	t                             *testing.T
	used, approved                bool
	balance, allowance            int64
	logs                          []types.Log
	receipts                      map[common.Hash]*types.Receipt
	reorg                         map[uint64]bool
	fail                          string
	calls, logReads, receiptReads int
}

func (c *testChain) HeaderByNumber(ctx context.Context, n *big.Int) (*types.Header, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.fail == "header" {
		return nil, errors.New("private")
	}
	if n == nil {
		n = big.NewInt(12)
	}
	h := &types.Header{Number: new(big.Int).Set(n), Time: 1999999890 + n.Uint64()}
	if v, ok := c.times[n.Uint64()]; ok {
		h.Time = v
	}
	if c.reorg[n.Uint64()] {
		h.Extra = []byte("reorg")
	}
	return h, nil
}
func (c *testChain) CallContract(ctx context.Context, m ethereum.CallMsg, n *big.Int) ([]byte, error) {
	c.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.To == nil || n == nil || !n.IsUint64() || m.From != (common.Address{}) {
		c.t.Fatal("lost pinned getter context")
	}
	var contractABI = reactor.NewLiquidLaneReactor().GetABI()
	switch *m.To {
	case addr(0x22):
		contractABI = account.NewMerkleAccount().GetABI()
	case addr(0x33):
		contractABI = erc20.NewERC20().GetABI()
	}
	method, err := contractABI.MethodById(m.Data)
	if err != nil {
		return nil, err
	}
	if c.fail == method.Name {
		return []byte{1}, nil
	}
	switch method.Name {
	case "eip712Domain":
		return method.Outputs.Pack([1]byte{15}, "Reactor", "1", big.NewInt(1), addr(0x11), [32]byte{}, []*big.Int{})
	case "isUsedNonce":
		v, _ := method.Inputs.Unpack(m.Data[4:])
		if v[0].(common.Address) != addr(0x22) {
			c.t.Fatal("wrong nonce owner")
		}
		return method.Outputs.Pack(c.used)
	case "signatures":
		return method.Outputs.Pack(c.approved)
	case "balanceOf":
		return method.Outputs.Pack(big.NewInt(c.balance))
	case "allowance":
		v, _ := method.Inputs.Unpack(m.Data[4:])
		if v[0].(common.Address) != addr(0x22) || v[1].(common.Address) != addr(0x11) {
			c.t.Fatal("wrong allowance owner/spender")
		}
		return method.Outputs.Pack(big.NewInt(c.allowance))
	}
	c.t.Fatal("unexpected getter", method.Name)
	return nil, nil
}
func (c *testChain) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	c.logReads++
	c.t.Fatal("Liquid Lane must locate fills through /orders and receipts")
	return nil, nil
}
func (c *testChain) TransactionReceipt(ctx context.Context, h common.Hash) (*types.Receipt, error) {
	c.receiptReads++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.fail == "receipt" {
		return nil, errors.New("private")
	}
	return c.receipts[h], nil
}

type harness struct {
	p                   *Provider
	c                   *testChain
	s                   provider.Snapshot
	cfg                 Config
	option              map[string]any
	quotes, gets, posts int
	requests            [][]byte
	lookups             []url.Values
	httpMode            string
	pages               []wire.RfqOrdersResponse
	server              *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{c: &testChain{t: t, approved: true, balance: 1000, allowance: 1000, receipts: map[common.Hash]*types.Receipt{}, reorg: map[uint64]bool{}}}
	output := map[string]any{"token": addr(0x44).Hex(), "amount": "900", "recipient": addr(0x22).Hex()}
	h.option = map[string]any{"solver": map[string]any{"id": "solver", "filler": addr(0x88).Hex()}, "quote": map[string]any{"quoteId": quoteID, "aggregatedOutputs": []any{map[string]any{"token": addr(0x44).Hex(), "amount": "900"}}, "orderInfo": map[string]any{"tokenIn": addr(0x33).Hex(), "amountIn": "1000", "outputs": []any{output}, "deadline": int64(2000000000), "nonce": "7"}}, "signatureData": map[string]any{"primaryType": "Request", "domain": map[string]any{"name": "Reactor", "version": "1", "chainId": 1, "verifyingContract": addr(0x11).Hex()}, "types": map[string]any{"Output": []any{map[string]any{"name": "token", "type": "address"}, map[string]any{"name": "amount", "type": "uint256"}, map[string]any{"name": "recipient", "type": "address"}}, "Request": []any{map[string]any{"name": "tokenIn", "type": "address"}, map[string]any{"name": "amountIn", "type": "uint256"}, map[string]any{"name": "outputs", "type": "Output[]"}, map[string]any{"name": "deadline", "type": "uint256"}, map[string]any{"name": "nonce", "type": "uint256"}, map[string]any{"name": "protocol", "type": "address"}}}, "value": map[string]any{"tokenIn": addr(0x33).Hex(), "amountIn": "1000", "outputs": []any{output}, "deadline": "2000000000", "nonce": "7", "protocol": addr(0x55).Hex()}}}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quote":
			h.quotes++
			var q wire.RfqQuoteRequest
			if json.NewDecoder(r.Body).Decode(&q) != nil || q.Swapper != addr(0x22).Hex() || q.Type != "EXACT_INPUT" || q.Amount != "1000" || len(q.Outputs) != 1 || q.Outputs[0].Recipient != q.Swapper {
				t.Error("wrong quote policy/account")
			}
			if h.httpMode == "404" {
				w.WriteHeader(404)
				return
			}
			if h.httpMode == "empty" {
				_ = json.NewEncoder(w).Encode(map[string]any{"quotes": []any{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"quotes": []any{h.option}})
		case "/api/v1/orders":
			h.gets++
			h.lookups = append(h.lookups, r.URL.Query())
			if h.httpMode == "missing cursor" {
				_, _ = w.Write([]byte(`{"orders":[]}`))
				return
			}
			if r.URL.Query().Get("swapper") != addr(0x22).Hex() || r.URL.Query().Get("orderStatus") != "" {
				t.Error("lookup narrowed status or account")
			}
			if len(h.pages) > 0 {
				i := min(h.gets-1, len(h.pages)-1)
				_ = json.NewEncoder(w).Encode(h.pages[i])
			} else {
				_, _ = w.Write([]byte(`{"orders":[],"cursor":null}`))
			}
		case "/api/v1/order":
			h.posts++
			var raw json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&raw)
			h.requests = append(h.requests, append([]byte(nil), raw...))
			if h.httpMode == "lost" {
				w.WriteHeader(503)
				return
			}
			if h.httpMode == "malformed acceptance" {
				_, _ = w.Write([]byte(`{`))
				return
			}
			_ = json.NewEncoder(w).Encode(wire.RfqOrderResponse{OrderID: orderID, OrderStatus: "open"})
		default:
			t.Error("unexpected HTTP", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(h.server.Close)
	h.cfg = Config{HTTP: protocolio.HTTPConfig{URL: h.server.URL + "/api/v1"}, Reactor: addr(0x11).Hex(), Protocol: addr(0x55).Hex(), TokenIn: addr(0x33).Hex(), TokenOut: addr(0x44).Hex(), AmountIn: "1000", MinAmountOut: "900", Action: "0:0", MaxLookupPages: 3}
	pr, err := New(h.cfg, provider.Deps{Chain: h.c, HTTP: h.server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	h.p = pr
	header, _ := h.c.HeaderByNumber(t.Context(), big.NewInt(10))
	h.s = provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(10), BlockHash: header.Hash(), Timestamp: header.Time, Account: addr(0x22), Executor: addr(0x66), Caller: addr(0x77)}
	return h
}
func (h *harness) authorized(t *testing.T) (provider.Output, provider.Snapshot) {
	t.Helper()
	out, err := h.p.Prepare(t.Context(), h.s)
	if err != nil {
		t.Fatal(err)
	}
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	a := account.NewMerkleAccount()
	abi := a.GetABI()
	args, err := abi.Methods["sign"].Inputs.Unpack(out.Actions["0:0"][4:])
	if err != nil {
		t.Fatal(err)
	}
	data, err := abi.Events["Sign"].Inputs.NonIndexed().Pack(args[1], args[2])
	if err != nil {
		t.Fatal(err)
	}
	header, _ := h.c.HeaderByNumber(t.Context(), big.NewInt(11))
	hash := common.HexToHash("0xaa")
	log := types.Log{Address: h.s.Account, Topics: []common.Hash{abi.Events["Sign"].ID, d.Digest, protocolio.Domain("Reactor", "1", h.s.ChainID, h.p.reactor)}, Data: data, BlockNumber: 11, BlockHash: header.Hash(), TxHash: hash}
	s := h.s
	s.BlockNumber = big.NewInt(12)
	header, _ = h.c.HeaderByNumber(t.Context(), s.BlockNumber)
	s.BlockHash = header.Hash()
	s.Timestamp = header.Time
	s.Execution = &provider.Execution{TxHash: hash, BlockNumber: big.NewInt(11), BlockHash: log.BlockHash, Logs: []types.Log{log}}
	return out, s
}
func (h *harness) addEvent(t *testing.T, d prepared, n uint64, cancel bool) {
	t.Helper()
	r := reactor.NewLiquidLaneReactor()
	a := r.GetABI()
	l := types.Log{Address: h.p.reactor, BlockNumber: n, TxHash: common.BigToHash(new(big.Int).SetUint64(n + 100)), Index: uint(len(h.c.logs))}
	header, _ := h.c.HeaderByNumber(t.Context(), new(big.Int).SetUint64(n))
	l.BlockHash = header.Hash()
	var err error
	if cancel {
		l.Topics = []common.Hash{a.Events["InvalidateNonce"].ID, common.BytesToHash(d.Account.Bytes())}
		l.Data, err = a.Events["InvalidateNonce"].Inputs.NonIndexed().Pack(d.Request.Nonce)
	} else {
		l.Topics = []common.Hash{a.Events["Fill"].ID}
		o := reactor.IReactorOrder{Request: d.Request, Swapper: d.Account, SwapperSignature: []byte{}, Filler: addr(0x99), Outputs: []reactor.IReactorOutput{{Token: h.p.tokenOut, Recipient: d.Account, Amount: big.NewInt(950)}}}
		l.Data, err = a.Events["Fill"].Inputs.NonIndexed().Pack(o)
	}
	if err != nil {
		t.Fatal(err)
	}
	h.c.logs = append(h.c.logs, l)
	copyLog := l
	if old := h.c.receipts[l.TxHash]; old != nil {
		old.Logs = append(old.Logs, &copyLog)
	} else {
		h.c.receipts[l.TxHash] = &types.Receipt{Status: 1, TxHash: l.TxHash, BlockNumber: new(big.Int).SetUint64(n), BlockHash: l.BlockHash, Logs: []*types.Log{&copyLog}}
	}
	h.order(l.TxHash)
	h.c.used = true
}
func (h *harness) order(hash common.Hash) {
	o := wire.RfqOrder{Type: "Priority", OrderID: orderID, OrderStatus: "filled", QuoteID: quoteID, Swapper: h.s.Account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "950", Recipient: h.s.Account.Hex()}}}
	if hash != (common.Hash{}) {
		tx := hash.Hex()
		o.TxHash = &tx
	}
	h.pages = []wire.RfqOrdersResponse{{Orders: []wire.RfqOrder{o}, Cursor: json.RawMessage("null")}}
}

func TestRequestMatchesIndependentCastVector(t *testing.T) {
	h := newHarness(t)
	out, err := h.p.Prepare(t.Context(), h.s)
	if err != nil {
		t.Fatal(err)
	}
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	if d.Digest != common.HexToHash("0xa686786d87fa12f8a8eac720cbef26aedf1c55975f6b93e72735b778aaf5f0ee") || len(out.Actions["0:0"]) != 804 || h.posts != 0 {
		t.Fatal("wrong request digest/layout or Prepare posted")
	}
}
func TestPrepareRejectsUntrustedQuoteAuthority(t *testing.T) {
	for _, mode := range []string{"domain", "domain extra", "schema", "primary", "typed amount", "typed protocol", "typed recipient", "quote input", "quote output", "aggregate", "nonce", "deadline", "uuid", "used", "malformed getter", "empty", "404", "below minimum", "leading zeroes"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			q := h.option["quote"].(map[string]any)
			info := q["orderInfo"].(map[string]any)
			sig := h.option["signatureData"].(map[string]any)
			value := sig["value"].(map[string]any)
			switch mode {
			case "domain":
				sig["domain"].(map[string]any)["chainId"] = 2
			case "domain extra":
				sig["domain"].(map[string]any)["salt"] = "0x00"
			case "schema":
				sig["types"].(map[string]any)["Other"] = []any{}
			case "primary":
				sig["primaryType"] = "Order"
			case "typed amount":
				value["amountIn"] = "1001"
			case "typed protocol":
				value["protocol"] = addr(99).Hex()
			case "typed recipient":
				value["outputs"] = []any{map[string]any{"token": addr(0x44).Hex(), "amount": "900", "recipient": addr(99).Hex()}}
			case "quote input":
				info["tokenIn"] = addr(99).Hex()
			case "quote output":
				info["outputs"] = []any{}
			case "aggregate":
				q["aggregatedOutputs"].([]any)[0].(map[string]any)["amount"] = "901"
			case "nonce":
				info["nonce"] = "0x07"
			case "deadline":
				info["deadline"] = int64(20000000000000000)
			case "uuid":
				q["quoteId"] = "bad"
			case "used":
				h.c.used = true
			case "malformed getter":
				h.c.fail = "eip712Domain"
			case "empty", "404":
				h.httpMode = mode
			case "below minimum":
				h.p.minimum = big.NewInt(901)
			case "leading zeroes":
				info["amountIn"] = "001000"
				info["nonce"] = "0007"
			}
			_, err := h.p.Prepare(t.Context(), h.s)
			if mode == "leading zeroes" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("untrusted quote accepted")
			}
			if mode == "empty" || mode == "404" || mode == "below minimum" {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatal("eligible idle lost", err)
				}
			}
			if h.posts != 0 {
				t.Fatal("Prepare published")
			}
		})
	}
}
func TestAdvanceRequiresExactAuthorizationAndImmutableOutput(t *testing.T) {
	for _, mode := range []string{"missing sign", "sign digest", "sign data", "action", "digest", "request", "chain", "signature", "inputs", "minimum", "reorg", "unapproved", "balance", "allowance"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			var d prepared
			_ = json.Unmarshal(out.Data, &d)
			switch mode {
			case "missing sign":
				s.Execution.Logs = nil
			case "sign digest":
				s.Execution.Logs[0].Topics[1] = common.Hash{}
			case "sign data":
				s.Execution.Logs[0].Data = []byte{1}
			case "action":
				out.Actions["0:0"][10] ^= 1
			case "digest":
				d.Digest = common.Hash{}
			case "request":
				d.Request.AmountIn = big.NewInt(9)
			case "chain":
				d.ChainID = "2"
			case "signature":
				d.Signature = "0x"
			case "inputs":
				out.Inputs = map[uint]string{1: "9"}
			case "minimum":
				h.p.minimum = big.NewInt(901)
			case "reorg":
				h.c.reorg[11] = true
			case "unapproved":
				h.c.approved = false
			case "balance":
				h.c.balance = 999
			case "allowance":
				h.c.allowance = 999
			}
			out.Data, _ = json.Marshal(d)
			_, err := h.p.Advance(t.Context(), s, out, nil)
			if err == nil || h.posts != 0 || h.gets != 0 {
				t.Fatalf("invalid authorization reached HTTP: %v gets%d posts%d", err, h.gets, h.posts)
			}
		})
	}
}
func TestUncertainPublicationReconcilesExactIdentity(t *testing.T) {
	h := newHarness(t)
	out, s := h.authorized(t)
	h.httpMode = "lost"
	p, err := h.p.Advance(t.Context(), s, out, nil)
	if err != nil || p.Done || p.Outcome != "publication-uncertain" || h.posts != 1 {
		t.Fatal(p, err)
	}
	p, err = h.p.Advance(t.Context(), s, out, p.State)
	if err != nil || h.posts != 2 || h.quotes != 1 || !bytes.Equal(h.requests[0], h.requests[1]) {
		t.Fatal("uncertain retry changed body/quote", err)
	}
	h.pages = []wire.RfqOrdersResponse{{Orders: []wire.RfqOrder{{Type: "Priority", OrderID: orderID, OrderStatus: "filled", QuoteID: quoteID, Swapper: s.Account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "950", Recipient: s.Account.Hex()}}}}, Cursor: json.RawMessage("null")}}
	p, err = h.p.Advance(t.Context(), s, out, p.State)
	if err != nil || p.Done || p.Reference != orderID || h.posts != 2 {
		t.Fatal("API status became settlement", p, err)
	}
	var body wire.RfqOrderRequest
	_ = json.Unmarshal(h.requests[0], &body)
	if body.Signature != "0x00" {
		t.Fatal("signature identity changed")
	}
}
func TestLookupMustBeCompleteBeforePublication(t *testing.T) {
	for _, mode := range []string{"missing orders", "missing cursor", "bad cursor", "repeated cursor", "wrong owner", "duplicate", "known missing", "unknown status"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			page := wire.RfqOrdersResponse{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage("null")}
			st := progressState{Version: 1, Digest: common.Hash{}}
			var d prepared
			_ = json.Unmarshal(out.Data, &d)
			st.Digest = d.Digest
			switch mode {
			case "missing orders":
				page.Orders = nil
			case "missing cursor":
				h.httpMode = "missing cursor"
			case "bad cursor":
				page.Cursor = json.RawMessage(`"bad"`)
			case "repeated cursor":
				page.Cursor = json.RawMessage(`"oc_a"`)
			case "known missing":
				st.OrderID = orderID
			default:
				o := wire.RfqOrder{Type: "Priority", OrderID: orderID, OrderStatus: "open", QuoteID: quoteID, Swapper: s.Account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "900", Recipient: s.Account.Hex()}}}
				if mode == "wrong owner" {
					o.Swapper = addr(2).Hex()
				}
				if mode == "unknown status" {
					o.OrderStatus = "unknown"
				}
				page.Orders = []wire.RfqOrder{o}
				if mode == "duplicate" {
					page.Orders = append(page.Orders, o)
				}
			}
			h.pages = []wire.RfqOrdersResponse{page}
			raw, _ := json.Marshal(st)
			p, err := h.p.Advance(t.Context(), s, out, raw)
			if err == nil || p.Done || h.posts != 0 {
				t.Fatalf("incomplete lookup published %s: %v", mode, err)
			}
		})
	}
}
func TestCanonicalEventsControlSettlement(t *testing.T) {
	for _, mode := range []string{"fill", "cancel", "fill then cancel", "used only", "failed receipt", "missing receipt", "removed", "wrong request", "wrong account", "revoked after fill", "signature error after fill", "balance error after fill", "allowance error after fill", "deadline equality", "expired"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			var d prepared
			_ = json.Unmarshal(out.Data, &d)
			if mode != "used only" && mode != "deadline equality" && mode != "expired" {
				if mode == "wrong request" {
					d.Request.AmountIn = big.NewInt(42)
				}
				if mode == "wrong account" {
					d.Account = addr(99)
				}
				h.addEvent(t, d, 12, mode == "cancel")
			}
			switch mode {
			case "fill then cancel":
				h.addEvent(t, d, 12, true)
			case "used only":
				h.c.used = true
			case "failed receipt":
				for _, r := range h.c.receipts {
					r.Status = 0
				}
			case "missing receipt":
				h.c.receipts = map[common.Hash]*types.Receipt{}
			case "removed":
				for _, r := range h.c.receipts {
					r.Logs[0].Removed = true
				}
			case "revoked after fill":
				h.c.approved = false
				h.c.balance = 0
				h.c.allowance = 0
			case "signature error after fill":
				h.c.fail = "signatures"
			case "balance error after fill":
				h.c.fail = "balanceOf"
			case "allowance error after fill":
				h.c.fail = "allowance"
			case "deadline equality":
				s.Timestamp = 2000000000
			case "expired":
				s.Timestamp = 2000000001
			}
			if mode == "deadline equality" || mode == "expired" {
				h.c.times = map[uint64]uint64{12: s.Timestamp}
				head, _ := h.c.HeaderByNumber(t.Context(), s.BlockNumber)
				s.BlockHash = head.Hash()
			}
			p, err := h.p.Advance(t.Context(), s, out, nil)
			done := mode == "fill" || mode == "fill then cancel" || mode == "revoked after fill" || mode == "expired" || mode == "signature error after fill" || mode == "balance error after fill" || mode == "allowance error after fill"
			if done {
				want := "settled"
				if mode == "expired" {
					want = "expired"
				}
				if err != nil || !p.Done || p.Outcome != want || h.posts != 0 {
					t.Fatal(p, err)
				}
			} else if mode == "deadline equality" {
				if err != nil || p.Done || h.posts != 1 {
					t.Fatal("equal deadline is live", p, err)
				}
			} else if err == nil || p.Done || h.posts != 0 {
				t.Fatal("untrusted event accepted", p, err)
			}
		})
	}
}
func TestOrderReceiptDoesNotScanHistory(t *testing.T) {
	h := newHarness(t)
	out, s := h.authorized(t)
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	other := d
	other.Account = addr(0x1234)
	for range 1025 {
		h.addEvent(t, other, 10, false)
	}
	h.addEvent(t, d, 12, false)
	progress, err := h.p.Advance(t.Context(), s, out, nil)
	if err != nil || !progress.Done || progress.Outcome != "settled" || h.c.logReads != 0 || h.c.receiptReads != 1 || h.gets != 1 || h.posts != 0 {
		t.Fatal("order receipt depends on unrelated chain history", progress, err)
	}
}

func TestConfigRejectsUnsafePolicy(t *testing.T) {
	for _, mode := range []string{"zero address", "equal tokens", "zero amount", "negative", "index", "duplicate index", "pages", "solver", "no reader"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			c := h.cfg
			switch mode {
			case "zero address":
				c.Reactor = common.Address{}.Hex()
			case "equal tokens":
				c.TokenOut = c.TokenIn
			case "zero amount":
				c.AmountIn = "0"
			case "negative":
				c.MinAmountOut = "-1"
			case "index":
				v := uint(4096)
				c.AmountInInput = &v
			case "duplicate index":
				v := uint(0)
				c.AmountInInput = &v
				c.AmountOutInput = &v
			case "pages":
				c.MaxLookupPages = 101
			case "solver":
				c.SolverIDs = []string{"a", "a"}
			}
			deps := provider.Deps{Chain: h.c, HTTP: h.server.Client()}
			if mode == "no reader" {
				deps.Chain = nil
			}
			if _, err := New(c, deps); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}

func TestPublicationAcceptanceAndCompletePagination(t *testing.T) {
	for _, mode := range []string{"malformed acceptance", "two complete pages", "matched second page"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			h.httpMode = mode
			if mode != "malformed acceptance" {
				h.pages = []wire.RfqOrdersResponse{{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage(`"oc_second"`)}, {Orders: []wire.RfqOrder{}, Cursor: json.RawMessage("null")}}
				if mode == "matched second page" {
					h.pages[1].Orders = []wire.RfqOrder{{Type: "Priority", OrderID: orderID, OrderStatus: "expired", QuoteID: quoteID, Swapper: s.Account.Hex(), Nonce: "007", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "01000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "950", Recipient: s.Account.Hex()}}}}
				}
			}
			p, err := h.p.Advance(t.Context(), s, out, nil)
			if err != nil || p.Done {
				t.Fatal(p, err)
			}
			switch mode {
			case "malformed acceptance":
				if p.Outcome != "publication-uncertain" || h.posts != 1 {
					t.Fatal("malformed acceptance lost pending identity", p)
				}
			case "two complete pages":
				if h.gets != 2 || h.posts != 1 || p.Reference != orderID {
					t.Fatal("complete pagination did not allow publication", p)
				}
			case "matched second page":
				if h.gets != 2 || h.posts != 0 || p.Reference != orderID {
					t.Fatal("existing terminal API hint republished or settled", p)
				}
			}
		})
	}
}
func TestEventReceiptAndCursorCorruptionFailClosed(t *testing.T) {
	for _, mode := range []string{"receipt member", "receipt block", "receipt tx index", "receipt duplicate", "receipt topic", "event orphan", "cursor version", "cursor digest"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			var d prepared
			_ = json.Unmarshal(out.Data, &d)
			h.addEvent(t, d, 12, false)
			r := h.c.receipts[h.c.logs[0].TxHash]
			var saved json.RawMessage
			st := progressState{Version: 1, Digest: d.Digest}
			switch mode {
			case "receipt member":
				r.Logs = nil
			case "receipt block":
				r.BlockNumber = big.NewInt(9)
			case "receipt tx index":
				r.TransactionIndex = 2
			case "receipt duplicate":
				r.Logs = append(r.Logs, r.Logs[0])
			case "receipt topic":
				l := *r.Logs[0]
				l.Topics = []common.Hash{{}}
				r.Logs[0] = &l
			case "event orphan":
				r.Logs[0].BlockHash = common.HexToHash("0xbad")
			case "cursor version":
				st.Version = 2
			case "cursor digest":
				st.Digest = common.Hash{}
			}
			if mode[:6] == "cursor" {
				saved, _ = json.Marshal(st)
			}
			p, err := h.p.Advance(t.Context(), s, out, saved)
			if err == nil || p.Done || h.posts != 0 {
				t.Fatal("corrupt evidence accepted", p, err)
			}
		})
	}
}
func TestPrepareRejectsAPIIntegerOverflowBeforeHTTP(t *testing.T) {
	for _, mode := range []string{"chain", "time", "block"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			s := h.s
			switch mode {
			case "chain":
				s.ChainID = new(big.Int).Lsh(big.NewInt(1), 53)
			case "time":
				s.Timestamp = maxSafeInteger + 1
			case "block":
				s.BlockNumber = new(big.Int).Lsh(big.NewInt(1), 65)
			}
			// Use an already fetched staged domain result to ensure the validation itself precedes HTTP.
			raw, err := h.c.CallContract(t.Context(), protocolio.ContractRead(h.p.reactor, reactor.NewLiquidLaneReactor().PackEip712Domain(), h.s.BlockNumber, true).Call, h.s.BlockNumber)
			if err != nil {
				t.Fatal(err)
			}
			_, err = h.p.prepare(t.Context(), s, protocolio.ReadResult{Data: raw})
			if err == nil || h.quotes != 0 {
				t.Fatal("overflow reached API", err)
			}
		})
	}
}
