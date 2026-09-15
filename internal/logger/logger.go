package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type contextKey struct{}

// Options configures the structured logger.
type Options struct {
	Level     slog.Level
	JSON      bool
	Output    io.Writer
	AddSource bool
}

// ParseLevel converts a string representation of log level into slog.Level.
func ParseLevel(lvl string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New creates a configured *slog.Logger.
func New(opts Options) *slog.Logger {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     opts.Level,
		AddSource: opts.AddSource,
	}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(out, handlerOpts)
	} else {
		handler = slog.NewTextHandler(out, handlerOpts)
	}

	return slog.New(handler)
}

// WithLogger returns a new context containing the provided logger.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext extracts the *slog.Logger from the context or returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

// InitDefault sets the global standard slog logger with the given options.
func InitDefault(opts Options) *slog.Logger {
	l := New(opts)
	slog.SetDefault(l)
	return l
}
