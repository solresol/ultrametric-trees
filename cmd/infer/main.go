// cmd/infer/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/solresol/ultrametric-trees/pkg/inference"
)

func main() {
	modelPath := flag.String("model", "", "Path to the trained model SQLite file")
	nodesTable := flag.String("nodes-table", "nodes", "Name of the nodes table")
	validationDBPath := flag.String("validation-database", "", "Path to the validation database")
	validationTable := flag.String("validation-data-table", "", "Name of the validation data table")
	outputDBPath := flag.String("output-database", "", "Path to the output database")
	outputTable := flag.String("output-table", "", "Name of the output table")
	flag.Parse()

	if *modelPath == "" || *validationDBPath == "" || *outputDBPath == "" {
		log.Fatal("Missing required arguments. Please provide model, validation-database, and output-database paths")
	}

	// Connect to model database
	modelDB, err := sql.Open("sqlite3", *modelPath)
	if err != nil {
		log.Fatalf("Error opening model database: %v", err)
	}
	defer modelDB.Close()

	// Initialize inference engine
	inferenceEngine, err := inference.NewModelInference(modelDB, *nodesTable)
	if err != nil {
		log.Fatalf("Error initializing inference engine: %v", err)
	}

	// Connect to validation database
	validationDB, err := sql.Open("sqlite3", *validationDBPath)
	if err != nil {
		log.Fatalf("Error opening validation database: %v", err)
	}
	defer validationDB.Close()

	// Connect to output database
	outputDB, err := sql.Open("sqlite3", *outputDBPath)
	if err != nil {
		log.Fatalf("Error opening output database: %v", err)
	}
	defer outputDB.Close()

	// Create output table
	err = createOutputTable(outputDB, *outputTable)
	if err != nil {
		log.Fatalf("Error creating output table: %v", err)
	}

	// Process validation data
	err = processValidationData(validationDB, outputDB, inferenceEngine, *validationTable, *outputTable)
	if err != nil {
		log.Fatalf("Error processing validation data: %v", err)
	}
}

func createOutputTable(db *sql.DB, tableName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY,
			input_id INTEGER,
			predicted_path TEXT,
			confidence REAL,
			when_predicted TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err := db.Exec(query)
	return err
}

func processValidationData(validationDB, outputDB *sql.DB, engine *inference.ModelInference, validationTable, outputTable string) error {
	// Query to get validation data
	query := fmt.Sprintf(`
		SELECT id, %s
		FROM %s
	`, getContextColumns(16), validationTable) // Assuming max context length of 16

	rows, err := validationDB.Query(query)
	if err != nil {
		return fmt.Errorf("error querying validation data: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		contexts := make([]sql.NullString, 16)
		scanArgs := make([]interface{}, 17)
		scanArgs[0] = &id
		for i := range contexts {
			scanArgs[i+1] = &contexts[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("error scanning row: %v", err)
		}

		// Convert contexts to string slice
		contextStrings := make([]string, 0, 16)
		for _, ctx := range contexts {
			if ctx.Valid {
				contextStrings = append(contextStrings, ctx.String)
			}
		}

		// Perform inference
		result, err := engine.InferSingle(contextStrings)
		if err != nil {
			log.Printf("Warning: inference failed for id %d: %v", id, err)
			continue
		}

		// Save result
		_, err = outputDB.Exec(fmt.Sprintf(`
			INSERT INTO %s (input_id, predicted_path, confidence)
			VALUES (?, ?, ?)
		`, outputTable), id, result.PredictedPath, result.Confidence)
		if err != nil {
			return fmt.Errorf("error saving result: %v", err)
		}
	}

	return nil
}

func getContextColumns(contextLength int) string {
	cols := make([]string, contextLength)
	for i := 1; i <= contextLength; i++ {
		cols[i-1] = fmt.Sprintf("context%d", i)
	}
	return strings.Join(cols, ", ")
}
