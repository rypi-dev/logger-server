package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "logs.db", "Path to SQLite DB")
	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("DB open error: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT level, COUNT(*) FROM logs GROUP BY level")
	if err != nil {
		log.Fatalf("Query error: %v", err)
	}
	defer rows.Close()

	fmt.Println("Log counts by level:")
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  %s: %d\n", level, count)
	}
}