package protocolio

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-errors/errors"
)

func logScanMethod(method string) bool {
	switch method {
	case "eth_getLogs", "eth_newFilter", "eth_getFilterLogs", "eth_getFilterChanges":
		return true
	default:
		return false
	}
}

// RejectLogScans checks every raw batch member before any endpoint attempt.
// Invalid JSON-RPC siblings do not hide a valid event scan later in the batch.
// Malformed bodies are rejected so a server cannot execute a valid JSON prefix.
func RejectLogScans(body []byte) error {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return errors.New("invalid JSON-RPC request")
	}
	var messages []json.RawMessage
	if body[0] == '[' {
		if err := json.Unmarshal(body, &messages); err != nil {
			return errors.New("invalid JSON-RPC request")
		}
	} else {
		if !json.Valid(body) {
			return errors.New("invalid JSON-RPC request")
		}
		messages = []json.RawMessage{body}
	}
	for _, raw := range messages {
		var message struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(raw, &message); err == nil && logScanMethod(message.Method) {
			return ErrLogScan
		}
	}
	return nil
}

type logScanGuardTransport struct{ next http.RoundTripper }

// GuardLogScansTransport also protects embedded/raw RPC clients sharing this transport.
func GuardLogScansTransport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &logScanGuardTransport{next: next}
}

func (t *logScanGuardTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body == nil {
		return t.next.RoundTrip(request)
	}
	// The guard buffers at most the SDK's existing bounded batch response budget.
	body, err := io.ReadAll(io.LimitReader(request.Body, maxBatchResponseBytes+1))
	_ = request.Body.Close()
	if err != nil {
		return nil, errors.New("read RPC request for log-scan policy")
	}
	if len(body) > maxBatchResponseBytes {
		return nil, errors.New("RPC request exceeds log-scan policy bound")
	}
	if err = RejectLogScans(body); err != nil {
		return nil, err
	}
	clone := request.Clone(request.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	return t.next.RoundTrip(clone)
}

func (t *logScanGuardTransport) CloseIdleConnections() {
	if next, ok := t.next.(interface{ CloseIdleConnections() }); ok {
		next.CloseIdleConnections()
	}
}
