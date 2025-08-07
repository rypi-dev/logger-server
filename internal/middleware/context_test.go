package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

// regex simple pour valider un UUID v4 (format hex-hex-4hex-hex-hex)
var uuidV4Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// helper handler qui vérifie traceID et userAgent dans le contexte et écrit OK
func makeContextChecker(t *testing.T, expectedTraceID, expectedUserAgent string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := GetTraceID(r.Context())
		userAgent := GetUserAgent(r.Context())

		if traceID != expectedTraceID {
			t.Errorf("expected traceID %q, got %q", expectedTraceID, traceID)
		}
		if userAgent != expectedUserAgent {
			t.Errorf("expected userAgent %q, got %q", expectedUserAgent, userAgent)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func TestEnrichLogContext(t *testing.T) {
	t.Run("avec X-Trace-ID dans le header", func(t *testing.T) {
		expectedTraceID := "trace-xyz"
		expectedUserAgent := "my-agent"

		handler := makeContextChecker(t, expectedTraceID, expectedUserAgent)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Trace-ID", expectedTraceID)
		req.Header.Set("User-Agent", expectedUserAgent)

		rec := httptest.NewRecorder()
		EnrichLogContext(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 got %d", rec.Code)
		}
	})

	t.Run("sans X-Trace-ID dans le header : UUID v4 généré", func(t *testing.T) {
		expectedUserAgent := "my-agent-2"

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := GetTraceID(r.Context())
			userAgent := GetUserAgent(r.Context())

			if !uuidV4Regex.MatchString(traceID) {
				t.Errorf("expected valid UUID v4, got %q", traceID)
			}
			if userAgent != expectedUserAgent {
				t.Errorf("expected userAgent %q, got %q", expectedUserAgent, userAgent)
			}

			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", expectedUserAgent)

		rec := httptest.NewRecorder()
		EnrichLogContext(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 got %d", rec.Code)
		}
	})

	t.Run("sans X-Trace-ID ni User-Agent dans le header : UUID v4 généré, userAgent vide", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := GetTraceID(r.Context())
			userAgent := GetUserAgent(r.Context())

			if !uuidV4Regex.MatchString(traceID) {
				t.Errorf("expected valid UUID v4, got %q", traceID)
			}
			if userAgent != "" {
				t.Errorf("expected empty userAgent, got %q", userAgent)
			}

			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/", nil) // no headers

		rec := httptest.NewRecorder()
		EnrichLogContext(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 got %d", rec.Code)
		}
	})

	t.Run("GetTraceID retourne vide si pas dans contexte", func(t *testing.T) {
		ctx := context.Background()
		got := GetTraceID(ctx)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("GetUserAgent retourne vide si pas dans contexte", func(t *testing.T) {
		ctx := context.Background()
		got := GetUserAgent(ctx)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}