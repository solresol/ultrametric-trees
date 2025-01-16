// cmd/evaluatemodel/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/solresol/ultrametric-trees/pkg/decode"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	"github.com/solresol/ultrametric-trees/pkg/inference"
)

func main() {
	runDescription := flag.String("run-description", "", "An informative name to describe the evaluation run")
	modelPaths := flag.String("model", "", "Comma-separated list of paths to trained model SQLite files")
	nodesTable := flag.String("nodes-table", "nodes", "Name of the nodes table")
	testdataDBPath := flag.String("test-data-database", "", "Path to the validation database")
	testdataTable := flag.String("test-data-table", "training_data", "Name of the validation data table")
	outputDBPath := flag.String("output-database", "", "Path to the output database")
	outputTable := flag.String("output-table", "inferences", "Name of the output table")
	limit := flag.Int64("limit", -1, "Stop after this many inferences")
	contextLength := flag.Int64("context-length", 16, "Length of the context window")
	timeFilterString := flag.String("model-cutoff-time", "2099-12-31 23:59:59", "Only use training nodes that are older than the given time (format: 2006-01-02 15:05:07)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *runDescription == "" {
		*runDescription = os.Getenv("ULTRATREE_EVAL_RUN_DESCRIPTION")
	}
	if *modelPaths == "" {
		*modelPaths = os.Getenv("ULTRATREE_EVAL_MODEL_PATHS")
	}
	if *testdataDBPath == "" {
		*testdataDBPath = os.Getenv("ULTRATREE_EVAL_TEST_DATA_DB_PATH")
	}
	if *outputDBPath == "" {
		*outputDBPath = os.Getenv("ULTRATREE_EVAL_OUTPUT_DB_PATH")
	}
	if *timeFilterString == "2099-12-31 23:59:59" {
		envTimeFilter := os.Getenv("ULTRATREE_EVAL_MODEL_CUTOFF_TIME")
		if envTimeFilter != "" {
			*timeFilterString = envTimeFilter
		}
	}

	if *runDescription == "" || *modelPaths == "" || *testdataDBPath == "" || *outputDBPath == "" {
		log.Fatal("Missing required arguments. Please provide run description, models, validation-database, and output-database paths")
	}

	if strings.ToLower(*outputTable) == "evaluation_runs" {
		log.Fatalf("Invalid name for the output table: %s", *outputTable)
	}

	modelPathList := strings.Split(*modelPaths, ",")
	if len(modelPathList) == 0 {
		log.Fatal("No model paths provided")
	}

	// Parse time filter
	timeFilter, err := time.Parse("2006-01-02 15:04:05", *timeFilterString)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %v", err)
	}

	// Initialize inference engines for all models
	var inferenceEngines []*inference.ModelInference
	totalModelSize := 0

	for _, modelPath := range modelPathList {
		modelPath = strings.TrimSpace(modelPath)
		modelDB, err := sql.Open("sqlite3", modelPath)
		if err != nil {
			log.Fatalf("Error opening model database %s: %v", modelPath, err)
		}
		defer modelDB.Close()

		engine, err := inference.NewModelInference(modelDB, *nodesTable, timeFilter)
		if err != nil {
			log.Fatalf("Error initializing inference engine for %s: %v", modelPath, err)
		}

		inferenceEngines = append(inferenceEngines, engine)
		totalModelSize += engine.Size()
	}

	// Create ensemble model
	ensemble := inference.NewEnsemblingModel(inferenceEngines)

	// Connect to validation database
	testdataDB, err := sql.Open("sqlite3", *testdataDBPath)
	if err != nil {
		log.Fatalf("Error opening validation database: %v", err)
	}
	defer testdataDB.Close()

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

	// Insert evaluation run record
	var evaluation_run_id int64
	err = outputDB.QueryRow(`
		insert into evaluation_runs (
			description, model_file, model_table, model_node_count,
			cutoff_date, context_length, validation_datafile,
			validation_table, output_table
		) values (?, ?, ?, ?, ?, ?, ?, ?, ?)
		returning evaluation_run_id`,
		*runDescription, *modelPaths, *nodesTable, totalModelSize,
		timeFilter, *contextLength, *testdataDBPath,
		*testdataTable, *outputTable).Scan(&evaluation_run_id)
	if err != nil {
		log.Fatalf("Error inserting validation run: %v", err)
	}

	// Process validation data with ensemble model
	totalLoss, err := processValidationData(modelPathList[0], testdataDB, outputDB, ensemble,
		*testdataTable, *outputTable, int(*limit),
		int(*contextLength), evaluation_run_id, *verbose)
	if err != nil {
		log.Fatalf("Error processing validation data: %v", err)
	}
	log.Printf("Total loss for %s: %f", *modelPaths, totalLoss)
}

func createOutputTable(db *sql.DB, tableName string) error {
	_, err := db.Exec(`
		create table if not exists evaluation_runs (
			evaluation_run_id integer primary key,
			evaluation_start_time timestamp default current_timestamp,
			evaluation_end_time timestamp,
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
			evaluation_run_id integer references evaluation_runs(evaluation_run_id),
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

func processValidationData(trainingDBPath string, testdataDB, outputDB *sql.DB,
	engine *inference.EnsemblingModel, testdataTable, outputTable string,
	limit int, contextLength int, evaluation_run_id int64, verbose bool) (float64, error) {

	// Open first model DB for word decoding
	trainingDB, err := sql.Open("sqlite3", trainingDBPath)
	if err != nil {
		return 0.0, fmt.Errorf("error opening training database for decoding: %v", err)
	}
	defer trainingDB.Close()

	if verbose {
		log.Printf("Running SELECT id, %s FROM %s", getContextColumns(contextLength), testdataTable)
	}
	query := fmt.Sprintf(`
		SELECT id, %s, targetword
		FROM %s
		ORDER BY id
	`, getContextColumns(contextLength), testdataTable)

	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := testdataDB.Query(query)
	if err != nil {
		return 0.0, fmt.Errorf("error querying validation data: %v", err)
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
		scanArgs[contextLength+1] = &correctAnswer
		for i := range contexts {
			scanArgs[i+1] = &contexts[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return 0.0, fmt.Errorf("error scanning row: %v", err)
		}

		contextStrings := make([]string, 0, contextLength)
		for _, ctx := range contexts {
			if ctx.Valid {
				contextStrings = append(contextStrings, ctx.String)
			}
		}

		contextString, err := decode.ShowContext(testdataDB, contextStrings)
		if err != nil {
			return 0.0, err
		}
		if verbose {
			log.Printf("INFERING %d: %s", id, contextString)
		}

		// Use ensemble inference instead of single model
		result, err := engine.InferFromEnsemble(contextStrings, verbose)
		if err != nil {
			log.Printf("Warning: ensemble inference failed for id %d: %v", id, err)
			continue
		}

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

		if verbose {
			log.Printf("Prediction for %d was %s (%s); the correct answer was %s (%s). Loss was %f",
				id, result.PredictedPath, predictionWord, correctAnswer, answerWord, loss)
		}

		_, err = outputDB.Exec(fmt.Sprintf(`
			INSERT INTO %s (
				input_id, evaluation_run_id, final_node_id,
				predicted_path, correct_path, loss
			) VALUES (?, ?, ?, ?, ?, ?)
		`, outputTable), id, evaluation_run_id, result.FinalNodeID,
			result.PredictedPath, correctAnswer, loss)

		if err != nil {
			return 0.0, fmt.Errorf("error saving result for %d: %v", id, err)
		}

		totalLoss += loss
		totalDepth += result.Depth
		totalInRegionHits += result.InRegion
		totalDataPoints++
	}

	_, err = outputDB.Exec(`
		update evaluation_runs set
			evaluation_end_time = current_timestamp,
			number_of_data_points = ?,
			total_loss = ?,
			average_depth = ?,
			average_in_region_hits = ?
		where evaluation_run_id = ?`,
		totalDataPoints,
		totalLoss,
		float64(totalDepth)/float64(totalDataPoints),
		float64(totalInRegionHits)/float64(totalDataPoints),
		evaluation_run_id)

	if err != nil {
		return 0.0, fmt.Errorf("error closing off validation run %d: %v", evaluation_run_id, err)
	}

	return totalLoss, nil
}

func getContextColumns(contextLength int) string {
	cols := make([]string, contextLength)
	for i := 1; i <= contextLength; i++ {
		cols[i-1] = fmt.Sprintf("context%d", i)
	}
	return strings.Join(cols, ", ")
}
