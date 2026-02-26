package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

// context keys
type ctxKey string

const (
	traceIDKey ctxKey = "trace_id"
	loggerKey  ctxKey = "logger"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// GenerateTraceID produces a random 32‑char hex string.
func GenerateTraceID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

// Middleware returns an HTTP middleware that injects a request-scoped logger.
func Middleware(log Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = GenerateTraceID()
			}
			ctx := context.WithValue(r.Context(), traceIDKey, traceID)
			requestLog := log.With(
				"trace_id", traceID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
			ctx = context.WithValue(ctx, loggerKey, requestLog)
			ww := &responseWriter{ResponseWriter: w}
			defer func() {
				duration := time.Since(start)
				requestLog.Info(ctx, "request completed",
					"status", ww.status,
					"duration_ms", duration.Milliseconds(),
				)
			}()
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

// FromContext retrieves a Logger from context or returns nil.
func FromContext(ctx context.Context) Logger {
	if log, ok := ctx.Value(loggerKey).(Logger); ok {
		return log
	}
	return nil
}
