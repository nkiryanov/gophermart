package logger

import (
	"log/slog"
	"os"
)

// Constants for logging levels
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Logger interface defines the logging contract
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	With(args ...any) Logger
	WithGroup(name string) Logger
}

// NewLogger creates a new text logger with the specified level
func NewLogger(level string) Logger {
	opts := &slog.HandlerOptions{
		Level:       parseLevelString(level),
		AddSource:   true,
		ReplaceAttr: replace,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &slogLogger{logger: logger}
}

// NewJSONLogger creates a new JSON logger with the specified level
func NewJSONLogger(level string) Logger {
	opts := &slog.HandlerOptions{
		Level:       parseLevelString(level),
		AddSource:   true,
		ReplaceAttr: replace,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &slogLogger{logger: logger}
}

// NewNoOpLogger creates a logger that discards all log messages
func NewNoOpLogger() Logger {
	logger := slog.New(slog.DiscardHandler)
	return &slogLogger{logger: logger}
}
