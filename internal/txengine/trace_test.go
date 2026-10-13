package txengine

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type traceRecorder interface {
	Ended() []sdktrace.ReadOnlySpan
}

func installTraces(tb testing.TB) *tracetest.SpanRecorder {
	tb.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	tb.Cleanup(func() {
		// Background, not tb.Context(): the test's context is already cancelled by cleanup time and
		// Shutdown would abandon the flush.
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(prevProvider)
	})
	return recorder
}

func endedSpan(tb testing.TB, rec traceRecorder, name string) sdktrace.ReadOnlySpan {
	tb.Helper()
	for _, s := range rec.Ended() {
		if s.Name() == name {
			return s
		}
	}
	tb.Fatalf("span %q not ended; ended spans: %v", name, spanNames(rec))
	return nil
}

func spanNames(rec traceRecorder) []string {
	ended := rec.Ended()
	out := make([]string, 0, len(ended))
	for _, s := range ended {
		out = append(out, s.Name())
	}
	return out
}

func spanAttr(s sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.Emit()
		}
	}
	return ""
}

func hasSpanEvent(s sdktrace.ReadOnlySpan, name string) bool {
	for _, e := range s.Events() {
		if e.Name == name {
			return true
		}
	}
	return false
}

func captureLogs(tb testing.TB, verbosity int) (logr.Logger, func() []string) {
	tb.Helper()
	var mu sync.Mutex
	var lines []string
	log := funcr.NewJSON(func(entry string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, entry)
	}, funcr.Options{Verbosity: verbosity})
	return log, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(lines)
	}
}

func requireTraceIDsOnce(tb testing.TB, lines []string) {
	tb.Helper()
	for _, line := range lines {
		for _, key := range []string{`"trace_id"`, `"span_id"`} {
			if n := strings.Count(line, key); n > 1 {
				tb.Fatalf("%s appears %d times in %s", key, n, line)
			}
		}
	}
}
