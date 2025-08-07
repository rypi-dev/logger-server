package middleware

import (
	"net/http"
	"time"

	"rypi-dev/logger-server/internal/audit/audit"
	"rypi-dev/logger-server/internal/logger/log_levels"
)

// ResponseWriterWrapper permet de capturer le status code HTTP
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
}

func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// AuditMiddleware crée un middleware HTTP qui audit chaque requête
func AuditMiddleware(logger audit.LoggerInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

			next.ServeHTTP(wrappedWriter, r)

			duration := time.Since(start)

			if logger != nil {
				audit.AuditEvent(logger, r, log_levels.LogLevelInfo, "HTTP request completed", wrappedWriter.StatusCode, map[string]interface{}{
					"duration_ms": duration.Milliseconds(),
				})
			}
		})
	}
}