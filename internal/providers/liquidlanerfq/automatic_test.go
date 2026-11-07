package liquidlanerfq

import (
	"encoding/json"
	"testing"

	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAutomaticRestoreRetainsExternalCursorBeforeIO(t *testing.T) {
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
	output, err := provider.EncodeOperation(integration.Policy(), cfg, inner)
	if err != nil {
		t.Fatal(err)
	}
	requirements := provider.OutputRequirements(inner, nil)
	record := provider.Record{Integration: integration.ID(), Policy: integration.Policy(), ID: provider.OperationID(requirements.Actions), Requirements: requirements, Output: output, Progress: provider.Progress{State: state, Reference: orderID}}
	beforeReads, beforeHTTP := h.c.calls, h.quotes+h.gets+h.posts
	restored, err := integration.Restore(record)
	if err != nil {
		t.Fatal(err)
	}
	retained := restored.(*automaticOperation).saved.Progress
	if string(retained.State) != string(state) || retained.Reference != orderID {
		t.Fatalf("external identity lost %+v", retained)
	}
	for _, fault := range []string{"account", "chain", "cursor", "reference"} {
		t.Run(fault, func(t *testing.T) {
			changed := record.Clone()
			switch fault {
			case "cursor":
				changed.Progress.State = json.RawMessage(`{"version":1,"nextBlock":0}`)
			case "reference":
				changed.Progress.Reference = "different-order"
			default:
				identity := saved
				if fault == "account" {
					identity.Account = addr(99)
				} else {
					identity.ChainID = "99"
				}
				corrupt := inner.Clone()
				corrupt.Data, err = json.Marshal(identity)
				if err != nil {
					t.Fatal(err)
				}
				changed.Output, err = provider.EncodeOperation(integration.Policy(), cfg, corrupt)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := integration.Restore(changed); err == nil {
				t.Fatalf("changed %s accepted", fault)
			}
		})
	}
	if h.c.calls != beforeReads || h.quotes+h.gets+h.posts != beforeHTTP {
		t.Fatal("offline restore performed I/O")
	}
	operations, err := integration.Compile(provider.Program{Snapshot: h.s, Tree: &recipes.Tree{}})
	if err != nil || len(operations) != 0 {
		t.Fatalf("unmatched compile %+v %v", operations, err)
	}
}
