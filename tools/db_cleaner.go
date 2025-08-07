package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3" // ou le driver de ta BDD
	"github.com/rypi-dev/logger-server/internal/utils"
)

func main() {
	dbPath := flag.String("db", "logs.db", "Path to SQLite DB file")
	limit := flag.Int("limit", 10000, "Number of recent logs to keep")
	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	query := utils.GenerateCleanupQuery()
	res, err := db.Exec(query, *limit)
	if err != nil {
		log.Fatalf("Cleanup query failed: %v", err)
	}

	rowsAffected, _ := res.RowsAffected()
	fmt.Printf("Cleanup done, rows deleted: %d\n", rowsAffected)
}