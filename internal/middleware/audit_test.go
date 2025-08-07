package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rypi-dev/logger-server/internal/audit/audit"
	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
)

// mockLogger implémente audit.LoggerInterface pour capter l'appel
type mockLogger struct {
	called bool
	entry  audit.LogEntry
}

func (m *mockLogger) Write(entry audit.LogEntry) error {
	m.called = true
	m.entry = entry
	return nil
}

func TestAuditMiddleware(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		logger := &mockLogger{}

		mw := AuditMiddleware(logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			time.Sleep(10 * time.Millisecond) // simuler une latence
			w.WriteHeader(http.StatusAccepted) // 202
			_, _ = w.Write([]byte("ok"))       // ignorer l'erreur d'écriture volontairement
		})

		req := httptest.NewRequest("GET", "http://example.com/foo", nil)
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req)

		resp := rec.Result()

		if !handlerCalled {
			t.Fatal("handler was not called")
		}

		if resp.StatusCode != http.StatusAccepted {
			t.Errorf("expected status %d got %d", http.StatusAccepted, resp.StatusCode)
		}

		if !logger.called {
			t.Fatal("expected logger.Write to be called")
		}

		if logger.entry.Level != string(log_levels.LogLevelInfo) {
			t.Errorf("expected log level %q got %q", log_levels.LogLevelInfo, logger.entry.Level)
		}

		if logger.entry.Message != "HTTP request completed" {
			t.Errorf("unexpected log message %q", logger.entry.Message)
		}

		if logger.entry.StatusCode != http.StatusAccepted {
			t.Errorf("expected status code %d in log entry, got %d", http.StatusAccepted, logger.entry.StatusCode)
		}

		durationRaw, ok := logger.entry.Context["duration_ms"]
		if !ok {
			t.Error("expected duration_ms in log context")
		} else {
			var duration int64
			switch v := durationRaw.(type) {
			case int64:
				duration = v
			case int:
				duration = int64(v)
			case float64:
				duration = int64(v)
			default:
				t.Errorf("unexpected type for duration_ms: %T", durationRaw)
			}

			if duration <= 0 || duration > 10000 {
				t.Errorf("duration_ms should be positive and less than 10000, got %d", duration)
			}
		}
	})

	t.Run("without logger", func(t *testing.T) {
		mw := AuditMiddleware(nil)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "http://example.com/foo", nil)
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req)

		if !handlerCalled {
			t.Fatal("handler was not called")
		}

		resp := rec.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status %d got %d", http.StatusOK, resp.StatusCode)
		}
	})
}