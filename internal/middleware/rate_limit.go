package middleware

import (
	"net/http"

	"github.com/rypi-dev/logger-server/internal/ratelimit/ratelimit"
)

// RateLimiterMiddleware applique la limitation de débit
func RateLimiterMiddleware(rl *ratelimit.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return rl.Middleware(next)
	}
}