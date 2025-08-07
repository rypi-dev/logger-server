package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rypi-dev/logger-server/internal/handler"
	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
	"go.uber.org/zap"
)

// mockLogger implémente LoggerInterface pour les tests
type mockLogger struct {
	logs      []handler.LogEntry
	queryFunc func(level string, page, limit int) ([]handler.LogEntry, error)
	writeFunc func(entry handler.LogEntry) error
}

func (m *mockLogger) QueryLogs(level string, page, limit int) ([]handler.LogEntry, error) {
	if m.queryFunc != nil {
		return m.queryFunc(level, page, limit)
	}
	return m.logs, nil
}

func (m *mockLogger) Write(entry handler.LogEntry) error {
	if m.writeFunc != nil {
		return m.writeFunc(entry)
	}
	m.logs = append(m.logs, entry)
	return nil
}

// helper pour décoder la réponse JSON d’erreur
func decodeErrorResponse(t *testing.T, body *bytes.Buffer) string {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response JSON: %v", err)
	}
	msg, ok := resp["error"].(string)
	if !ok {
		t.Fatalf("response JSON does not contain 'error' field as string")
	}
	return msg
}

func TestHandleGetLogs(t *testing.T) {
	mock := &mockLogger{
		queryFunc: func(level string, page, limit int) ([]handler.LogEntry, error) {
			return []handler.LogEntry{
				{Message: "log1", Level: "info", Timestamp: time.Now()},
			}, nil
		},
	}
	h := handler.NewHandler(mock, zap.NewNop())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantErrMsg string
	}{
		{"Valid params", "?page=1&limit=10&level=info", http.StatusOK, ""},
		{"Invalid page param", "?page=abc", http.StatusBadRequest, "invalid 'page' parameter"},
		{"Invalid limit param", "?limit=abc", http.StatusBadRequest, "invalid 'limit' parameter"},
		{"Invalid level param", "?level=invalid", http.StatusBadRequest, "invalid 'level' parameter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/logs"+tt.query, nil)
			w := httptest.NewRecorder()

			h.Router().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus == http.StatusOK {
				// Content-Type JSON check
				if ct := w.Header().Get("Content-Type"); ct != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", ct)
				}

				// Decode logs slice
				var logs []handler.LogEntry
				if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
					t.Errorf("failed to decode logs JSON: %v", err)
				}
				if len(logs) == 0 {
					t.Errorf("expected logs array to be non-empty")
				}
			} else {
				errMsg := decodeErrorResponse(t, w.Body)
				if errMsg != tt.wantErrMsg {
					t.Errorf("expected error message %q, got %q", tt.wantErrMsg, errMsg)
				}
			}
		})
	}
}

func TestHandleGetLogs_InternalError(t *testing.T) {
	mock := &mockLogger{
		queryFunc: func(level string, page, limit int) ([]handler.LogEntry, error) {
			return nil, errors.New("internal error")
		},
	}
	h := handler.NewHandler(mock, zap.NewNop())

	req := httptest.NewRequest("GET", "/logs", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	errMsg := decodeErrorResponse(t, w.Body)
	if errMsg != "failed to query logs" {
		t.Errorf("expected error message 'failed to query logs', got %q", errMsg)
	}
}

func TestHandleLogs(t *testing.T) {
	mock := &mockLogger{
		writeFunc: func(entry handler.LogEntry) error {
			return nil
		},
	}
	h := handler.NewHandler(mock, zap.NewNop())

	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantStatus  int
		wantErrMsg  string
	}{
		{
			name:        "Valid log entry",
			contentType: "application/json",
			body:        []byte(`{"level":"info","message":"test log","timestamp":"2025-08-07T12:00:00Z"}`),
			wantStatus:  http.StatusNoContent,
		},
		{
			name:        "Missing Content-Type",
			contentType: "",
			body:        []byte(`{"level":"info","message":"test log"}`),
			wantStatus:  http.StatusUnsupportedMediaType,
			wantErrMsg:  "Content-Type must be application/json",
		},
		{
			name:        "Invalid JSON",
			contentType: "application/json",
			body:        []byte(`{invalid json}`),
			wantStatus:  http.StatusBadRequest,
			wantErrMsg:  "invalid JSON",
		},
		{
			name:        "Invalid log entry validation",
			contentType: "application/json",
			body:        []byte(`{"level":"","message":""}`), // Suppose Validate() rejects empty level/message
			wantStatus:  http.StatusBadRequest,
			wantErrMsg:  "", // On veut juste vérifier qu’il y a une erreur, le message peut varier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/logs", bytes.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			h.Router().ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantStatus != http.StatusNoContent {
				errMsg := decodeErrorResponse(t, w.Body)
				if tt.wantErrMsg != "" && errMsg != tt.wantErrMsg {
					t.Errorf("expected error message %q, got %q", tt.wantErrMsg, errMsg)
				}
			}
		})
	}
}

func TestHandleLogs_WriteError(t *testing.T) {
	mock := &mockLogger{
		writeFunc: func(entry handler.LogEntry) error {
			return errors.New("write failed")
		},
	}
	h := handler.NewHandler(mock, zap.NewNop())

	body := []byte(`{"level":"info","message":"test log"}`)
	req := httptest.NewRequest("POST", "/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	errMsg := decodeErrorResponse(t, w.Body)
	if errMsg != "failed to write log" {
		t.Errorf("expected error message 'failed to write log', got %q", errMsg)
	}
}

func TestHandleGetLogLevels(t *testing.T) {
	mock := &mockLogger{}
	h := handler.NewHandler(mock, zap.NewNop())

	req := httptest.NewRequest("GET", "/log-levels", nil)
	w := httptest.NewRecorder()

	h.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var levels []string
	if err := json.NewDecoder(w.Body).Decode(&levels); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(levels) == 0 {
		t.Errorf("expected at least one log level")
	}
}