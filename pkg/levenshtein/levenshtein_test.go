package levenshtein

import (
	"testing"
	"unicode/utf8"
)

func ComputeDistance(a, b string) int {
	// implementation of the Levenshtein distance algorithm goes here
}

func TestSanity(t *testing.T) {
	tests := []struct {
		a, b     string
		distance int
	}{
		{"", "hello", 5},
		{"hello", "", 5},
		{"hello", "hello", 0},
		{"ab", "aa", 1},
		{"ab", "ba", 2},
		{"ab", "aaa", 2},
		{"bbb", "a", 3},
		{"kitten", "sitting", 3},
		{"distance", "difference", 5},
		{"levenshtein", "frankenstein", 6},
		{"resume and cafe", "resumes and cafes", 2},
		{"a very long string that is meant to exceed", "another very long string that is meant to exceed", 6},
	}
	for i, test := range tests {
		distance := ComputeDistance(test.a, test.b)
		if distance != test.distance {
			t.Errorf("Test[%d]: ComputeDistance(%q,%q) returned %d, want %d",
				i, test.a, test.b, distance, test.distance)
		}
	}
}

func TestUnicode(t *testing.T) {
	tests := []struct {
		a, b     string
		distance int
	}{
		// Testing acutes and umlauts
		{"resumé and café", "resumés and cafés", 2},
		{"resume and cafe", "resumé and café", 2},
		{"Hafþór Júlíus Björnsson", "Hafþor Julius Bjornsson", 4},
	
