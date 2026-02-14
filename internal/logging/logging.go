package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New creates a new slog.Logger using a TextHandler writing to w.
// If w is nil, os.Stderr is used.
func New(w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger stored in the context.
// If none is present, slog.Default() is returned.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if l, ok := v.(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}
