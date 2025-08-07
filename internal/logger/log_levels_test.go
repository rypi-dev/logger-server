package logger_test

import (
	"testing"

	"github.com/rypi-dev/logger-server/internal/logger"
)

func TestIsValidLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"TRACE", true},
		{"trace", true},
		{"TrAcE", true},
		{"debug", true},
		{"INFO", true},
		{"warn", true},
		{"ERROR", true},
		{"fatal", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := logger.IsValidLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("IsValidLogLevel(%q) = %v; want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  logger.LogLevel
	}{
		{"trace", logger.LogLevelTrace},
		{"TRACE", logger.LogLevelTrace},
		{"TrAcE", logger.LogLevelTrace},
		{"debug", logger.LogLevelDebug},
		{"info", logger.LogLevelInfo},
		{"warn", logger.LogLevelWarn},
		{"error", logger.LogLevelError},
		{"fatal", logger.LogLevelFatal},
		{"invalid", logger.LogLevel("INVALID")}, // normalise quand même en majuscule
		{"", logger.LogLevel("")},
	}

	for _, tt := range tests {
		got := logger.NormalizeLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLogLevel(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestLevelLessThan(t *testing.T) {
	tests := []struct {
		a, b logger.LogLevel
		want bool
	}{
		{logger.LogLevelTrace, logger.LogLevelDebug, true},
		{logger.LogLevelDebug, logger.LogLevelTrace, false},
		{logger.LogLevelInfo, logger.LogLevelWarn, true},
		{logger.LogLevelError, logger.LogLevelFatal, true},
		{logger.LogLevelFatal, logger.LogLevelFatal, false},
		{logger.LogLevelWarn, logger.LogLevelInfo, false},
	}

	for _, tt := range tests {
		got := logger.LevelLessThan(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("LevelLessThan(%q, %q) = %v; want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAllLogLevels(t *testing.T) {
	want := []logger.LogLevel{
		logger.LogLevelTrace,
		logger.LogLevelDebug,
		logger.LogLevelInfo,
		logger.LogLevelWarn,
		logger.LogLevelError,
		logger.LogLevelFatal,
	}

	got := logger.AllLogLevels()

	if len(got) != len(want) {
		t.Fatalf("AllLogLevels() length = %d; want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllLogLevels()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}