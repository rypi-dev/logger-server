package logger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"rypi-dev/logger-server/internal/logger/log_levels"
	"rypi-dev/logger-server/internal/utils/utils"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteLogger struct {
	mu              sync.Mutex
	db              *sql.DB
	maxRows         int
	insertStmt      *sql.Stmt
	cleanupInterval time.Duration
	cleanupCtx      context.Context
	cleanupCancel   context.CancelFunc
	minLevel        log_levels.LogLevel
	wg              sync.WaitGroup
}

// NewSQLiteLogger initialise la DB SQLite avec optimisations, crée table/index,
// prépare statement insert, lance goroutine de cleanup périodique si maxRows > 0.
func NewSQLiteLogger(path string, maxRows int, minLevel log_levels.LogLevel, cleanupInterval time.Duration) (*SQLiteLogger, error) {
	// Ajout des paramètres WAL + busy timeout (en ms)
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", path)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// PRAGMA pour optimiser les performances sous charge
	pragmas := []string{
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA temp_store=MEMORY;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %q: %w", p, err)
		}
	}

	// Création table logs si elle n'existe pas
	if _, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		context TEXT
	);`); err != nil {
		db.Close()
		return nil, err
	}

	// Index pour accélérer les requêtes filtrées par level + timestamp DESC
	if _, err = db.Exec(`
	CREATE INDEX IF NOT EXISTS idx_logs_level_timestamp ON logs(level, timestamp DESC);
	`); err != nil {
		db.Close()
		return nil, err
	}

	insertStmt, err := db.Prepare(`
	INSERT INTO logs(level, message, timestamp, context) VALUES (?, ?, ?, ?);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	if cleanupInterval == 0 {
		cleanupInterval = 5 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Normalisation du minLevel dès la création pour éviter erreurs
	minLevel = log_levels.NormalizeLogLevel(string(minLevel))

	logger := &SQLiteLogger{
		db:              db,
		maxRows:         maxRows,
		insertStmt:      insertStmt,
		cleanupInterval: cleanupInterval,
		cleanupCtx:      ctx,
		cleanupCancel:   cancel,
		minLevel:        minLevel,
	}

	if maxRows > 0 {
		logger.wg.Add(1)
		go logger.cleanupLoop()
	}

	return logger, nil
}

func (l *SQLiteLogger) cleanupLoop() {
	defer l.wg.Done()
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.cleanup(); err != nil {
				fmt.Printf("SQLiteLogger cleanup error: %v\n", err)
			}
		case <-l.cleanupCtx.Done():
			return
		}
	}
}

func (l *SQLiteLogger) cleanup() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.maxRows <= 0 {
		return nil
	}

	stmt := utils.GenerateCleanupQuery()
	_, err := l.db.Exec(stmt, l.maxRows)
	return err
}

func (l *SQLiteLogger) QueryLogs(level log_levels.LogLevel, page, limit int) ([]LogEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	page, limit, err := utils.ValidatePageLimit(page, limit)
	if err != nil {
		return nil, err
	}

	if level != "" && !log_levels.IsValidLogLevel(string(level)) {
		return nil, errors.New("invalid log level")
	}

	offset := (page - 1) * limit

	query := `SELECT level, message, timestamp, context FROM logs`
	args := []interface{}{}

	if level != "" {
		query += " WHERE level = ?"
		args = append(args, string(level))
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var entry LogEntry
		var ts string
		var ctxJSON sql.NullString

		if err := rows.Scan(&entry.Level, &entry.Message, &ts, &ctxJSON); err != nil {
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

	return logs, nil
}

func (l *SQLiteLogger) Write(entry LogEntry) error {
	entryLevel := log_levels.NormalizeLogLevel(entry.Level)

	if !log_levels.IsValidLogLevel(string(entryLevel)) {
		l.totalErrors++ // Ajouter un compteur ici si tu veux suivre les erreurs
		return fmt.Errorf("invalid log level: %s", entry.Level)
	}

	if log_levels.LevelLessThan(entryLevel, l.minLevel) {
		// Log trop bas pour être pris en compte
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	ctxJSON, err := utils.MarshalContext(entry.Context)
	if err != nil {
		// Log l’erreur mais n’empêche pas l’entrée d’être enregistrée
		fmt.Printf("context marshal error: %v\n", err)
		ctxJSON = "{}"
	}

	ts := entry.Timestamp.Format(utils.TimestampLayout)

	_, err = l.insertStmt.Exec(string(entryLevel), entry.Message, ts, ctxJSON)
	if err != nil {
		l.totalErrors++
	}
	return err
}

func (l *SQLiteLogger) Close() error {
	l.cleanupCancel()
	l.wg.Wait() // Attend que cleanupLoop soit fini

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