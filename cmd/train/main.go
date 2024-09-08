package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	_ "github.com/mattn/go-sqlite3"
)

// Initialises a table of node-mapping-to-row with
// exemplar.RootNodeID.  It then uses a probabilistic estimate to find
// a good exemplar for that root node. It then updates the
// node-mapping-to-row table (nodeBucketTable) for that split (setting
// exemplar_value to the exemplar, loss to the estimated loss and
// data_quantity to the number of training data elements

func initializeFirstLeaf(db *sql.DB,
	trainingDataTable, nodeBucketTable, nodesTable string,
	exemplarGuesses, costGuesses int,
	rng *rand.Rand) (error) {

	// Create a table for the nodes hierarchy
	query := fmt.Sprintf("create table if not exists %s (id integer primary key autoincrement, exemplar_value text, data_quantity integer, loss float, contextk int, inner_region_prefix text, inner_region_node_id integer, outer_region_node, when_created datetime default current_timestamp, when_children_populated datetime, has_children bool default false, being_analysed bool default false)", nodesTable)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("Cannot create a table of nodes called %s: %v", nodesTable, err)
	}

	// We really care about loss levels for childless nodes. Let's keep track of that
	query = fmt.Sprintf("create index if not exists %s_children on %s(loss) where not has_children and not being_analysed",
		nodesTable, nodesTable)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Cannot create a table of nodes called %s: %v", nodesTable, err)
	}


	// Populate it with the first row. We could do this later, but it's nice to have the half-ready
	// state visible.
	query = fmt.Sprintf("insert or ignore into %s (id) values (%d)", nodesTable, int(exemplar.RootNodeID))
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Could not create the root node in %s: %v", nodesTable, err)
	}

	// Create the node-mapping-to-row table
	query = fmt.Sprintf("create table if not exists %s (id integer references %s (id), node_id integer references nodes(id), primary key (id, node_id))", nodeBucketTable, trainingDataTable)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Cannot create a table called %s: %v", nodeBucketTable, err)
	}

	// Populate the node-mapping-to-row table
	query = fmt.Sprintf("insert or ignore into %s (id, node_id) select id, %d from %s",
		nodeBucketTable, int(exemplar.RootNodeID), trainingDataTable)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Could not populate the table %s with records from %s: %v",
			nodeBucketTable, trainingDataTable, err)
	}

	// The node-mapping-to-row table will get a lot of queries searching on those columns; often
	// because of updates.
	query = fmt.Sprintf("create index if not exists %s_node_id on %s(node_id)", nodeBucketTable, nodeBucketTable)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Could not create an index %s_node_id: %v", nodeBucketTable, err)
	}
	query = fmt.Sprintf("create unique index if not exists %s_by_id on %s(id)", nodeBucketTable, nodeBucketTable)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("Could not create an index %s_by_id: %v", nodeBucketTable, err)
	}

	// The tables are in reasonable shape, so now it's very much like the back-half of an
	// exemplar choosing process. Let's get the data we need. rows is a large in-memory array
	// of synsets
	rows, err := exemplar.LoadRows(db, trainingDataTable, nodeBucketTable, exemplar.RootNodeID)
	if err != nil {
		return fmt.Errorf("Error loading rows: %v", err)
	}

	// Find the best exemplar (or, to be more honest, the one we can find quickly)
	bestExemplar, bestLoss, err := exemplar.FindBestExemplar(rows, exemplarGuesses, costGuesses, rng)

	if err != nil {
		return fmt.Errorf("Could not get best exemplar: %v", err)
	}

	// Great: now our top-level nodes table has an exemplar, a loss and a quantity. We're
	// just about ready for the recursive training process to start.
	_, err = db.Exec(`
		UPDATE nodes
		SET exemplar_value = ?, loss = ?, data_quantity = ?
		WHERE id = ?
	`, bestExemplar.String(), bestLoss, len(rows), exemplar.RootNodeID)
	if err != nil {
		return fmt.Errorf("Error updating nodes table: %v", err)
	}

	fmt.Printf("Updated node %d with exemplar %s, loss %f, and data quantity %d\n", exemplar.RootNodeID, bestExemplar.String(), bestLoss, len(rows))

	return nil
}


func initialisationRequired(db *sql.DB, trainingDataTable, nodeBucketTable, nodesTable string) (bool, error) {
	trainingDataExists, err := exemplar.TableExists(db, trainingDataTable)
	if err != nil {
		return true, fmt.Errorf("Could not detect whether %s exists: %v", trainingDataTable, err)
	}

	nodeBucketTableExists, err := exemplar.TableExists(db, nodeBucketTable)
	if err != nil {
		return true, fmt.Errorf("Could not detect whether %s exists: %v", nodeBucketTable, err)
	}

	nodesTableExists, err := exemplar.TableExists(db, nodesTable)
	if err != nil {
		return true, fmt.Errorf("Could not detect whether %s exists: %v", nodesTable, err)
	}

	if !trainingDataExists {
		return true, fmt.Errorf("%s does not exist", trainingDataTable)
	}

	trainingIsEmpty, err := exemplar.IsTableEmpty(db, trainingDataTable)
	if err != nil {
		return true, fmt.Errorf("Could not detect whether there was any training data in %s: %v", trainingDataTable, err)
	}
	if trainingIsEmpty {
		return true, fmt.Errorf("There is no training data in %s", trainingDataTable)
	}

	if !nodeBucketTableExists {
		return true, nil
	}

	if !nodesTableExists {
		return true, nil
	}

	// Let's check if it has the right number of rows
	sameSize, err := exemplar.CompareTableRowCounts(db, trainingDataTable, nodeBucketTable)
	if err != nil {
		return true, fmt.Errorf("Could not compare the sizes of %s and %s: %v", trainingDataTable, nodeBucketTable, err)
	}
	if !sameSize {
		return true, nil
	}

	nodesIsEmpty, err := exemplar.IsTableEmpty(db, nodesTable)
	if err != nil {
		return true, fmt.Errorf("Could not detect whether the nodes table %s was empty or not: %v",
				nodesTable, err)
	}
	if nodesIsEmpty {
		return true, nil
	}
	return false, nil
}


//  How createGoodSplit works: it iterates [split-count-try]
//  times... each iteration consists of randomly picking a k for
//  LoadContextNWithinNode, then it gets all the possible synsets that
//  it returns; then it iterates [num-circles-per-split] times picking
//  a random possible synset each time, calling
//  exemplar.FindBestExemplar on the inside array and again on the
//  outside array, and adding up the two losses.

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

func createGoodSplit(db *sql.DB,
	nodesTable string,
	nodeID exemplar.NodeID,
	trainingDataTable string,
	nodeBucketTable string,
	splitCountTry int,
	numCirclesPerSplit int,
	exemplarGuesses int,
	costGuesses int,
	contextLength int,
	rng *rand.Rand) (float64, error) {

	var bestContextK int
	var bestCircle exemplar.Synsetpath
	bestTotalLoss := float64(1<<63 - 1) // Initialize with max float64 value
	var insideLossOfBest, outsideLossOfBest float64
	var bestInsideExemplar, bestOutsideExemplar exemplar.Synsetpath
	var bestInsideRows, bestOutsideRows []exemplar.DataFrameRow

	for i := 0; i < splitCountTry; i++ {
		k := rng.Intn(contextLength) + 1
		sourceRows, err := exemplar.LoadContextNWithinNode(db, trainingDataTable, nodeBucketTable, nodeID, k, contextLength)
		if err != nil {
			return 0.0, fmt.Errorf("Error loading context rows: %v", err)
		}

		targetRows, err := exemplar.LoadRows(db, trainingDataTable, nodeBucketTable, nodeID)
		if err != nil {
			return 0.0, fmt.Errorf("Error loading target rows: %v", err)
		}

		possibleSynsets := exemplar.GetAllPossibleSynsets(sourceRows)

		for j := 0; j < numCirclesPerSplit; j++ {
			randomSynset := possibleSynsets[rng.Intn(len(possibleSynsets))]
			inside, outside := exemplar.SplitByFilter(sourceRows, targetRows, randomSynset)

			insideExemplar, insideLoss, err := exemplar.FindBestExemplar(inside, exemplarGuesses, costGuesses, rng)
			if err != nil {
				log.Printf("Error finding inside exemplar: %v", err)
				continue
			}

			outsideExemplar, outsideLoss, err := exemplar.FindBestExemplar(outside, exemplarGuesses, costGuesses, rng)
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
		fmt.Errorf("Error starting transaction: %v", err)
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
		fmt.Errorf("Error creating inner node: %v", err)
	}

	// Create outer node
	var outerNodeID int64
	err = tx.QueryRow(`
		INSERT INTO nodes (exemplar_value, data_quantity, loss)
		VALUES (?, ?, ?)
		RETURNING id
	`, bestOutsideExemplar.String(), len(bestOutsideRows), outsideLossOfBest).Scan(&outerNodeID)
	if err != nil {
		fmt.Errorf("Error creating outer node: %v", err)
	}

	// Update parent node
	_, err = tx.Exec(`
		UPDATE nodes
		SET contextk = ?, inner_region_prefix = ?, inner_region_node_id = ?, outer_region_node = ?,
		    when_children_populated = current_timestamp, has_children = true
		WHERE id = ?
	`, bestContextK, bestCircle.String(), innerNodeID, outerNodeID, nodeID)
	if err != nil {
		fmt.Errorf("Error updating parent node: %v", err)
	}
	query := fmt.Sprintf("update %s set being_analysed = false where id = %d", nodesTable, nodeID)
	_, err = db.Exec(query)
	if err != nil {
		fmt.Errorf("Could not record that we are no longer analysing %d on table %s: %v",
			nodeID, nodesTable, err)
	}

	// Update node_id for inside rows
	insideIDs := make([]int, len(bestInsideRows))
	for i, row := range bestInsideRows {
		insideIDs[i] = row.RowID
	}
	if err := exemplar.UpdateNodeIDs(tx, nodeBucketTable, insideIDs, exemplar.NodeID(innerNodeID)); err != nil {
		fmt.Errorf("Error updating inside node IDs: %v", err)
	}

	// Update node_id for outside rows
	outsideIDs := make([]int, len(bestOutsideRows))
	for i, row := range bestOutsideRows {
		outsideIDs[i] = row.RowID
	}
	if err := exemplar.UpdateNodeIDs(tx, nodeBucketTable, outsideIDs, exemplar.NodeID(outerNodeID)); err != nil {
		fmt.Errorf("Error updating outside node IDs: %v", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		fmt.Errorf("Error committing transaction: %v", err)
	}

	fmt.Printf("Step completed successfully:\n")
	fmt.Printf("Context K: %d\n", bestContextK)
	fmt.Printf("Best Circle: %s\n", bestCircle.String())
	fmt.Printf("Total Loss: %f\n", bestTotalLoss)
	fmt.Printf("Inner Node ID: %d, Exemplar: %s, Size: %d\n", innerNodeID, bestInsideExemplar.String(), len(bestInsideRows))
	fmt.Printf("Outer Node ID: %d, Exemplar: %s, Size: %d\n", outerNodeID, bestOutsideExemplar.String(), len(bestOutsideRows))

	return bestTotalLoss, nil
}




func main() {
	database := flag.String("database", "", "SQLite database file")
	trainingDataTable := flag.String("training-data", "training_data", "Table name where the training data is stored")
	nodeBucketTable := flag.String("node-bucket", "node_bucket", "Table name where the mapping between rows in the training data and their current nodes is stored")
	nodesTable := flag.String("node-table", "nodes", "The table where the node hierarchy is stored")
	exemplarGuesses := flag.Int("exemplar-guesses", 1000, "Number of exemplar guesses")
	costGuesses := flag.Int("cost-guesses", 1000, "Number of cost guesses per exemplar")
	seed := flag.Int64("seed", 1, "Random number seed")
	splitCountTry := flag.Int("split-count-try", 100, "Number of split attempts")
	contextLength := flag.Int("context-length", 16, "Context length")
	numCirclesPerSplit := flag.Int("num-circles-per-split", 10, "Number of circles to try per split")
	nodeSplittingThreshold := flag.Int("node-splitting-threshold", 1, "If a node is smaller than this, don't try to split it")
	minRunTime := flag.Duration("time", time.Hour * 1000000, "Minimum amount of time to run the program (e.g., 10s, 5m)")

	flag.Parse()
	fmt.Printf("Running for at least %v...\n", *minRunTime)
	timer := time.NewTimer(*minRunTime)

	if *database == "" {
		log.Fatal("--database is required")
	}

	db, err := sql.Open("sqlite3", *database)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	rng := rand.New(rand.NewSource(*seed))

	needsInit, err := initialisationRequired(db, *trainingDataTable, *nodeBucketTable, *nodesTable)
	if err != nil {
		log.Fatalf("Initialisation checks failed: %v", err)
	}
	if needsInit {
		err = initializeFirstLeaf(db, *trainingDataTable, *nodeBucketTable, *nodesTable, *exemplarGuesses, *costGuesses, rng)
		if err != nil {
			log.Fatalf("Could not initialize first leaf: %v", err)
		}
	}

	for {
		select {
		case <- timer.C:
			break;
		default:
			splitStartTime := time.Now()
			nextNode, currentCost, err := exemplar.MostUrgentToImprove(db, *nodesTable, *nodeSplittingThreshold)
			if err != nil {
				log.Fatalf("Could not find the most urgent node to work ing: %v", err)
			}
			if nextNode == exemplar.NoNodeID {
				fmt.Printf("Training is complete")
				return
			}
			fmt.Printf("Will split node %d because its current cost is %f\n", int(nextNode), currentCost)
			query := fmt.Sprintf("update %s set being_analysed = true where id = %d", *nodesTable, nextNode)
			_, err = db.Exec(query)
			if err != nil {
				log.Fatalf("Could not set being_analysed = true on row %d of %s", int(nextNode), *nodesTable)
			}
			
			newLoss, err := createGoodSplit(db, *nodesTable, nextNode, *trainingDataTable, *nodeBucketTable, *splitCountTry, *numCirclesPerSplit, *exemplarGuesses, *costGuesses, *contextLength, rng)
			if err != nil {
				log.Fatalf("Could not split %s on node %d using training data in %s and node bucket information in %s (splitCountTry=%d, contextLength=%d because: %v", *nodesTable, int(nextNode), *trainingDataTable, *nodeBucketTable, *splitCountTry, *contextLength, err)
			}
			
			query = fmt.Sprintf("update %s set being_analysed = false where id = %d", *nodesTable, int(nextNode))
			_, err = db.Exec(query)
			if err != nil {
				log.Fatalf("Could not set being_analysed = false on row %d of %s", int(nextNode), *nodesTable)
			}
			improvement := currentCost - newLoss
			elapsed := time.Since(splitStartTime)
			fmt.Printf("Total loss reduced by %f in %v\n", improvement, elapsed)
			// Perhaps I should check whether the improvement was positive
			// On the other hand, the a negative improvement is just an illusion caused
			// by inaccurate loss estimation, I think.
			// Maybe I should also monitor validation loss
		}
	}
}
