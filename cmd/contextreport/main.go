package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
create table if not exists context_snapshots (
	context_snapshot_id integer primary key,
	filename text not null,
	when_captured timestamp default current_timestamp
);
create table if not exists context_usage (
	context_snapshot_id integer references context_snapshots,
	k integer,
	appearance_count integer
);`

func createSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

func generateHistogram(inputDB, outputDB *sql.DB, inputFilename string) error {
	// Start a transaction for the output database
	tx, err := outputDB.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert into context_snapshots
	result, err := tx.Exec(
		"INSERT INTO context_snapshots (filename) VALUES (?)",
		inputFilename,
	)
	if err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}

	snapshotID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting snapshot ID: %w", err)
	}

	// Query the input database to get context value counts
	rows, err := inputDB.Query(`
		SELECT contextk, COUNT(*) as count 
		FROM nodes
                WHERE contextk is not null 
		GROUP BY contextk 
		ORDER BY contextk
	`)
	if err != nil {
		return fmt.Errorf("querying input database: %w", err)
	}
	defer rows.Close()

	// Prepare the insert statement for context_usage
	stmt, err := tx.Prepare(
		"INSERT INTO context_usage (context_snapshot_id, k, appearance_count) VALUES (?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("preparing insert statement: %w", err)
	}
	defer stmt.Close()

	// Process each context value and its count
	for rows.Next() {
		var k, count int
		if err := rows.Scan(&k, &count); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}

		_, err = stmt.Exec(snapshotID, k, count)
		if err != nil {
			return fmt.Errorf("inserting usage data: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("reading rows: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func main() {
	inputFile := flag.String("input", "", "Input SQLite database file")
	outputFile := flag.String("output", "", "Output SQLite database file")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		log.Fatal("Both input and output database files must be specified")
	}

	// Open input database
	inputDB, err := sql.Open("sqlite3", *inputFile)
	if err != nil {
		log.Fatalf("Opening input database: %v", err)
	}
	defer inputDB.Close()

	// Open output database
	outputDB, err := sql.Open("sqlite3", *outputFile)
	if err != nil {
		log.Fatalf("Opening output database: %v", err)
	}
	defer outputDB.Close()

	// Create schema in output database
	if err := createSchema(outputDB); err != nil {
		log.Fatalf("Creating schema: %v", err)
	}

	// Generate histogram
	if err := generateHistogram(inputDB, outputDB, *inputFile); err != nil {
		log.Fatalf("Generating histogram: %v", err)
	}

	fmt.Printf("Histogram data from %s successfully written to output database %s\n",
		*inputFile, *outputFile)
}
