package logger_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"rypi-dev/logger-server/internal/logger/logger"
	"rypi-dev/logger-server/internal/utils/utils"
)

// Sample audit entry for tests
func sampleAuditEntry() logger.AuditEntry {
	return logger.AuditEntry{
		Level:     "INFO",
		Message:   "user login",
		Timestamp: time.Now().Truncate(time.Second),
		IP:        "127.0.0.1",
		Path:      "/login",
		Status:    200,
		Context: map[string]interface{}{
			"user": "john",
		},
	}
}

func TestNewSQLiteAuditLogger_CreatesTable(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer l.Close()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("expected db file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected db file not to be empty")
	}
}

func TestWriteAudit_Success(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleAuditEntry()

	if err := l.WriteAudit(entry); err != nil {
		t.Errorf("WriteAudit returned error: %v", err)
	}
}

func TestWriteAudit_InvalidLevel(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleAuditEntry()
	entry.Level = "INVALID_LEVEL"

	if err := l.WriteAudit(entry); err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestWriteAudit_BadContext(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleAuditEntry()
	entry.Context = map[string]interface{}{
		"bad": func() {}, // Non serializable
	}

	if err := l.WriteAudit(entry); err == nil {
		t.Error("expected error for invalid context JSON")
	}
}

func TestQueryAuditLogs_Success(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	for i := 0; i < 5; i++ {
		e := sampleAuditEntry()
		e.Message = "msg " + string(rune('A'+i))
		if err := l.WriteAudit(e); err != nil {
			t.Fatalf("failed to write entry: %v", err)
		}
	}

	results, err := l.QueryAuditLogs("INFO", 1, 10)
	if err != nil {
		t.Errorf("QueryAuditLogs returned error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestQueryAuditLogs_InvalidLevel(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, err = l.QueryAuditLogs("WRONG", 1, 10)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestQueryAuditLogs_InvalidPagination(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, err = l.QueryAuditLogs("INFO", 0, -5)
	if err == nil {
		t.Error("expected error for invalid pagination values")
	}
}

func TestCloseSQLiteAuditLogger(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "audit.db")

	l, err := logger.NewSQLiteAuditLogger(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Errorf("second Close returned error: %v", err)
	}
}