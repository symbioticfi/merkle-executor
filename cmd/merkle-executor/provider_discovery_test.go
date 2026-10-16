package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAutomaticProvidersRequireExternalMaterial(t *testing.T) {
	var names []string
	for _, registration := range automaticProviders {
		names = append(names, registration.name)
	}
	if want := []string{"cowswap", "merkl", "liquidlane-rfq"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("automatic providers: %v, want %v", names, want)
	}
}

func TestDaemonRejectsManualRuntimeSources(t *testing.T) {
	for _, name := range []string{"static", "rpc", "http", "merkl", "cowswap", "liquidlane", "liquidlane-rfq", "redemptions", "securitize"} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
			defer server.Close()
			cfg := testCLIConfig(t.TempDir())
			cfg.Chain.RPCURL = server.URL
			path := writeCLIConfig(t, t.TempDir(), cfg)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			prefix, _, _ := strings.Cut(string(raw), "strategies:")
			raw = []byte(prefix + "strategies:\n  - executor: '" + cfg.Strategies[0] + "'\n    providers:\n      - name: " + name + "\n        config: {}\n")
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			err = runBot(t.Context(), path, true, true, new(bytes.Buffer))
			if err == nil || calls.Load() != 0 {
				t.Fatalf("manual runtime source reached network: %v calls=%d", err, calls.Load())
			}
		})
	}
}
