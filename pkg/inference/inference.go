// pkg/inference/inference.go
package inference

import (
	"database/sql"
	"fmt"
	"log"
	"github.com/solresol/ultrametric-trees/pkg/node"
)

// InferenceResult represents the output of inference on a single context
type InferenceResult struct {
	ContextID     int
	PredictedPath string
	Loss    float64
}

// ModelInference handles the inference process for a trained model
type ModelInference struct {
	db         *sql.DB
	nodesTable string
	nodes      []node.Node
}

// NewModelInference creates a new inference engine from a trained model
func NewModelInference(db *sql.DB, nodesTable string) (*ModelInference, error) {
	nodes, err := node.FetchNodes(db, nodesTable)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %v", err)
	}

	return &ModelInference{
		db:         db,
		nodesTable: nodesTable,
		nodes:      nodes,
	}, nil
}

// InferSingle performs inference on a single context
func (m *ModelInference) InferSingle(context []string) (*InferenceResult, error) {
	contextString, err := m.ShowContext(context)
	if err != nil {
		return nil, err
	}
	log.Printf("INFERING %s", contextString)
	currentNode := m.findRootNode()
	if currentNode == nil {
		return nil, fmt.Errorf("could not find root node")
	}

	// Traverse the tree based on context
	for currentNode.HasChildren {
		nextNode, err := m.traverseNode(currentNode, context)
		if err != nil {
			return nil, err
		}
		currentNode = nextNode
	}
	decodedWord, err := m.DecodePath(currentNode.ExemplarValue.String)
	if err != nil {
		return nil, fmt.Errorf("Could not decode %s: %v", currentNode.ExemplarValue.String)
	}
	log.Printf("Conclusion: %s = %s", currentNode.ExemplarValue.String, decodedWord)

	// Return the prediction from the leaf node
	return &InferenceResult{
		PredictedPath: currentNode.ExemplarValue.String,
		Loss:    1.0 - currentNode.Loss.Float64,
	}, nil
}

func (m *ModelInference) findRootNode() *node.Node {
	for i := range m.nodes {
		if m.nodes[i].ID == 1 { // Root node ID is 1
			return &m.nodes[i]
		}
	}
	return nil
}

func (m *ModelInference) traverseNode(current *node.Node, context []string) (*node.Node, error) {
	if !current.ContextK.Valid {
		return nil, fmt.Errorf("invalid context index in node %d", current.ID)
	}
	contextIdx := int(current.ContextK.Int64 - 1) // Convert from 1-based to 0-based
	if contextIdx >= len(context) {
		return nil, fmt.Errorf("context index %d out of range", contextIdx)
	}
	// Check if the context matches the inner region
	contextValue := context[contextIdx]
	decodedValue, _ := m.DecodePath(contextValue)
	log.Printf("Node %d looks at context K = %d. It is asking whether `%s' (%s) is in %s", current.ID, current.ContextK.Int64, decodedValue, contextValue, current.InnerRegionPrefix.String)

	if matches, err := m.matchesInnerRegion(contextValue, current.InnerRegionPrefix.String); err != nil {
		return nil, err
	} else if matches {
		log.Printf("It is inside that, so we will go to %d", current.InnerRegionNodeID.Int64)
		return m.findNodeByID(int(current.InnerRegionNodeID.Int64))
	}

	// If not in inner region, go to outer region
	log.Printf("It is outside that, so we will go to %d", current.OuterRegionNodeID.Int64)
	return m.findNodeByID(int(current.OuterRegionNodeID.Int64))
}

func (m *ModelInference) findNodeByID(id int) (*node.Node, error) {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			return &m.nodes[i], nil
		}
	}
	return nil, fmt.Errorf("node %d not found", id)
}


// DecodePath looks up a synset path in the decodings table and returns the most common word
func (m *ModelInference) DecodePath(path string) (string, error) {
	var word string
	var count int
	err := m.db.QueryRow(`
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
func (m *ModelInference) ShowContext(context []string) (string, error) {
	s := ""
	fmt.Println("Context:")
	for _, path := range context {
		word, err := m.DecodePath(path)
		if err != nil {
			word = fmt.Sprintf("<unknown:%s>", path)
		}
		s = fmt.Sprintf("%s %s", word, s)
	}
	return s, nil
}


func (m *ModelInference) matchesInnerRegion(contextValue, regionPrefix string) (bool, error) {
	// Query the database to check if the context value matches the region prefix
	var count int
	err := m.db.QueryRow(`
		SELECT COUNT(*) FROM decodings 
		WHERE path = ? AND word = ?
	`, regionPrefix, contextValue).Scan(&count)
	
	if err != nil {
		return false, fmt.Errorf("error checking region match: %v", err)
	}
	
	return count > 0, nil
}

