package keeper

import (
	"bytes"
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"strings"
	"testing"
)

type failedLoadStore struct {
	Store
	fail bool
}

func (s *failedLoadStore) Load(key string, v any) (bool, error) {
	if s.fail {
		return false, errors.New("secret-provider-payload")
	}
	return s.Store.Load(key, v)
}

type captureLog struct{ buffer *bytes.Buffer }

func (l captureLog) Init(logr.RuntimeInfo)                 {}
func (l captureLog) Enabled(int) bool                      { return true }
func (l captureLog) Info(_ int, msg string, _ ...any)      { l.buffer.WriteString(msg) }
func (l captureLog) Error(err error, msg string, _ ...any) { l.buffer.WriteString(msg + err.Error()) }
func (l captureLog) WithValues(...any) logr.LogSink        { return l }
func (l captureLog) WithName(string) logr.LogSink          { return l }
func TestGlobalStoreFailureIsSanitizedObservedAndRetryable(t *testing.T) {
	k, store, _, sender, _ := recoveryHarness(t)
	failing := &failedLoadStore{Store: store}
	k.Store = failing
	if err := k.CheckRecovery(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	k.Log = logr.New(captureLog{&logs})
	var observations []string
	k.Observe = func(strategy, outcome string) { observations = append(observations, strategy+":"+outcome) }
	failing.fail = true
	if err := k.Tick(context.Background()); err == nil {
		t.Fatal("store failure lost")
	}
	if len(observations) != 1 || observations[0] != "_global:error" || logs.Len() == 0 {
		t.Fatalf("global failure was not observed: %v %q", observations, logs.String())
	}
	if strings.Contains(logs.String(), "secret-provider-payload") || len(sender.requests) != 0 {
		t.Fatal("failure leaked payload or submitted")
	}
	failing.fail = false
	if err := k.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.requests) != 1 || len(observations) != 2 || observations[1] != "cow:ok" {
		t.Fatal("retry did not resume normal strategy processing")
	}
}
