package processor

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type Processor struct {
	inputDB    *sql.DB
	traverseDB *sql.DB
	outputDB   *sql.DB
}

func NewProcessor(inputPath, traversePath, outputPath string) (*Processor, error) {
	inputDB, err := sql.Open("sqlite3", inputPath)
	if err != nil {
		return nil, fmt.Errorf("error opening input database: %w", err)
	}

	traverseDB, err := sql.Open("sqlite3", traversePath)
	if err != nil {
		inputDB.Close()
		return nil, fmt.Errorf("error opening traverse database: %w", err)
	}

	outputDB, err := sql.Open("sqlite3", outputPath)
	if err != nil {
		inputDB.Close()
		traverseDB.Close()
		return nil, fmt.Errorf("error opening output database: %w", err)
	}

	return &Processor{
		inputDB:    inputDB,
		traverseDB: traverseDB,
		outputDB:   outputDB,
	}, nil
}

func (p *Processor) Close() {
	p.inputDB.Close()
	p.traverseDB.Close()
	p.outputDB.Close()
}

func (p *Processor) Process() error {
	// Create output table
	_, err := p.outputDB.Exec(`
		CREATE TABLE IF NOT EXISTS training (
			predecessor16 TEXT, predecessor15 TEXT, predecessor14 TEXT,
			predecessor13 TEXT, predecessor12 TEXT, predecessor11 TEXT,
			predecessor10 TEXT, predecessor9 TEXT, predecessor8 TEXT,
			predecessor7 TEXT, predecessor6 TEXT, predecessor5 TEXT,
			predecessor4 TEXT, predecessor3 TEXT, predecessor2 TEXT,
			predecessor1 TEXT, target TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating output table: %w", err)
	}

	// Process sentences
	rows, err := p.inputDB.Query("SELECT id, sentence FROM sentences")
	if err != nil {
		return fmt.Errorf("error querying sentences: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sentenceID int
		var sentence string
		err := rows.Scan(&sentenceID, &sentence)
		if err != nil {
			return fmt.Errorf("error scanning sentence row: %w", err)
		}

		err = p.processSentence(sentenceID)
		if err != nil {
			return fmt.Errorf("error processing sentence %d: %w", sentenceID, err)
		}
	}

	return nil
}

func (p *Processor) processSentence(sentenceID int) error {
	// Query words in the sentence
	rows, err := p.inputDB.Query("SELECT word, resolved_synset FROM words WHERE sentence_id = ? ORDER BY word_number", sentenceID)
	if err != nil {
		return fmt.Errorf("error querying words: %w", err)
	}
	defer rows.Close()

	var words []string
	var synsets []string

	for rows.Next() {
		var word, synset string
		err := rows.Scan(&word, &synset)
		if err != nil {
			return fmt.Errorf("error scanning word row: %w", err)
		}

		if synset == "" && !isPunctuationOrPronoun(word) {
			return nil // Skip this sentence
		}

		words = append(words, word)
		synsets = append(synsets, synset)
	}

	// Process words and generate training data
	for i := range words {
		predecessors := make([]string, 16)
		for j := 0; j < 16; j++ {
			if i-j-1 >= 0 {
				predecessors[15-j] = synsets[i-j-1]
			} else {
				predecessors[15-j] = ""
			}
		}

		// Insert into output database
		_, err := p.outputDB.Exec(`
			INSERT INTO training (
				predecessor16, predecessor15, predecessor14, predecessor13,
				predecessor12, predecessor11, predecessor10, predecessor9,
				predecessor8, predecessor7, predecessor6, predecessor5,
				predecessor4, predecessor3, predecessor2, predecessor1,
				target
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, predecessors[0], predecessors[1], predecessors[2], predecessors[3],
			predecessors[4], predecessors[5], predecessors[6], predecessors[7],
			predecessors[8], predecessors[9], predecessors[10], predecessors[11],
			predecessors[12], predecessors[13], predecessors[14], predecessors[15],
			synsets[i])

		if err != nil {
			return fmt.Errorf("error inserting training data: %w", err)
		}
	}

	return nil
}
