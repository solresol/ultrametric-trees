package main

import (
	"github.com/solresol/ultrametric-trees/pkg/decode"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/solresol/ultrametric-trees/pkg/node"
	"github.com/solresol/ultrametric-trees/pkg/exemplar"
)

func main() {
	database := flag.String("database", "", "SQLite database file")
	tableName := flag.String("tablename", "nodes", "Table name for nodes")
	timeStr := flag.String("time", "", "Optional timestamp to filter nodes")
	nodeId := flag.Int("node-id", 0, "Node ID to filter")

	flag.Parse()

	if *database == "" {
		log.Fatal("The --database argument is required")
	}

	db, err := sql.Open("sqlite3", *database)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	var nodes []exemplar.DataFrameRow
	if *nodeId != 0 {
		dataFrameRows, err := exemplar.LoadRows(db, *tableName, "node_id", exemplar.NodeID(*nodeId))
		if err != nil || len(dataFrameRows) == 0 {
			log.Fatalf("Node with ID %d not found or error occurred: %v", *nodeId, err)
		}
		nodes = convertDataFrameRowsToNodes(dataFrameRows)
		if err != nil || len(nodes) == 0 {
			log.Fatalf("Node with ID %d not found or error occurred: %v", *nodeId, err)
		}
	} else if *timeStr != "" {
		timestamp, err := time.Parse(time.RFC3339, *timeStr)
		if err != nil {
			log.Fatalf("Invalid time format: %v", err)
		}
		nodes, err = node.FetchNodesAsOf(db, *tableName, timestamp)
	} else {
		nodes, err = node.FetchNodes(db, *tableName)
	}

	if err != nil {
		log.Fatalf("Error fetching nodes: %v", err)
	}

	for i, n := range nodes {
		if *nodeId != 0 && i > 0 {
			break
		}
		fmt.Printf("ID: %d\n", n.ID)
		if n.ExemplarValue.Valid {
			decodedExemplarValue, err := decode.DecodePath(db, n.ExemplarValue.String)
			if err != nil {
				decodedExemplarValue = "<decoding failed>"
			}
			fmt.Printf("ExemplarValue: %s (%s)\n", n.ExemplarValue.String, decodedExemplarValue)
		} else {
			fmt.Printf("ExemplarValue: %v\n", n.ExemplarValue)
		}
		fmt.Printf("DataQuantity: %v\n", n.DataQuantity)
		fmt.Printf("Loss: %v\n", n.Loss)
		fmt.Printf("ContextK: %v\n", n.ContextK)
		if n.InnerRegionPrefix.Valid {
			decodedInnerRegionPrefix, err := decode.DecodePath(db, n.InnerRegionPrefix.String)
			if err != nil {
				decodedInnerRegionPrefix = "<decoding failed>"
func convertDataFrameRowsToNodes(dataFrameRows []exemplar.DataFrameRow) ([]node.Node, error) {
	var nodes []node.Node
	for _, row := range dataFrameRows {
		node := node.Node{
			ID:                  row.RowID,
			ExemplarValue:       sql.NullString{String: row.TargetWord.String(), Valid: true},
			DataQuantity:        0, // Placeholder
			Loss:                0.0, // Placeholder
			ContextK:            0, // Placeholder
			InnerRegionPrefix:   sql.NullString{Valid: false}, // Placeholder
			InnerRegionNodeID:   0, // Placeholder
			OuterRegionNodeID:   0, // Placeholder
			WhenCreated:         time.Now(), // Placeholder for current time
			WhenChildrenPopulated: time.Time{}, // Placeholder
			HasChildren:         false, // Placeholder
			BeingAnalysed:       false, // Placeholder
			TableName:           "", // Placeholder
		}
		nodes = append(nodes, node)
	}
	return nodes
}
			}
			fmt.Printf("InnerRegionPrefix: %s (%s)\n", n.InnerRegionPrefix.String, decodedInnerRegionPrefix)
		} else {
			fmt.Printf("InnerRegionPrefix: %v\n", n.InnerRegionPrefix)
		}
		fmt.Printf("InnerRegionNodeID: %v\n", n.InnerRegionNodeID)
		fmt.Printf("OuterRegionNodeID: %v\n", n.OuterRegionNodeID)
		fmt.Printf("WhenCreated: %v\n", n.WhenCreated)
		fmt.Printf("WhenChildrenPopulated: %v\n", n.WhenChildrenPopulated)
		fmt.Printf("HasChildren: %v\n", n.HasChildren)
		fmt.Printf("BeingAnalysed: %v\n", n.BeingAnalysed)
		fmt.Printf("TableName: %s\n\n", n.TableName)
	}
}
