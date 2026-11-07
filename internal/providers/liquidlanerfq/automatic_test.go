package liquidlanerfq

import (
	"encoding/json"
	"testing"

	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAutomaticRestoreAndLegacyImportRetainExternalCursorBeforeIO(t *testing.T) {
	h := newHarness(t)
	inner, err := h.p.Prepare(t.Context(), h.s)
	if err != nil {
		t.Fatal(err)
	}
	cfg := h.cfg
	cfg.HTTP = consumerHTTP()
	cfg.MinValiditySeconds = minValidity
	cfg.MaxValiditySeconds = maxValidity
	cfg.MaxLookupPages = 10
	deps := registry.Deps{Chain: h.c, HTTP: h.server.Client()}
	integration, err := NewAutomatic(deps)
	if err != nil {
		t.Fatal(err)
	}
	var saved prepared
	if err = json.Unmarshal(inner.Data, &saved); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(progressState{Version: 1, Digest: saved.Digest, NextBlock: saved.PreparedBlock, OrderID: orderID, Lookup: &lookupState{Cursor: "oc_next", Cursors: map[string]bool{"oc_next": true}, IDs: map[string]bool{orderID: true}, Match: orderID}})
	legacy, err := derived.Encode(automaticPolicy, cfg, inner, true)
	if err != nil {
		t.Fatal(err)
	}
	beforeReads, beforeHTTP := h.c.calls, h.quotes+h.gets+h.posts
	records, err := ImportLegacy(deps, h.s, legacy, provider.Progress{State: state, Reference: orderID})
	if err != nil || len(records) != 1 || string(records[0].Progress.State) != string(state) || records[0].Progress.Reference != orderID {
		t.Fatalf("external identity lost %+v %v", records, err)
	}
	wrong := h.s
	wrong.Account = addr(99)
	if _, err = ImportLegacy(deps, wrong, legacy, provider.Progress{}); err == nil {
		t.Fatal("legacy account mismatch accepted")
	}
	if _, err = integration.Restore(records[0]); err != nil {
		t.Fatal(err)
	}
	records[0].Progress.State = json.RawMessage(`{"version":1,"nextBlock":0}`)
	if _, err = integration.Restore(records[0]); err == nil {
		t.Fatal("malformed durable cursor accepted")
	}
	if h.c.calls != beforeReads || h.quotes+h.gets+h.posts != beforeHTTP {
		t.Fatal("offline restore or import performed I/O")
	}
	operations, err := integration.Compile(provider.Program{Snapshot: h.s, Tree: &recipes.Tree{}})
	if err != nil || len(operations) != 0 {
		t.Fatalf("unmatched compile %+v %v", operations, err)
	}
}
