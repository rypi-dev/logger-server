package logger_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rypi-dev/logger-server/internal/logger/logger"
	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
)

func sampleLogEntry(level string) logger.LogEntry {
	return logger.LogEntry{
		Level:     level,
		Message:   "something happened",
		Timestamp: time.Now().Truncate(time.Second),
		Context: map[string]interface{}{
			"user": "alice",
		},
	}
}

func TestNewSQLiteLogger_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer l.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("DB file not created: %v", err)
	}
}

func TestSQLiteLogger_WriteAndQuery_Success(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleLogEntry("INFO")
	if err := l.Write(entry); err != nil {
		t.Errorf("Write failed: %v", err)
	}

	results, err := l.QueryLogs("INFO", 1, 10)
	if err != nil {
		t.Errorf("QueryLogs failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestSQLiteLogger_Write_BelowMinLevel(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "WARN", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleLogEntry("DEBUG") // DEBUG < WARN

	err = l.Write(entry)
	if err == nil {
		t.Error("expected error for log below minLevel")
	}
}

func TestSQLiteLogger_Write_InvalidLevel(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleLogEntry("WRONGLEVEL")
	err = l.Write(entry)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestSQLiteLogger_Write_BadContext(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	entry := sampleLogEntry("INFO")
	entry.Context = map[string]interface{}{"f": func() {}} // not serializable

	err = l.Write(entry)
	if err == nil {
		t.Error("expected error for bad context")
	}
}

func TestSQLiteLogger_QueryLogs_InvalidLevel(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, err = l.QueryLogs("BADLEVEL", 1, 10)
	if err == nil {
		t.Error("expected error for invalid query level")
	}
}

func TestSQLiteLogger_QueryLogs_InvalidPagination(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, err = l.QueryLogs("INFO", 0, -10)
	if err == nil {
		t.Error("expected error for invalid pagination")
	}
}

func TestSQLiteLogger_Cleanup_RemovesOldLogs(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	// maxRows = 5, interval très court
	l, err := logger.NewSQLiteLogger(dbPath, 5, "DEBUG", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Insère 10 logs rapidement
	for i := 0; i < 10; i++ {
		entry := sampleLogEntry("INFO")
		entry.Message = "log " + string(rune(i+'0'))
		l.Write(entry)
	}

	// Attend que cleanup ait le temps de tourner
	time.Sleep(500 * time.Millisecond)

	results, err := l.QueryLogs("INFO", 1, 20)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}
	if len(results) > 5 {
		t.Errorf("Expected max 5 logs after cleanup, got %d", len(results))
	}
}

func TestSQLiteLogger_Close_IsSafeTwice(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "logs.db")

	l, err := logger.NewSQLiteLogger(dbPath, 0, "INFO", 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("Second Close error: %v", err)
	}
}