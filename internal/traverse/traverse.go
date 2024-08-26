package traverse

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/fluhus/gostuff/nlp/wordnet"
)

var (
	lemmaCounters map[string]int
	pathCache     map[string]string
)

func init() {
	lemmaCounters = make(map[string]int)
	pathCache = make(map[string]string)
}

func TraverseSynset(wn *wordnet.WordNet, synset *wordnet.Synset, db *sql.DB) {
	path := getPath(wn, synset)
	
	// Generate synset name in the format foo.n.01
	lemma := synset.Word[0]
	pos := synset.Pos
	lemmaKey := strings.ToLower(lemma) + "." + pos
	lemmaCounters[lemmaKey]++
	synsetName := fmt.Sprintf("%s.%s.%02d", lemma, pos, lemmaCounters[lemmaKey])

	// Save the current synset's path and name to the database
	savePath(db, path, synsetName)

	// Traverse hyponyms
	for _, pointer := range synset.Pointer {
		if pointer.Symbol == wordnet.Hyponym {
			hyponym := wn.Synset[pointer.Synset]
			TraverseSynset(wn, hyponym, db)
		}
	}
}

func getPath(wn *wordnet.WordNet, synset *wordnet.Synset) string {
	if path, ok := pathCache[synset.Offset]; ok {
		return path
	}

	var path string
	if synset.Offset == "entity.n.01" {
		path = "1"
	} else {
		// Find the hypernym
		var hypernym *wordnet.Synset
		for _, pointer := range synset.Pointer {
			if pointer.Symbol == wordnet.Hypernym {
				hypernym = wn.Synset[pointer.Synset]
				break
			}
		}
		if hypernym == nil {
			log.Printf("Warning: No hypernym found for %s", synset.Offset)
			return ""
		}

		// Get the path of the hypernym
		hypernymPath := getPath(wn, hypernym)
		if hypernymPath == "" {
			return ""
		}

		// Find the index of this synset among its siblings
		siblings := getSortedHyponyms(wn, hypernym)
		index := 1
		for i, sibling := range siblings {
			if sibling.Offset == synset.Offset {
				index = i + 1
				break
			}
		}

		path = fmt.Sprintf("%s.%d", hypernymPath, index)
	}

	pathCache[synset.Offset] = path
	return path
}

func getSortedHyponyms(wn *wordnet.WordNet, synset *wordnet.Synset) []*wordnet.Synset {
	var hyponyms []*wordnet.Synset
	for _, pointer := range synset.Pointer {
		if pointer.Symbol == wordnet.Hyponym {
			hyponyms = append(hyponyms, wn.Synset[pointer.Synset])
		}
	}
	sort.Slice(hyponyms, func(i, j int) bool {
		return hyponyms[i].Offset < hyponyms[j].Offset
	})
	return hyponyms
}

func savePath(db *sql.DB, path, synsetName string) {
	if path == "" {
		return
	}
	_, err := db.Exec(`
		INSERT OR IGNORE INTO synset_paths (path, synset_name)
		VALUES (?, ?)`,
		path, synsetName)
	
	if err != nil {
		log.Printf("Error saving path: %v\n", err)
	}
}
