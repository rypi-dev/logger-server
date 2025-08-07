package internal

import (
	"encoding/json"
	"net/http"
	"errors"
	"fmt"
	"strings"
	"time"
	"rypi-dev/logger-server/internal/logger/log_levels"
)

// LogEntry représente une entrée de log reçue par l'API.
//
// example:
// {
//   "level": "INFO",
//   "message": "User logged in",
//   "timestamp": "2025-08-06T14:12:00Z",
//   "context": {"user_id": 42}
// }
type LogEntry struct {
	Level     string                 `json:"level" example:"INFO"`                           // Niveau de log
	Message   string                 `json:"message" example:"User logged in"`               // Message de log
	Timestamp time.Time              `json:"timestamp" example:"2025-08-06T14:12:00Z"`       // Timestamp RFC3339
	Context   map[string]interface{} `json:"context,omitempty" example:"{\"user_id\": 42}"` // Données additionnelles
}

type ctxKey string

type LoggerInterface interface {
	Write(entry LogEntry) error
	QueryLogs(level log_levels.LogLevel, page, limit int) ([]LogEntry, error)
}

const (
	MaxMessageLength 	= 1024
	MaxContextSizeBytes = 2048
	MaxContextKeys   	= 10
	DefaultLogLevel  	= "INFO"
	ctxKeyTraceID   ctxKey = "traceID"
	ctxKeyUserAgent ctxKey = "userAgent"
)

var (
    ErrInvalidLogLevel = errors.New("invalid log level")
    ErrEmptyMessage    = errors.New("message is required")
    ErrMessageTooLong  = errors.New("message too long")
    ErrContextTooLarge = errors.New("context too large")
    ErrLevelRequired   = errors.New("level is required")
)

// Validate vérifie que l'entrée de log respecte les contraintes de format.
func (e *LogEntry) Validate() error {
	if strings.TrimSpace(e.Message) == "" {
		return ErrEmptyMessage
	}
	if len(e.Message) > MaxMessageLength {
		return ErrMessageTooLong
	}
	if e.Context != nil {
		if len(e.Context) > MaxContextKeys {
			return ErrContextTooLarge
		}
		contextBytes, err := json.Marshal(e.Context)
		if err != nil {
			return fmt.Errorf("invalid context JSON: %w", err)
		}
		if len(contextBytes) > MaxContextSizeBytes {
			return fmt.Errorf("context too large: exceeds %d bytes", MaxContextSizeBytes)
		}
	}
	if e.Level == "" {
		return ErrLevelRequired
	}
	levelNorm := log_levels.NormalizeLogLevel(e.Level)
	if !log_levels.IsValidLogLevel(string(levelNorm)) {
		return fmt.Errorf("%w: %s", ErrInvalidLogLevel, e.Level)
	}
	// Optionnel : mettre à jour e.Level seulement si tu veux normaliser ici
	e.Level = string(levelNorm)
	return nil
}

// EnrichLogEntryFromRequest enrichit une LogEntry avec traceID et userAgent du contexte HTTP
func EnrichLogEntryFromRequest(r *http.Request, entry *LogEntry) *LogEntry {
	if entry.Context == nil {
		entry.Context = make(map[string]interface{})
	}

	traceID, ok := r.Context().Value(ctxKeyTraceID).(string)
	if ok && traceID != "" {
		entry.Context["trace_id"] = traceID
	}

	userAgent, ok := r.Context().Value(ctxKeyUserAgent).(string)
	if ok && userAgent != "" {
		entry.Context["user_agent"] = userAgent
	}

	return entry
}