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
				return WordData{}, fmt.Errorf("Unrecognized word from a closed set: %s", strings.ToLower(word))
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
// (Although what it should do is output an <END> marker.)

func main() {
	inputDB := flag.String("input-database", "", "Path to the input SQLite database")
	outputDB := flag.String("output-database", "", "Path to the output SQLite database")
	contextLength := flag.Int("context-length", 16, "Context length for word sequences")
	modulo := flag.Int("modulo", 0, "Modulo for story selection")
	congruent := flag.Int("congruent", 0, "Congruent value for story selection")
	outputTable := flag.String("output-table", "training_data", "Name of the output table for training data")
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

	createOutputTables(outputConn, *contextLength, *outputTable)

	stories, err := getStories(inputConn, *modulo, *congruent)
	if err != nil {
		log.Fatalf("Error getting stories: %v", err)
	}

	for _, storyID := range stories {
		err = processStory(inputConn, outputConn, storyID, *contextLength, *outputTable)
		if err != nil {
			log.Fatalf("Could not process story %d: %v", storyID, err)
		}
	}

	fmt.Println("Data preparation completed successfully.")
}

func createOutputTables(db *sql.DB, contextLength int, outputTable string) {
	// Create training_data table
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, targetword TEXT", outputTable)
	for i := 1; i <= contextLength; i++ {
		query += fmt.Sprintf(", context%d TEXT", i)
	}
	query += ")"
	fmt.Printf("Executing %s\n", query)

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating output table: %v", err)
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

func processStory(inputDB, outputDB *sql.DB, storyID, contextLength int, outputTable string) (error) {
	words, err := getWordsForStory(inputDB, storyID)
	if err != nil {
		return fmt.Errorf("Error getting words for story %d: %v", storyID, err)
	}

	startOfText, err := getPath(inputDB, -1, "<START-OF-TEXT>", "(punctuation.other)")
	if err != nil {
		return fmt.Errorf("Could not get the <START-OF-TEXT> marker: %v\n", err)
	}
	// fmt.Printf("Start of text = %s\n", startOfText.Path)
	
	endOfText, err := getPath(inputDB, -1, "<END-OF-TEXT>", "(punctuation.other)")
	if err != nil {
		return fmt.Errorf("Could not get the <END-OF-TEXT> marker: %v\n", err)
	}

	buffer := make([]WordData, 0, contextLength+1)
	for i := 0; i < contextLength; i++ {
		buffer = append(buffer, startOfText)
	}

	for _, word := range words {
		if word.Path == "" {
			// If we can't find the path for a word, then we can't use this
			// as a prediction token, nor can we use it for predicting anything.
			// We'll have to refill the buffer from scratch
			buffer = buffer[:0] // Clear the buffer
			continue
		}

		buffer = append(buffer, word)

		if len(buffer) == contextLength+1 {
			err = insertTrainingData(outputDB, buffer, contextLength, outputTable)
			if err != nil {
				return fmt.Errorf("Could not insert training data for word ID = %d (%s): %v",
					word.WordID, word.Word, err)
			}
			buffer = buffer[1:] // Remove the oldest word
		}
	}

	buffer = append(buffer, endOfText)
	err = insertTrainingData(outputDB, buffer, contextLength, outputTable)
	if err != nil {
		return fmt.Errorf("Could not insert the end of text marker on story %d: %v", storyID, err)
	}
	// Clear the buffer at the end of the story
	buffer = buffer[:0]
	return nil
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

func insertTrainingData(db *sql.DB, buffer []WordData, contextLength int, outputTable string) (error) {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Error starting transaction: %v", err)
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
		return fmt.Errorf("Error inserting training data: %v", err)
	}

	// Update decodings table
	for _, word := range buffer {
		_, err := tx.Exec(`
			INSERT INTO decodings (path, word, usage_count)
			VALUES (?, ?, 1)
			ON CONFLICT(path, word) DO UPDATE SET usage_count = usage_count + 1
		`, word.Path, word.Word)
		if err != nil {
			return fmt.Errorf("Error updating decodings: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Error committing transaction: %v", err)
	}
	return nil
}
