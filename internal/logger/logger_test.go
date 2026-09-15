package logger_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/logger"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, logger.ParseLevel(tt.input))
		})
	}
}

func TestNew_TextHandler(t *testing.T) {
	buf := new(bytes.Buffer)
	opts := logger.Options{
		Level:  slog.LevelDebug,
		JSON:   false,
		Output: buf,
	}

	l := logger.New(opts)
	require.NotNil(t, l)

	l.Info("hello world", "component", "test")
	assert.Contains(t, buf.String(), "hello world")
	assert.Contains(t, buf.String(), "component=test")
}

func TestNew_JSONHandler(t *testing.T) {
	buf := new(bytes.Buffer)
	opts := logger.Options{
		Level:  slog.LevelInfo,
		JSON:   true,
		Output: buf,
	}

	l := logger.New(opts)
	require.NotNil(t, l)

	l.Info("structured msg", "count", 42)
	assert.Contains(t, buf.String(), `"msg":"structured msg"`)
	assert.Contains(t, buf.String(), `"count":42`)
	assert.Contains(t, buf.String(), `"level":"INFO"`)
}

func TestContextLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	l := logger.New(logger.Options{
		Level:  slog.LevelInfo,
		Output: buf,
	})

	ctx := logger.WithLogger(t.Context(), l)
	extracted := logger.FromContext(ctx)
	assert.Equal(t, l, extracted)

	// Fallback to default when not set in context
	defaultLogger := logger.FromContext(t.Context())
	assert.NotNil(t, defaultLogger)
}
