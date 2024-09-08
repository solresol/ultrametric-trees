package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/solresol/ultrametric-trees/pkg/node"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
)

func main() {
	dbPath := flag.String("database", "", "Path to the SQLite database file")
	tableName := flag.String("table", "nodes", "Name of the nodes table")
	timestamp := flag.String("time", "", "Timestamp to display nodes (format: 2006-01-02 15:04:05)")
	flag.Parse()

	if *dbPath == "" || *timestamp == "" {
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
	err := displayNodeAndChildren(db, "", int(exemplar.RootNodeID), nodeMap)
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

func displayNodeAndChildren(db *sql.DB, prefix string, nodeID int, nodeMap map[int]node.Node) (error) {
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

	if n.InnerRegionNodeID.Valid && n.OuterRegionNode.Valid {
		exists, region, err := getWordFromPath(db, n.InnerRegionPrefix.String)
		if !exists {
			region = n.InnerRegionPrefix.String
		}
		err = displayNodeAndChildren(db, prefix+" ", int(n.InnerRegionNodeID.Int64), nodeMap)
		if err != nil {
			return err
		}
		fmt.Printf("%s  / when context%d is inside [%s]\n", prefix, n.ContextK.Int64, region)
		fmt.Printf("%s- Node %d: {suggested [%s], loss %f, %d usages}\n", prefix, n.ID, suggestion,
			n.Loss.Float64, n.DataQuantity.Int64)
		fmt.Printf("%s  \\ when context%d is outside %s\n", prefix, n.ContextK.Int64, region)
		err = displayNodeAndChildren(db, prefix+" ", int(n.OuterRegionNode.Int64), nodeMap)
		if err != nil {
			return err
		}
			
	} else {
		fmt.Printf("%s- Node %d: suggestion is %s, loss %f, %d usages\n", prefix, n.ID, suggestion, n.Loss.Float64, n.DataQuantity.Int64)
	}
	return nil;
}

func displayNode(n node.Node, childMap map[int][]node.Node, depth int) {
	indent := strings.Repeat(" ", depth)
	fmt.Printf("%s- Node %d: %s\n", indent, n.ID, n.ExemplarValue.String)

	children := childMap[n.ID]
	sort.Slice(children, func(i, j int) bool {
		return children[i].ID < children[j].ID
	})

	for _, child := range children {
		displayNode(child, childMap, depth+1)
	}
}
