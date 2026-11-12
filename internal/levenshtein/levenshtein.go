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

// SuggestFor finds the best matching candidate for input, comparing case
// insensitively. A candidate qualifies when its edit distance is at most the
// threshold for the input's length, so every suggester (flags, commands,
// config keys, features, tags, predictors, groups) judges near misses the same
// way:
//
//	1 rune      0 (no suggestion short of a case-insensitive match)
//	2-3 runes   1
//	4+ runes    min(4, max(2, runes/3))
//
// A transposition counts as two edits, so "prot" still suggests "port". The cap
// keeps a long input from accepting an unrelated candidate.
func SuggestFor(input string, candidates []string) string {
	input = strings.ToLower(input)

	return suggest(input, candidates, threshold(utf8.RuneCountInString(input)))
}

// threshold returns the largest edit distance SuggestFor accepts for an input
// of the given rune count.
func threshold(runes int) int {
	switch {
	case runes <= 1:
		return 0
	case runes <= 3:
		return 1
	default:
		return min(4, max(2, runes/3))
	}
}

// suggest returns the closest candidate within maxDistance edits of input, or
// "" when none qualifies. The first candidate wins a tie.
func suggest(input string, candidates []string, maxDistance int) string {
	inputLen := utf8.RuneCountInString(input)
	bestCandidate := ""
	bestDistance := maxDistance + 1

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
