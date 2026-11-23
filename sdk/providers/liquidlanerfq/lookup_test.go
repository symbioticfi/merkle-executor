package liquidlanerfq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
)

func TestLookupBeyondThousandOrdersContinuesAcrossTicks(t *testing.T) {
	for _, mode := range []string{"new order", "existing order", "duplicate quote", "duplicate ID", "repeated cursor", "uncertain publication"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			h.p.cfg.MaxLookupPages = 10
			out, s := h.authorized(t)
			for page := 0; page < 11; page++ {
				rows := []wire.RfqOrder{}
				n := 100
				if page == 10 {
					n = 1
				}
				for j := 0; j < n; j++ {
					id := page*100 + j + 1
					rows = append(rows, wire.RfqOrder{Type: "Priority", OrderID: fmt.Sprintf("%08x-2222-4222-8222-222222222222", id), QuoteID: fmt.Sprintf("%08x-3333-4333-8333-333333333333", id), OrderStatus: "filled", Swapper: s.Account.Hex()})
				}
				cursor := json.RawMessage(fmt.Sprintf(`"oc_page%d"`, page+1))
				if page == 10 {
					cursor = json.RawMessage("null")
				}
				h.pages = append(h.pages, wire.RfqOrdersResponse{Orders: rows, Cursor: cursor})
			}
			if mode == "existing order" || mode == "duplicate quote" {
				h.pages[0].Orders[0] = wire.RfqOrder{Type: "Priority", OrderID: orderID, OrderStatus: "open", QuoteID: quoteID, Swapper: s.Account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "900", Recipient: s.Account.Hex()}}}
			}
			switch mode {
			case "duplicate quote":
				h.pages[10].Orders[0] = h.pages[0].Orders[0]
				h.pages[10].Orders[0].OrderID = "99999999-9999-4999-8999-999999999999"
			case "duplicate ID":
				h.pages[10].Orders[0] = h.pages[0].Orders[0]
			case "repeated cursor":
				h.pages[10].Cursor = json.RawMessage(`"oc_page1"`)
			case "uncertain publication":
				h.httpMode = "lost"
			}
			progress, err := out.Task.Advance(t.Context(), s)
			if err != nil || progress.Done || progress.Outcome != "lookup-scanning" || h.gets != 10 || h.posts != 0 {
				t.Fatalf("lookup did not checkpoint without publishing: %+v %v gets=%d posts=%d", progress, err, h.gets, h.posts)
			}
			progress, err = out.Task.Advance(t.Context(), s)
			if h.lookups[10].Get("cursor") != "oc_page10" {
				t.Fatal("next tick did not resume the page", h.lookups[10])
			}
			switch mode {
			case "duplicate quote", "duplicate ID", "repeated cursor":
				if err == nil || h.posts != 0 {
					t.Fatalf("invalid cross-tick lookup published: %v posts=%d", err, h.posts)
				}
			case "existing order":
				if err != nil || progress.Reference != orderID || progress.Outcome != "published" || h.posts != 0 {
					t.Fatal(progress, err, h.posts)
				}
			default:
				if err != nil || progress.Done || h.posts != 1 {
					t.Fatal(progress, err, h.posts)
				}

				if mode == "uncertain publication" {
					h.gets = 0
					progress, err = out.Task.Advance(t.Context(), s)
					if err != nil || progress.Outcome != "lookup-scanning" || h.posts != 1 || h.lookups[11].Get("cursor") != "" {
						t.Fatalf("ambiguous POST reused completed absence lookup: %+v %v posts=%d", progress, err, h.posts)
					}
				}
			}
		})
	}
}

func TestOrderTransactionHintSurvivesNextLookupTick(t *testing.T) {
	h := newHarness(t)
	out, s := h.authorized(t)
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	h.addEvent(t, d, 12, false)
	h.p.cfg.MaxLookupPages = 1
	h.pages[0].Cursor = json.RawMessage(`"oc_next"`)
	h.pages = append(h.pages, wire.RfqOrdersResponse{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage("null")})
	progress, err := out.Task.Advance(t.Context(), s)
	if err != nil || progress.Done || progress.Outcome != "lookup-scanning" || h.c.receiptReads != 0 {
		t.Fatal("partial lookup consumed receipt hint", progress, err)
	}
	progress, err = out.Task.Advance(t.Context(), s)
	if err != nil || !progress.Done || progress.Outcome != "settled" || progress.Reference != orderID || h.c.receiptReads != 1 || h.c.logReads != 0 || h.posts != 0 {
		t.Fatal("next tick lost matching order transaction", progress, err)
	}
}

func TestOrderMayPrecedeTransactionHash(t *testing.T) {
	h := newHarness(t)
	out, s := h.authorized(t)
	var d prepared
	_ = json.Unmarshal(out.Data, &d)
	h.addEvent(t, d, 12, false)
	h.pages[0].Orders[0].TxHash = nil
	progress, err := out.Task.Advance(t.Context(), s)
	if err != nil || progress.Done || progress.Outcome != "awaiting-receipt" || progress.Reference != orderID || h.posts != 0 || h.c.receiptReads != 0 {
		t.Fatal("API indexing race released or republished order", progress, err)
	}
	h.order(h.c.logs[0].TxHash)
	progress, err = out.Task.Advance(t.Context(), s)
	if err != nil || !progress.Done || progress.Outcome != "settled" || h.posts != 0 {
		t.Fatal("later transaction hash did not settle", progress, err)
	}
}

func TestMalformedOrderTransactionHashCannotSettleOrPublish(t *testing.T) {
	for _, hash := range []string{"", "0x01", "0x" + string(make([]byte, 64)), "0x0000000000000000000000000000000000000000000000000000000000000000"} {
		h := newHarness(t)
		out, s := h.authorized(t)
		h.order([32]byte{})
		h.pages[0].Orders[0].TxHash = &hash
		progress, err := out.Task.Advance(t.Context(), s)
		if err == nil || progress.Done || h.posts != 0 || h.c.receiptReads != 0 {
			t.Fatal("malformed transaction hash reached settlement", progress, err)
		}
	}
}

type requestTransport func(*http.Request) (*http.Response, error)

func (f requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTimedOutLookupResumesAtLastValidatedPage(t *testing.T) {
	for _, mode := range []string{"timeout", "timeout with duplicate"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			row := wire.RfqOrder{Type: "Priority", OrderID: orderID, QuoteID: "00000000-2222-4222-8222-222222222222", OrderStatus: "expired", Swapper: s.Account.Hex()}
			h.pages = []wire.RfqOrdersResponse{
				{Orders: []wire.RfqOrder{row}, Cursor: json.RawMessage(`"oc_next"`)},
				{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage("null")},
			}
			if mode == "timeout with duplicate" {
				h.pages[1].Orders = []wire.RfqOrder{row}
			}
			timedOut := false
			client := &http.Client{Transport: requestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("cursor") == "oc_next" && !timedOut {
					timedOut = true
					return nil, context.DeadlineExceeded
				}
				return h.server.Client().Transport.RoundTrip(r)
			})}
			var err error
			h.p.http, err = protocolio.NewHTTP(h.cfg.HTTP, client)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = out.Task.Advance(t.Context(), s); !errors.Is(err, context.DeadlineExceeded) || h.posts != 0 {
				t.Fatalf("interrupted lookup: posts=%d err=%v", h.posts, err)
			}
			progress, err := out.Task.Advance(t.Context(), s)
			if len(h.lookups) < 2 || h.lookups[len(h.lookups)-1].Get("cursor") != "oc_next" {
				t.Fatalf("retried wrong page: %v", h.lookups)
			}
			if mode == "timeout with duplicate" {
				if err == nil || h.posts != 0 {
					t.Fatalf("lost duplicate check: posts=%d err=%v", h.posts, err)
				}
			} else if err != nil || h.posts != 1 || progress.Reference != orderID {
				t.Fatalf("valid resumed lookup: %+v, posts=%d err=%v", progress, h.posts, err)
			}
		})
	}
}
