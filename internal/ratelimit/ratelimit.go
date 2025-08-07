package ratelimit

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"rypi-dev/logger-server/internal/logger/log_levels"
	"rypi-dev/logger-server/internal/utils/utils"

	"github.com/prometheus/client_golang/prometheus"
)

type RateLimiter struct {
	mu            sync.RWMutex
	requests      map[string]*clientData
	maxRequests   int
	maxClients    int
	window        time.Duration
	cleanupTicker *time.Ticker
	quit          chan struct{}
	onceStop      sync.Once

	requestsTotal prometheus.Counter
	blockedTotal  prometheus.Counter
	activeClients prometheus.Gauge

	minLevel       log_levels.LogLevel
	perLevelLimits map[log_levels.LogLevel]int
}

type clientData struct {
	count     int
	firstSeen time.Time
}

// NewRateLimiterWithLevel crée un rate limiter avec seuil minimal de niveau et règles par niveau
func NewRateLimiterWithLevel(maxRequests int, window time.Duration, maxClients int, minLevel log_levels.LogLevel, perLevelLimits map[log_levels.LogLevel]int) (*RateLimiter, error) {
	if err := utils.ValidateMaxRequests(maxRequests); err != nil {
		return nil, err
	}
	if err := utils.ValidateWindow(window); err != nil {
		return nil, err
	}

	rl := &RateLimiter{
		requests:      make(map[string]*clientData),
		maxRequests:   maxRequests,
		maxClients:    maxClients,
		window:        window,
		cleanupTicker: time.NewTicker(5 * time.Minute),
		quit:          make(chan struct{}),
		minLevel:      minLevel,
		perLevelLimits: perLevelLimits,
	}

	rl.initMetrics()

	go rl.cleanupLoop()

	return rl, nil
}

func (rl *RateLimiter) initMetrics() {
	rl.requestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ratelimiter_requests_total",
		Help: "Total number of requests processed",
	})
	rl.blockedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ratelimiter_blocked_total",
		Help: "Total number of requests blocked",
	})
	rl.activeClients = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ratelimiter_active_clients",
		Help: "Current number of active clients",
	})

	// Ignore panic if metrics already registered
	prometheus.MustRegister(rl.requestsTotal, rl.blockedTotal, rl.activeClients)
}

// Middleware applique le rate limit selon niveau log dans header "X-Log-Level"
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.GetClientIP(r)

		levelStr := r.Header.Get("X-Log-Level")
		level := log_levels.NormalizeLogLevel(levelStr)

		if log_levels.LevelLessThan(level, rl.minLevel) {
			// Niveau trop bas, pas de rate limit
			next.ServeHTTP(w, r)
			return
		}

		maxReq := rl.maxRequests
		if rl.perLevelLimits != nil {
			if lvlMax, ok := rl.perLevelLimits[level]; ok {
				maxReq = lvlMax
			}
		}

		allowed, retryAfter := rl.allow(ip, maxReq)
		if !allowed {
			seconds := int(retryAfter.Seconds())
			if seconds < 0 {
				seconds = 0
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string, maxRequests int) (bool, time.Duration) {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Eviction si trop de clients avant d'ajouter
	if !rl.exists(ip) && len(rl.requests) >= rl.maxClients {
		rl.evictOldest()
	}

	client, exists := rl.requests[ip]
	if !exists || now.Sub(client.firstSeen) > rl.window {
		rl.requests[ip] = &clientData{
			count:     1,
			firstSeen: now,
		}
		rl.activeClients.Set(float64(len(rl.requests)))
		rl.requestsTotal.Inc()
		return true, 0
	}

	if client.count >= maxRequests {
		rl.blockedTotal.Inc()
		return false, rl.window - now.Sub(client.firstSeen)
	}

	client.count++
	rl.requestsTotal.Inc()
	return true, 0
}

func (rl *RateLimiter) exists(ip string) bool {
	_, ok := rl.requests[ip]
	return ok
}

// Evict oldest client (appelé avec lock)
func (rl *RateLimiter) evictOldest() {
	if len(rl.requests) <= rl.maxClients {
		return
	}

	var oldestIP string
	var oldestTime time.Time

	for ip, data := range rl.requests {
		if oldestTime.IsZero() || data.firstSeen.Before(oldestTime) {
			oldestIP = ip
			oldestTime = data.firstSeen
		}
	}

	delete(rl.requests, oldestIP)
}

// Cleanup loop pour nettoyage périodique
func (rl *RateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.cleanup()
		case <-rl.quit:
			return
		}
	}
}

// Cleanup supprime les clients expirés et évince les plus vieux si trop nombreux
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, data := range rl.requests {
		if now.Sub(data.firstSeen) > rl.window {
			delete(rl.requests, ip)
		}
	}

	for len(rl.requests) > rl.maxClients {
		rl.evictOldest()
	}

	rl.activeClients.Set(float64(len(rl.requests)))
}

// Stop arrête proprement le nettoyage périodique
func (rl *RateLimiter) Stop() {
	rl.onceStop.Do(func() {
		close(rl.quit)
		rl.cleanupTicker.Stop()
	})
}

// AllowTest expose allow pour les tests
func (rl *RateLimiter) AllowTest(ip string, maxReq int) (bool, time.Duration) {
    return rl.allow(ip, maxReq)  // `allow` en minuscule
}

// CleanupTest permet de déclencher cleanup manuellement dans les tests
func (rl *RateLimiter) CleanupTest() {
	rl.Cleanup()
}

// ClientsSnapshot retourne les clients pour tests
func (rl *RateLimiter) ClientsSnapshot() map[string]*clientData {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	copy := make(map[string]*clientData)
	for k, v := range rl.requests {
		copy[k] = v
	}
	return copy
}

// ClientExists vérifie l'existence d'un client
func (rl *RateLimiter) ClientExists(ip string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	_, exists := rl.requests[ip]
	return exists
}