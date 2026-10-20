package telemetry

import (
	"context"
	"github.com/go-errors/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Tracer struct{ name string }

func NewTracer(name string) *Tracer { return &Tracer{name: name} }
func (t *Tracer) Raw() trace.Tracer { return otel.Tracer(t.name) }
func (t *Tracer) Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	ctx, span := t.Raw().Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, func(err error) { EndSpan(span, err) }
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
