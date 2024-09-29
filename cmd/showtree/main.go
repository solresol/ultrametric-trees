package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/solresol/ultrametric-trees/pkg/node"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
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
		log.Fatal("Please provide both database path and timestamp")
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	nodes, err := node.FetchNodes(db, *tableName)
	if err != nil {
		log.Fatalf("Error fetching nodes: %v", err)
	}

	t, err := time.Parse("2006-01-02 15:04:05", *timestamp)
	if err != nil {
		log.Fatalf("Error parsing timestamp: %v", err)
	}

	activeNodes := node.FilterNodes(nodes, t, true)
	err = displayTree(db, activeNodes)
	if err != nil {
		log.Fatalf("Could not displayTree: %v", err)
	}
}

func displayTree(db *sql.DB,nodes []node.Node) (error) {
	nodeMap := make(map[int]node.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	// Start the recursive display from the root node
	err := displayNodeAndChildren(db, 0, int(exemplar.RootNodeID), nodeMap, "", "[DEFAULT]", false)
	return err
	
}

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

func displayNodeAndChildren(db *sql.DB, depth int, nodeID int, nodeMap map[int]node.Node, insideMessage string, outsideOfMessage string, nodeWasInside bool) (error) {
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
	if !showChildren {
		fmt.Printf("%s- Depth %d, Node %d: suggestion is %s when %s, loss %f, %d usages\n", prefix, depth, n.ID, suggestion, myMessage, n.Loss.Float64, n.DataQuantity.Int64)
		return nil;
	}

	exists, region, err := getWordFromPath(db, n.InnerRegionPrefix.String)
	if !exists {
		region = n.InnerRegionPrefix.String
	}
	fmt.Printf("%s- Depth %d, Node %d: {suggested [%s] when %s, loss %f, %d usages}\n", prefix, depth, n.ID, suggestion, myMessage,
		n.Loss.Float64, n.DataQuantity.Int64)
	// Check to see if the split on the outer node is using the same feature variable as
	// the inner node. If it is, then we don't have nested if statements, we have a case
	// statement, which slightly changes the way we want to display it.
	grandchildAdoption := false
	outerChild, exists := nodeMap[int(n.OuterRegionNodeID.Int64)]
	if exists {
		if outerChild.ContextK.Int64 == n.ContextK.Int64 {
			grandchildAdoption = true
		}
	}

	var outerChildMessage string
	if grandchildAdoption {
		outerChildMessage = fmt.Sprintf("(%s,%s)", outsideOfMessage, region)
	} else {
		outerChildMessage = fmt.Sprintf("context%d is outside %s", n.ContextK.Int64, region)
	}


	insideChildMessage :=  fmt.Sprintf("context%d is inside [%s]", n.ContextK.Int64, region)

	err = displayNodeAndChildren(db, depth + 1, int(n.InnerRegionNodeID.Int64), nodeMap, insideChildMessage, outerChildMessage, true)
	if err != nil {
		return err
	}

	

	if grandchildAdoption {
		err = displayNodeAndChildren(db, depth, int(n.OuterRegionNodeID.Int64), nodeMap, "", outerChildMessage, false)
	} else {
		err = displayNodeAndChildren(db, depth + 1, int(n.OuterRegionNodeID.Int64), nodeMap, "", outerChildMessage, false)
	}
		
	if err != nil {
		return err
	}
	return nil;
}

