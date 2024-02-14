// Package logger provides a convenience function to constructing a logger
// for use. This is required not just for applications but for testing.
package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/rschio/rinha/internal/web"
)

// New constructs a slog Logger that writes to stdout and
// provides human-readable timestamps.
func New(service string) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
	}
	jh := slog.NewJSONHandler(os.Stdout, &opts)
	return slog.New(withTraceID{Handler: jh}).With("service", service)
}

type withTraceID struct {
	slog.Handler
}

func (h withTraceID) Handle(ctx context.Context, r slog.Record) error {
	r.Add("trace_id", web.GetTraceID(ctx))

	return h.Handler.Handle(ctx, r)
}

func (h withTraceID) WithAttrs(attrs []slog.Attr) slog.Handler {
	hwa := h.Handler.WithAttrs(attrs)
	return withTraceID{Handler: hwa}
}

func (h withTraceID) WithGroup(name string) slog.Handler {
	hwg := h.Handler.WithGroup(name)
	return withTraceID{Handler: hwg}
}

func InfocCtx(ctx context.Context, log *slog.Logger, caller int, msg string, args ...any) {
	if !log.Enabled(ctx, slog.LevelInfo) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(caller, pcs[:]) // skip [Callers, Infof]

	r := slog.NewRecord(time.Now(), slog.LevelInfo, msg, pcs[0])
	r.Add(args...)

	log.Handler().Handle(ctx, r)
}
