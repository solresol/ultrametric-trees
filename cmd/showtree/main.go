package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/solresol/ultrametric-trees/pkg/exemplar"
	"github.com/solresol/ultrametric-trees/pkg/node"
)

func main() {
	dbPath := flag.String("database", "", "Path to the SQLite database file")
	tableName := flag.String("table", "nodes", "Name of the nodes table")
	timestamp := flag.String("time", "", "Timestamp to display nodes (format: 2006-01-02 15:04:05)")
	flag.Parse()

	if *timestamp == "" {
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		timestamp = &currentTime
	}

	if *dbPath == "" {
		log.Fatal("Please provide the filename of the database")
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	t, err := time.Parse("2006-01-02 15:04:05", *timestamp)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %v", err)
	}

	activeNodes, err := node.FetchNodesAsOf(db, *tableName, t)
	if err != nil {
		log.Fatalf("Error fetching filtered nodes: %v", err)
	}
	nodeMap := make(map[int]node.Node)
	for _, n := range activeNodes {
		nodeMap[n.ID] = n
	}

	err = displayTree(db, nodeMap)
	if err != nil {
		log.Fatalf("Could not displayTree: %v", err)
	}
}

func displayTree(db *sql.DB, nodeMap map[int]node.Node) error {
	// Start the recursive display from the root node
	// err := displayNodeAndChildren(db, 0, int(exemplar.RootNodeID), nodeMap, "", "[DEFAULT]", false)
	err := displayNodeRecursively(db, 0, int(exemplar.RootNodeID), nodeMap, "Root node")
	return err

}

// This almost exactly duplicates decode.DecodePath
func getWordFromPath(db *sql.DB, path string) (bool, string, error) {
	var w string
	var c int
	err := db.QueryRow("select word, count(*) from decodings where path = ? group by word order by 2 desc limit 1", path).Scan(&w, &c)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("Could not read word from path: %v", err)
	}
	return true, w, nil
}

func findParent(nodeMap map[int]node.Node, childId int) (int, bool) {
	// Dreadfully inefficient
	for nodeId, nodeObj := range nodeMap {
		if nodeObj.InnerRegionNodeID.Valid && int(nodeObj.InnerRegionNodeID.Int64) == childId {
			return nodeId, true
		}
		if nodeObj.OuterRegionNodeID.Valid && int(nodeObj.OuterRegionNodeID.Int64) == childId {
			return nodeId, false
		}
	}
	return -1, false
}

type InnerRegion struct {
	RegionPrefix string
	RegionNodeID int
}

func flattenDescendantsWithSameContext(nodeMap map[int]node.Node, currentNode node.Node) ([]InnerRegion, int) {
	var descendants []InnerRegion
	loopNode := currentNode

	if !loopNode.ContextK.Valid {
		// That means I have no children. I can't really start. Maybe this should be an error?
		return descendants, -1
	}
	context := currentNode.ContextK.Int64

	for {
		if !loopNode.ContextK.Valid {
			return descendants, -1
		}
		if loopNode.ContextK.Int64 != context {
			return descendants, loopNode.ID
		}

		outerChild, exists := nodeMap[int(loopNode.OuterRegionNodeID.Int64)]
		innerRegion := InnerRegion{RegionPrefix: loopNode.InnerRegionPrefix.String,
			RegionNodeID: int(loopNode.InnerRegionNodeID.Int64)}
		descendants = append(descendants, innerRegion)
		if exists {
			loopNode = outerChild
		} else {
			// No more descendants... outerChild doesn't exist, possibly because
			// we have a moment-in-time snapshot
			return descendants, -1
		}
	}
}

func displayInnerDescendants(db *sql.DB, depth int, regions []InnerRegion, nodeMap map[int]node.Node, context int) error {
	prefix := strings.Repeat(" ", depth)
	//fmt.Printf("%sTHERE ARE %d DESCENDANTS AT Depth %d,\n", prefix, len(regions), depth)
	for _, value := range regions {
		exists, region, err := getWordFromPath(db, value.RegionPrefix)
		if err != nil {
			return fmt.Errorf("Failed to get word %s from path: %v", value.RegionPrefix, err)
		}
		if !exists {
			region = value.RegionPrefix
		}
		thisMessage := fmt.Sprintf("%sNode %d (child at Depth %d, when context%d = %s)", prefix, value.RegionNodeID, depth, context, region)
		err = displayNodeRecursively(db, depth, value.RegionNodeID, nodeMap, thisMessage)
		if err != nil {
			return fmt.Errorf("Could not display inner descendant %d: %v", value.RegionNodeID, err)
		}
	}
	return nil
}

func displayOuterDescendant(db *sql.DB, depth int, outerNodeID int, nodeMap map[int]node.Node, context int, regionsWeAreOutOf []InnerRegion) error {
	prefix := strings.Repeat(" ", depth)
	displayRegionsWeAreOutOf := ""
	for idx, value := range regionsWeAreOutOf {
		exists, region, err := getWordFromPath(db, value.RegionPrefix)
		if err != nil {
			return fmt.Errorf("Failed to get word %s from path: %v", value.RegionPrefix, err)
		}
		if !exists {
			region = value.RegionPrefix
		}
		if idx == 0 {
			displayRegionsWeAreOutOf = region
		} else {
			displayRegionsWeAreOutOf = fmt.Sprintf("%s; %s", displayRegionsWeAreOutOf, region)
		}
	}
	myMessage := fmt.Sprintf("%sNode %d (child at Depth %d, when context%d is not in {%s})", prefix, outerNodeID, depth, context, displayRegionsWeAreOutOf)
	err := displayNodeRecursively(db, depth, outerNodeID, nodeMap, myMessage)
	if err != nil {
		return fmt.Errorf("Could not display outer descendant %d: %v", outerNodeID, err)
	}
	return nil
}

func displayNodeRecursively(db *sql.DB, depth int, nodeID int, nodeMap map[int]node.Node, nodeText string) error {
	prefix := strings.Repeat(" ", depth)
	n, exists := nodeMap[nodeID]
	if !exists {
		return fmt.Errorf("%s- Node %d: Not found\n", prefix, nodeID)
	}
	exists, suggestion, err := getWordFromPath(db, n.ExemplarValue.String)
	if err != nil {
		return fmt.Errorf("Could not get word from path for %s: %v", n.ExemplarValue.String, err)
	}
	showChildren := true
	if !n.InnerRegionNodeID.Valid || !n.OuterRegionNodeID.Valid {
		showChildren = false
	} else {
		_, exists = nodeMap[int(n.InnerRegionNodeID.Int64)]
		if !exists {
			showChildren = false
		}
		_, exists = nodeMap[int(n.OuterRegionNodeID.Int64)]
		if !exists {
			showChildren = false
		}
	}

	if !showChildren {
		fmt.Printf("%s -- predict the word *%s*, loss = %f, %d training samples\n", nodeText, suggestion, n.Loss.Float64, n.DataQuantity.Int64)
		return nil
	}
	fmt.Printf("%s -- (obsolete: predicted the word *%s*, loss = %f, %d training samples)\n", nodeText, suggestion, n.Loss.Float64, n.DataQuantity.Int64)
	sameContextDescendants, outer := flattenDescendantsWithSameContext(nodeMap, n)
	err = displayInnerDescendants(db, depth+1, sameContextDescendants, nodeMap, int(n.ContextK.Int64))
	if err != nil {
		return fmt.Errorf("Error while displaying inner descendants: %v", err)
	}
	if outer != -1 {
		err = displayOuterDescendant(db, depth+1, outer, nodeMap, int(n.ContextK.Int64), sameContextDescendants)
		if err != nil {
			return err
		}
	}
	return nil
}

func displayNodeAndChildren(db *sql.DB, depth int, nodeID int, nodeMap map[int]node.Node, insideMessage string, outsideOfMessage string, nodeWasInside bool) error {
	prefix := strings.Repeat(" ", depth)
	n, exists := nodeMap[nodeID]
	if !exists {
		return fmt.Errorf("%s- Node %d: Not found\n", prefix, nodeID)
	}
	exists, suggestion, err := getWordFromPath(db, n.ExemplarValue.String)
	if err != nil {
		return err
	}
	if !exists {
		suggestion = n.ExemplarValue.String
	}

	showChildren := true
	if !n.InnerRegionNodeID.Valid || !n.OuterRegionNodeID.Valid {
		showChildren = false
	} else {
		_, exists = nodeMap[int(n.InnerRegionNodeID.Int64)]
		if !exists {
			showChildren = false
		}
		_, exists = nodeMap[int(n.OuterRegionNodeID.Int64)]
		if !exists {
			showChildren = false
		}
	}
	var myMessage string
	if nodeWasInside {
		myMessage = insideMessage
	} else {
		myMessage = outsideOfMessage
	}
	parent, wasInsideParent := findParent(nodeMap, nodeID)
	if wasInsideParent != nodeWasInside {
		fmt.Printf("Some sort of logic error on node %d\n")
	}
	if !showChildren {
		fmt.Printf("%s- Depth %d, Node %d, Parent %d: suggestion is %s when %s, loss %f, %d usages\n", prefix, depth, n.ID, parent, suggestion, myMessage, n.Loss.Float64, n.DataQuantity.Int64)
		return nil
	}

	exists, region, err := getWordFromPath(db, n.InnerRegionPrefix.String)
	if !exists {
		region = n.InnerRegionPrefix.String
	}
	fmt.Printf("%s- Depth %d, Node %d, Parent %d: {suggested [%s] when %s, loss %f, %d usages}\n", prefix, depth, n.ID, parent, suggestion, myMessage,
		n.Loss.Float64, n.DataQuantity.Int64)
	sameContextDescendants, outer := flattenDescendantsWithSameContext(nodeMap, n)
	err = displayInnerDescendants(db, depth, sameContextDescendants, nodeMap, int(n.ContextK.Int64))
	if err != nil {
		return fmt.Errorf("Error while displaying inner descendants: %v", err)
	}
	err = displayOuterDescendant(db, depth, outer, nodeMap, int(n.ContextK.Int64), sameContextDescendants)
	grandchildAdoption := false

	var outerChildMessage string
	if grandchildAdoption {
		outerChildMessage = fmt.Sprintf("(%s,%s)", outsideOfMessage, region)
	} else {
		outerChildMessage = fmt.Sprintf("context%d is outside %s", n.ContextK.Int64, region)
	}

	insideChildMessage := fmt.Sprintf("context%d is inside [%s]", n.ContextK.Int64, region)

	err = displayNodeAndChildren(db, depth+1, int(n.InnerRegionNodeID.Int64), nodeMap, insideChildMessage, outerChildMessage, true)
	if err != nil {
		return err
	}

	if grandchildAdoption {
		err = displayNodeAndChildren(db, depth, int(n.OuterRegionNodeID.Int64), nodeMap, "", outerChildMessage, false)
	} else {
		err = displayNodeAndChildren(db, depth+1, int(n.OuterRegionNodeID.Int64), nodeMap, "", outerChildMessage, false)
	}

	if err != nil {
		return err
	}
	return nil
}
