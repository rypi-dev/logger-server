package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"rypi-dev/logger-server/internal/utils/utils"
	"rypi-dev/logger-server/internal/logger/log_levels" // si tu veux valider les niveaux
)

type FileLogger struct {
	mu           sync.Mutex
	file         *os.File
	maxSize      int64
	maxBackups   int
	currSize     int64
	path         string
	totalWritten int64
	totalErrors  int64
}

func NewFileLogger(path string, maxSize int64, maxBackups int) (*FileLogger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &FileLogger{
		file:       f,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		currSize:   info.Size(),
		path:       path,
	}, nil
}

type logEntryJSON struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

func (l *FileLogger) Write(entry LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Vérifie la validité du niveau si nécessaire
	if !log_levels.IsValidLogLevel(entry.Level) {
		l.totalErrors++
		err := fmt.Errorf("invalid log level: %s", entry.Level)
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	ts := entry.Timestamp.Format(utils.TimestampLayout)

	jsonEntry := logEntryJSON{
		Level:     entry.Level,
		Message:   entry.Message,
		Timestamp: ts,
		Context:   entry.Context,
	}

	data, err := json.Marshal(jsonEntry)
	if err != nil {
		l.totalErrors++
		fmt.Fprintf(os.Stderr, "[logger] failed to marshal entry: %v\n", err)
		return err
	}

	data = append(data, '\n')

	if l.currSize+int64(len(data)) > l.maxSize {
		if err := l.rotate(); err != nil {
			l.totalErrors++
			fmt.Fprintf(os.Stderr, "[logger] rotation failed: %v\n", err)
			return err
		}
	}

	n, err := l.file.Write(data)
	if err != nil {
		l.totalErrors++
		fmt.Fprintf(os.Stderr, "[logger] write error: %v\n", err)
		return err
	}

	l.currSize += int64(n)
	l.totalWritten++
	return nil
}

func (l *FileLogger) rotate() error {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return err
		}
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s.%s", l.path, timestamp)
	if err := os.Rename(l.path, backupName); err != nil {
		return err
	}

	dir := filepath.Dir(l.path)
	base := filepath.Base(l.path)
	files, err := os.ReadDir(dir)
	if err == nil {
		var backups []os.DirEntry
		prefix := base + "."
		for _, f := range files {
			if !f.IsDir() && strings.HasPrefix(f.Name(), prefix) {
				backups = append(backups, f)
			}
		}

		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Name() > backups[j].Name()
		})

		if len(backups) > l.maxBackups {
			for i := l.maxBackups; i < len(backups); i++ {
				os.Remove(filepath.Join(dir, backups[i].Name()))
			}
		}
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	l.file = f
	l.currSize = 0
	return nil
}

func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}