package middleware

import (
	"net/http"
	"time"
)

type logger interface {
	Info(msg string, args ...any)
}

type logData struct {
	responseStatus int
	responseSize   int
}

type logWriter struct {
	http.ResponseWriter
	data logData
}

func (w *logWriter) Write(p []byte) (int, error) {
	size, err := w.ResponseWriter.Write(p)
	w.data.responseSize += size
	return size, err
}

func (w *logWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.data.responseStatus = statusCode
}

func LoggerMiddleware(l logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lw := &logWriter{
				ResponseWriter: w,
				data:           logData{responseStatus: http.StatusOK, responseSize: 0},
			}

			next.ServeHTTP(lw, r)

			l.Info(
				"got HTTP request",
				"method", r.Method,
				"uri", r.RequestURI,
				"duration", time.Since(start),
				"status", lw.data.responseStatus,
				"size", lw.data.responseSize,
			)
		})

	}
}
