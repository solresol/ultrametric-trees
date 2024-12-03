// cmd/infer/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/solresol/ultrametric-trees/pkg/inference"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	"github.com/solresol/ultrametric-trees/pkg/decode"
)

func main() {
	modelPath := flag.String("model", "", "Path to the trained model SQLite file")
	nodesTable := flag.String("nodes-table", "nodes", "Name of the nodes table")
	validationDBPath := flag.String("validation-database", "", "Path to the validation database")
	validationTable := flag.String("validation-data-table", "training_data", "Name of the validation data table")
	outputDBPath := flag.String("output-database", "", "Path to the output database")
	outputTable := flag.String("output-table", "inferences", "Name of the output table")
	limit := flag.Int64("limit", -1, "Stop after this many inferences")
	contextLength := flag.Int64("context-length", 16, "Length of the context window")
	timeFilterString := flag.String("model-cutoff-time", "9999-12-31 23:59:59", "Only use training nodes that are older than the given time (format: 2006-01-02 15:05:07)")
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

	timeFilter, err := time.Parse("2006-01-02 15:04:05", *timeFilterString)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %v", err)
	}

	// Initialize inference engine
	inferenceEngine, err := inference.NewModelInference(modelDB, *nodesTable, timeFilter)
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
	err = processValidationData(modelDB, validationDB, outputDB, inferenceEngine, *validationTable, *outputTable, int(*limit), int(*contextLength))
	if err != nil {
		log.Fatalf("Error processing validation data: %v", err)
	}
}

func createOutputTable(db *sql.DB, tableName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY,
			final_node_id integer,
			input_id INTEGER,
			predicted_path TEXT,
			correct_path TEXT,
			loss REAL,
			when_predicted TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err := db.Exec(query)
	return err
}

func processValidationData(trainingDB, validationDB, outputDB *sql.DB, engine *inference.ModelInference, validationTable, outputTable string, limit int, contextLength int) error {
	log.Printf("Running SELECT id, %s FROM %s", getContextColumns(16), validationTable)
	// Query to get validation data
	query := fmt.Sprintf(`
		SELECT id, %s, targetword
		FROM %s
		ORDER BY id
	`, getContextColumns(contextLength), validationTable) // Assuming max context length of 16

	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}
	rows, err := validationDB.Query(query)
	if err != nil {
		return fmt.Errorf("error querying validation data: %v", err)
	}
	defer rows.Close()
	totalLoss := 0.0

	for rows.Next() {
		var id int
		var correctAnswer string
		contexts := make([]sql.NullString, contextLength)
		scanArgs := make([]interface{}, contextLength + 2)
		scanArgs[0] = &id
		scanArgs[17] = &correctAnswer
		for i := range contexts {
			scanArgs[i+1] = &contexts[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("error scanning row: %v", err)
		}

		// Convert contexts to string slice
		contextStrings := make([]string, 0, contextLength)
		for _, ctx := range contexts {
			if ctx.Valid {
				contextStrings = append(contextStrings, ctx.String)
			}
		}
		contextString, err := decode.ShowContext(validationDB, contextStrings)
		if err != nil {
			return err
		}
		log.Printf("INFERING %d: %s", id, contextString)


		// Perform inference
		result, err := engine.InferSingle(contextStrings)
		if err != nil {
			log.Printf("Warning: inference failed for id %d: %v", id, err)
			continue
		}

		// Save result
		predictionSynset, err := exemplar.ParseSynsetpath(result.PredictedPath)
		if err != nil {
			log.Printf("Could not turn the prediction %s into a synsetpath: %v", result.PredictedPath, err)
			continue
		}
		predictionWord, _ := decode.DecodePath(trainingDB, result.PredictedPath)
		correctAnswerSynset, err := exemplar.ParseSynsetpath(correctAnswer)
		if err != nil {
			log.Printf("Could not turn the answer %s into a synsetpath: %v", correctAnswer, err)
			continue
		}
		answerWord, _ := decode.DecodePath(trainingDB, correctAnswer)
		loss := exemplar.CalculateCost(predictionSynset, correctAnswerSynset)
		log.Printf("Prediction for %d was %s (%s); the correct answer was %s (%s). Loss was %f", id, result.PredictedPath, predictionWord, correctAnswer, answerWord, loss)
		_, err = outputDB.Exec(fmt.Sprintf(`
			INSERT INTO %s (input_id, final_node_id, predicted_path, correct_path, loss)
			VALUES (?, ?, ?, ?, ?)
		`, outputTable), id, result.FinalNodeID, result.PredictedPath, correctAnswer, loss)
		if err != nil {
			return fmt.Errorf("error saving result: %v", err)
		}
		totalLoss += loss
	}
	log.Printf("Total loss: %f", totalLoss)
	return nil
}

func getContextColumns(contextLength int) string {
	cols := make([]string, contextLength)
	for i := 1; i <= contextLength; i++ {
		cols[i-1] = fmt.Sprintf("context%d", i)
	}
	return strings.Join(cols, ", ")
}
