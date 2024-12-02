// pkg/inference/inference.go
package inference

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"github.com/solresol/ultrametric-trees/pkg/node"
	"github.com/solresol/ultrametric-trees/pkg/decode"
)

// InferenceResult represents the output of inference on a single context
type InferenceResult struct {
	FinalNodeID int
	PredictedPath string
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
	currentNode := m.findRootNode()
	if currentNode == nil {
		return nil, fmt.Errorf("could not find root node")
	}

	depth := 0
	matches := 0
	// Traverse the tree based on context
	for currentNode.HasChildren {
		nextNode, inner, err := m.traverseNode(currentNode, context)
		if err != nil {
			return nil, err
		}
		currentNode = nextNode
		if inner {
			matches++
		}
		depth++
	}
	// decodedWord, err := decode.DecodePath(m.db, currentNode.ExemplarValue.String)
	//if err != nil {
	//	return nil, fmt.Errorf("Could not decode %s: %v", currentNode.ExemplarValue.String)
	//}
	// log.Printf("Conclusion after %d steps (%d matching): [Node %d] %s = %s", depth, matches, currentNode.ID, currentNode.ExemplarValue.String, decodedWord)
	// Return the prediction from the leaf node
	return &InferenceResult{
		FinalNodeID: currentNode.ID,
		PredictedPath: currentNode.ExemplarValue.String,
		// Loss:    1.0 - currentNode.Loss.Float64,
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

func (m *ModelInference) traverseNode(current *node.Node, context []string) (*node.Node, bool, error) {
	if !current.ContextK.Valid {
		return nil, false, fmt.Errorf("invalid context index in node %d", current.ID)
	}
	contextIdx := int(current.ContextK.Int64 - 1) // Convert from 1-based to 0-based
	if contextIdx >= len(context) {
		return nil, false, fmt.Errorf("context index %d out of range", contextIdx)
	}
	// Check if the context matches the inner region
	contextValue := context[contextIdx]
	decodedValue, _ := decode.DecodePath(m.db, contextValue)
	decodedRegion, _ := decode.DecodePath(m.db, current.InnerRegionPrefix.String)
	decodedExemplar, _ := decode.DecodePath(m.db, current.ExemplarValue.String)

	if strings.HasPrefix(current.InnerRegionPrefix.String, contextValue) {
		log.Printf("Node %d matched. It wanted context%d which is `%s' (%s) to be in %s (%s), which suggests predicting %s (%s)", current.ID, current.ContextK.Int64, decodedValue, contextValue, current.InnerRegionPrefix.String, decodedRegion, current.ExemplarValue.String, decodedExemplar)
		//log.Printf("It is inside that, so we will go to %d", current.InnerRegionNodeID.Int64)
		n, err := m.findNodeByID(int(current.InnerRegionNodeID.Int64))
		if err != nil {
			return nil, false, fmt.Errorf("Could not find inner node %d: %v", current.InnerRegionNodeID.Int64, err)
		}
		return n, true, nil
	}

	// If not in inner region, go to outer region
	//log.Printf("It is outside that, so we will go to %d", current.OuterRegionNodeID.Int64)
	n, err := m.findNodeByID(int(current.OuterRegionNodeID.Int64))
	if err != nil {
		return nil, false, fmt.Errorf("Could not find outer node %d: %v", current.InnerRegionNodeID.Int64, err)
	}
	return n, false, nil
}

func (m *ModelInference) findNodeByID(id int) (*node.Node, error) {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			return &m.nodes[i], nil
		}
	}
	return nil, fmt.Errorf("node %d not found", id)
}
