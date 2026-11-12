package levenshtein_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/levenshtein"
)

func TestDistance(t *testing.T) {
	t.Parallel()

	t.Run("identical strings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 0, levenshtein.Distance("kitten", "kitten"))
	})

	t.Run("substitution", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 3, levenshtein.Distance("kitten", "sitting"))
	})

	t.Run("insertion", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 1, levenshtein.Distance("flg", "flag"))
	})

	t.Run("empty source", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 4, levenshtein.Distance("", "test"))
	})

	t.Run("empty target", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 4, levenshtein.Distance("test", ""))
	})

	t.Run("multi-byte rune counts as one edit", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 1, levenshtein.Distance("café", "cafe"))
	})
}

func TestSuggestFor_Threshold(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		input      string
		candidates []string
		expected   string
	}{
		"transposition in a 4-rune input":    {"prot", []string{"port"}, "port"},
		"distance 3 in a 5-rune input":       {"conga", []string{"range"}, ""},
		"distance 2 in a 7-rune input":       {"histroy", []string{"history"}, "history"},
		"distance 1 in a 3-rune input":       {"hlp", []string{"help"}, "help"},
		"distance 2 in a 3-rune input":       {"foo", []string{"xor"}, ""},
		"distance 2 in another 3-rune input": {"can", []string{"len"}, ""},
		"distance 3 in a 10-rune input":      {"abcdefghij", []string{"abcdefgxyz"}, "abcdefgxyz"},
		"distance 4 in a 30-rune input":      {strings.Repeat("a", 30), []string{strings.Repeat("a", 26) + "bbbb"}, strings.Repeat("a", 26) + "bbbb"},
		"distance 5 in a 30-rune input":      {strings.Repeat("a", 30), []string{strings.Repeat("a", 25) + "bbbbb"}, ""},
		"single rune":                        {"x", []string{"xy"}, ""},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			match := levenshtein.SuggestFor(testCase.input, testCase.candidates)

			assert.Equal(t, testCase.expected, match)
		})
	}
}

func TestSuggestFor(t *testing.T) {
	t.Parallel()

	candidates := []string{"help", "version", "verbose", "output"}

	t.Run("case-insensitive match", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("VERSIN", candidates)

		assert.Equal(t, "version", match)
	})

	t.Run("no match within threshold", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("completely_different", candidates)

		assert.Equal(t, "", match)
	})

	t.Run("empty candidates", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("val", nil)

		assert.Equal(t, "", match)
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("", candidates)

		assert.Equal(t, "", match)
	})

	t.Run("tie break picks first candidate", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("abcd", []string{"abce", "abcf"})

		assert.Equal(t, "abce", match)
	})

	t.Run("preserves candidate casing", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.SuggestFor("branchname", []string{"BranchName", "main"})

		assert.Equal(t, "BranchName", match)
	})
}
