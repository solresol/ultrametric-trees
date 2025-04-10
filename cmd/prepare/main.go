package main

import (
	"crypto/sha256"
	"database/sql"
	"github.com/surge/porter2"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type WordData struct {
	WordID int
	Word   string
	Synset sql.NullString
	Path   string
}

var hashedPseudoSynsetPrefix = map[string]string{
	"(noun.other)":        "1.",
	"(verb.other)":        "3.",
	"(propernoun.other)":  "1.3.",
	"(preposition.other)": "6.",
	"(adjective.other)":   "2.",
	"(adverb.other)":      "4.",
	"(other.other)":       "8.",
}

func hashThing(thing string) string {
	stemmed := porter2.Stem(thing)
	hash := sha256.Sum256([]byte(stemmed))
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

func getPath(db *sql.DB, wordID int, word string, synset sql.NullString) (WordData, error) {
	// log.Printf("The synset for word %s (%d) is %v", word, wordID, synset)
	if !synset.Valid || synset.String == "" {
		// log.Printf("Cannot make a useful path for %s (%d) because synset is empty", word, wordID)
		return WordData{WordID: wordID, Word: word, Synset: synset, Path: ""}, nil
	}
	//log.Printf("The word %s (%d) does have a valid synset", word, wordID)

	fields := strings.Split(synset.String, ".")
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
	if isEnumeratedPseudoSynset(synset.String) {
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

	prefix, ok := hashedPseudoSynsetPrefix[synset.String]
	if !ok {
		return WordData{}, fmt.Errorf("unknown pseudo-synset: %s", synset)
	}
	hashedWord := hashThing(word)
	return WordData{WordID: wordID, Word: word, Synset: synset, Path: prefix + hashedWord}, nil
}

type OutputChoice string

const (
    OutputPaths OutputChoice = "paths"
    Words OutputChoice = "words"
    OutputHashes  OutputChoice = "hash"
)

func IsValidOutputChoice(choice string) bool {
    switch OutputChoice(choice) {
    case OutputPaths, Words, OutputHashes:
        return true
    default:
        return false
    }
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
	outputChoice := flag.String("output-choice", "paths", "Whether to output paths (the experiment) or words (the baseline). Defaults to paths")
	flag.Parse()

	if *inputDB == "" || *outputDB == "" {
		log.Fatal("Both --input-database and --output-database are required")
	}

	if !IsValidOutputChoice(*outputChoice) {
		log.Fatal("Error: --output-choice must be one of the defined OutputChoice constants.")
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

	log.Printf("Creating tables")

	createOutputTables(outputConn, *contextLength, *outputTable)

	log.Printf("Getting stories")

	storyChan, err := getStories(inputConn, *modulo, *congruent)
	if err != nil {
		log.Fatalf("Error getting stories: %v", err)
	}

	processedCount := 0
	startTime := time.Now()
	newRecordCount := 0
	overlapRecordCount := 0

	for storyIteration := range storyChan {
		percentComplete := 100.0 * float64(storyIteration.TotalStories-storyIteration.NumberLeftToCheck) / float64(storyIteration.TotalStories)
		if storyIteration.StatusOnly {
			log.Printf("Skipping stories where all words are unresolved. %d stories remaining to examine. %.2f%% complete.", storyIteration.NumberLeftToCheck, percentComplete)
			continue
		}
		storyID := storyIteration.StoryID
		processedCount++
		outputChoice := OutputChoice(*outputChoice)
		newlyAdded, newOverlaps, err := processStory(inputConn, outputConn, storyID, *contextLength, *outputTable, outputChoice)
		newRecordCount += newlyAdded
		overlapRecordCount += newOverlaps
		if err != nil {
			log.Fatalf("Could not process story %d: %v", storyID, err)
		}
		elapsed := time.Since(startTime)
		storiesPerSecond := float64(processedCount) / elapsed.Seconds()
		log.Printf("Progress (#%d, %.2f stories/sec), %d new records, %d overlapping records, %.2f%% complete", processedCount, storiesPerSecond, newRecordCount, overlapRecordCount, percentComplete)
	}

	log.Printf("Data preparation completed successfully. %d new training records, %d existing training records untouched", newRecordCount, overlapRecordCount)
}

func createOutputTables(db *sql.DB, contextLength int, outputTable string) {
	// Create training_data table
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, targetword TEXT, targetword_id INTEGER", outputTable)
	for i := 1; i <= contextLength; i++ {
		query += fmt.Sprintf(", context%d TEXT", i)
	}
	query += ", when_added datetime default current_timestamp)"
	log.Printf("Executing %s\n", query)

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

	query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_targetword ON %s (targetword)", outputTable, outputTable)
	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating targetword index: %v", err)
	}

	query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_by_time ON %s (when_added)", outputTable, outputTable)
	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating when_added index: %v", err)
	}
	log.Printf("All indexes are in place")
}

func resolvedWordsInStory(db *sql.DB, storyID int) (int, error) {
	query := `SELECT count(resolved_synset) FROM words JOIN sentences s ON sentence_id = s.id WHERE s.story_id = ?`
	var count int
	err := db.QueryRow(query, storyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting resolved words in story %d: %v", storyID, err)
	}
	return count, nil
}

type StoryIteration struct {
	StoryID           int
	NumberLeftToCheck int
	TotalStories      int
	StatusOnly        bool
}

func getStories(db *sql.DB, modulo, congruent int) (<-chan StoryIteration, error) {

	query := "SELECT DISTINCT id FROM stories ORDER BY id"
	if modulo > 0 {
		query = fmt.Sprintf("SELECT DISTINCT id FROM stories WHERE id %% %d = %d ORDER BY id", modulo, congruent)
	}
	log.Printf("Getting stories by running %s", query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	var storyIDs []int
	defer rows.Close()

	for rows.Next() {
		var storyID int
		if err := rows.Scan(&storyID); err != nil {
			return nil, fmt.Errorf("Could not scan storyID: %v", err)
		}
		storyIDs = append(storyIDs, storyID)
	}

	storyChannel := make(chan StoryIteration, 100) // Buffer size of 100 can be adjusted
	go func() {
		defer close(storyChannel)
		sinceLastMessage := 0
		for idx, storyID := range storyIDs {
			resolvedCount, err := resolvedWordsInStory(db, storyID)
			if err != nil {
				log.Printf("Error counting words in story ID: %v", err)
				// Assume it's zero, which will mean we will skip it
			}
			var storyIteration StoryIteration
			storyIteration.StoryID = storyID
			storyIteration.NumberLeftToCheck = len(storyIDs) - idx - 1
			storyIteration.StatusOnly = resolvedCount == 0
			storyIteration.TotalStories = len(storyIDs)
			if !storyIteration.StatusOnly || sinceLastMessage > 1000 || (idx == len(storyIDs)-1) {
				storyChannel <- storyIteration
				sinceLastMessage = 0
			} else {
				sinceLastMessage++
			}
		}
	}()

	return storyChannel, nil
}

func processStory(inputDB, outputDB *sql.DB, storyID, contextLength int, outputTable string, outputChoice OutputChoice) (int, int, error) {
	log.Printf("Processing story %d with context length %d into %s", storyID, contextLength, outputTable)
	words, annotationCount, err := getWordsForStory(inputDB, storyID)
	if err != nil {
		return 0, 0, fmt.Errorf("Error getting words for story %d: %v", storyID, err)
	}

	if annotationCount == 0 {
		log.Printf("Story %d has no annotated words in it. Skipping.", storyID)
		return 0, 0, nil
	}

	log.Printf("Found %d words in story %d, of which %d were annotated", len(words), storyID, annotationCount)

	// Fake up a word ID for text position markers
	startOfTextMarker := -(storyID * 2)
	endOfTextMarker := -(storyID * 2) - 1
	startOfText, err := getPath(inputDB, startOfTextMarker, "<START-OF-TEXT>", sql.NullString{
		Valid:  true,
		String: "(punctuation.other)",
	})
	if err != nil {
		return 0, 0, fmt.Errorf("Could not get the <START-OF-TEXT> marker: %v\n", err)
	}
	// fmt.Printf("Start of text = %s\n", startOfText.Path)

	endOfText, err := getPath(inputDB, endOfTextMarker, "<END-OF-TEXT>", sql.NullString{
		Valid:  true,
		String: "(punctuation.other)",
	})
	if err != nil {
		return 0, 0, fmt.Errorf("Could not get the <END-OF-TEXT> marker: %v\n", err)
	}

	buffer := make([]WordData, 0, contextLength+1)
	for i := 0; i < contextLength; i++ {
		buffer = append(buffer, startOfText)
	}
	// log.Printf("Starting the iteration over those words in story %d", storyID)

	overlapSize := 0
	newlyAddedData := 0
	for idx, word := range words {
		if (idx%100 == 0) || (idx == len(words)-1) {
			//log.Printf("Adding words for story %d. Progress: %d/%d", storyID, idx+1, len(words))
		}
		if word.Path == "" {
			//log.Printf("SKIPPING  %s (%d) path=%s. Buffer length %d", word.Word, word.WordID, word.Path, len(buffer))

			// If we can't find the path for a word, then we can't use this
			// as a prediction token, nor can we use it for predicting anything.
			// We'll have to refill the buffer from scratch

			// Addendum. Is this true? Maybe the empty path is a thing
			// we can use.
			buffer = buffer[:0] // Clear the buffer
			continue
		}

		buffer = append(buffer, word)
		// log.Printf("Added %s (%d) path=%s. Buffer length %d", word.Word, word.WordID, word.Path, len(buffer))
		// This isn't quite right either. We might want to train on texts
		// that are shorter than the contextLength (the beginning of a story
		// for example).
		if len(buffer) == contextLength+1 {
			existsAlready, err := insertTrainingData(outputDB, buffer, contextLength, outputTable, outputChoice)
			if err != nil {
				return newlyAddedData, overlapSize, fmt.Errorf("Could not insert training data for word ID = %d (%s): %v",
					word.WordID, word.Word, err)
			}
			buffer = buffer[1:] // Remove the oldest word
			if existsAlready {
				overlapSize++
			} else {
				newlyAddedData++
			}
		}
	}

	//log.Printf("Appending endOfText marker. Buffer length is %d", len(buffer))
	buffer = append(buffer, endOfText)
	if len(buffer) == contextLength+1 {
		existsAlready, err := insertTrainingData(outputDB, buffer, contextLength, outputTable, outputChoice)
		if err != nil {
			return newlyAddedData, overlapSize, fmt.Errorf("Could not insert the end of text marker on story %d: %v", storyID, err)
		}
		if existsAlready {
			overlapSize++
		} else {
			newlyAddedData++
		}
	}
	// Clear the buffer at the end of the story. Not sure if this is necessary, since buffer
	// is a local variable.
	buffer = buffer[:0]

	log.Printf("Finished processing story %d, added %d training records, %d records already present", storyID, newlyAddedData, overlapSize)
	return newlyAddedData, overlapSize, nil
}

func getWordsForStory(db *sql.DB, storyID int) ([]WordData, int, error) {
	query := `
		SELECT w.id, w.word, w.resolved_synset
		FROM words w
		JOIN sentences s ON w.sentence_id = s.id
		WHERE s.story_id = ?
		ORDER BY s.sentence_number, w.word_number
	`

	// log.Printf("Getting words for story %d", storyID)
	rows, err := db.Query(query, storyID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	annotationCount := 0
	var words []WordData
	for rows.Next() {
		var wordID int
		var word string
		var synset sql.NullString
		if err := rows.Scan(&wordID, &word, &synset); err != nil {
			return nil, annotationCount, err
		}

		wordData, err := getPath(db, wordID, word, synset)
		if err != nil {
			log.Printf("Error getting path for word %s (ID: %d): %v", word, wordID, err)
			continue
		}
		if wordData.Path != "" {
			annotationCount += 1
		}
		words = append(words, wordData)
	}

	return words, annotationCount, nil
}

func insertTrainingData(db *sql.DB, buffer []WordData, contextLength int, outputTable string, outputChoice OutputChoice) (bool, error) {
	query := fmt.Sprintf("select count(*) from %s where targetword_id = %d", outputTable, buffer[contextLength].WordID)
	var numberOfAppearances int
	err := db.QueryRow(query).Scan(&numberOfAppearances)
	if err != nil {
		return false, fmt.Errorf("Could not count the appearances of targetword_id = %d: %v", buffer[contextLength].WordID, err)
	}
	if numberOfAppearances > 1 {
		return true, fmt.Errorf("The word ID = %d appears %d times in the database", buffer[contextLength].WordID, numberOfAppearances)
	}
	if numberOfAppearances == 1 {
		// It already exists, quite normal situation. Don't need to insert anything
		return true, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("Error starting transaction: %v", err)
	}

	// Insert into training_data table
	query = fmt.Sprintf("INSERT INTO %s (targetword", outputTable)
	for i := 1; i <= contextLength; i++ {
		query += fmt.Sprintf(", context%d", i)
	}
	query += ", targetword_id) VALUES (?"
	for i := 1; i <= contextLength; i++ {
		query += ", ?"
	}
	query += ",?)"

	args := make([]interface{}, contextLength+2)
	// Helper function to get data based on output choice
	getDataForOutput := func(wordData WordData, outputChoice OutputChoice) string {
		if outputChoice == OutputHashes {
			return hashThing(wordData.Word)
		} else if outputChoice == OutputPaths {
			return wordData.Path
		}
		return wordData.Word
	}

	// We always want to have the targetword being a path. We always want to predict
	// paths
	args[0] = getDataForOutput(buffer[contextLength], OutputPaths)
	// But that the thing that we predict from... that changes.
	for i := 0; i < contextLength; i++ {
		args[i+1] = getDataForOutput(buffer[contextLength-1-i], outputChoice)
	}
	args[contextLength+1] = buffer[contextLength].WordID
	_, err = tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return false, fmt.Errorf("Error inserting training data: %v", err)
	}

	// Update decodings table
	if outputChoice != OutputHashes { // Only update decodings if we're not using hash
		for _, word := range buffer {
			_, err := tx.Exec(`
				INSERT INTO decodings (path, word, usage_count)
				VALUES (?, ?, 1)
				ON CONFLICT(path, word) DO UPDATE SET usage_count = usage_count + 1
			`, word.Path, word.Word)
			if err != nil {
				return false, fmt.Errorf("Error updating decodings: %v", err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return false, fmt.Errorf("Error committing transaction: %v", err)
	}
	return false, nil
}
