package logger

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"os"
	"testing"
)

func capture(t *testing.T, fn func()) (stdout string, stderr string) {
	origOut, origErr := os.Stdout, os.Stderr
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	rOut, wOut, err := os.Pipe()
	require.NoError(t, err, "failed to create stdout pipe")
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err, "failed to create stderr pipe")

	os.Stdout, os.Stderr = wOut, wErr

	fn()

	err = wOut.Close()
	require.NoError(t, err, "failed to close stdout pipe")
	err = wErr.Close()
	require.NoError(t, err, "failed to close stderr pipe")

	outBytes, err := io.ReadAll(rOut)
	require.NoError(t, err, "failed to read stdout pipe")
	errBytes, err := io.ReadAll(rErr)
	require.NoError(t, err, "failed to read stderr pipe")

	return string(outBytes), string(errBytes)
}

func TestLogger_parseLevel(t *testing.T) {
	t.Run("valid value", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected slog.Level
		}{
			{"Debug level", "DEBUG", slog.LevelDebug},
			{"Debug level lowercase", "debug", slog.LevelDebug},
			{"Info level", "INFO", slog.LevelInfo},
			{"Info level lowercase", "info", slog.LevelInfo},
			{"Warn level", "WARN", slog.LevelWarn},
			{"Warn level lowercase", "warn", slog.LevelWarn},
			{"Error level", "ERROR", slog.LevelError},
			{"Error level lowercase", "error", slog.LevelError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := parseLevel(tt.input)

				require.NoError(t, err, "parseLevelString(%q) should not return an error", tt.input)
				require.Equal(t, tt.expected, got, "parseLevelString(%q) should return %v", tt.input, tt.expected)
			})
		}
	})

	t.Run("not valid", func(t *testing.T) {
		tests := []struct {
			name  string
			value string
		}{
			{
				name:  "empty level",
				value: "",
			},
			{
				name:  "unknown level",
				value: "uknown",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := parseLevel(tt.value)

				require.Error(t, err)
			})
		}
	})
}

func TestLogger_NewTextLogger(t *testing.T) {
	stdout, stderr := capture(t, func() {
		logger, err := NewTextLogger(LevelInfo)
		require.NoError(t, err)

		logger.Info("test message", "key", "value")
	})

	require.Empty(t, stdout, "Text logger should not write to stderr by default")
	require.NotEmpty(t, stderr, "Text logger should write to stderr")

	require.Contains(t, stderr, "test message")
	require.Contains(t, stderr, "key=value")
	require.Contains(t, stderr, "INFO")

}

func TestLogger_NewJSONLogger(t *testing.T) {
	stdout, stderr := capture(t, func() {
		logger, err := NewJSONLogger(LevelInfo)
		require.NoError(t, err, "NewJSONLogger should not return an error")

		logger.Info("test message", "key", "value")
	})

	require.Empty(t, stdout, "JSON logger should not write to stdout by default")
	require.NotEmpty(t, stderr, "JSON logger should write to stderr")

	var entry map[string]any
	err := json.Unmarshal([]byte(stderr), &entry)
	require.NoError(t, err, "JSON log should be valid")
	require.Equal(t, "test message", entry["msg"], "JSON log should contain the message")
	require.Equal(t, "INFO", entry["level"], "JSON log should contain the level")
	require.Equal(t, "value", entry["key"], "JSON log should contain key-value pairs")
}

func TestLogger_NewNoOpLogger(t *testing.T) {
	stdout, stderr := capture(t, func() {
		logger := NewNoOpLogger()
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")
	})

	require.Empty(t, stdout, "NoOp logger should not write to stdout")
	require.Empty(t, stderr, "NoOp logger should not write to stderr")
}

func TestLogger_Levels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		logFn    func(Logger)
		isLogged bool
	}{
		{"Debug logger logs debug", LevelDebug, func(l Logger) { l.Debug("test") }, true},
		{"Debug logger logs info", LevelDebug, func(l Logger) { l.Info("test") }, true},
		{"Debug logger logs warn", LevelDebug, func(l Logger) { l.Warn("test") }, true},
		{"Debug logger logs error", LevelDebug, func(l Logger) { l.Error("test") }, true},

		{"Info logger skips debug", LevelInfo, func(l Logger) { l.Debug("test") }, false},
		{"Info logger logs info", LevelInfo, func(l Logger) { l.Info("test") }, true},
		{"Info logger logs warn", LevelInfo, func(l Logger) { l.Warn("test") }, true},
		{"Info logger logs error", LevelInfo, func(l Logger) { l.Error("test") }, true},

		{"Warn logger skips debug", LevelWarn, func(l Logger) { l.Debug("test") }, false},
		{"Warn logger skips info", LevelWarn, func(l Logger) { l.Info("test") }, false},
		{"Warn logger logs warn", LevelWarn, func(l Logger) { l.Warn("test") }, true},
		{"Warn logger logs error", LevelWarn, func(l Logger) { l.Error("test") }, true},

		{"Error logger skips debug", LevelError, func(l Logger) { l.Debug("test") }, false},
		{"Error logger skips info", LevelError, func(l Logger) { l.Info("test") }, false},
		{"Error logger skips warn", LevelError, func(l Logger) { l.Warn("test") }, false},
		{"Error logger logs error", LevelError, func(l Logger) { l.Error("test") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := capture(t, func() {
				logger, err := NewTextLogger(tt.level)
				require.NoError(t, err, "NewLogger should not return an error")

				tt.logFn(logger)
			})

			hasStderrLog := len(stderr) > 0
			require.Empty(t, stdout, "Logger should not write to stdout")
			require.Equal(t, tt.isLogged, hasStderrLog, "Logger level %s: expected isLogged=%v, got hasStderrLog=%v", tt.level, tt.isLogged, hasStderrLog)
		})
	}
}

func TestLogger_With(t *testing.T) {
	stdout, stderr := capture(t, func() {
		logger, err := NewTextLogger(LevelInfo)
		require.NoError(t, err, "NewTextLogger should not return an error")

		withLogger := logger.With("component", "test", "version", "1.0")

		withLogger.Info("test message")
	})

	require.Empty(t, stdout, "Logger.With() should not write to stdout")
	require.NotEmpty(t, stderr, "Logger.With() should write to stderr")

	require.Contains(t, stderr, "component=test")
	require.Contains(t, stderr, "version=1.0")
	require.Contains(t, stderr, "test message")
}
