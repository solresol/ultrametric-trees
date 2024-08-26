package traverse

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/fluhus/gostuff/nlp/wordnet"
)

func TraverseSynset(wn *wordnet.WordNet, synset *wordnet.Synset, path string, db *sql.DB) {
	// Generate synset name in the format foo.n.01
	synsetName := fmt.Sprintf("%s.%s.%s", synset.Word[0], synset.Pos, strings.TrimPrefix(synset.Offset, "0"))

	// Save the current synset's path and name to the database
	savePath(db, path, synsetName)

	// Recursively traverse each hyponym
	childNum := 1
	for _, pointer := range synset.Pointer {
		if pointer.Symbol == wordnet.Hyponym {
			hyponym := wn.Synset[pointer.Synset]
			newPath := fmt.Sprintf("%s.%d", path, childNum)
			TraverseSynset(wn, hyponym, newPath, childNum)
			childNum++
		}
	}
}

func savePath(db *sql.DB, path, synsetName string) {
	_, err := db.Exec(`
		INSERT INTO synset_paths (path, synset_name)
		VALUES (?, ?)`,
		path, synsetName)
	
	if err != nil {
		log.Printf("Error saving path: %v\n", err)
	}
}
