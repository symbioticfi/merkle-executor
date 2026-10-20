package telemetry

import (
	"context"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
)

// WithLogger stores the base logger so each nested span receives its own IDs.
func WithLogger(ctx context.Context, log logr.Logger) context.Context {
	return logr.NewContext(ctx, log)
}

// Log adds the active span's IDs to the context logger at retrieval.
func Log(ctx context.Context) logr.Logger {
	log := logr.FromContextOrDiscard(ctx)
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return log.WithValues("trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	return log
}
