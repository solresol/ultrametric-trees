// pkg/inference/inference.go
package inference

import (
	"database/sql"
	"fmt"
	"github.com/solresol/ultrametric-trees/pkg/decode"
	"github.com/solresol/ultrametric-trees/pkg/node"
	"log"
	"strings"
	"time"
)

// InferenceResult represents the output of inference on a single context
type InferenceResult struct {
	FinalNodeID   int
	PredictedPath string
	Depth         int
	InRegion      int
}

// ModelInference handles the inference process for a trained model
type ModelInference struct {
	db         *sql.DB
	nodesTable string
	nodes      []node.Node
	nodesTableLookup map[int]*node.Node
}

// NewModelInference creates a new inference engine from a trained model
func NewModelInference(db *sql.DB, nodesTable string, timeFilter time.Time) (*ModelInference, error) {
	nodes, err := node.FetchNodesAsOf(db, nodesTable, timeFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %v", err)
	}

	nodesTableLookup := make(map[int]*node.Node)
	for i := range nodes {
		nodesTableLookup[nodes[i].ID] = &nodes[i]
	}

	return &ModelInference{
		db:         db,
		nodesTable: nodesTable,
		nodes:      nodes,
		nodesTableLookup: nodesTableLookup,
	}, nil
}

func (m *ModelInference) Size() int {
	return len(m.nodes)
}

// InferSingle performs inference on a single context
func (m *ModelInference) InferSingle(context []string, verbose bool) (*InferenceResult, error) {
	currentNode := m.findRootNode()
	if currentNode == nil {
		return nil, fmt.Errorf("could not find root node")
	}

	depth := 0
	matches := 0
	// Traverse the tree based on context
	for currentNode.HasChildren {
		nextNode, inner, err := m.traverseNode(currentNode, context, verbose)
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
		FinalNodeID:   currentNode.ID,
		PredictedPath: currentNode.ExemplarValue.String,
		Depth:         depth,
		InRegion:      matches,
		// Loss:    1.0 - currentNode.Loss.Float64,
	}, nil
}

func (m *ModelInference) findRootNode() *node.Node {
	return m.nodesTableLookup[1] // Root node ID is 1
	return nil
}

func (m *ModelInference) traverseNode(current *node.Node, context []string, verbose bool) (*node.Node, bool, error) {
	if !current.ContextK.Valid {
		return nil, false, fmt.Errorf("invalid context index in node %d", current.ID)
	}
	contextIdx := int(current.ContextK.Int64 - 1) // Convert from 1-based to 0-based
	if contextIdx >= len(context) {
		return nil, false, fmt.Errorf("context index %d out of range", contextIdx)
	}
	// Check if the context matches the inner region
	contextValue := context[contextIdx]

	if strings.HasPrefix(current.InnerRegionPrefix.String, contextValue) {
		if verbose {
			decodedValue, _ := decode.DecodePath(m.db, contextValue)
			decodedRegion, _ := decode.DecodePath(m.db, current.InnerRegionPrefix.String)
			decodedExemplar, _ := decode.DecodePath(m.db, current.ExemplarValue.String)
			log.Printf("Node %d matched. It wanted context%d which is `%s' (%s) to be in %s (%s), which suggests predicting %s (%s)", current.ID, current.ContextK.Int64, decodedValue, contextValue, current.InnerRegionPrefix.String, decodedRegion, current.ExemplarValue.String, decodedExemplar)
			//log.Printf("It is inside that, so we will go to %d", current.InnerRegionNodeID.Int64)
		}
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
	if node, exists := m.nodesTableLookup[id]; exists {
		return node, nil
	}
	return nil, fmt.Errorf("node %d not found", id)
}
