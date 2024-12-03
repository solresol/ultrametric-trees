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
	"github.com/solresol/ultrametric-trees/pkg/decode"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	"github.com/solresol/ultrametric-trees/pkg/inference"
)

func main() {
	runDescription := flag.String("run-description", "", "An informative name to describe the validation run")
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

	if *runDescription == "" || *modelPath == "" || *validationDBPath == "" || *outputDBPath == "" {
		log.Fatal("Missing required arguments. Please provide run description, model, validation-database, and output-database paths")
	}

	if strings.ToLower(*outputTable) == "validation_runs" {
		log.Fatalf("Invalid name for the output table: %s", *outputTable)
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
	modelSize := inferenceEngine.Size()

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
	var validation_run_id int64
	// Create validation run
	err = outputDB.QueryRow(`insert into validation_runs (description, model_file, model_table, model_node_count, cutoff_date, context_length, validation_datafile, validation_table, output_table) values (?,?,?,?,?,?,?,?,?) returning validation_run_id`,
		*runDescription, *modelPath, *nodesTable, modelSize, timeFilter, *contextLength, *validationDBPath, *validationTable, *outputTable).Scan(&validation_run_id)
	if err != nil {
		log.Fatalf("Error inserting validation run: %v", err)
	}

	// Process validation data
	err = processValidationData(modelDB, validationDB, outputDB, inferenceEngine, *validationTable, *outputTable, int(*limit), int(*contextLength), validation_run_id)
	if err != nil {
		log.Fatalf("Error processing validation data: %v", err)
	}
}

func createOutputTable(db *sql.DB, tableName string) error {
	_, err := db.Exec(`
                create table if not exists validation_runs (
                   validation_run_id integer primary key,
                   validation_start_time timestamp default current_timestamp,
                   validation_end_time timestamp,
                   description text not null,
                   model_file text not null,
                   model_table text not null,
                   model_node_count integer,
                   cutoff_date timestamp,
                   context_length integer,
                   validation_datafile text not null,
                   validation_table text not null,
                   output_table text not null,
                   number_of_data_points integer,
                   total_loss float,
                   average_depth float,
                   average_in_region_hits float
                )`)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY,
			validation_run_id integer references validation_runs(validation_run_id),
			final_node_id integer,
			input_id INTEGER,
			predicted_path TEXT,
			correct_path TEXT,
			loss REAL,
			when_predicted TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, tableName)

	_, err = db.Exec(query)
	return err
}

func processValidationData(trainingDB, validationDB, outputDB *sql.DB, engine *inference.ModelInference, validationTable, outputTable string, limit int, contextLength int, validation_run_id int64) error {
	log.Printf("Running SELECT id, %s FROM %s", getContextColumns(contextLength), validationTable)
	// Query to get validation data
	query := fmt.Sprintf(`
		SELECT id, %s, targetword
		FROM %s
		ORDER BY id
	`, getContextColumns(contextLength), validationTable)

	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}
	rows, err := validationDB.Query(query)
	if err != nil {
		return fmt.Errorf("error querying validation data: %v", err)
	}
	defer rows.Close()
	totalLoss := 0.0
	totalDepth := 0
	totalInRegionHits := 0
	totalDataPoints := 0

	for rows.Next() {
		var id int
		var correctAnswer string
		contexts := make([]sql.NullString, contextLength)
		scanArgs := make([]interface{}, contextLength+2)
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
			INSERT INTO %s (input_id, validation_run_id, final_node_id, predicted_path, correct_path, loss)
			VALUES (?, ?, ?, ?, ?, ?)
		`, outputTable), id, validation_run_id, result.FinalNodeID, result.PredictedPath, correctAnswer, loss)
		if err != nil {
			return fmt.Errorf("error saving result for %d: %v", id, err)
		}
		totalLoss += loss
		totalDepth += result.Depth
		totalInRegionHits += result.InRegion
		totalDataPoints++
	}
	log.Printf("Total loss: %f", totalLoss)
	_, err = outputDB.Exec("update validation_runs set validation_end_time = current_timestamp, number_of_data_points = ?, total_loss = ?, average_depth = ?, average_in_region_hits = ? where validation_run_id = ?",
		totalDataPoints,
		totalLoss,
		float64(totalDepth)/float64(totalDataPoints),
		float64(totalInRegionHits)/float64(totalDataPoints),
		validation_run_id)
	if err != nil {
		return fmt.Errorf("error closing off validation run %d: %v", validation_run_id, err)
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
