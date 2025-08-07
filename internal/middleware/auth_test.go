package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rypi-dev/logger-server/internal/audit/audit"
	"rypi-dev/logger-server/internal/logger/log_levels"
)

// mockLogger pour capter les appels audit
type mockLogger struct {
	called bool
	entry  audit.LogEntry
}

func (m *mockLogger) Write(entry audit.LogEntry) error {
	m.called = true
	m.entry = entry
	return nil
}

// Helper DRY pour créer les requêtes avec headers
func newRequestWithHeaders(method, url string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestApiKeyMiddleware(t *testing.T) {
	const validKey = "secret123"

	t.Run("valid API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddleware(validKey, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-API-Key": validKey,
		})

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if !handlerCalled {
			t.Fatal("handler should be called when API key is valid")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d got %d", http.StatusOK, rec.Code)
		}
		if logger.called {
			t.Error("logger should not be called on successful auth")
		}
	})

	t.Run("invalid API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddleware(validKey, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-API-Key": "wrongkey",
		})

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if handlerCalled {
			t.Fatal("handler should NOT be called when API key is invalid")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d got %d", http.StatusUnauthorized, rec.Code)
		}
		if !logger.called {
			t.Fatal("logger should be called on failed auth")
		}
		if logger.entry.Message != "Unauthorized access attempt (API key)" {
			t.Errorf("unexpected log message %q", logger.entry.Message)
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddleware(validKey, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})

		req := newRequestWithHeaders("GET", "/", nil) // no headers

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if handlerCalled {
			t.Fatal("handler should NOT be called when API key is missing")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d got %d", http.StatusUnauthorized, rec.Code)
		}
		if !logger.called {
			t.Fatal("logger should be called on missing key")
		}
	})
}

func TestApiKeyMiddlewareWithLevel(t *testing.T) {
	const validKey = "secret123"
	minLevel := log_levels.LogLevelWarn

	t.Run("log level below minLevel, no API key required", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddlewareWithLevel(validKey, minLevel, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-Log-Level": string(log_levels.LogLevelInfo), // Info < Warn
		})

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if !handlerCalled {
			t.Fatal("handler should be called when log level below minLevel")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d got %d", http.StatusOK, rec.Code)
		}
		if logger.called {
			t.Error("logger should NOT be called when no auth required")
		}
	})

	t.Run("log level at or above minLevel with valid API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddlewareWithLevel(validKey, minLevel, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-Log-Level": string(log_levels.LogLevelWarn),
			"X-API-Key":   validKey,
		})

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if !handlerCalled {
			t.Fatal("handler should be called with valid API key and sufficient log level")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d got %d", http.StatusOK, rec.Code)
		}
		if logger.called {
			t.Error("logger should NOT be called on successful auth")
		}
	})

	t.Run("log level at or above minLevel with invalid API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddlewareWithLevel(validKey, minLevel, logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-Log-Level": string(log_levels.LogLevelError),
			"X-API-Key":   "badkey",
		})

		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if handlerCalled {
			t.Fatal("handler should NOT be called with invalid API key")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d got %d", http.StatusUnauthorized, rec.Code)
		}
		if !logger.called {
			t.Fatal("logger should be called on failed auth")
		}
		if logger.entry.Message != "Unauthorized access attempt for high-level log without valid API key" {
			t.Errorf("unexpected log message %q", logger.entry.Message)
		}
	})

	t.Run("malformed X-Log-Level header falls back and requires API key", func(t *testing.T) {
		logger := &mockLogger{}
		mw := ApiKeyMiddlewareWithLevel(validKey, minLevel, logger)

		req := newRequestWithHeaders("GET", "/", map[string]string{
			"X-Log-Level": "UNKNOWN_LEVEL",
			"X-API-Key":   "badkey",
		})
		rec := httptest.NewRecorder()

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
		})

		mw(handler).ServeHTTP(rec, req)

		if handlerCalled {
			t.Fatal("handler should NOT be called with invalid API key on malformed level")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d got %d", http.StatusUnauthorized, rec.Code)
		}
		if !logger.called {
			t.Fatal("logger should be called on failed auth with malformed level")
		}
		if logger.entry.Message != "Unauthorized access attempt for high-level log without valid API key" {
			t.Errorf("unexpected log message %q", logger.entry.Message)
		}

		event, ok := logger.entry.Context["event"]
		if !ok || event != "api_key_check" {
			t.Errorf("expected context[event] to be 'api_key_check', got %v", event)
		}

		requestedLevel, ok := logger.entry.Context["requested_level"]
		if !ok {
			t.Error("expected context[requested_level] to be present")
		} else if requestedLevel != log_levels.LogLevelInfo {
			t.Errorf("expected requested_level to fallback to LogLevelInfo, got %v", requestedLevel)
		}
	})
}