package web

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type ctxKey int

const key ctxKey = 1

// Values represent state for each request.
type Values struct {
	TraceID string
	Tracer  trace.Tracer
	Now     time.Time
}

// GetValues returns the values from the context.
func GetValues(ctx context.Context) *Values {
	v, ok := ctx.Value(key).(*Values)
	if !ok {
		return &Values{
			TraceID: trace.TraceID{}.String(),
			Tracer:  noop.NewTracerProvider().Tracer(""),
			Now:     time.Now().UTC(),
		}
	}

	return v
}

// SetValues set web values to the context.
func SetValues(ctx context.Context, v *Values) context.Context {
	return context.WithValue(ctx, key, v)
}

// GetTraceID returns the trace id from the context.
func GetTraceID(ctx context.Context) string {
	v, ok := ctx.Value(key).(*Values)
	if !ok {
		return trace.TraceID{}.String()
	}
	return v.TraceID
}

func GetTime(ctx context.Context) time.Time {
	v, ok := ctx.Value(key).(*Values)
	if !ok {
		return time.Now().UTC()
	}
	return v.Now
}

// AddSpan adds a OpenTelemetry span to the trace and context.
func AddSpan(ctx context.Context, spanName string, keyValues ...attribute.KeyValue) (context.Context, trace.Span) {
	v, ok := ctx.Value(key).(*Values)
	if !ok || v.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := v.Tracer.Start(ctx, spanName)
	for _, kv := range keyValues {
		span.SetAttributes(kv)
	}

	return ctx, span
}
