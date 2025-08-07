package utils

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rypi-dev/logger-server/internal/logger/log_levels"
)

// Format utilisé pour tous les timestamps
const TimestampLayout = time.RFC3339

// SafeParseTimestamp essaie de parser un timestamp; retourne time.Now() en cas d'erreur
func SafeParseTimestamp(ts string) time.Time {
	parsed, err := time.Parse(TimestampLayout, ts)
	if err != nil {
		return time.Now()
	}
	return parsed
}

// MarshalContext sérialise une map vers JSON string
func MarshalContext(ctx map[string]interface{}) (string, error) {
	if ctx == nil {
		return "", nil
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalContext désérialise une string JSON vers une map
func UnmarshalContext(jsonStr string) (map[string]interface{}, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var ctx map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

// GenerateCleanupQuery génère la requête SQL de nettoyage
func GenerateCleanupQuery() string {
	return `
	DELETE FROM logs
	WHERE id NOT IN (
		SELECT id FROM logs ORDER BY id DESC LIMIT ?
	)
	`
}

// ValidatePageLimit s'assure que page/limit sont valides, sinon applique des valeurs par défaut
func ValidatePageLimit(page, limit int) (int, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 1000 {
		return 0, 0, errors.New("limit too large")
	}
	return page, limit, nil
}

// GetClientIP extrait l'IP client en tenant compte des proxies
func GetClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ValidateWindow vérifie qu'une durée est raisonnable (ex : > 0)
func ValidateWindow(window time.Duration) error {
	if window <= 0 {
		return errors.New("window duration must be > 0")
	}
	return nil
}

// ValidateMaxRequests vérifie que maxRequests est dans un intervalle acceptable
func ValidateMaxRequests(max int) error {
	if max < 1 || max > 100000 {
		return errors.New("maxRequests must be between 1 and 100000")
	}
	return nil
}

// GetAPIKey récupère la clé API dans les headers et la nettoie (trim espaces)
func GetAPIKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

// WriteJSONError écrit une erreur en JSON avec status code
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ParseAndValidatePageLimit parse et valide les paramètres page et limit
func ParseAndValidatePageLimit(pageStr, limitStr string) (int, int, error) {
	page := 1
	limit := 50 // valeur par défaut

	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p <= 0 {
			return 0, 0, errors.New("invalid 'page' parameter")
		}
		page = p
	}

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, errors.New("invalid 'limit' parameter")
		}
		if l < 1 {
			limit = 1
		} else if l > 100 {
			limit = 100
		} else {
			limit = l
		}
	}

	return page, limit, nil
}

// QueryParams regroupe les paramètres standards qu'on veut récupérer
type QueryParams struct {
	Page     int
	Limit    int
	LogLevel log_levels.LogLevel
}

// ParseQueryParams parse page, limit et logLevel d'une requête HTTP
func ParseQueryParams(r *http.Request) (*QueryParams, error) {
	page, limit, err := ParseAndValidatePageLimit(r.URL.Query().Get("page"), r.URL.Query().Get("limit"))
	if err != nil {
		return nil, err
	}

	levelStr := r.URL.Query().Get("level")
	level := log_levels.NormalizeLogLevel(levelStr)
	if levelStr != "" && !log_levels.IsValidLogLevel(string(level)) {
		return nil, errors.New("invalid log level")
	}

	return &QueryParams{
		Page:     page,
		Limit:    limit,
		LogLevel: level,
	}, nil
}

// ValidateContentTypeJSON middleware: vérifie Content-Type == application/json
func ValidateContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			WriteJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LimitBodySize middleware: limite la taille du corps de la requête
func LimitBodySize(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

type PaginatedResponse struct {
    Data       interface{} `json:"data"`
    Page       int         `json:"page"`
    Limit      int         `json:"limit"`
    TotalItems int         `json:"total_items"`
    TotalPages int         `json:"total_pages"`
}