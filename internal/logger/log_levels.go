package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type LogLevel string

const (
	LogLevelTrace LogLevel = "TRACE"
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// Set of allowed log levels (used internally)
var allowedLogLevels = map[string]LogLevel{
	"TRACE": LogLevelTrace,
	"DEBUG": LogLevelDebug,
	"INFO":  LogLevelInfo,
	"WARN":  LogLevelWarn,
	"ERROR": LogLevelError,
	"FATAL": LogLevelFatal,
}

// For ordering levels by severity
var orderMap = map[LogLevel]int{
	LogLevelTrace: 0,
	LogLevelDebug: 1,
	LogLevelInfo:  2,
	LogLevelWarn:  3,
	LogLevelError: 4,
	LogLevelFatal: 5,
}

// IsValidLogLevel checks if the level is valid
func IsValidLogLevel(level string) bool {
	_, ok := allowedLogLevels[strings.ToUpper(level)]
	return ok
}

// NormalizeLogLevel ensures the level is uppercased and valid
func NormalizeLogLevel(level string) LogLevel {
	return LogLevel(strings.ToUpper(level))
}

// LevelLessThan returns true if a is less severe than b
func LevelLessThan(a, b LogLevel) bool {
	return orderMap[a] < orderMap[b]
}

// AllLogLevels returns all valid log levels
func AllLogLevels() []LogLevel {
	return []LogLevel{
		LogLevelTrace,
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
		LogLevelFatal,
	}
}

// String implements fmt.Stringer
func (l LogLevel) String() string {
	return string(l)
}

// MarshalJSON ensures the log level is marshaled as string
func (l LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

// UnmarshalJSON enforces valid log levels during parsing (e.g. from Fluent Bit)
func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized := strings.ToUpper(raw)
	if val, ok := allowedLogLevels[normalized]; ok {
		*l = val
		return nil
	}
	return errors.New(fmt.Sprintf("invalid log level: %s", raw))
}