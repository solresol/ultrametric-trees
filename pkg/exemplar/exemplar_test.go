package exemplar

import (
	"database/sql"
	"reflect"
	"testing"
	_ "github.com/mattn/go-sqlite3"
)

func TestSynsetpathRoundTrip(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  Synsetpath
	}{
		{"Simple path", "1.2.3", Synsetpath{Path: []int{1, 2, 3}}},
		{"Single number", "42", Synsetpath{Path: []int{42}}},
		{"Long path", "1.2.3.4.5.6.7.8.9.10", Synsetpath{Path: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test parsing
			got, err := ParseSynsetpath(tc.input)
			if err != nil {
				t.Fatalf("ParseSynsetpath(%q) returned unexpected error: %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseSynsetpath(%q) = %v, want %v", tc.input, got, tc.want)
			}

			// Test string conversion
			gotStr := tc.want.String()
			if gotStr != tc.input {
				t.Errorf("Synsetpath(%v).String() = %q, want %q", tc.want, gotStr, tc.input)
			}
		})
	}
}

func TestParseSynsetpathError(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Non-numeric", "1.2.three.4"},
		{"Invalid format", "1..2.3"},
		{"Negative number", "1.-2.3"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSynsetpath(tc.input)
			if err == nil {
				t.Errorf("ParseSynsetpath(%q) did not return an error, want error", tc.input)
			}
		})
	}
}


func TestTableExists(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Error creating test table: %v", err)
	}

	tests := []struct {
		name      string
		tableName string
		expected  bool
	}{
		{"Existing table", "test_table", true},
		{"Non-existing table", "non_existing_table", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TableExists(db, tt.tableName)
			if err != nil {
				t.Fatalf("TableExists(%q) returned unexpected error: %v", tt.tableName, err)
			}
			if result != tt.expected {
				t.Errorf("TableExists(%q) = %v, want %v", tt.tableName, result, tt.expected)
			}
		})
	}
}


func TestCompareTableRowCounts(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create test tables
	_, err = db.Exec(`
		CREATE TABLE table1 (id INTEGER PRIMARY KEY);
		CREATE TABLE table2 (id INTEGER PRIMARY KEY);
		CREATE TABLE table3 (id INTEGER PRIMARY KEY);
		INSERT INTO table1 (id) VALUES (1), (2), (3);
		INSERT INTO table2 (id) VALUES (1), (2), (3);
		INSERT INTO table3 (id) VALUES (1), (2);
	`)
	if err != nil {
		t.Fatalf("Error creating test tables: %v", err)
	}

	tests := []struct {
		name     string
		table1   string
		table2   string
		expected bool
	}{
		{"Equal tables", "table1", "table2", true},
		{"Unequal tables", "table1", "table3", false},
		{"Same table", "table1", "table1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareTableRowCounts(db, tt.table1, tt.table2)
			if err != nil {
				t.Fatalf("CompareTableRowCounts(%q, %q) returned unexpected error: %v", tt.table1, tt.table2, err)
			}
			if result != tt.expected {
				t.Errorf("CompareTableRowCounts(%q, %q) = %v, want %v", tt.table1, tt.table2, result, tt.expected)
			}
		})
	}
}


func TestIsTableEmpty(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create test tables
	_, err = db.Exec(`
		CREATE TABLE empty_table (id INTEGER PRIMARY KEY);
		CREATE TABLE non_empty_table (id INTEGER PRIMARY KEY);
		INSERT INTO non_empty_table (id) VALUES (1);
	`)
	if err != nil {
		t.Fatalf("Error creating test tables: %v", err)
	}

	tests := []struct {
		name      string
		tableName string
		expected  bool
	}{
		{"Empty table", "empty_table", true},
		{"Non-empty table", "non_empty_table", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsTableEmpty(db, tt.tableName)
			if err != nil {
				t.Fatalf("IsTableEmpty(%q) returned unexpected error: %v", tt.tableName, err)
			}
			if result != tt.expected {
				t.Errorf("IsTableEmpty(%q) = %v, want %v", tt.tableName, result, tt.expected)
			}
		})
	}

	// Test with non-existent table
	_, err = IsTableEmpty(db, "non_existent_table")
	if err == nil {
		t.Error("IsTableEmpty with non-existent table should return an error")
	}
}
