package protocolio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRedactsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		limit  int64
	}{{"rejected", 400, `{"secret":"never-print"}`, 100}, {"oversized", 200, strings.Repeat("x", 50), 10}, {"invalid-json", 200, "never-print", 100}} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			c, e := NewHTTP(HTTPConfig{URL: s.URL, MaxResponseBytes: tc.limit}, s.Client())
			if e != nil {
				t.Fatal(e)
			}
			var body any
			_, e = c.JSON(context.Background(), http.MethodGet, "/rewards", nil, &body)
			if e == nil || strings.Contains(e.Error(), "never-print") || strings.Contains(e.Error(), s.URL) {
				t.Fatalf("unredacted or missing error: %v", e)
			}
		})
	}
}
func TestHTTPNeverFollowsRedirectWithCredential(t *testing.T) {
	hits := 0
	dest := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer dest.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-test-token" {
			t.Error("missing environment credential")
		}
		http.Redirect(w, r, dest.URL, http.StatusTemporaryRedirect)
	}))
	defer s.Close()
	t.Setenv("PROTOCOL_TEST_TOKEN", "private-test-token")
	c, e := NewHTTP(HTTPConfig{URL: s.URL, APIKeyEnv: "PROTOCOL_TEST_TOKEN"}, s.Client())
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.JSON(context.Background(), http.MethodGet, "/quote", nil, nil)
	if e == nil || hits != 0 {
		t.Fatalf("redirect followed: %d, %v", hits, e)
	}
}
func TestHTTPCancelledContextAndCredentialURL(t *testing.T) {
	if _, e := NewHTTP(HTTPConfig{URL: "https://user:password@example.invalid"}, nil); e == nil {
		t.Fatal("credential URL accepted")
	}
	c, e := NewHTTP(HTTPConfig{URL: "https://example.invalid"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.JSON(ctx, http.MethodGet, "/", nil, nil); e == nil {
		t.Fatal("cancel ignored")
	}
}
