package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/symbioticfi/merkle-executor/internal/config"
)

func TestDaemonRejectsManualRuntimeSources(t *testing.T) {
	for _, name := range []string{"static", "rpc", "http", "merkl", "cowswap", "liquidlane", "liquidlane-rfq", "redemptions", "securitize"} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
			defer server.Close()
			cfg := testCLIConfig(t.TempDir())
			cfg.Chain.RPCURL = server.URL
			cfg.Strategies[0].Providers = []config.Provider{{Name: name, Config: serviceNode(t, map[string]any{})}}
			err := runBot(t.Context(), writeCLIConfig(t, t.TempDir(), cfg), true, true, new(bytes.Buffer))
			if err == nil || !strings.Contains(err.Error(), "strategy.providers is obsolete") || calls.Load() != 0 {
				t.Fatalf("manual runtime source reached network: %v calls=%d", err, calls.Load())
			}
		})
	}
}
