package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/rypi-dev/logger-server/internal/audit/audit"
	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
	"github.com/rypi-dev/logger-server/internal/utils/utils"
)

const MaxRequestBodySize = 4096

type Handler struct {
	logger       LoggerInterface
	serverLogger *zap.Logger
}

func NewHandler(logger LoggerInterface, serverLogger *zap.Logger) *Handler {
	return &Handler{
		logger:       logger,
		serverLogger: serverLogger,
	}
}

func (h *Handler) Router() http.Handler {
	r := mux.NewRouter()
	rl, err := ratelimit.NewRateLimiterWithLevel(
		100,
		1*time.Minute,
		1000,
		log_levels.LogLevelInfo,
		map[log_levels.LogLevel]int{
			log_levels.LogLevelError: 200,
			log_levels.LogLevelWarn: 150,
		},
	)
	if err != nil {
		panic(err)
	}

	r.Use(
		middleware.RateLimiterMiddleware(rl),
		middleware.EnrichLogContext,
		middleware.AuditMiddleware(h.logger),
	)

	// Ajout endpoints REST
	r.HandleFunc("/log", h.handleLogs).Methods("POST")      // support Fluent Bit /log
	r.HandleFunc("/log", h.handleGetLogs).Methods("GET")   // récupère les logs
	r.HandleFunc("/log-levels", h.handleGetLogLevels).Methods("GET") // retourne les niveaux

	// Healthcheck
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	return r
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, ip string, status int, msg string, duration time.Duration) {
	utils.WriteJSONError(w, status, msg)
	h.logAudit(ip, r.Method, r.URL.Path, status, duration)
}

func (h *Handler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := utils.GetClientIP(r)

	page := 1
	limit := 50

	if p := r.URL.Query().Get("page"); p != "" {
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 {
			h.writeError(w, r, ip, http.StatusBadRequest, "invalid 'page' parameter", time.Since(start))
			return
		}
		page = v
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil {
			h.writeError(w, r, ip, http.StatusBadRequest, "invalid 'limit' parameter", time.Since(start))
			return
		}
		if v < 1 {
			limit = 1
		} else if v > 100 {
			limit = 100
		} else {
			limit = v
		}
	}

	levelFilter := r.URL.Query().Get("level")
	if levelFilter != "" && !log_levels.IsValidLogLevel(levelFilter) {
		h.writeError(w, r, ip, http.StatusBadRequest, "invalid 'level' parameter", time.Since(start))
		return
	}

	logs, err := h.logger.QueryLogs(levelFilter, page, limit)
	if err != nil {
		h.writeError(w, r, ip, http.StatusInternalServerError, "failed to query logs", time.Since(start))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		h.writeError(w, r, ip, http.StatusInternalServerError, "failed to encode logs", time.Since(start))
		return
	}

	h.logAudit(ip, r.Method, r.URL.Path, http.StatusOK, time.Since(start))
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := utils.GetClientIP(r)

	if r.Header.Get("Content-Type") != "application/json" {
		h.writeError(w, r, ip, http.StatusUnsupportedMediaType, "Content-Type must be application/json", time.Since(start))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, r, ip, http.StatusBadRequest, "invalid body", time.Since(start))
		return
	}

	var entry LogEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		h.writeError(w, r, ip, http.StatusBadRequest, "invalid JSON", time.Since(start))
		return
	}

	if err := entry.Validate(); err != nil {
		h.writeError(w, r, ip, http.StatusBadRequest, err.Error(), time.Since(start))
		return
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	if err := h.logger.Write(entry); err != nil {
		h.writeError(w, r, ip, http.StatusInternalServerError, "failed to write log", time.Since(start))
		return
	}

	// Log de réception (utile en dev/observabilité)
	if h.serverLogger != nil {
		h.serverLogger.Info("Log received",
			zap.String("ip", ip),
			zap.String("service", entry.Service),
			zap.String("level", entry.Level),
			zap.String("message", entry.Message),
		)
	}

	h.logAudit(ip, r.Method, r.URL.Path, http.StatusCreated, time.Since(start))

	// Retour explicite
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "log received",
	})
}

func (h *Handler) handleGetLogLevels(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := utils.GetClientIP(r)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(log_levels.AllLogLevels()); err != nil {
		h.writeError(w, r, ip, http.StatusInternalServerError, "failed to encode log levels", time.Since(start))
		return
	}

	h.logAudit(ip, r.Method, r.URL.Path, http.StatusOK, time.Since(start))
}

func (h *Handler) logAudit(ip, method, path string, status int, duration time.Duration) {
	if h.serverLogger != nil {
		h.serverLogger.Info("HTTP request",
			zap.String("ip", ip),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
		)
	} else {
		fmt.Printf("%s %s %s %d %v\n", ip, method, path, status, duration)
	}
}