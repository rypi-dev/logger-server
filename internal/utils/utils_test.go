package utils_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rypi-dev/logger-server/internal/logger/log_levels"
	"rypi-dev/logger-server/internal/utils"
)

func TestSafeParseTimestamp(t *testing.T) {
	now := time.Now()
	valid := now.Format(utils.TimestampLayout)

	// Test valid timestamp
	parsed := utils.SafeParseTimestamp(valid)
	if !parsed.Equal(now) && parsed.Sub(now) > time.Second {
		t.Errorf("expected parsed time close to %v, got %v", now, parsed)
	}

	// Test invalid timestamp returns now (approximate)
	parsed2 := utils.SafeParseTimestamp("invalid")
	if time.Since(parsed2) > time.Second {
		t.Errorf("expected time.Now() on invalid timestamp, got %v", parsed2)
	}
}

func TestMarshalUnmarshalContext(t *testing.T) {
	ctx := map[string]interface{}{
		"user": "bob",
		"id":   123,
	}

	jsonStr, err := utils.MarshalContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonStr, `"user":"bob"`) {
		t.Error("marshal result unexpected")
	}

	// Unmarshal back
	m, err := utils.UnmarshalContext(jsonStr)
	if err != nil {
		t.Fatal(err)
	}
	if m["user"] != "bob" || m["id"].(float64) != 123 {
		t.Error("unmarshal result unexpected")
	}

	// nil map returns empty string, no error
	s, err := utils.MarshalContext(nil)
	if err != nil || s != "" {
		t.Error("MarshalContext with nil should return empty string without error")
	}

	// empty string returns nil map, no error
	m2, err := utils.UnmarshalContext("")
	if err != nil || m2 != nil {
		t.Error("UnmarshalContext with empty string should return nil map without error")
	}

	// invalid JSON returns error
	_, err = utils.UnmarshalContext("{invalid}")
	if err == nil {
		t.Error("expected error unmarshaling invalid JSON")
	}
}

func TestGenerateCleanupQuery(t *testing.T) {
	query := utils.GenerateCleanupQuery()
	if !strings.Contains(query, "DELETE FROM logs") {
		t.Error("unexpected cleanup query")
	}
}

func TestValidatePageLimit(t *testing.T) {
	tests := []struct {
		page, limit int
		wantPage, wantLimit int
		wantErr bool
	}{
		{0, 0, 1, 10, false},
		{1, 1001, 0, 0, true},
		{5, 50, 5, 50, false},
		{3, -1, 3, 10, false},
	}

	for _, tt := range tests {
		p, l, err := utils.ValidatePageLimit(tt.page, tt.limit)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePageLimit(%d, %d) error = %v, wantErr %v", tt.page, tt.limit, err, tt.wantErr)
		}
		if p != tt.wantPage || l != tt.wantLimit {
			t.Errorf("ValidatePageLimit(%d, %d) = (%d, %d), want (%d, %d)", tt.page, tt.limit, p, l, tt.wantPage, tt.wantLimit)
		}
	}
}

func TestGetClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	if ip := utils.GetClientIP(req); ip != "1.1.1.1" {
		t.Errorf("expected 1.1.1.1, got %s", ip)
	}

	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "3.3.3.3")
	if ip := utils.GetClientIP(req); ip != "3.3.3.3" {
		t.Errorf("expected 3.3.3.3, got %s", ip)
	}

	req.Header.Del("X-Real-IP")
	req.RemoteAddr = "4.4.4.4:1234"
	if ip := utils.GetClientIP(req); ip != "4.4.4.4" {
		t.Errorf("expected 4.4.4.4, got %s", ip)
	}

	req.RemoteAddr = "invalid-addr"
	if ip := utils.GetClientIP(req); ip != "invalid-addr" {
		t.Errorf("expected fallback to RemoteAddr, got %s", ip)
	}
}

func TestValidateWindow(t *testing.T) {
	if err := utils.ValidateWindow(0); err == nil {
		t.Error("expected error on zero window")
	}
	if err := utils.ValidateWindow(-1); err == nil {
		t.Error("expected error on negative window")
	}
	if err := utils.ValidateWindow(time.Second); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMaxRequests(t *testing.T) {
	cases := []struct {
		val int
		wantErr bool
	}{
		{0, true},
		{100001, true},
		{1, false},
		{50000, false},
	}

	for _, c := range cases {
		err := utils.ValidateMaxRequests(c.val)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateMaxRequests(%d) error = %v, wantErr %v", c.val, err, c.wantErr)
		}
	}
}

func TestGetAPIKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "  mykey  ")
	if key := utils.GetAPIKey(req); key != "mykey" {
		t.Errorf("expected 'mykey', got '%s'", key)
	}
}

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	utils.WriteJSONError(rr, http.StatusBadRequest, "bad error")

	resp := rr.Result()
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "bad error" {
		t.Errorf("unexpected error message: %v", body)
	}
}

func TestParseAndValidatePageLimit(t *testing.T) {
	tests := []struct {
		pageStr, limitStr string
		wantPage, wantLimit int
		wantErr bool
	}{
		{"", "", 1, 50, false},
		{"2", "20", 2, 20, false},
		{"-1", "10", 0, 0, true},
		{"abc", "10", 0, 0, true},
		{"1", "-1", 1, 1, false},
		{"1", "101", 1, 100, false},
		{"1", "abc", 0, 0, true},
	}

	for _, tt := range tests {
		p, l, err := utils.ParseAndValidatePageLimit(tt.pageStr, tt.limitStr)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseAndValidatePageLimit(%q,%q) error = %v, wantErr %v", tt.pageStr, tt.limitStr, err, tt.wantErr)
		}
		if p != tt.wantPage || l != tt.wantLimit {
			t.Errorf("ParseAndValidatePageLimit(%q,%q) = (%d,%d), want (%d,%d)", tt.pageStr, tt.limitStr, p, l, tt.wantPage, tt.wantLimit)
		}
	}
}

func TestParseQueryParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/?page=2&limit=5&level=INFO", nil)

	params, err := utils.ParseQueryParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if params.Page != 2 {
		t.Errorf("expected page=2, got %d", params.Page)
	}
	if params.Limit != 5 {
		t.Errorf("expected limit=5, got %d", params.Limit)
	}
	if params.LogLevel != "INFO" {
		t.Errorf("expected log level INFO, got %s", params.LogLevel)
	}

	// Invalid level
	req2 := httptest.NewRequest("GET", "/?level=INVALID", nil)
	_, err = utils.ParseQueryParams(req2)
	if err == nil {
		t.Error("expected error on invalid log level")
	}
}

func TestValidateContentTypeJSON(t *testing.T) {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	handler := utils.ValidateContentTypeJSON(next)

	// Test correct Content-Type
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	if !handlerCalled {
		t.Error("expected next handler to be called")
	}

	// Reset
	handlerCalled = false

	// Test wrong Content-Type
	req2 := httptest.NewRequest("POST", "/", nil)
	req2.Header.Set("Content-Type", "text/plain")
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr2, req2)
	if handlerCalled {
		t.Error("expected next handler not to be called on invalid Content-Type")
	}
	if rr2.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected status 415, got %d", rr2.Code)
	}
}

func TestLimitBodySize(t *testing.T) {
	var body bytes.Buffer
	body.WriteString(strings.Repeat("x", 10))
	req := httptest.NewRequest("POST", "/", &body)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Test body size limit by trying to read beyond limit
		data := make([]byte, 20)
		n, err := r.Body.Read(data)
		if err != nil && err.Error() != "EOF" {
			t.Errorf("unexpected read error: %v", err)
		}
		if n > 10 {
			t.Errorf("read more data than expected: %d", n)
		}
	})

	handler := utils.LimitBodySize(5, next) // limit < body length

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected next handler to be called")
	}
}