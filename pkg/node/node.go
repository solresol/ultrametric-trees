package node

import (
	"database/sql"
	"fmt"
	"time"
	"sort"
)

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
