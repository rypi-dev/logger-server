package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockRateLimiter permet de simuler le RateLimiter avec une Middleware personnalisée
type mockRateLimiter struct {
	called bool
}

func (m *mockRateLimiter) Middleware(next http.Handler) http.Handler {
	m.called = true
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func TestRateLimiterMiddleware_CallsMiddleware(t *testing.T) {
	mock := &mockRateLimiter{}

	middleware := RateLimiterMiddleware(mock)

	calledNext := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !mock.called {
		t.Error("expected RateLimiter.Middleware to be called")
	}

	if !calledNext {
		t.Error("expected next handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}