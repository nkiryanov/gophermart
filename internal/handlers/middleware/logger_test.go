package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type loggerFunc func(string, ...any)

func (f loggerFunc) Info(msg string, v ...any) { f(msg, v...) }

func TestLoggerMiddleware(t *testing.T) {
	called := 0
	var msg string
	var args []any

	logger := loggerFunc(func(m string, v ...any) {
		called++
		msg = m
		args = v
	})

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, err := w.Write([]byte("hi"))
		require.NoError(t, err, "should write response")
	})

	middleware := LoggerMiddleware(logger)
	srv := httptest.NewServer(middleware(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err, "should make request to test server")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "should read response body")
	defer resp.Body.Close() // nolint:errcheck

	require.Equalf(t, http.StatusTeapot, resp.StatusCode, "should return status Teapot. Resp: %s", string(body))
	require.Equal(t, "hi", string(body), "should return 'hi' in response")

	require.Equal(t, 1, called, "logger should be called once")
	require.Equal(t, "got HTTP request", msg, "logger should log 'got HTTP request'")
	require.Len(t, args, 10, "logger should log 10 fields")
	require.Equal(t, "method", args[0])
	require.Equal(t, "GET", args[1])
	require.Equal(t, "uri", args[2])
	require.Equal(t, "/test", args[3])
	require.Equal(t, "duration", args[4])
	require.NotEmpty(t, args[5], "duration should not be empty")
	require.Equal(t, "status", args[6])
	require.Equal(t, http.StatusTeapot, args[7])
	require.Equal(t, "size", args[8])
	require.Equal(t, 2, args[9], "size should be 2 (length of 'hi')")
}
