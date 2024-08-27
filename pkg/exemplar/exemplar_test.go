package exemplar

import (
	"reflect"
	"testing"
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
