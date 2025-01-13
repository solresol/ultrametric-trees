// cmd/remove-children/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("database", "", "Path to the SQLite database")
	nodeID := flag.Int("node", -1, "ID of the node to remove children from")
	flag.Parse()

	if *dbPath == "" || *nodeID == -1 {
		log.Fatal("Both --database and --node arguments are required")
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Start a transaction for the entire operation
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback()

	err = RemoveNodeChildren(tx, *nodeID)
	if err != nil {
		log.Fatalf("Error removing children: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}
}

// RemoveNodeChildren recursively removes all children of the specified node
func RemoveNodeChildren(tx *sql.Tx, nodeID int) error {
	// First, get the node's information
	var innerNodeID, outerNodeID sql.NullInt64
	err := tx.QueryRow(`
		SELECT inner_region_node_id, outer_region_node
		FROM nodes
		WHERE id = ?
	`, nodeID).Scan(&innerNodeID, &outerNodeID)
	if err != nil {
		return fmt.Errorf("error getting node info: %v", err)
	}

	// Recursively remove children's children first
	if innerNodeID.Valid {
		err = RemoveNodeChildren(tx, int(innerNodeID.Int64))
		if err != nil {
			return fmt.Errorf("error removing inner node children: %v", err)
		}
	}

	if outerNodeID.Valid {
		err = RemoveNodeChildren(tx, int(outerNodeID.Int64))
		if err != nil {
			return fmt.Errorf("error removing outer node children: %v", err)
		}
	}

	// Update node_bucket to point to parent for any rows pointing to children
	if innerNodeID.Valid {
		_, err = tx.Exec(`
			UPDATE node_bucket 
			SET node_id = ?
			WHERE node_id = ?
		`, nodeID, innerNodeID.Int64)
		if err != nil {
			return fmt.Errorf("error updating node_bucket for inner node: %v", err)
		}

		// Delete the inner node
		_, err = tx.Exec("DELETE FROM nodes WHERE id = ?", innerNodeID.Int64)
		if err != nil {
			return fmt.Errorf("error deleting inner node: %v", err)
		}
	}

	if outerNodeID.Valid {
		_, err = tx.Exec(`
			UPDATE node_bucket 
			SET node_id = ?
			WHERE node_id = ?
		`, nodeID, outerNodeID.Int64)
		if err != nil {
			return fmt.Errorf("error updating node_bucket for outer node: %v", err)
		}

		// Delete the outer node
		_, err = tx.Exec("DELETE FROM nodes WHERE id = ?", outerNodeID.Int64)
		if err != nil {
			return fmt.Errorf("error deleting outer node: %v", err)
		}
	}

	// Update the parent node to remove references to children
	_, err = tx.Exec(`
		UPDATE nodes 
		SET has_children = false,
                        contextk = null,
			when_children_populated = null,
			inner_region_prefix = null,
			inner_region_node_id = null,
			outer_region_node = null,
			contextk = null
		WHERE id = ?
	`, nodeID)
	if err != nil {
		return fmt.Errorf("error updating parent node: %v", err)
	}

	return nil
}
