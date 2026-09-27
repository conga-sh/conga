package levenshtein_test

import (
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

func TestSuggest(t *testing.T) {
	t.Parallel()

	candidates := []string{"help", "version", "verbose", "output"}

	t.Run("nearest candidate", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("hlp", candidates, 3)

		assert.Equal(t, "help", match)
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("VERSIN", candidates, 4)

		assert.Equal(t, "version", match)
	})

	t.Run("no match within threshold", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("completely_different", candidates, 4)

		assert.Equal(t, "", match)
	})

	t.Run("default threshold when maxDistance is zero", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("hlp", candidates, 0)

		assert.Equal(t, "help", match)
	})

	t.Run("candidate at exact threshold excluded", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("help", []string{"helpx"}, 1)

		assert.Equal(t, "", match)
	})

	t.Run("empty candidates", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("val", nil, 3)

		assert.Equal(t, "", match)
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("", candidates, 3)

		assert.Equal(t, "", match)
	})

	t.Run("tie break picks first candidate", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("abc", []string{"abd", "abb"}, 2)

		assert.Equal(t, "abd", match)
	})

	t.Run("preserves candidate casing", func(t *testing.T) {
		t.Parallel()

		match := levenshtein.Suggest("branchname", []string{"BranchName", "main"}, 3)

		assert.Equal(t, "BranchName", match)
	})
}
