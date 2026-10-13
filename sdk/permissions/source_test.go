package permissions

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestSourceFetchesExactActiveRootWithBoundedFailures(t *testing.T) {
	root := common.HexToHash("0x123")
	body := `{"chainId":31337,"root":"` + root.Hex() + `","adapter":"0x0000000000000000000000000000000000000099","leaves":[],"display":{"unknown":true}}`
	for _, mode := range []string{"valid", "404", "oversize", "malformed", "redirect", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			calls, redirects := 0, 0
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects++; _, _ = w.Write([]byte(body)) }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v1/merkle-trees/"+root.Hex() || r.URL.RawQuery != "chainId=31337" {
					t.Error("source requested another root/chain")
				}
				switch mode {
				case "404":
					http.Error(w, "private body", http.StatusNotFound)
					return
				case "redirect":
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
					return
				case "malformed":
					_, _ = w.Write([]byte("private malformed"))
					return
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat("x", 1025)))
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			source, err := New(protocolio.HTTPConfig{URL: server.URL + "/api/v1", MaxResponseBytes: 1024}, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			raw, err := source.PermissionTree(ctx, big.NewInt(31337), recipes.Snapshot{Root: root})
			if mode == "valid" {
				if err != nil || string(raw) != body {
					t.Fatalf("wire bytes changed: %s %v", raw, err)
				}
			} else if err == nil {
				t.Fatal("failed source accepted")
			}
			if redirects != 0 || calls > 1 {
				t.Fatal("redirect/fallback sent another request")
			}
			if err != nil && (strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), server.URL)) {
				t.Fatal("source leaked response or URL")
			}
		})
	}
}

func TestFixedHostPolicyKeepsLibraryOverridesSeparate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout int
		limit   int64
		invalid bool
	}{
		{"omitted", 0, 0, false}, {"fixed", HostTimeoutSeconds, HostMaxResponseBytes, false}, {"custom timeout", 11, 0, true}, {"custom limit", 0, 4096, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := protocolio.HTTPConfig{URL: "http://127.0.0.1:1/api/v1", TimeoutSeconds: tc.timeout, MaxResponseBytes: tc.limit}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatalf("flexible library config rejected: %v", err)
			}
			fixed, err := NormalizeHostConfig(cfg)
			if tc.invalid {
				if err == nil {
					t.Fatal("host override accepted")
				}
				return
			}
			if err != nil || fixed.TimeoutSeconds != HostTimeoutSeconds || fixed.MaxResponseBytes != HostMaxResponseBytes {
				t.Fatalf("host policy %+v %v", fixed, err)
			}
		})
	}
}
