package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type failedDiagnosticsTransport struct{ cause error }

func (f failedDiagnosticsTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, f.cause
}

func TestRPCDiagnosticsSurviveReadBoundariesWithoutSecrets(t *testing.T) {
	for _, mode := range []string{"http", "rpc", "canceled", "transport", "eof"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request rpcRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Method == "eth_chainId" {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x1"})
				} else if mode == "http" {
					http.Error(w, "secret-body Bearer secret-token", http.StatusTooManyRequests)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32005, "message": "rate limit exceeded secret-message", "data": "secret-data"}})
				}
			}))
			defer server.Close()
			endpoint := strings.Replace(server.URL, "http://", "http://secret-user:secret-password@", 1)
			client, err := Dial(t.Context(), endpoint+"/secret-path?key=secret-query", "", big.NewInt(1))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var transportErr error
			switch mode {
			case "transport":
				transportErr = &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
			case "eof":
				transportErr = io.EOF
			}
			if transportErr != nil {
				// Closing a real server can race keep-alive reuse and return either
				// EOF or ECONNREFUSED. Inject each cause at the HTTP boundary instead.
				client.http.CloseIdleConnections()
				client.http.Transport = failedDiagnosticsTransport{cause: transportErr}
			}
			target := common.HexToAddress("0x0dabc0d9b270c9b0c4c77aaceaa712b56d0f9178")
			data := append([]byte{0x8d, 0x29, 0xd3, 0x0e}, []byte("secret-calldata")...)
			block := big.NewInt(26409887)
			snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: common.Address{1}, BlockNumber: block}
			for _, boundary := range []string{"direct", "provider", "batch"} {
				t.Run(boundary, func(t *testing.T) {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					if mode == "canceled" {
						cancel()
					}
					switch boundary {
					case "direct":
						_, err = client.CallContract(ctx, ethereum.CallMsg{To: &target, Data: data}, block)
					case "provider":
						_, err = protocolio.CallData(ctx, client, snapshot, target, data)
					case "batch":
						var results []protocolio.ReadResult
						results, err = client.BatchRead(ctx, []protocolio.ReadRequest{protocolio.ContractRead(target, data, block, true)})
						if err == nil {
							_, err = results[0].CallData()
						}
					}
					if err == nil {
						t.Fatal("failed read succeeded")
					}
					for _, rendered := range []string{err.Error(), fmt.Sprintf("%+v", err), errors.Errorf("resolve runtime: %w", err).Error()} {
						for _, want := range []string{"eth_call", strings.ToLower(target.Hex()), "0x8d29d30e", "0x192fb9f"} {
							if !strings.Contains(strings.ToLower(rendered), want) {
								t.Errorf("diagnostic missing %q: %s", want, rendered)
							}
						}
						if strings.Contains(rendered, "secret-") || strings.Contains(rendered, server.URL) || strings.Contains(rendered, hexutil.Encode(data[4:])[2:]) {
							t.Fatal("diagnostic leaked credentials, payload or provider text")
						}
					}
					want := "http_status=429"
					switch mode {
					case "rpc":
						want = "rpc_code=-32005"
					case "transport":
						want = "connection refused"
					case "eof":
						want = "EOF"
					case "canceled":
						want = "context canceled"
						if !errors.Is(err, context.Canceled) {
							t.Fatal("cancellation identity was lost")
						}
					}
					if !strings.Contains(err.Error(), want) {
						t.Errorf("diagnostic missing failure reason %q: %s", want, err)
					}
					if transportErr != nil && !errors.Is(err, transportErr) {
						t.Fatal("transport error identity was lost")
					}
				})
			}
		})
	}
}
