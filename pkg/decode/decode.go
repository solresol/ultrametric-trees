package decode

import (
	"database/sql"
	"fmt"
	"log"
	"github.com/solresol/ultrametric-trees/pkg/node"	
)

// DecodePath looks up a synset path in the decodings table and returns the most common word
func DecodePath(db *sql.DB, path string) (string, error) {
	var word string
	var count int
	err := db.QueryRow(`
		SELECT word, COUNT(*) as count 
		FROM decodings 
		WHERE path = ? 
		GROUP BY word 
		ORDER BY count DESC 
		LIMIT 1
	`, path).Scan(&word, &count)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no word found for path: %s", path)
	}
	if err != nil {
		return "", fmt.Errorf("error decoding path: %v", err)
	}

	return word, nil
}

// ShowContext takes a context array and prints each element decoded to its word form
func ShowContext(db *sql.DB, context []string) (string, error) {
	s := ""
	for _, path := range context {
		word, err := DecodePath(db, path)
		if err != nil {
			word = fmt.Sprintf("<unknown:%s>", path)
		}
		s = fmt.Sprintf("%s %s", word, s)
	}
	return s, nil
}

func NodeAncestry(db *sql.DB, n node.Node) (string, error) {
	ancestors, err := node.FetchAncestry(db, n)
	if err != nil {
		return "", err
	}
	display := "{} -> "
	log.Printf("There were %d ancestors for node %d", len(ancestors), n.ID)
	for idx, a := range ancestors {
		if idx == len(ancestors) - 1 {
			// Then we just show this node.
			decodedExemplar, err := DecodePath(db, a.ExemplarValue.String)
			if err != nil {
				return display, fmt.Errorf("Failed to decode final descendant: %v", err)
			}
			display = fmt.Sprintf("%s [Node %d says 'predict %s (%s)']", display, a.ID, a.ExemplarValue.String, decodedExemplar)
			continue
		}
		nextAncestor := ancestors[idx + 1]
		decodedRegion, err := DecodePath(db, a.InnerRegionPrefix.String)
		if err != nil {
			// log.Printf("Couldn't decode %s: %v", a.InnerRegionPrefix.String, err)
			// We'll just assume we're OK
			decodedRegion = fmt.Sprintf("<%s>", a.InnerRegionPrefix.String)
		}
		if nextAncestor.ID == int(a.InnerRegionNodeID.Int64) {
			display = fmt.Sprintf("%s [Node %d] if context%d is inside %s (%s) AND ", display, a.ID, a.ContextK.Int64, a.InnerRegionPrefix.String, decodedRegion)
		} else {
			display = fmt.Sprintf("%s [Node %d] if context%d is outside %s (%s) AND ", display, a.ID, a.ContextK.Int64, a.InnerRegionPrefix.String, decodedRegion)
		}
	}
	return display, nil
}
