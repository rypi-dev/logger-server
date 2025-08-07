package logger_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rypi-dev/logger-server/internal/logger"
)

func sampleEntry() logger.LogEntry {
	return logger.LogEntry{
		Level:     "INFO",
		Message:   "test",
		Timestamp: time.Now(),
		Context:   map[string]interface{}{},
	}
}

func TestNewFileLogger_Success(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	fl, err := logger.NewFileLogger(logPath, 1024*1024, 3)
	if err != nil {
		t.Fatalf("NewFileLogger error: %v", err)
	}
	defer fl.Close()

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty log file on creation, got size %d", info.Size())
	}
}

func TestNewFileLogger_ErrorInvalidDir(t *testing.T) {
	logPath := "/root/invalid.log"
	_, err := logger.NewFileLogger(logPath, 1024, 1)
	if err == nil {
		t.Error("expected error for invalid directory path")
	}
}

func TestFileLogger_Write_NormalAndError(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "write.log")

	fl, err := logger.NewFileLogger(logPath, 1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()

	entry := sampleEntry()

	err = fl.Write(entry)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if fl.currSize == 0 {
		t.Error("currSize should be > 0 after Write")
	}

	if fl.totalWritten != 1 {
		t.Errorf("expected totalWritten=1, got %d", fl.totalWritten)
	}

	// Vérifier que le fichier contient la ligne écrite
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("log file is empty after Write")
	}

	// Forcer une erreur JSON en passant une structure non sérialisable
	type BadEntry struct {
		F func()
	}

	bad := BadEntry{
		F: func() {},
	}

	err = fl.Write(logger.LogEntry{
		Level:     "ERROR",
		Message:   "bad entry",
		Timestamp: time.Now(),
		Context:   map[string]interface{}{"bad": bad},
	})
	if err == nil {
		t.Error("expected error from Write with bad JSON")
	}

	if fl.totalErrors == 0 {
		t.Error("expected totalErrors to be incremented after JSON marshal error")
	}
}

func TestFileLogger_Write_TriggersRotate(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rotate.log")

	fl, err := logger.NewFileLogger(logPath, 50, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()

	entry := sampleEntry()

	var rotateTriggered bool
	oldRotate := fl.rotate
	fl.rotate = func() error {
		rotateTriggered = true
		return oldRotate()
	}

	for i := 0; i < 10; i++ {
		if err := fl.Write(entry); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if !rotateTriggered {
		t.Error("expected rotation to be triggered")
	}

	if fl.currSize >= fl.maxSize {
		t.Error("expected currSize reset after rotation")
	}
}

func TestFileLogger_Rotate_BackupManagement(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rotate.log")

	fl, err := logger.NewFileLogger(logPath, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()

	entry := sampleEntry()

	if err := fl.Write(entry); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		bkpName := logPath + "." + time.Now().Add(time.Duration(i)*time.Minute).Format("20060102_150405")
		f, err := os.Create(bkpName)
		if err != nil {
			t.Fatalf("failed to create backup file: %v", err)
		}
		f.Close()
	}

	err = fl.rotate()
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var backups []string
	prefix := filepath.Base(logPath) + "."
	for _, f := range files {
		if !f.IsDir() && len(f.Name()) > len(prefix) && f.Name()[:len(prefix)] == prefix {
			backups = append(backups, f.Name())
		}
	}

	if len(backups) > fl.maxBackups {
		t.Errorf("expected max %d backups, got %d", fl.maxBackups, len(backups))
	}
}

func TestFileLogger_Rotate_ErrorRenameAndOpen(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rotate.log")

	fl, err := logger.NewFileLogger(logPath, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()

	entry := sampleEntry()

	if err := fl.Write(entry); err != nil {
		t.Fatal(err)
	}

	// Remplacement temporaire de os.Rename
	oldRename := os.Rename
	defer func() { os.Rename = oldRename }()
	os.Rename = func(oldpath, newpath string) error {
		return errors.New("rename error")
	}

	err = fl.rotate()
	if err == nil {
		t.Error("expected error on rename failure")
	}

	// Restauration de os.Rename
	os.Rename = oldRename

	// Remplacement temporaire de os.OpenFile
	oldOpenFile := os.OpenFile
	defer func() { os.OpenFile = oldOpenFile }()
	os.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, errors.New("openfile error")
	}

	err = fl.rotate()
	if err == nil {
		t.Error("expected error on open file failure")
	}
}

func TestFileLogger_Close(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "close.log")

	fl, err := logger.NewFileLogger(logPath, 1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = fl.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	err = fl.Close()
	if err != nil {
		t.Errorf("Close called twice error: %v", err)
	}
}

func TestFileLogger_CloseAndWrite(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "close_write.log")

	fl, err := logger.NewFileLogger(logPath, 1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := fl.Close(); err != nil {
		t.Fatal(err)
	}

	entry := sampleEntry()
	err = fl.Write(entry)
	if err == nil {
		t.Error("expected error writing after Close")
	}
}

func TestFileLogger_ConcurrentWrite(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "concurrent.log")

	fl, err := logger.NewFileLogger(logPath, 1024*1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()

	entry := sampleEntry()
	const n = 100

	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			if err := fl.Write(entry); err != nil {
				t.Errorf("Write error: %v", err)
			}
		}()
	}

	for i := 0; i < n; i++ {
		<-done
	}

	if fl.totalWritten != int64(n) {
		t.Errorf("expected totalWritten=%d, got %d", n, fl.totalWritten)
	}
}