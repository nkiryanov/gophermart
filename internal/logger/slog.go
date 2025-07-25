package logger

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// slogLogger implementation of Logger interface based on slog
type slogLogger struct {
	logger *slog.Logger
}

// logWithSource handles correct source information by skipping wrapper frames
func (l *slogLogger) logWithSource(level slog.Level, msg string, args ...any) {
	if !l.logger.Enabled(context.Background(), level) {
		return
	}

	var pc uintptr
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	pc = pcs[0]

	record := slog.NewRecord(time.Now(), level, msg, pc)
	record.Add(args...)
	_ = l.logger.Handler().Handle(context.Background(), record)
}

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

// parseLevelString converts string level to slog.Level, defaults to INFO
func parseLevelString(level string) slog.Level {
	switch strings.ToLower(level) {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// replace removes the directory from the source's filename
func replace(groups []string, a slog.Attr) slog.Attr {
	// Remove the directory from the source's filename.
	// Implementation copy-pasted from https://pkg.go.dev/log/slog@go1.24.5#example-package-Wrapping
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		source.File = filepath.Base(source.File)
	}

	return a
}
