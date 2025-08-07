package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

var levels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

func randomLevel() string {
	return levels[rand.Intn(len(levels))]
}

func randomMessage() string {
	msgs := []string{
		"User logged in",
		"File not found",
		"Connection timeout",
		"Database query executed",
		"Cache miss",
		"Unexpected error occurred",
		"Request received",
	}
	return msgs[rand.Intn(len(msgs))]
}

func generateLogEntry() LogEntry {
	return LogEntry{
		Level:     randomLevel(),
		Message:   randomMessage(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Context: map[string]interface{}{
			"user_id": rand.Intn(1000),
			"ip":      fmt.Sprintf("192.168.1.%d", rand.Intn(255)),
		},
	}
}

func sendLog(url string, entry LogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP error status: %d", resp.StatusCode)
	}

	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	url := flag.String("url", "http://localhost:8888", "Fluent Bit HTTP input URL")
	count := flag.Int("count", 10, "Number of logs to send")
	interval := flag.Int("interval", 500, "Interval between logs in milliseconds")
	flag.Parse()

	fmt.Printf("Sending %d logs to %s every %dms\n", *count, *url, *interval)

	for i := 0; i < *count; i++ {
		entry := generateLogEntry()
		err := sendLog(*url, entry)
		if err != nil {
			fmt.Printf("Error sending log #%d: %v\n", i+1, err)
		} else {
			fmt.Printf("Sent log #%d: %s - %s\n", i+1, entry.Level, entry.Message)
		}
		time.Sleep(time.Duration(*interval) * time.Millisecond)
	}
}
