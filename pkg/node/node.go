package node

import (
	"database/sql"
	"fmt"
	"time"
	"sort"
)

// I'd like to change the type of these nodes from int to a nodeID type

type Node struct {
	ID                    int
	ExemplarValue         sql.NullString
	DataQuantity          sql.NullInt64
	Loss                  sql.NullFloat64
	ContextK              sql.NullInt64
	InnerRegionPrefix     sql.NullString
	InnerRegionNodeID     sql.NullInt64
	OuterRegionNodeID     sql.NullInt64
	WhenCreated           time.Time
	WhenChildrenPopulated sql.NullTime
	HasChildren           bool
	BeingAnalysed         bool
	TableName             string
}

func FetchNodeByID(db *sql.DB, tableName string, nodeID int) (Node, error) {
	var n Node	
	query := fmt.Sprintf("SELECT * from %s WHERE ID = %d", tableName, nodeID)
	err := db.QueryRow(query).Scan(
		&n.ID, &n.ExemplarValue, &n.DataQuantity, &n.Loss, &n.ContextK,
		&n.InnerRegionPrefix, &n.InnerRegionNodeID, &n.OuterRegionNodeID, &n.WhenCreated,
		&n.WhenChildrenPopulated, &n.HasChildren, &n.BeingAnalysed,
	)
	n.TableName = tableName
	if err != nil {
		return n, fmt.Errorf("Could not retrieve node %d from %s: %v", nodeID, tableName, err)
	}
	return n, nil
}


func FetchParent(db *sql.DB, node Node) (Node, bool, error) {
	// Open to SQL injection attacks if you can set node.TableName
	query := fmt.Sprintf("SELECT ID from %s where inner_region_node_id = %d or outer_region_node = %d", node.TableName, node.ID, node.ID)
	var parentID int
	var parentNode Node
	err := db.QueryRow(query).Scan(&parentID)
	if err == sql.ErrNoRows {
		return parentNode, false, nil
	}
	if err != nil {
		return parentNode, false, err
	}
	parentNode, err = FetchNodeByID(db, node.TableName, parentID)
	return parentNode, true, err
}

func FetchAncestry(db *sql.DB, node Node) ([]Node, error) {
	var ancestors []Node
	thisAncestor := node
	// Maybe to be safe I should count the number of ancestors, and give up if I hit a million or so?
	for {
		// log.Printf("Looking for ancestor of %d", thisAncestor.ID)
		nextAncestor, exists, err := FetchParent(db, thisAncestor)
		if !exists {
			break
		}
		if err != nil {
			return ancestors, fmt.Errorf("Could not fetch ancestor for %d from %s: %v", thisAncestor.ID, thisAncestor.TableName, err)
		}
		ancestors = append(ancestors, nextAncestor)
		thisAncestor = nextAncestor
	}
	// All done, but it's the wrong way around.
	var topDownAncestors []Node
	for i := len(ancestors)-1; i >= 0; i-- {
		topDownAncestors = append(topDownAncestors,ancestors[i])
	}
	return topDownAncestors, nil
}

func FetchNodes(db *sql.DB, tableName string) ([]Node, error) {
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY when_created", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		err := rows.Scan(
			&n.ID, &n.ExemplarValue, &n.DataQuantity, &n.Loss, &n.ContextK,
			&n.InnerRegionPrefix, &n.InnerRegionNodeID, &n.OuterRegionNodeID, &n.WhenCreated,
			&n.WhenChildrenPopulated, &n.HasChildren, &n.BeingAnalysed,
		)
		n.TableName = tableName
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func FilterNodes(nodes []Node, timestamp time.Time, includeWithChildren bool) []Node {
	var filtered []Node
	for _, node := range nodes {
		if node.WhenCreated.Before(timestamp) &&
			(!node.WhenChildrenPopulated.Valid ||
				node.WhenChildrenPopulated.Time.After(timestamp) ||
				(includeWithChildren && node.WhenChildrenPopulated.Time.Before(timestamp))) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func GetSignificantTimestamps(nodes []Node) []time.Time {
	timestampMap := make(map[time.Time]bool)
	for _, node := range nodes {
		timestampMap[node.WhenCreated] = true
		if node.WhenChildrenPopulated.Valid {
			timestampMap[node.WhenChildrenPopulated.Time] = true
		}
	}

	var timestamps []time.Time
	for t := range timestampMap {
		timestamps = append(timestamps, t)
	}

	// Sort timestamps
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	return timestamps
}
