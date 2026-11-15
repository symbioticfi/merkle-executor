package liquidlanerfq

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
)

func TestTaskKeepsQuoteAndLookupInMemory(t *testing.T) {
	h := newHarness(t)
	h.p.cfg.MaxLookupPages = 1
	out, snapshot := h.authorized(t)
	task := out.Task

	if _, err := task.Advance(t.Context(), h.s); err == nil || h.posts != 0 || h.gets != 0 {
		t.Fatal("task published before confirmed authorization", err)
	}
	// The task keeps its own exact prepared bytes; mutable plan copies cannot
	// change the authorized request or the subsequent publication.
	clear(out.Actions["0:0"])
	clear(out.Data)
	h.pages = []wire.RfqOrdersResponse{
		{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage(`"oc_next"`)},
		{Orders: []wire.RfqOrder{}, Cursor: json.RawMessage("null")},
	}
	progress, err := task.Advance(t.Context(), snapshot)
	if err != nil || progress.Done || progress.Outcome != "lookup-scanning" || h.gets != 1 || h.posts != 0 {
		t.Fatal("first tick did not retain bounded lookup", progress, err)
	}
	progress, err = task.Advance(t.Context(), snapshot)
	if err != nil || progress.Done || progress.Reference != orderID || h.posts != 1 || h.lookups[1].Get("cursor") != "oc_next" {
		t.Fatal("next tick lost quote or lookup identity", progress, err)
	}
	h.order(common.Hash{})
	progress, err = task.Advance(t.Context(), snapshot)
	if err != nil || progress.Done || progress.Reference != orderID || h.posts != 1 || h.quotes != 1 {
		t.Fatal("next tick republished or requoted existing order", progress, err)
	}
}
