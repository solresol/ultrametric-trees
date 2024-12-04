package exemplar

import (
	"github.com/solresol/ultrametric-trees/pkg/node"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)


type Synsetpath struct {
	Path []int
}

type DataFrameRow struct {
	RowID      int
	TargetWord Synsetpath
}

func ParseSynsetpath(s string) (Synsetpath, error) {
	parts := strings.Split(s, ".")
	path := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return Synsetpath{}, fmt.Errorf("invalid synsetpath: %s", s)
		}
		if num < 0 {
			return Synsetpath{}, fmt.Errorf("negative number not allowed in synsetpath: %s", s)
		}
		path[i] = num
	}
	return Synsetpath{Path: path}, nil
}

func (sp Synsetpath) String() string {
	parts := make([]string, len(sp.Path))
	for i, num := range sp.Path {
		parts[i] = strconv.Itoa(num)
	}
	return strings.Join(parts, ".")
}

func LoadRows(db *sql.DB, dataframeTable string, nodeBucketTable string, nodeID int) ([]DataFrameRow, error) {
	query := fmt.Sprintf("SELECT id, targetword FROM %s JOIN %s USING (id) WHERE node_id = ? order by id", dataframeTable, nodeBucketTable)

	rows, err := db.Query(query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DataFrameRow
	for rows.Next() {
		var r DataFrameRow
		var targetWordStr string
		if err := rows.Scan(&r.RowID, &targetWordStr); err != nil {
			return nil, err
		}
		synsetpath, err := ParseSynsetpath(targetWordStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing synsetpath for row %d: %v", r.RowID, err)
		}
		r.TargetWord = synsetpath
		result = append(result, r)
	}
	// Conversion from DataFrameRow to node.Node already done above
return result, nil
}

// LoadContextNWithinNode is basically the same as LoadRows, except that instead of selecting targetword, it will be selecting contextk and filtering on nodeID. It returns an array, which has to be in the same order as LoadRows returns it (i.e. both should be sorted by ID).

func LoadContextNWithinNode(db *sql.DB, dataframeTable string, nodeBucketTable string, nodeID int, k int, contextLength int) ([]node.Node, error) {
	if k < 1 || k > contextLength {
		return nil, fmt.Errorf("k must be between 1 and %d", contextLength)
	}

	query := fmt.Sprintf("SELECT id, context%d FROM %s JOIN %s USING (id) WHERE node_id = ? ORDER BY id", k, dataframeTable, nodeBucketTable)

	rows, err := db.Query(query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []node.Node
	for rows.Next() {
		var r node.Node
		var contextWordStr string
		if err := rows.Scan(&r.RowID, &contextWordStr); err != nil {
			return nil, err
		}
		synsetpath, err := ParseSynsetpath(contextWordStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing synsetpath for row %d: %v", r.RowID, err)
		}
		r.TargetWord = synsetpath
		result = append(result, r)
	}
	// Conversion from DataFrameRow to node.Node already done above
return result, nil
}

// GetAllPossibleSynsets returns all possible synsets and synset
// truncations from an array of DataFrameRows. If it were given
// {1.2.3, 1.4, 2.3.4, 2.3.3} it will return {1, 1.2, 1.2.3, 1.4, 2,
// 2.3, 2.3.4, 2.3.3 }. It shouldn't return duplicates, so it might
// make sense to create a map from the stringified version of those
// synsets and synset truncations.

func GetAllPossibleSynsets(rows []DataFrameRow) []Synsetpath {
	synsetMap := make(map[string]Synsetpath)

	for _, row := range rows {
		for i := 1; i <= len(row.TargetWord.Path); i++ {
			truncated := Synsetpath{Path: row.TargetWord.Path[:i]}
			synsetMap[truncated.String()] = truncated
		}
	}

	result := make([]Synsetpath, 0, len(synsetMap))
	for _, synset := range synsetMap {
		result = append(result, synset)
	}

	return result
}

// SplitByFilter is given two DataFrameRow arrays (source and target
// which are the same length) and a synset (synset_filter) and returns
// an "inside" array and an "outside" array. It iterates over (source
// and target zipped), and if the current element from source is equal
// to synset_filter, or synset_filter is a truncation of it, then
// we'll call that "in" (put the current target element on to the end
// of the "inside" array) , otherwise "out" (put it onto the "outside"
// array).

func SplitByFilter(source, target []DataFrameRow, synsetFilter Synsetpath) ([]DataFrameRow, []DataFrameRow) {
	var inside, outside []DataFrameRow

	for i, src := range source {
		if len(src.TargetWord.Path) >= len(synsetFilter.Path) &&
			strings.HasPrefix(src.TargetWord.String(), synsetFilter.String()) {
			inside = append(inside, target[i])
		} else {
			outside = append(outside, target[i])
		}
	}

	return inside, outside
}

// The cost of a comparator is related to the amount of path it has in
// common with the exemplar. The path will be like 1.2.1.3.4.5.7.1 --
// dot separated numbers (as text). The cost is 2^{- (the count of the
// number of equal numbers from the left hand side)} . So if the
// exemplar is 1.2.3 and the comparator is 1.4.3, they are only the
// same for one number, so the loss is 2^{-1} = 0.5.  . If the
// comparator had been 1.2.3.1.5, then the loss would have been 2^{-3}
// = 0.125

func CalculateCost(exemplar, comparator Synsetpath) float64 {
	commonPrefixLength := 0
	for i := 0; i < len(exemplar.Path) && i < len(comparator.Path); i++ {
		if exemplar.Path[i] != comparator.Path[i] {
			break
		}
		commonPrefixLength++
	}

	return math.Pow(2, -float64(commonPrefixLength))
}

//  FindBestExemplar iterates [exemplarGuesses] number of times, picking a random
//  element from that array each time and getting the targetword field
//  (this is called the exemplar). Then it calculates the cost of the
//  exemplar.

// How to calculate the cost of an exemplar: iterate [cost-guesses]
// number of times. Each time, pick a random element from the array
// and get the targetword field (this is called the comparator
// element), and sum up the cost of the comparators.

// By taking the sample of [cost-guesses] and extrapolating it to the
// total number of rows in the database, it can estimate the loss of
// that exemplar.

func FindBestExemplar(rows []DataFrameRow, exemplarGuesses, costGuesses int, rng *rand.Rand) (Synsetpath, float64, error) {
	if len(rows) == 0 {
		return Synsetpath{}, 0, fmt.Errorf("no rows provided to FindBestExemplar")
	}

	bestExemplar := Synsetpath{}
	bestLoss := math.Inf(1)

	for i := 0; i < exemplarGuesses; i++ {
		exemplar := rows[rng.Intn(len(rows))].TargetWord
		totalCost := 0.0

		for j := 0; j < costGuesses; j++ {
			comparator := rows[rng.Intn(len(rows))].TargetWord
			totalCost += CalculateCost(exemplar, comparator)
		}

		estimatedLoss := totalCost / float64(costGuesses) * float64(len(rows))
		if estimatedLoss < bestLoss {
			bestExemplar = exemplar
			bestLoss = estimatedLoss
		}
	}

	return bestExemplar, bestLoss, nil
}

func UpdateNodeIDs(tx *sql.Tx, table string, rowIDs []int, newNodeID int) error {
	if len(rowIDs) == 0 {
		return nil
	}

	if len(rowIDs) < 1000 {
		placeholders := make([]string, len(rowIDs))
		args := make([]interface{}, len(rowIDs)+1)
		args[0] = newNodeID
		for i, id := range rowIDs {
			placeholders[i] = "?"
			args[i+1] = id
		}
		query := fmt.Sprintf("UPDATE %s SET node_id = ? WHERE id IN (%s)", table, strings.Join(placeholders, ","))
		_, err := tx.Exec(query, args...)
		return err
	}

	// For large updates, use a temporary table
	_, err := tx.Exec("CREATE TEMPORARY TABLE temp_ids (id INTEGER PRIMARY KEY)")
	if err != nil {
		return err
	}
	defer tx.Exec("DROP TABLE temp_ids")

	stmt, err := tx.Prepare("INSERT INTO temp_ids (id) VALUES (?)")
	if err != nil {
		return err
	}
	// defer stmt.Close()  // needed?

	for _, id := range rowIDs {
		_, err = stmt.Exec(id)
		if err != nil {
			return err
		}
	}

	query := fmt.Sprintf("UPDATE %s SET node_id = ? WHERE id IN (SELECT id FROM temp_ids)", table)
	_, err = tx.Exec(query, newNodeID)
	if err != nil {
		return err
	}
	return nil
}

func CompareTableRowCounts(db *sql.DB, table1, table2 string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT
			(SELECT COUNT(*) FROM %s) AS count1,
			(SELECT COUNT(*) FROM %s) AS count2
	`, table1, table2)

	var count1, count2 int
	err := db.QueryRow(query).Scan(&count1, &count2)
	if err != nil {
		return false, fmt.Errorf("error comparing row counts: %v", err)
	}

	return count1 == count2, nil
}

func TableExists(db *sql.DB, tableName string) (bool, error) {
	query := `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name=?
	`
	var name string
	err := db.QueryRow(query, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error checking if table exists: %v", err)
	}
	return true, nil
}

func IsTableEmpty(db *sql.DB, tableName string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s LIMIT 1)", tableName)
	var exists bool
	err := db.QueryRow(query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if table is empty: %v", err)
	}
	return !exists, nil
}

func MostUrgentToImprove(db *sql.DB, nodesTable string, minSizeToConsider int) (node.NodeID, float64, error) {
	query := fmt.Sprintf(`
		SELECT id, loss
		FROM %s
		WHERE not has_children AND not being_analysed
		AND data_quantity >= ?
		ORDER BY loss DESC
		LIMIT 1
	`, nodesTable)

	var id int
	var loss float64
	err := db.QueryRow(query, minSizeToConsider).Scan(&id, &loss)
	if err == sql.ErrNoRows {
		return NoNodeID, 0.0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("error finding most urgent node to improve: %v", err)
	}

	return NodeID(id), loss, nil
}
