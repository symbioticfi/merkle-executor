package protocolio

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-errors/errors"
)

func TestRejectLogScansExaminesEveryRawMember(t *testing.T) {
	for _, method := range []string{"eth_getLogs", "eth_newFilter", "eth_getFilterLogs", "eth_getFilterChanges"} {
		for _, body := range []string{`{"method":"` + method + `"}`, `[{"method":"eth_call"},{"method":"` + method + `"}]`, `[7,{"method":"` + method + `"}]`, `[{"method":7},{"method":"` + method + `"}]`} {
			if err := RejectLogScans([]byte(body)); !errors.Is(err, ErrLogScan) {
				t.Fatalf("scan escaped raw policy: %s %v", body, err)
			}
		}
	}
	for _, body := range []string{`{"method":"eth_call","params":["eth_getLogs"]}`, `[{"method":"unknown"},null,7]`, `[]`} {
		raw := []byte(body)
		original := bytes.Clone(raw)
		if err := RejectLogScans(raw); err != nil {
			t.Fatalf("unrelated/malformed request changed: %s %v", body, err)
		}
		if !bytes.Equal(raw, original) {
			t.Fatal("policy changed caller request bytes")
		}
	}
	for _, body := range []string{"", `{`, `{"method":"eth_getLogs"} trailing`, `[{"method":"eth_getLogs"}] trailing`} {
		if RejectLogScans([]byte(body)) == nil {
			t.Fatalf("malformed JSON prefix escaped policy: %s", body)
		}
	}
}

func TestGuardLogScansTransportForwardsOnlyUnchangedAllowedRequests(t *testing.T) {
	calls := 0
	allowed := []byte(`[{"method":"eth_call","id":1},{"method":"eth_getBlockByNumber","id":2}]`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, allowed) || r.Header.Get("X-Test") != "preserved" {
			t.Error("guard changed allowed request")
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := &http.Client{Transport: GuardLogScansTransport(server.Client().Transport)}
	defer client.CloseIdleConnections()
	for _, body := range [][]byte{[]byte(`[{"method":"unknown"},{"method":"eth_getLogs"}]`), []byte(`[7,{"method":"eth_getFilterChanges"}]`), allowed} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Test", "preserved")
		response, err := client.Do(req)
		if bytes.Equal(body, allowed) {
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
		} else if !errors.Is(err, ErrLogScan) {
			t.Fatalf("raw request was not blocked: %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("blocked request reached backend: %d calls", calls)
	}
}
