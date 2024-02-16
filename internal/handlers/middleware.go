package handlers

import (
	"net/http"
	"time"

	"github.com/rschio/rinha/internal/web"
	"go.opentelemetry.io/otel/trace"
)

func middlewareWeb(tracer trace.Tracer, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "web")
		defer span.End()

		v := web.Values{
			TraceID: span.SpanContext().TraceID().String(),
			Tracer:  tracer,
			Now:     time.Now().UTC(),
		}
		ctx = web.SetValues(ctx, &v)
		r = r.WithContext(ctx)

		h(w, r)
	})
}
