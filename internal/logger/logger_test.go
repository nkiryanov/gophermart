package logger

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_parseLevelString(t *testing.T) {
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
		{"Unknown level", "UNKNOWN", slog.LevelInfo}, // should default to INFO
		{"Empty string", "", slog.LevelInfo},         // should default to INFO
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLevelString(tc.input)
			assert.Equal(t, tc.expected, got, "parseLevelString(%q) should return %v", tc.input, tc.expected)
		})
	}
}

func TestLogger_NewLogger(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	logger := NewLogger(LevelInfo)
	logger.Info("test message", "key", "value")

	// Restore stdout and read output
	err = w.Close()
	require.NoError(t, err)
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	outputStr := string(output)

	// Check that output contains expected elements
	assert.Contains(t, outputStr, "test message", "Logger output should contain the message")
	assert.Contains(t, outputStr, "key=value", "Logger output should contain key-value pairs")
	assert.Contains(t, outputStr, "INFO", "Logger output should contain log level")
}

func TestLogger_NewJSONLogger(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	logger := NewJSONLogger(LevelInfo)
	logger.Info("test message", "key", "value")

	// Restore stdout and read output
	err = w.Close()
	require.NoError(t, err)
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	// Try to parse as JSON to verify format
	var logEntry map[string]interface{}
	err = json.Unmarshal(output, &logEntry)
	require.NoError(t, err, "JSON logger output should be valid JSON")

	// Check JSON structure
	assert.Equal(t, "test message", logEntry["msg"], "JSON log should contain the message")
	assert.Equal(t, "INFO", logEntry["level"], "JSON log should contain the level")
	assert.Equal(t, "value", logEntry["key"], "JSON log should contain key-value pairs")
}

func TestLogger_NewNoOpLogger(t *testing.T) {
	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	os.Stderr = w

	logger := NewNoOpLogger()
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Restore outputs
	err = w.Close()
	require.NoError(t, err)
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	// NoOp logger should produce no output
	assert.Empty(t, output, "NoOp logger should produce no output")
}

func TestLogger_Levels(t *testing.T) {
	tests := []struct {
		name        string
		loggerLevel string
		logFunction func(Logger)
		shouldLog   bool
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			logger := NewLogger(tc.loggerLevel)
			tc.logFunction(logger)

			// Restore stdout and read output
			err = w.Close()
			require.NoError(t, err)
			os.Stdout = oldStdout

			output, err := io.ReadAll(r)
			require.NoError(t, err)
			hasOutput := len(output) > 0

			assert.Equal(t, tc.shouldLog, hasOutput, "Logger level %s: expected shouldLog=%v, got hasOutput=%v", tc.loggerLevel, tc.shouldLog, hasOutput)
		})
	}
}

func TestLogger_With(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	logger := NewLogger(LevelInfo)
	contextLogger := logger.With("component", "test", "version", "1.0")
	contextLogger.Info("test message")

	// Restore stdout and read output
	err = w.Close()
	require.NoError(t, err)
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	require.NoError(t, err)
	outputStr := string(output)

	// Check that context is included
	assert.Contains(t, outputStr, "component=test", "Logger.With() should add context to log entries")
	assert.Contains(t, outputStr, "version=1.0", "Logger.With() should add all context pairs")
}

func TestLogger_WithGroup(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	logger := NewJSONLogger(LevelInfo) // Use JSON for easier parsing
	groupedLogger := logger.WithGroup("database")
	groupedLogger.Info("connection established", "host", "localhost")

	// Restore stdout and read output
	err = w.Close()
	require.NoError(t, err)
	os.Stdout = oldStdout

	output, err := io.ReadAll(r)
	require.NoError(t, err)

	// Parse JSON to check structure
	var logEntry map[string]interface{}
	err = json.Unmarshal(output, &logEntry)
	require.NoError(t, err, "Failed to parse JSON log")

	// Check that group is created
	database, ok := logEntry["database"].(map[string]interface{})
	require.True(t, ok, "WithGroup should create a group in the log output")
	assert.Equal(t, "localhost", database["host"], "WithGroup should group attributes under the specified name")
}

func TestLoggerChaining(t *testing.T) {
	// Test that With and WithGroup can be chained
	logger := NewNoOpLogger()

	chainedLogger := logger.
		With("service", "auth").
		WithGroup("request").
		With("id", "123")

	assert.NotNil(t, chainedLogger)
	assert.Implements(t, (*Logger)(nil), chainedLogger)

	// Should not panic when logging
	assert.NotPanics(t, func() {
		chainedLogger.Info("chained logger test")
	})
}