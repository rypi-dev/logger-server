package audit_test

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"rypi-dev/logger-server/internal/audit"
	"rypi-dev/logger-server/internal/logger/log_levels"
)

// mockLogger implémente LoggerInterface pour les tests d'audit
type mockLogger struct {
	wroteEntry  audit.LogEntry
	writeCalled bool
	writeErr    error
}

func (m *mockLogger) Write(entry audit.LogEntry) error {
	m.wroteEntry = entry
	m.writeCalled = true
	return m.writeErr
}

func TestAuditEvent(t *testing.T) {
	mock := &mockLogger{}

	req := httptest.NewRequest("GET", "/test/path?query=1", nil)
	req.Header.Set("User-Agent", "UnitTestAgent")

	extra := map[string]interface{}{
		"custom_key": "custom_value",
	}

	audit.AuditEvent(mock, req, log_levels.Info, "Test audit message", 200, extra)

	if !mock.writeCalled {
		t.Fatal("expected Write to be called on logger")
	}

	entry := mock.wroteEntry

	if entry.Level != string(log_levels.Info) {
		t.Errorf("expected level %s, got %s", log_levels.Info, entry.Level)
	}

	if entry.Message != "Test audit message" {
		t.Errorf("expected message 'Test audit message', got %q", entry.Message)
	}

	if entry.Context["client_ip"] == "" {
		t.Error("expected client_ip in context")
	}

	if entry.Context["method"] != "GET" {
		t.Errorf("expected method GET, got %v", entry.Context["method"])
	}

	if entry.Context["path"] != "/test/path" {
		t.Errorf("expected path /test/path, got %v", entry.Context["path"])
	}

	if entry.Context["status"] != 200 {
		t.Errorf("expected status 200, got %v", entry.Context["status"])
	}

	if entry.Context["user_agent"] != "UnitTestAgent" {
		t.Errorf("expected user_agent UnitTestAgent, got %v", entry.Context["user_agent"])
	}

	if entry.Context["custom_key"] != "custom_value" {
		t.Errorf("expected custom_key custom_value, got %v", entry.Context["custom_key"])
	}

	if time.Since(entry.Timestamp) > time.Second {
		t.Error("timestamp is not recent")
	}
}

func TestAuditEvent_LoggerNil(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AuditEvent panicked with nil logger")
		}
	}()
	audit.AuditEvent(nil, req, log_levels.Info, "message", 200, nil)
}

func TestAuditEvent_WriteError(t *testing.T) {
	mock := &mockLogger{
		writeErr: errors.New("write failed"),
	}

	req := httptest.NewRequest("GET", "/path", nil)
	audit.AuditEvent(mock, req, log_levels.Info, "msg", 200, nil)

	if !mock.writeCalled {
		t.Fatal("expected Write to be called on logger")
	}
}