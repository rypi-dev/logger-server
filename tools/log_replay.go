package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
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
	filePath := flag.String("file", "", "JSON file containing logs (array)")
	url := flag.String("url", "http://localhost:8080/log", "Server URL")
	delayMs := flag.Int("delay", 1000, "Delay between logs in ms")
	flag.Parse()

	if *filePath == "" {
		fmt.Println("Please provide a log file with -file")
		os.Exit(1)
	}

	file, err := os.Open(*filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// On attend un tableau JSON : [ {...}, {...}, ... ]
	t, err := decoder.Token()
	if err != nil {
		fmt.Printf("Error reading JSON: %v\n", err)
		os.Exit(1)
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		fmt.Println("JSON file must be an array of log entries")
		os.Exit(1)
	}

	for decoder.More() {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			fmt.Printf("Error decoding log entry: %v\n", err)
			continue
		}

		if err := sendLog(*url, entry); err != nil {
			fmt.Printf("Error sending log: %v\n", err)
		}

		time.Sleep(time.Duration(*delayMs) * time.Millisecond)
	}

	fmt.Println("Replay completed")
}