package internal_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"rypi-dev/logger-server/internal"
	"rypi-dev/logger-server/internal/logger/log_levels"
)

func TestLogEntry_Validate(t *testing.T) {
	// Helper to create a valid LogEntry base
	baseEntry := func() *internal.LogEntry {
		return &internal.LogEntry{
			Level:     "info",
			Message:   "Test message",
			Timestamp: time.Now(),
			Context:   map[string]interface{}{"key": "value"},
		}
	}

	t.Run("valid entry", func(t *testing.T) {
		e := baseEntry()
		err := e.Validate()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if e.Level != "INFO" {
			t.Errorf("expected normalized level INFO, got %s", e.Level)
		}
	})

	t.Run("empty message", func(t *testing.T) {
		e := baseEntry()
		e.Message = "   "
		err := e.Validate()
		if !errors.Is(err, internal.ErrEmptyMessage) {
			t.Errorf("expected ErrEmptyMessage, got %v", err)
		}
	})

	t.Run("message too long", func(t *testing.T) {
		e := baseEntry()
		e.Message = strings.Repeat("x", internal.MaxMessageLength+1)
		err := e.Validate()
		if !errors.Is(err, internal.ErrMessageTooLong) {
			t.Errorf("expected ErrMessageTooLong, got %v", err)
		}
	})

	t.Run("context too many keys", func(t *testing.T) {
		e := baseEntry()
		e.Context = make(map[string]interface{})
		for i := 0; i < internal.MaxContextKeys+1; i++ {
			e.Context[string(rune('a'+i))] = i
		}
		err := e.Validate()
		if !errors.Is(err, internal.ErrContextTooLarge) {
			t.Errorf("expected ErrContextTooLarge, got %v", err)
		}
	})

	t.Run("context too large JSON", func(t *testing.T) {
		e := baseEntry()
		// Create context whose JSON size exceeds MaxContextSizeBytes
		longStr := strings.Repeat("x", internal.MaxContextSizeBytes)
		e.Context = map[string]interface{}{"big": longStr}
		err := e.Validate()
		if err == nil || !strings.Contains(err.Error(), "context too large") {
			t.Errorf("expected context too large error, got %v", err)
		}
	})

	t.Run("empty level", func(t *testing.T) {
		e := baseEntry()
		e.Level = ""
		err := e.Validate()
		if !errors.Is(err, internal.ErrLevelRequired) {
			t.Errorf("expected ErrLevelRequired, got %v", err)
		}
	})

	t.Run("invalid level", func(t *testing.T) {
		e := baseEntry()
		e.Level = "BADLEVEL"
		err := e.Validate()
		if err == nil || !strings.Contains(err.Error(), internal.ErrInvalidLogLevel.Error()) {
			t.Errorf("expected ErrInvalidLogLevel, got %v", err)
		}
	})
}

func TestEnrichLogEntryFromRequest(t *testing.T) {
	baseEntry := &internal.LogEntry{
		Level:     "INFO",
		Message:   "msg",
		Timestamp: time.Now(),
		Context:   nil,
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, internal.ctxKeyTraceID, "trace-123")
	ctx = context.WithValue(ctx, internal.ctxKeyUserAgent, "agent-xyz")

	req := &http.Request{Header: make(http.Header), RequestURI: "/", Method: "GET", Body: nil, URL: nil}
	req = req.WithContext(ctx)

	// Test enrich adds keys if none exist
	entry := internal.EnrichLogEntryFromRequest(req, baseEntry)
	if entry.Context == nil {
		t.Fatal("expected context map initialized")
	}
	if entry.Context["trace_id"] != "trace-123" {
		t.Errorf("expected trace_id 'trace-123', got %v", entry.Context["trace_id"])
	}
	if entry.Context["user_agent"] != "agent-xyz" {
		t.Errorf("expected user_agent 'agent-xyz', got %v", entry.Context["user_agent"])
	}

	// Test enrich with existing context preserves existing keys
	existingCtx := map[string]interface{}{"foo": "bar"}
	entry2 := &internal.LogEntry{
		Level:     "INFO",
		Message:   "msg",
		Timestamp: time.Now(),
		Context:   existingCtx,
	}
	entry2 = internal.EnrichLogEntryFromRequest(req, entry2)
	if entry2.Context["foo"] != "bar" {
		t.Errorf("expected existing context key 'foo' to be preserved")
	}
	if entry2.Context["trace_id"] != "trace-123" {
		t.Errorf("expected trace_id 'trace-123', got %v", entry2.Context["trace_id"])
	}
}