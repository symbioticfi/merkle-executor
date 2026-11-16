package protocolio

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
)

func TestFailureDiagnosticsExposeOnlyAllowlistedReasons(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
		want  string
	}{
		{"http", &rpc.HTTPError{StatusCode: http.StatusForbidden, Status: "secret-status", Body: []byte("secret-body")}, "http_status=403"},
		{"rpc", &batchRPCError{Code: -32602, Message: "invalid params secret-message", Data: "secret-data"}, "rpc_code=-32602 classification=invalid params"},
		{"dns", &net.DNSError{Err: "secret-error", Name: "secret-host"}, "DNS error"},
		{"refused", &url.Error{Op: "Post", URL: "https://secret-user:secret-password@secret-host?key=secret-key", Err: syscall.ECONNREFUSED}, "connection refused"},
		{"deadline", errors.Errorf("secret-context: %w", context.DeadlineExceeded), "context deadline exceeded"},
		{"tls", &net.OpError{Op: "remote error", Net: "secret-network", Err: errors.New("tls: bad certificate secret-server")}, "TLS error"},
		{"eof", io.ErrUnexpectedEOF, "unexpected EOF"},
		{"unknown", errors.New("secret-provider-response"), "unknown error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Failure("read RPC", tc.cause)
			for _, rendered := range []string{err.Error(), fmt.Sprintf("%+v", err), Failure("read protocol", err).Error()} {
				if !strings.Contains(rendered, tc.want) || strings.Contains(rendered, "secret-") {
					t.Fatalf("unsafe or incomplete diagnostic: %s", rendered)
				}
			}
			if !errors.Is(err, tc.cause) {
				t.Fatal("diagnostic lost the original cause")
			}
		})
	}
}
