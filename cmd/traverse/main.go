package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fluhus/gostuff/nlp/wordnet"
	_ "github.com/mattn/go-sqlite3"
	"github.com/solresol/ultrametric-trees/internal/traverse"
)

func main() {
	// Define command-line flags
	wordnetPath := flag.String("wordnet", "", "Path to WordNet database directory")
	sqlitePath := flag.String("sqlite", "wordnet.db", "Path to output SQLite database")
	
	// Parse command-line flags
	flag.Parse()

	// Check if WordNet path is provided
	if *wordnetPath == "" {
		fmt.Println("Please provide the path to the WordNet database directory using the -wordnet flag")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parse WordNet database
	wn, err := wordnet.Parse(*wordnetPath)
	if err != nil {
		log.Fatalf("Error parsing WordNet: %v", err)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", *sqlitePath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS synset_paths (
			path TEXT PRIMARY KEY,
			synset_name TEXT UNIQUE
		)
	`)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	}

	// Traverse all synsets
	for _, synset := range wn.Synset {
		traverse.TraverseSynset(wn, synset, db)
	}
	fmt.Println("Synset traversal completed")
}
