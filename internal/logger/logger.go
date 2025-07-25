package logger

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Constants for logging levels
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"

	EnvDevelopment = "dev"
	EnvProduction  = "prod"
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

// Logger interface implementation using slog
type slogLogger struct {
	logger *slog.Logger
}

// Creates new default logger
// Should be used only on application startup, when logger configuration from cli or environment is not available
func NewDefault() Logger {
	opts := &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		AddSource:   true,
		ReplaceAttr: replace,
	}

	handler := slog.NewTextHandler(os.Stderr, opts) // Write log to stderr as default logger do
	logger := slog.New(handler)

	return &slogLogger{logger: logger}
}

// Creates a new text logger with the specified level
func NewTextLogger(level string) (Logger, error) {
	l, err := parseLevelString(level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:       l,
		AddSource:   true,
		ReplaceAttr: replace,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)

	return &slogLogger{logger: logger}, nil
}

// Creates a new JSON logger with the specified level
func NewJSONLogger(level string) (Logger, error) {
	l, err := parseLevelString(level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:       l,
		AddSource:   true,
		ReplaceAttr: replace,
	}

	handler := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(handler)

	return &slogLogger{logger: logger}, nil
}

// NewNoOpLogger creates a logger that discards all log messages
func NewNoOpLogger() Logger {
	logger := slog.New(slog.DiscardHandler)
	return &slogLogger{logger: logger}
}

func NewDevLogger(level string) (Logger, error) {
	return NewTextLogger(level)
}

func NewProdLogger(level string) (Logger, error) {
	return NewJSONLogger(level)
}

// parseLevelString converts string level to slog.Level, defaults to INFO
func (l *slogLogger) Debug(msg string, args ...any) {
	l.logWithSource(slog.LevelDebug, msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.logWithSource(slog.LevelInfo, msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.logWithSource(slog.LevelWarn, msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.logWithSource(slog.LevelError, msg, args...)
}

// With returns a logger with additional key-value pairs
func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{logger: l.logger.With(args...)}
}

// WithGroup returns a logger with attributes grouped under the given name
func (l *slogLogger) WithGroup(name string) Logger {
	return &slogLogger{logger: l.logger.WithGroup(name)}
}

func parseLevelString(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case LevelDebug:
		return slog.LevelDebug, nil
	case LevelInfo:
		return slog.LevelInfo, nil
	case LevelWarn:
		return slog.LevelWarn, nil
	case LevelError:
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.New("unknown log level")
	}
}

// Remove the directory from the source's filename
// Implementation copy-pasted from https://pkg.go.dev/log/slog@go1.24.5#example-package-Wrapping
func replace(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		source.File = filepath.Base(source.File)
	}

	return a
}

// Log with correct source
// / Implementation inspired by https://pkg.go.dev/log/slog@go1.24.5#example-package-Wrapping
func (l *slogLogger) logWithSource(level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(context.Background(), level) {
		return
	}

	var pc uintptr
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Skip the first 3 frames to get the caller of this function
	pc = pcs[0]

	record := slog.NewRecord(time.Now(), level, msg, pc)
	record.Add(args...)
	_ = l.logger.Handler().Handle(context.Background(), record)
}
