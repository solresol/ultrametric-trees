package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	_ "github.com/mattn/go-sqlite3"
)

// It is given a CLI argument --node-id (which defaults to 1). It
// looks at the --table table and selects all the rows where node =
// [node-id] and loads them into an array. It then uses a probabilistic
// estimate to find a good exemplar. updates the nodes table
// where the node_id = [node-id], setting exemplar_value to the
// exemplar, loss to the estimated loss and data_quantity to the
// number of rows in the [table] that had that node_id.

func main() {
	database := flag.String("database", "", "SQLite database file")
	table := flag.String("table", "training_data", "Table name")
	exemplarGuesses := flag.Int("exemplar-guesses", 1000, "Number of exemplar guesses")
	costGuesses := flag.Int("cost-guesses", 1000, "Number of cost guesses per exemplar")
	seed := flag.Int64("seed", 1, "Random number seed")
	nodeIDint := flag.Int("node-id", 1, "Node ID to process")
	flag.Parse()

	if *database == "" {
		log.Fatal("--database is required")
	}

	db, err := sql.Open("sqlite3", *database)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	nodeID := exemplar.NodeID(*nodeIDint)
	rows, err := exemplar.LoadRows(db, *table, nodeID)
	if err != nil {
		log.Fatalf("Error loading rows: %v", err)
	}

	bestExemplar, bestLoss, err := exemplar.FindBestExemplar(rows, *exemplarGuesses, *costGuesses, *seed)

	if err != nil {
		log.Fatalf("Could not get best exemplar: %v", err)
	}

	_, err = db.Exec(`
		UPDATE nodes 
		SET exemplar_value = ?, loss = ?, data_quantity = ? 
		WHERE id = ?
	`, bestExemplar.String(), bestLoss, len(rows), nodeID)
	if err != nil {
		log.Fatalf("Error updating nodes table: %v", err)
	}

	fmt.Printf("Updated node %d with exemplar %s, loss %f, and data quantity %d\n", nodeID, bestExemplar.String(), bestLoss, len(rows))
}
