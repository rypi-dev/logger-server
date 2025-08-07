package logger

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
	"github.com/rypi-dev/logger-server/internal/utils/utils"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteAuditLogger struct {
	db         *sql.DB
	insertStmt *sql.Stmt
	mu         sync.RWMutex
	minLevel   log_levels.LogLevel
}

// NewSQLiteAuditLogger crée un logger SQLite pour les audits avec filtrage minLevel.
func NewSQLiteAuditLogger(path string, minLevel log_levels.LogLevel) (*SQLiteAuditLogger, error) {
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		ip TEXT,
		path TEXT,
		status INTEGER,
		context TEXT
	);
	`); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp DESC);`); err != nil {
		db.Close()
		return nil, err
	}

	stmt, err := db.Prepare(`
	INSERT INTO audit_logs(level, message, timestamp, ip, path, status, context)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	minLevel = log_levels.NormalizeLogLevel(string(minLevel))

	return &SQLiteAuditLogger{
		db:         db,
		insertStmt: stmt,
		minLevel:   minLevel,
	}, nil
}

func (l *SQLiteAuditLogger) WriteAudit(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	level := log_levels.NormalizeLogLevel(entry.Level)
	if !log_levels.IsValidLogLevel(string(level)) {
		return fmt.Errorf("invalid log level: %s", entry.Level)
	}

	if log_levels.LevelLessThan(level, l.minLevel) {
		// Niveau trop bas, on ignore
		return nil
	}

	ctxJSON, err := utils.MarshalContext(entry.Context)
	if err != nil {
		// Log erreur JSON sans bloquer l'insertion
		fmt.Printf("failed to marshal audit context: %v\n", err)
		ctxJSON = "{}"
	}

	ts := entry.Timestamp.Format(utils.TimestampLayout)

	_, err = l.insertStmt.Exec(string(level), entry.Message, ts, entry.IP, entry.Path, entry.Status, ctxJSON)
	return err
}

func (l *SQLiteAuditLogger) QueryAuditLogs(level string, page, limit int) ([]AuditEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	page, limit, err := utils.ValidatePageLimit(page, limit)
	if err != nil {
		return nil, err
	}

	if level != "" {
		levelNorm := log_levels.NormalizeLogLevel(level)
		level = string(levelNorm)
		if !log_levels.IsValidLogLevel(level) {
			return nil, errors.New("invalid log level")
		}
	}

	offset := (page - 1) * limit

	query := `SELECT level, message, timestamp, ip, path, status, context FROM audit_logs`
	args := []interface{}{}

	if level != "" {
		query += " WHERE level = ?"
		args = append(args, level)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var ts string
		var ctxJSON sql.NullString

		if err := rows.Scan(&entry.Level, &entry.Message, &ts, &entry.IP, &entry.Path, &entry.Status, &ctxJSON); err != nil {
			return nil, err
		}

		entry.Timestamp = utils.SafeParseTimestamp(ts)

		if ctxJSON.Valid && ctxJSON.String != "" {
			ctx, err := utils.UnmarshalContext(ctxJSON.String)
			if err != nil {
				entry.Context = nil
			} else {
				entry.Context = ctx
			}
		}

		logs = append(logs, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

func (l *SQLiteAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error

	if l.insertStmt != nil {
		if err := l.insertStmt.Close(); err != nil {
			firstErr = err
		}
	}
	if l.db != nil {
		if err := l.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}