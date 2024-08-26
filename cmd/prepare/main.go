package main

import (
	"crypto/sha256"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type WordData struct {
	WordID int
	Word   string
	Synset string
	Path   string
}

var hashedPseudoSynsetPrefix = map[string]string{
	"(noun.other)":        "1.",
	"(verb.other)":        "3.",
	"(propernoun.other)": "1.3.",
	"(preposition.other)": "6.",
	"(adjective.other)":   "2.",
	"(adverb.other)":      "4.",
	"(other.other)":       "8.",
}

func hashThing(thing string) string {
	hash := sha256.Sum256([]byte(thing))
	return fmt.Sprintf("%d", int(hash[0])<<24|int(hash[1])<<16|int(hash[2])<<8|int(hash[3]))
}

func isEnumeratedPseudoSynset(synset string) bool {
	enumeratedSynsets := map[string]bool{
		"(pronoun.other)":     true,
		"(punctuation.other)": true,
		"(conjunction.other)": true,
		"(article.other)":     true,
	}
	return enumeratedSynsets[synset]
}

func getPath(db *sql.DB, wordID int, word, synset string) (WordData, error) {
	if synset == "" {
		return WordData{WordID: wordID, Word: word, Synset: synset, Path: ""}, nil
	}

	fields := strings.Split(synset, ".")
	if len(fields) == 3 {
		var path string
		err := db.QueryRow("SELECT path FROM synset_paths WHERE synset_name = ?", synset).Scan(&path)
		if err != nil {
			if err == sql.ErrNoRows {
				return WordData{}, fmt.Errorf("non-existent (but plausible) synset: %s for word %s [word_id=%d]", synset, word, wordID)
			}
			return WordData{}, err
		}
		return WordData{WordID: wordID, Word: word, Synset: synset, Path: path}, nil
	}

	// Handle pseudo-synsets
	if isEnumeratedPseudoSynset(synset) {
		var path string
		err := db.QueryRow("SELECT path FROM synset_paths WHERE synset_name = ?", strings.ToLower(word)).Scan(&path)
		if err != nil {
			if err == sql.ErrNoRows {
				// A new thing that we don't recognize, from a closed set
				return WordData{WordID: wordID, Word: word, Synset: synset, Path: ""}, nil
			}
			return WordData{}, err
		}
		return WordData{WordID: wordID, Word: word, Synset: synset, Path: path}, nil
	}

	prefix, ok := hashedPseudoSynsetPrefix[synset]
	if !ok {
		return WordData{}, fmt.Errorf("unknown pseudo-synset: %s", synset)
	}
	hashedWord := hashThing(word)
	return WordData{WordID: wordID, Word: word, Synset: synset, Path: prefix + hashedWord}, nil
}



// prepare's CLI arguments:
//  --input-database 
//  --context-length (default 16)
//  --output-database
//
// It has two optional CLI arguments (--modulo and --congruent). If the story number is congruent
// to [congruent] modulo [modulo] then we use it, otherwise we ignore it.
//
// It goes through each story in the database (or just --congurent and
// --modulo if specified) from beginning to end. It keeps a buffer of
// the last [context-length+1] words.

// When the buffer is full, it outputs the synset-paths for those
// words into a table called "training_data" in columns (targetword,
// context1, context2, ... contextN) where targetword is the path of
// the most recent word seen, context1 is the word before it, context2
// is the word before that and so on. Then it drops out the path word
// that was contextN, and every other path shuffles down. (This
// generally means that the buffer will be full when it reads the next
// word).

// If it hits a word with no path, then it clears the whole buffer.

// When you get to the end of a story, clear the whole buffer.

func main() {
	inputDB := flag.String("input-database", "", "Path to the input SQLite database")
	outputDB := flag.String("output-database", "", "Path to the output SQLite database")
	contextLength := flag.Int("context-length", 16, "Context length for word sequences")
	modulo := flag.Int("modulo", 0, "Modulo for story selection")
	congruent := flag.Int("congruent", 0, "Congruent value for story selection")
	outputTable := flag.String("output-table", "training_data", "Name of the output table for training data")
	noNode := flag.Bool("no-node", false, "If set, do not create the 'node' column in the output table")
	flag.Parse()

	if *inputDB == "" || *outputDB == "" {
		log.Fatal("Both --input-database and --output-database are required")
	}

	inputConn, err := sql.Open("sqlite3", *inputDB)
	if err != nil {
		log.Fatalf("Error opening input database: %v", err)
	}
	defer inputConn.Close()

	outputConn, err := sql.Open("sqlite3", *outputDB)
	if err != nil {
		log.Fatalf("Error opening output database: %v", err)
	}
	defer outputConn.Close()

	createOutputTables(outputConn, *contextLength, *outputTable, *noNode)

	stories, err := getStories(inputConn, *modulo, *congruent)
	if err != nil {
		log.Fatalf("Error getting stories: %v", err)
	}

	for _, storyID := range stories {
		processStory(inputConn, outputConn, storyID, *contextLength, *outputTable, *noNode)
	}

	fmt.Println("Data preparation completed successfully.")
}

func createOutputTables(db *sql.DB, contextLength int, outputTable string, noNode bool) {
	// Create training_data table
	_, err := db.Exec("create table if not exists nodes (id integer primary key autoincrement, exemplar_value text, data_quantity integer, loss float, inner_region_prefix text, inner_region_node_id integer, outer_region_node)")
	_, err = db.Exec("insert or ignore into nodes (id) values (1)")
	
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, targetword TEXT", outputTable)
	for i := 1; i <= contextLength; i++ {
		query += fmt.Sprintf(", context%d TEXT", i)
	}
	if !noNode {
		query += ", node_id INTEGER not null references nodes(id) default 1"
	}
	query += ")"
	fmt.Printf("Executing %s\n", query)

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating output table: %v", err)
	}

	// Create index on node column if it exists
	if !noNode {
		indexingCommand := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_node ON %s (node_id)", outputTable, outputTable)
		fmt.Printf("Running indexing command: %s\n", indexingCommand)
		_, err = db.Exec(indexingCommand)
		if err != nil {
			log.Fatalf("Error creating index on node column: %v", err)
		}
	}

	// Create decodings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS decodings (
			path TEXT,
			word TEXT,
			usage_count INTEGER,
			PRIMARY KEY (path, word)
		)
	`)
	if err != nil {
		log.Fatalf("Error creating decodings table: %v", err)
	}

	// Create indexes
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_decodings_path ON decodings (path)")
	if err != nil {
		log.Fatalf("Error creating index on decodings.path: %v", err)
	}

	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_decodings_word ON decodings (word)")
	if err != nil {
		log.Fatalf("Error creating index on decodings.word: %v", err)
	}
}


func getStories(db *sql.DB, modulo, congruent int) ([]int, error) {
	query := "SELECT DISTINCT story_id FROM sentences ORDER BY story_id"
	if modulo > 0 {
		query = fmt.Sprintf("SELECT DISTINCT story_id FROM sentences WHERE story_id %% %d = %d ORDER BY story_id", modulo, congruent)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []int
	for rows.Next() {
		var storyID int
		if err := rows.Scan(&storyID); err != nil {
			return nil, err
		}
		stories = append(stories, storyID)
	}

	return stories, nil
}

func processStory(inputDB, outputDB *sql.DB, storyID, contextLength int, outputTable string, noNode bool) {
	words, err := getWordsForStory(inputDB, storyID)
	if err != nil {
		log.Printf("Error getting words for story %d: %v", storyID, err)
		return
	}

	buffer := make([]WordData, 0, contextLength+1)

	for _, word := range words {
		if word.Path == "" {
			buffer = buffer[:0] // Clear the buffer
			continue
		}

		buffer = append(buffer, word)

		if len(buffer) == contextLength+1 {
			insertTrainingData(outputDB, buffer, contextLength, outputTable, noNode)
			buffer = buffer[1:] // Remove the oldest word
		}
	}

	// Clear the buffer at the end of the story
	buffer = buffer[:0]
}

func getWordsForStory(db *sql.DB, storyID int) ([]WordData, error) {
	query := `
		SELECT w.id, w.word, w.resolved_synset
		FROM words w
		JOIN sentences s ON w.sentence_id = s.id
		WHERE s.story_id = ?
		ORDER BY s.sentence_number, w.word_number
	`

	rows, err := db.Query(query, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var words []WordData
	for rows.Next() {
		var wordID int
		var word, synset string
		if err := rows.Scan(&wordID, &word, &synset); err != nil {
			return nil, err
		}

		wordData, err := getPath(db, wordID, word, synset)
		if err != nil {
			log.Printf("Error getting path for word %s (ID: %d): %v", word, wordID, err)
			continue
		}

		words = append(words, wordData)
	}

	return words, nil
}

func insertTrainingData(db *sql.DB, buffer []WordData, contextLength int, outputTable string, noNode bool) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Insert into training_data table
	query := fmt.Sprintf("INSERT INTO %s (targetword", outputTable)
	for i := 1; i <= contextLength; i++ {
		query += fmt.Sprintf(", context%d", i)
	}
	query += ") VALUES (?"
	for i := 1; i <= contextLength; i++ {
		query += ", ?"
	}
	query += ")"

	args := make([]interface{}, contextLength+1)
	args[0] = buffer[contextLength].Path
	for i := 0; i < contextLength; i++ {
		args[i+1] = buffer[contextLength-1-i].Path
	}
	_, err = tx.Exec(query, args...)
	if err != nil {
		log.Printf("Error inserting training data: %v", err)
		return
	}

	// Update decodings table
	for _, word := range buffer {
		_, err := tx.Exec(`
			INSERT INTO decodings (path, word, usage_count)
			VALUES (?, ?, 1)
			ON CONFLICT(path, word) DO UPDATE SET usage_count = usage_count + 1
		`, word.Path, word.Word)
		if err != nil {
			log.Printf("Error updating decodings: %v", err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
	}
}
