package audit

import (
	"fmt"
	"net/http"
	"time"

	"rypi-dev/logger-server/internal/logger/log_levels"
	"rypi-dev/logger-server/internal/utils/utils"
)

func AuditEvent(logger LoggerInterface, r *http.Request, level log_levels.LogLevel, message string, status int, extra map[string]interface{}) {
	if logger == nil {
		return
	}

	ctx := map[string]interface{}{
		"client_ip":  utils.GetClientIP(r),
		"method":     r.Method,
		"path":       r.URL.Path,
		"status":     status,
		"user_agent": r.UserAgent(),
	}

	for k, v := range extra {
		ctx[k] = v
	}

	entry := LogEntry{
		Level:     string(level),
		Message:   message,
		Timestamp: time.Now(),
		Context:   ctx,
	}

	if err := logger.Write(entry); err != nil {
		fmt.Printf("⚠️ Audit log failed: %v\n", err)
	}
}