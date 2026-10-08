package observability

import (
	"context"
	"github.com/go-errors/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"sync/atomic"
)

// This bridge supplies the engine's logging/tracing API without importing the
// vault-solver application's exporters or integration-specific telemetry.
const (
	AttrSolver    = attribute.Key("solver")
	AttrTxLabel   = attribute.Key("tx.label")
	AttrTxHash    = attribute.Key("tx.hash")
	AttrTxNonce   = attribute.Key("tx.nonce")
	AttrTxOutcome = attribute.Key("tx.outcome")
	AttrTxAttempt = attribute.Key("tx.attempt")
)

var enabled atomic.Bool

func SetEnabled(value bool) bool { return enabled.Swap(value) }
func InvalidateTracers()         {} // Raw resolves the global provider on every call.
type Tracer struct{ name string }

func NewTracer(name, _ string) *Tracer { return &Tracer{name: name} }
func (t *Tracer) Raw() trace.Tracer    { return otel.Tracer(t.name) }
func (t *Tracer) Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	return t.StartLinked(ctx, name, nil, attrs...)
}
func (t *Tracer) StartLinked(ctx context.Context, name string, links []trace.Link, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	if !enabled.Load() {
		return ctx, func(error) {}
	}
	ctx, span := t.Raw().Start(ctx, name, trace.WithAttributes(attrs...), trace.WithLinks(links...))
	return withStamped(ctx, span.SpanContext()), func(err error) { EndSpan(span, err) }
}
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
}
func Decline(ctx context.Context, decision, reason string) {
	trace.SpanFromContext(ctx).AddEvent("declined", trace.WithAttributes(attribute.String("decision", decision), attribute.String("reason", reason)))
}
func EndSpan(span trace.Span, err error) {
	if errors.Is(err, context.Canceled) {
		span.AddEvent("cancelled")
	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
