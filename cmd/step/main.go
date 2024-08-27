package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"

	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	_ "github.com/mattn/go-sqlite3"
)

//  How step: it iterates [split-count-try] times... each iteration consists of randomly picking a k for LoadContextNWithinNode, then it gets all the possible synsets that it returns; then it iterates [num-circles-per-split] times picking a random possible synset each time, calling exemplar.FindBestExemplar on the inside array and again on the outside array, and adding up the two losses.

// In the end, it will have a contextK, a bestCircle, a total loss, a
// best inside exemplar, the number of elements on the inside array, a
// best outside exemplar and the number of elements on the outside
// array. It creates two new nodes in the nodes table.
//
// * An "inner" node, where the exemplar_value is the best inside
// exemplar, data_quantity = the size of the inner array, loss = the
// inner loss
// * An "outer" node where the exemplar_value is the best
// outside exemplar, data_quantity = the size of the outer array, loss
// = the outer loss
//
// Then it updates the parent node in the nodes table
//
// * contextk = contextK
// * inner_region_prefix = bestCircle
// * inner_region_node = (the newly created inner node id)
// * outer_region_node = (the newly created outer node id)

func main() {
	database := flag.String("database", "", "SQLite database file")
	table := flag.String("table", "training_data", "Table name")
	exemplarGuesses := flag.Int("exemplar-guesses", 1000, "Number of exemplar guesses")
	costGuesses := flag.Int("cost-guesses", 1000, "Number of cost guesses per exemplar")
	seed := flag.Int64("seed", 1, "Random number seed")
	nodeID := flag.Int("node-id", 1, "Node ID to process")
	splitCountTry := flag.Int("split-count-try", 100, "Number of split attempts")
	contextLength := flag.Int("context-length", 16, "Context length")
	numCirclesPerSplit := flag.Int("num-circles-per-split", 10, "Number of circles to try per split")
	flag.Parse()

	if *database == "" {
		log.Fatal("--database is required")
	}

	db, err := sql.Open("sqlite3", *database)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	rng := rand.New(rand.NewSource(*seed))

	var bestContextK int
	var bestCircle exemplar.Synsetpath
	bestTotalLoss := float64(1<<63 - 1) // Initialize with max float64 value
	var insideLossOfBest, outsideLossOfBest float64
	var bestInsideExemplar, bestOutsideExemplar exemplar.Synsetpath
	var bestInsideRows, bestOutsideRows []exemplar.DataFrameRow

	for i := 0; i < *splitCountTry; i++ {
		k := rng.Intn(*contextLength) + 1
		sourceRows, err := exemplar.LoadContextNWithinNode(db, *table, exemplar.NodeID(*nodeID), k, *contextLength)
		if err != nil {
			log.Fatalf("Error loading context rows: %v", err)
		}

		targetRows, err := exemplar.LoadRows(db, *table, exemplar.NodeID(*nodeID))
		if err != nil {
			log.Fatalf("Error loading target rows: %v", err)
		}

		possibleSynsets := exemplar.GetAllPossibleSynsets(sourceRows)

		for j := 0; j < *numCirclesPerSplit; j++ {
			randomSynset := possibleSynsets[rng.Intn(len(possibleSynsets))]
			inside, outside := exemplar.SplitByFilter(sourceRows, targetRows, randomSynset)

			insideExemplar, insideLoss, err := exemplar.FindBestExemplar(inside, *exemplarGuesses, *costGuesses, *seed)
			if err != nil {
				log.Printf("Error finding inside exemplar: %v", err)
				continue
			}

			outsideExemplar, outsideLoss, err := exemplar.FindBestExemplar(outside, *exemplarGuesses, *costGuesses, *seed)
			if err != nil {
				log.Printf("Error finding outside exemplar: %v", err)
				continue
			}

			totalLoss := insideLoss + outsideLoss

			if totalLoss < bestTotalLoss {
				bestTotalLoss = totalLoss
				bestContextK = k
				bestCircle = randomSynset
				bestInsideExemplar = insideExemplar
				bestOutsideExemplar = outsideExemplar
				bestInsideRows = inside
				bestOutsideRows = outside
				insideLossOfBest = insideLoss
				outsideLossOfBest = outsideLoss
			}
		}
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Create inner node
	var innerNodeID int64
	err = tx.QueryRow(`
		INSERT INTO nodes (exemplar_value, data_quantity, loss)
		VALUES (?, ?, ?)
		RETURNING id
	`, bestInsideExemplar.String(), len(bestInsideRows), insideLossOfBest).Scan(&innerNodeID)
	if err != nil {
		log.Fatalf("Error creating inner node: %v", err)
	}

	// Create outer node
	var outerNodeID int64
	err = tx.QueryRow(`
		INSERT INTO nodes (exemplar_value, data_quantity, loss)
		VALUES (?, ?, ?)
		RETURNING id
	`, bestOutsideExemplar.String(), len(bestOutsideRows), outsideLossOfBest).Scan(&outerNodeID)
	if err != nil {
		log.Fatalf("Error creating outer node: %v", err)
	}

	// Update parent node
	_, err = tx.Exec(`
		UPDATE nodes
		SET contextk = ?, inner_region_prefix = ?, inner_region_node_id = ?, outer_region_node = ?
                    when_children_populated = current_timestamp
		WHERE id = ?
	`, bestContextK, bestCircle.String(), innerNodeID, outerNodeID, *nodeID)
	if err != nil {
		log.Fatalf("Error updating parent node: %v", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}

	// Update node_id for inside rows
	insideIDs := make([]int, len(bestInsideRows))
	for i, row := range bestInsideRows {
		insideIDs[i] = row.RowID
	}
	if err := exemplar.UpdateNodeIDs(db, *table, insideIDs, exemplar.NodeID(innerNodeID)); err != nil {
		log.Fatalf("Error updating inside node IDs: %v", err)
	}

	// Update node_id for outside rows
	outsideIDs := make([]int, len(bestOutsideRows))
	for i, row := range bestOutsideRows {
		outsideIDs[i] = row.RowID
	}
	if err := exemplar.UpdateNodeIDs(db, *table, outsideIDs, exemplar.NodeID(outerNodeID)); err != nil {
		log.Fatalf("Error updating outside node IDs: %v", err)
	}

	fmt.Printf("Step completed successfully:\n")
	fmt.Printf("Context K: %d\n", bestContextK)
	fmt.Printf("Best Circle: %s\n", bestCircle.String())
	fmt.Printf("Total Loss: %f\n", bestTotalLoss)
	fmt.Printf("Inner Node ID: %d, Exemplar: %s, Size: %d\n", innerNodeID, bestInsideExemplar.String(), len(bestInsideRows))
	fmt.Printf("Outer Node ID: %d, Exemplar: %s, Size: %d\n", outerNodeID, bestOutsideExemplar.String(), len(bestOutsideRows))
}
