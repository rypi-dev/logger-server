package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"rypi-dev/logger-server/internal"
)

func main() {
	filePath := flag.String("file", "", "JSON file containing logs (array)")
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

	t, err := decoder.Token()
	if err != nil {
		fmt.Printf("Error reading JSON: %v\n", err)
		os.Exit(1)
	}

	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		fmt.Println("JSON file must be an array of log entries")
		os.Exit(1)
	}

	total := 0
	errorsCount := 0

	for decoder.More() {
		var entry internal.LogEntry
		if err := decoder.Decode(&entry); err != nil {
			fmt.Printf("Error decoding log entry: %v\n", err)
			errorsCount++
			continue
		}

		total++

		if err := entry.Validate(); err != nil {
			errorsCount++
			fmt.Printf("Validation error in entry %d: %v\n", total, err)
		}
	}

	fmt.Printf("\nValidation completed: %d entries checked, %d errors found\n", total, errorsCount)
}