package middleware

import (
	"net/http"

	"github.com/rypi-dev/logger-server/internal/audit/audit"
	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
	"github.com/rypi-dev/logger-server/internal/utils/utils"
)

// verifyAPIKey vérifie si la clé API est valide
func verifyAPIKey(r *http.Request, validKey string) bool {
	key := utils.GetAPIKey(r)
	return key != "" && key == validKey
}

// ApiKeyMiddleware vérifie la clé API
func ApiKeyMiddleware(validKey string, logger audit.LoggerInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !verifyAPIKey(r, validKey) {
				audit.AuditEvent(logger, r, log_levels.LogLevelWarn, "Unauthorized access attempt (API key)", http.StatusUnauthorized, nil)
				utils.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ApiKeyMiddlewareWithLevel combine clé API + niveau log
func ApiKeyMiddlewareWithLevel(validKey string, minLevel log_levels.Level, logger audit.LoggerInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			levelStr := r.Header.Get("X-Log-Level")
			level := log_levels.NormalizeLogLevel(levelStr)

			// Si niveau trop faible : pas besoin de clé
			if level < minLevel {
				next.ServeHTTP(w, r)
				return
			}

			if !verifyAPIKey(r, validKey) {
				audit.AuditEvent(logger, r, log_levels.LogLevelWarn, "Unauthorized access attempt for high-level log without valid API key", http.StatusUnauthorized, map[string]interface{}{
					"event":           "api_key_check",
					"requested_level": level,
				})
				utils.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}