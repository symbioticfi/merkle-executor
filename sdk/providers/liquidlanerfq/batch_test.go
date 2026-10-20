package liquidlanerfq

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestRequestMatchesGoEthereumTypedData(t *testing.T) {
	h := newHarness(t)
	out, err := h.p.Prepare(t.Context(), h.s)
	if err != nil {
		t.Fatal(err)
	}
	var d prepared
	if err = json.Unmarshal(out.Data, &d); err != nil {
		t.Fatal(err)
	}
	sig := h.option["signatureData"].(map[string]any)
	raw, err := json.Marshal(sig["types"])
	if err != nil {
		t.Fatal(err)
	}
	var schema apitypes.Types
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	schema["EIP712Domain"] = []apitypes.Type{{Name: "name", Type: "string"}, {Name: "version", Type: "string"}, {Name: "chainId", Type: "uint256"}, {Name: "verifyingContract", Type: "address"}}
	typed := apitypes.TypedData{Types: schema, PrimaryType: "Request", Domain: apitypes.TypedDataDomain{Name: "Reactor", Version: "1", ChainId: math.NewHexOrDecimal256(1), VerifyingContract: h.p.reactor.Hex()}, Message: sig["value"].(map[string]any)}
	digest, _, err := apitypes.TypedDataAndHash(typed)
	if err != nil || common.BytesToHash(digest) != d.Digest {
		t.Fatal("independent typed data differs", err)
	}
}

type batchChain struct {
	*testChain
	groups    [][]protocolio.ReadRequest
	failBatch bool
	short     bool
}

func (b *batchChain) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	b.groups = append(b.groups, append([]protocolio.ReadRequest(nil), requests...))
	if b.failBatch {
		return nil, errors.New("failed batch")
	}
	if b.short {
		return []protocolio.ReadResult{}, nil
	}
	out := make([]protocolio.ReadResult, len(requests))
	for i, r := range requests {
		switch r.Kind {
		case protocolio.ReadCall:
			out[i].Data, out[i].Err = b.CallContract(ctx, r.Call, r.Block)
		case protocolio.ReadHeader:
			out[i].Header, out[i].Err = b.HeaderByNumber(ctx, r.Block)
		case protocolio.ReadLogs:
			out[i].Logs, out[i].Err = b.FilterLogs(ctx, r.Filter)
		case protocolio.ReadReceipt:
			out[i].Receipt, out[i].Err = b.TransactionReceipt(ctx, r.Hash)
		default:
			b.t.Fatal("unexpected read kind")
		}
	}
	return out, nil
}
func TestStagedPreparationIdentityAndBatchParity(t *testing.T) {
	h := newHarness(t)
	direct, err := h.p.Prepare(t.Context(), h.s)
	if err != nil {
		t.Fatal(err)
	}
	calls, quotes := h.c.calls, h.quotes
	pin := protocolio.PreparationPin{ChainID: h.s.ChainID.String(), BlockNumber: h.s.BlockNumber.Uint64(), BlockHash: h.s.BlockHash, Timestamp: h.s.Timestamp, Executor: h.s.Executor, Account: h.s.Account, Caller: h.s.Caller}
	plan, err := h.p.PlanPreparation(pin)
	if err != nil {
		t.Fatal(err)
	}
	if h.c.calls != calls || h.quotes != quotes || len(plan.Requests) != 1 || !plan.Requests[0].MulticallSafe {
		t.Fatal("planning performed I/O")
	}
	for _, mutate := range []func(*provider.Snapshot){func(s *provider.Snapshot) { s.Account = addr(99) }, func(s *provider.Snapshot) { s.BlockHash = common.HexToHash("0x01") }, func(s *provider.Snapshot) { s.BlockNumber = big.NewInt(9) }} {
		s := h.s
		mutate(&s)
		_, err = plan.Resolve(t.Context(), s, func(int) protocolio.ReadResult {
			t.Fatal("mismatch consumed staged result")
			return protocolio.ReadResult{}
		})
		if err == nil || h.quotes != quotes {
			t.Fatal("staged identity was accepted")
		}
	}
	b := &batchChain{testChain: h.c}
	h.p.chain = b
	reads, err := protocolio.ReadGroup(t.Context(), b, plan.Requests...)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := plan.Resolve(t.Context(), h.s, reads.At)
	if err != nil || !reflect.DeepEqual(staged, direct) {
		t.Fatal("staged output differs", err)
	}
	out, s := h.authorized(t)
	b.groups = nil
	progress, err := h.p.Advance(t.Context(), s, out, nil)
	if err != nil || progress.Done {
		t.Fatal(progress, err)
	}
	found := false
	for _, group := range b.groups {
		if len(group) == 5 {
			found = true
			for i, r := range group {
				if r.Kind != protocolio.ReadCall || r.MulticallSafe != (i < 3) || r.Block.Cmp(s.BlockNumber) != 0 || r.Call.From != (common.Address{}) {
					t.Fatal("Advance grouping changed isolation or pin")
				}
			}
		}
	}
	if !found {
		t.Fatal("Advance did not group three safe and two isolated getters")
	}
}
func TestBatchFailureNeverFallsBack(t *testing.T) {
	for _, mode := range []string{"transport", "shape"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			b := &batchChain{testChain: h.c, failBatch: mode == "transport", short: mode == "shape"}
			h.p.chain = b
			_, err := h.p.Prepare(t.Context(), h.s)
			if err == nil || h.c.calls != 0 || h.quotes != 0 || len(b.groups) != 1 {
				t.Fatal("failed batch retried or fetched quote", err)
			}
		})
	}
}
func TestQuoteHTTPLimitsAndCancellation(t *testing.T) {
	for _, mode := range []string{"cancel", "oversize", "status"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "cancel" {
					cancel()
					return
				}
				if mode == "status" {
					w.WriteHeader(http.StatusUnauthorized)
				}
				_, _ = w.Write(bytes.Repeat([]byte("private-secret-payload"), 100))
			}))
			defer srv.Close()
			var err error
			h.p.http, err = protocolio.NewHTTP(protocolio.HTTPConfig{URL: srv.URL, MaxResponseBytes: 128}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = h.p.Prepare(ctx, h.s)
			if err == nil || strings.Contains(err.Error(), "private-secret-payload") || strings.Contains(err.Error(), srv.URL) || h.posts != 0 {
				t.Fatal("HTTP failure leaked payload or proceeded", err)
			}
		})
	}
}
func TestDenseSingleBlockAndUnrelatedInvalidations(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "Fill limit", true: "indexed unrelated invalidation"}[cancel], func(t *testing.T) {
			h := newHarness(t)
			out, s := h.authorized(t)
			var d prepared
			_ = json.Unmarshal(out.Data, &d)
			other := d
			other.Account = addr(0x1234)
			for range 1025 {
				h.addEvent(t, other, 10, cancel)
			}
			h.c.used = false
			p, err := h.p.Advance(t.Context(), s, out, nil)
			if cancel {
				if err != nil || p.Done || h.posts != 1 {
					t.Fatal("unrelated invalidations blocked owner", p, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "single block") || h.posts != 0 {
				t.Fatal("single-block limit silently truncated", p, err)
			}
		})
	}
}
