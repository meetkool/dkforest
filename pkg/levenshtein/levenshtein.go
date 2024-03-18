package levenshtein

import (
	"math"
	"strings"
)

// ComputeDistance computes the Levenshtein Distance between the two
// strings passed as an argument.
//
// Works on runes (Unicode code points) but does not normalize
// the input strings.
func ComputeDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}

	if len(b) == 0 {
		return len(a)
	}

	if a == b {
		return 0
	}

	s1 := []rune(a)
	s2 := []rune(b)

	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	lenS1 := len(s1)
	lenS2 := len(s2)

	x := make([]uint16, lenS1+1)

	for i := 1; i < len(x); i++ {
		x[i] = uint16(i)
	}

	for i := 1; i <= lenS2; i++ {
		prev := uint16(i)
		for j := 1; j <= lenS1; j++ {
		
