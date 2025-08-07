package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

// ctxKeyTraceID et ctxKeyUserAgent doivent être des types non exportés 
// pour éviter collisions dans le contexte (ex: type string alias ou struct{}).
type ctxKey string

const (
	ctxKeyTraceID  ctxKey = "traceID"
	ctxKeyUserAgent ctxKey = "userAgent"
)

// EnrichLogContext ajoute traceID et userAgent dans le contexte de la requête
func EnrichLogContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = generateUUID()
		}
		userAgent := r.Header.Get("User-Agent")

		ctx := context.WithValue(r.Context(), ctxKeyTraceID, traceID)
		ctx = context.WithValue(ctx, ctxKeyUserAgent, userAgent)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok {
		return v
	}
	return ""
}

func GetUserAgent(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUserAgent).(string); ok {
		return v
	}
	return ""
}


// generateUUID génère un UUID v4 simple
func generateUUID() string {
	// Implémentation simple d'UUID v4
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}