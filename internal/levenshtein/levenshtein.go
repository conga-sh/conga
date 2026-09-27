// Package levenshtein provides edit distance calculation and typo suggestions for commands and flags.
//
// Performance & Allocation Invariant:
// ASCII string comparisons use a dedicated byte-based algorithm operating directly on string
// headers without converting to []byte, avoiding string-to-byte conversion allocations.
// For words <= 32 characters, stack arrays are used to eliminate heap allocations entirely.
// Do NOT unify distanceBytes and distanceRunes into a generic slice function; converting strings
// to []byte inside Suggest loops introduces heap allocations on every candidate comparison.
package levenshtein

import (
	"strings"
	"unicode/utf8"
)

// Distance computes the Levenshtein edit distance between source and target,
// measured in Unicode code points. ASCII inputs take a byte-based fast path;
// non-ASCII inputs are compared rune-by-rune.
func Distance(source, target string) int {
	if isASCII(source) && isASCII(target) {
		return distanceBytes(source, target)
	}

	return distanceRunes([]rune(source), []rune(target))
}

// Suggest finds the best matching candidate for input where the case-insensitive
// edit distance is strictly less than maxDistance. If maxDistance <= 0, a default
// threshold of 3 is used.
func Suggest(input string, candidates []string, maxDistance int) string {
	return suggest(strings.ToLower(input), candidates, maxDistance)
}

func suggest(input string, candidates []string, maxDistance int) string {
	if maxDistance <= 0 {
		maxDistance = 3
	}

	inputLen := utf8.RuneCountInString(input)
	bestCandidate := ""
	bestDistance := maxDistance

	for _, cand := range candidates {
		target := strings.ToLower(cand)

		if abs(inputLen-utf8.RuneCountInString(target)) >= bestDistance {
			continue
		}

		d := Distance(input, target)
		if d < bestDistance {
			bestDistance = d
			bestCandidate = cand

			if d == 0 {
				return cand
			}
		}
	}

	return bestCandidate
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}

	return true
}

// distanceBytes calculates Levenshtein distance on raw string bytes.
// It deliberately accepts string (not []byte or generic []T) and reuses a stack buffer
// for target lengths <= 32 to avoid heap allocations on the ASCII fast path.
func distanceBytes(source, target string) int {
	var (
		prevBuf, currBuf [33]int
		prev, curr       []int
	)

	if len(target) <= 32 {
		prev = prevBuf[:len(target)+1]
		curr = currBuf[:len(target)+1]
	} else {
		prev = make([]int, len(target)+1)
		curr = make([]int, len(target)+1)
	}

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(source); i++ {
		curr[0] = i

		for j := 1; j <= len(target); j++ {
			cost := 0
			if source[i-1] != target[j-1] {
				cost = 1
			}

			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}

		prev, curr = curr, prev
	}

	return prev[len(target)]
}

// distanceRunes calculates Levenshtein distance across Unicode runes for non-ASCII strings.
func distanceRunes(source, target []rune) int {
	prev := make([]int, len(target)+1)
	curr := make([]int, len(target)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(source); i++ {
		curr[0] = i

		for j := 1; j <= len(target); j++ {
			cost := 0
			if source[i-1] != target[j-1] {
				cost = 1
			}

			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}

		prev, curr = curr, prev
	}

	return prev[len(target)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
