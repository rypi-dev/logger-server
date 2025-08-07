package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
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
	var (
		url      = flag.String("url", "http://localhost:8080/log", "Server URL (direct Go server)")
		rate     = flag.Int("rate", 100, "Logs per second")
		duration = flag.Int("duration", 10, "Duration in seconds")
	)
	flag.Parse()

	ticker := time.NewTicker(time.Second / time.Duration(*rate))
	defer ticker.Stop()

	var wg sync.WaitGroup
	done := time.After(time.Duration(*duration) * time.Second)

	fmt.Printf("Load testing %s at %d logs/s for %d seconds\n", *url, *rate, *duration)

loop:
	for {
		select {
		case <-done:
			break loop
		case <-ticker.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				entry := LogEntry{
					Level:     "INFO",
					Message:   "Load test message",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				}
				err := sendLog(*url, entry)
				if err != nil {
					fmt.Printf("Error sending log: %v\n", err)
				}
			}()
		}
	}

	wg.Wait()
	fmt.Println("Load test completed")
}