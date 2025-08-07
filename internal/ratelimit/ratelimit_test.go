package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rypi-dev/logger-server/internal/logger/log_levels"
	"github.com/rypi-dev/logger-server/internal/middleware/ratelimit"
)

func TestNewRateLimiterWithLevel_ValidConfig(t *testing.T) {
	rl, err := ratelimit.NewRateLimiterWithLevel(5, time.Second, 10, "INFO", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer rl.Stop()
}

func TestMiddleware_AllowRequest(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(5, time.Minute, 10, "INFO", nil)
	defer rl.Stop()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Log-Level", "INFO")
	req.RemoteAddr = "1.2.3.4:1234"

	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}
}

func TestMiddleware_BlockRequest_TooMany(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(2, time.Minute, 10, "INFO", nil)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Log-Level", "INFO")
	req.RemoteAddr = "1.2.3.4:5678"

	// Effectuer 3 requêtes pour dépasser la limite
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestMiddleware_LevelBelowMin_NotLimited(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(1, time.Minute, 10, "INFO", nil)
	defer rl.Stop()

	handlerCalled := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled++
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Log-Level", "DEBUG") // En dessous de INFO
	req.RemoteAddr = "5.6.7.8:9999"

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		rl.Middleware(handler).ServeHTTP(rr, req)
	}

	if handlerCalled != 3 {
		t.Errorf("expected handler to be called 3 times, got %d", handlerCalled)
	}
}

func TestAllow_ResetsAfterWindow(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(1, 100*time.Millisecond, 10, "INFO", nil)
	defer rl.Stop()

	ip := "1.2.3.4"

	// First call should pass
	allowed, _ := rl.AllowTest(ip, 1)
	if !allowed {
		t.Error("expected first call to be allowed")
	}

	// Second call should be blocked
	allowed, _ = rl.AllowTest(ip, 1)
	if allowed {
		t.Error("expected second call to be blocked")
	}

	// Wait until window expires
	time.Sleep(120 * time.Millisecond)

	// Should be allowed again
	allowed, _ = rl.AllowTest(ip, 1)
	if !allowed {
		t.Error("expected call after window to be allowed")
	}
}

func TestMiddleware_PerLevelLimits(t *testing.T) {
	limits := map[log_levels.LogLevel]int{
		"ERROR": 1,
	}
	rl, _ := ratelimit.NewRateLimiterWithLevel(5, time.Minute, 10, "DEBUG", limits)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1111"
	req.Header.Set("X-Log-Level", "ERROR")

	// Première requête OK
	rr := httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Deuxième requête bloquée
	rr = httptest.NewRecorder()
	rl.Middleware(handler).ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}

func TestEviction_WhenMaxClientsExceeded(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(5, time.Minute, 2, "INFO", nil)
	defer rl.Stop()

	ips := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}

	// Dépasser la limite de clients
	for _, ip := range ips {
		rl.AllowTest(ip, 5)
	}

	if len(rl.ClientsSnapshot()) > 2 {
		t.Errorf("expected at most 2 clients, got %d", len(rl.ClientsSnapshot()))
	}
}

func TestCleanup_RemovesExpiredClients(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(5, 50*time.Millisecond, 10, "INFO", nil)
	defer rl.Stop()

	ip := "1.2.3.4"
	rl.AllowTest(ip, 5)

	time.Sleep(100 * time.Millisecond)

	rl.CleanupTest()

	if rl.ClientExists(ip) {
		t.Error("expected client to be removed after cleanup")
	}
}

func TestStop_IsSafe(t *testing.T) {
	rl, _ := ratelimit.NewRateLimiterWithLevel(5, time.Second, 10, "INFO", nil)
	rl.Stop()
	rl.Stop() // Should be safe to call again
}