package liquidlanerfq

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestCancellationUsesOrdersAcrossExpiry(t *testing.T) {
	for _, test := range []struct {
		name          string
		used, expired bool
	}{
		{"cancelled", true, false}, {"cancelled after expiry", true, true},
		{"snapshot before cancellation", false, false}, {"expired snapshot before cancellation", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t)
			out, snapshot := h.authorized(t)
			h.order(common.Hash{})
			h.pages[0].Orders[0].OrderStatus = "cancelled"
			h.c.used = test.used
			h.c.fail = "receipt"
			if test.expired {
				snapshot.Timestamp = 2000000001
				h.c.times = map[uint64]uint64{12: snapshot.Timestamp}
				header, _ := h.c.HeaderByNumber(t.Context(), snapshot.BlockNumber)
				snapshot.BlockHash = header.Hash()
			}
			progress, err := out.Task.Advance(t.Context(), snapshot)
			want := "awaiting-receipt"
			if test.used {
				want = "cancelled"
			}
			if err != nil || progress.Done != test.used || progress.Outcome != want || h.posts != 0 || h.c.logReads != 0 || h.c.receiptReads != 0 {
				t.Fatal("wrong API cancellation outcome", progress, err)
			}
		})
	}
}

func TestCancellationSurvivesPaginatedLookup(t *testing.T) {
	h := newHarness(t)
	out, snapshot := h.authorized(t)
	h.order(common.Hash{})
	h.c.used = true
	h.pages[0].Orders[0].OrderStatus = "cancelled"
	h.pages[0].Cursor = json.RawMessage(`"oc_next"`)
	h.pages = append(h.pages, h.pages[0])
	h.pages[1].Orders = h.pages[1].Orders[:0]
	h.pages[1].Cursor = json.RawMessage(`null`)
	h.p.cfg.MaxLookupPages = 1
	first, err := out.Task.Advance(t.Context(), snapshot)
	if err != nil || first.Done || first.Outcome != "lookup-scanning" {
		t.Fatal(first, err)
	}
	final, err := out.Task.Advance(t.Context(), snapshot)
	if err != nil || !final.Done || final.Outcome != "cancelled" || h.posts != 0 {
		t.Fatal(final, err)
	}
}
