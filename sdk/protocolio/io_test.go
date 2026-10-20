package protocolio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-errors/errors"
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
			c, e := NewHTTP(HTTPConfig{URL: s.URL, MaxResponseBytes: tc.limit}, s.Client(), nil)
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

type httpRoundTripFunc func(*http.Request) (*http.Response, error)

func (f httpRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type httpFailureBody struct {
	read   func([]byte) (int, error)
	closed bool
}

func (b *httpFailureBody) Read(p []byte) (int, error) { return b.read(p) }
func (b *httpFailureBody) Close() error               { b.closed = true; return nil }

func assertHTTPErrorRedacted(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("missing HTTP failure")
	}
	for _, private := range []string{"synthetic-private", "fixture.invalid", "/private-path", "private-test-token"} {
		if strings.Contains(err.Error(), private) {
			t.Fatalf("HTTP error exposed private data: %v", err)
		}
	}
	var rawURL *url.Error
	if errors.As(err, &rawURL) {
		t.Fatal("HTTP error exposed a raw URL cause")
	}
}

func TestHTTPFailureCauseRedaction(t *testing.T) {
	for _, phase := range []string{"headers", "body"} {
		for _, mode := range []string{"plain", "custom-cancel-cause", "custom-deadline-cause", "wrapped-canceled", "wrapped-deadline"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				ctx := t.Context()
				cancel := func() {}
				privateCause := errors.New("synthetic-private-cause")
				var want error
				switch mode {
				case "custom-cancel-cause":
					var cancelCause context.CancelCauseFunc
					ctx, cancelCause = context.WithCancelCause(ctx)
					cancel = func() { cancelCause(privateCause) }
					want = context.Canceled
				case "custom-deadline-cause":
					ctx, cancel = context.WithDeadlineCause(ctx, time.Now().Add(100*time.Millisecond), privateCause)
					want = context.DeadlineExceeded
				case "wrapped-canceled":
					want = context.Canceled
				case "wrapped-deadline":
					want = context.DeadlineExceeded
				}
				defer cancel()
				failure := func() error {
					switch mode {
					case "custom-cancel-cause", "custom-deadline-cause":
						if mode == "custom-cancel-cause" {
							cancel()
						}
						<-ctx.Done()
						return errors.Errorf("synthetic-private-transport: %w", context.Cause(ctx))
					case "wrapped-canceled", "wrapped-deadline":
						return errors.Errorf("synthetic-private-transport: %w", want)
					default:
						return privateCause
					}
				}
				var body *httpFailureBody
				transport := httpRoundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.Header.Get("Authorization") != "Bearer private-test-token" {
						t.Fatal("missing fixture credential")
					}
					if phase == "headers" {
						return nil, failure()
					}
					body = &httpFailureBody{read: func(p []byte) (int, error) {
						return copy(p, "synthetic-private-response"), failure()
					}}
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body}, nil
				})
				c, err := NewHTTP(HTTPConfig{URL: "http://fixture.invalid/private-base", APIKeyEnv: "PROTOCOL_TEST_TOKEN"}, &http.Client{Transport: transport}, func(string) string { return "private-test-token" })
				if err != nil {
					t.Fatal(err)
				}
				status, err := c.JSON(ctx, http.MethodGet, "/private-path", nil, nil)
				assertHTTPErrorRedacted(t, err)
				if body != nil && !body.closed {
					t.Fatal("failed response body was not closed")
				}
				wantStatus := 0
				if phase == "body" {
					wantStatus = http.StatusOK
				}
				if status != wantStatus || errors.Is(err, privateCause) {
					t.Fatalf("raw cause or wrong status: %d %v", status, err)
				}
				for _, sentinel := range []error{context.Canceled, context.DeadlineExceeded} {
					if errors.Is(err, sentinel) != errors.Is(want, sentinel) {
						t.Fatalf("wrong safe sentinel: err=%v want=%v", err, want)
					}
				}
			})
		}
	}
}

func TestHTTPCredentialsRequireExplicitLookup(t *testing.T) {
	t.Setenv("PROTOCOL_TEST_TOKEN", "implicit-environment-secret")
	for _, tc := range []struct {
		name   string
		lookup func(string) string
		want   string
	}{
		{"no lookup", nil, ""},
		{"missing", func(string) string { return "" }, ""},
		{"invalid", func(string) string { return "secret\r\nheader" }, ""},
		{"valid", func(key string) string {
			if key != "PROTOCOL_TEST_TOKEN" {
				t.Fatalf("unexpected credential reference: %q", key)
			}
			return "host-secret"
		}, "host-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewHTTP(HTTPConfig{URL: "https://fixture.invalid", APIKeyEnv: "PROTOCOL_TEST_TOKEN"}, nil, tc.lookup)
			if tc.want == "" {
				if err == nil || err.Error() != "configured protocol API key is missing or invalid" {
					t.Fatalf("missing or invalid credential accepted: %v", err)
				}
				return
			}
			if err != nil || client.bearer != tc.want {
				t.Fatalf("host credential not used: %v", err)
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
			t.Error("missing host credential")
		}
		http.Redirect(w, r, dest.URL, http.StatusTemporaryRedirect)
	}))
	defer s.Close()
	c, e := NewHTTP(HTTPConfig{URL: s.URL, APIKeyEnv: "PROTOCOL_TEST_TOKEN"}, s.Client(), func(string) string { return "private-test-token" })
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.JSON(context.Background(), http.MethodGet, "/quote", nil, nil)
	if e == nil || hits != 0 {
		t.Fatalf("redirect followed: %d, %v", hits, e)
	}
}
func TestHTTPCancelledContextAndCredentialURL(t *testing.T) {
	if _, e := NewHTTP(HTTPConfig{URL: "https://user:password@example.invalid"}, nil, nil); e == nil {
		t.Fatal("credential URL accepted")
	}
	c, e := NewHTTP(HTTPConfig{URL: "https://example.invalid"}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.JSON(ctx, http.MethodGet, "/", nil, nil); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel ignored")
	}
}
