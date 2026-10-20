package liquidlanerfq

import (
	"encoding/json"
	"fmt"
	"testing"

	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestLookupBeyondThousandOrdersSurvivesRestart(t *testing.T) {
	for _, mode := range []string{"new order", "existing order", "duplicate quote", "duplicate ID", "repeated cursor", "uncertain publication", "crash after publication"} {
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
			case "uncertain publication", "crash after publication":
				h.httpMode = "lost"
			}
			progress, err := h.p.Advance(t.Context(), s, out, nil)
			if err != nil || progress.Done || progress.Outcome != "lookup-scanning" || h.gets != 10 || h.posts != 0 {
				t.Fatalf("lookup did not checkpoint without publishing: %+v %v gets=%d posts=%d", progress, err, h.gets, h.posts)
			}
			checkpoint := append(json.RawMessage(nil), progress.State...)
			// Only serialized provider progress is carried into a newly constructed runtime.
			h.p, err = New(h.p.cfg, provider.Deps{Chain: h.c, HTTP: h.server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			progress, err = h.p.Advance(t.Context(), s, out, append(json.RawMessage(nil), progress.State...))
			if h.lookups[10].Get("cursor") != "oc_page10" {
				t.Fatal("restart did not resume the saved page", h.lookups[10])
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

				if mode == "crash after publication" {
					// The POST was accepted, then the process died before saving its result.
					// The appended remote order must be found from the previous checkpoint.
					h.pages[10].Orders[0] = wire.RfqOrder{Type: "Priority", OrderID: orderID, OrderStatus: "open", QuoteID: quoteID, Swapper: s.Account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: addr(0x33).Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: addr(0x44).Hex(), Amount: "900", Recipient: s.Account.Hex()}}}
					h.gets = 10
					h.p, err = New(h.p.cfg, provider.Deps{Chain: h.c, HTTP: h.server.Client()})
					if err != nil {
						t.Fatal(err)
					}
					progress, err = h.p.Advance(t.Context(), s, out, checkpoint)
					if err != nil || progress.Reference != orderID || h.posts != 1 {
						t.Fatalf("crash lost published identity: %+v %v posts=%d", progress, err, h.posts)
					}
				}

				if mode == "uncertain publication" {
					h.gets = 0
					progress, err = h.p.Advance(t.Context(), s, out, progress.State)
					if err != nil || progress.Outcome != "lookup-scanning" || h.posts != 1 || h.lookups[11].Get("cursor") != "" {
						t.Fatalf("ambiguous POST reused completed absence lookup: %+v %v posts=%d", progress, err, h.posts)
					}
				}
			}
		})
	}
}

func TestMalformedLookupCheckpointNeverPublishes(t *testing.T) {
	for _, mode := range []string{"cursor", "history", "ids", "match"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			h.p.cfg.MaxLookupPages = 1
			out, s := h.authorized(t)
			h.pages = []wire.RfqOrdersResponse{{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage(`"oc_next"`)}}
			progress, err := h.p.Advance(t.Context(), s, out, nil)
			if err != nil {
				t.Fatal(err)
			}
			var saved progressState
			if err := json.Unmarshal(progress.State, &saved); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "cursor":
				saved.Lookup.Cursor = "bad"
			case "history":
				saved.Lookup.Cursors["oc_next"] = false
			case "ids":
				saved.Lookup.IDs = nil
			case "match":
				saved.Lookup.Match = orderID
			}
			raw, _ := json.Marshal(saved)
			if _, err := h.p.Advance(t.Context(), s, out, raw); err == nil || h.posts != 0 {
				t.Fatalf("corrupt lookup allowed publication: %v posts=%d", err, h.posts)
			}
		})
	}
}
