package spec

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestAllCongaTagCandidates_CoversEveryContainer pins allCongaTagCandidates to
// the union of the per-container candidate lists, so a tag added to any list
// without updating the literal fails here instead of silently degrading typo
// suggestions.
func TestAllCongaTagCandidates_CoversEveryContainer(t *testing.T) {
	t.Parallel()

	union := slices.Concat(
		flagTagCandidates,
		argTagCandidates,
		builtinCommandTagCandidates,
		userCommandTagCandidates,
		configFileTagCandidates,
		featuresFieldTagCandidates,
		featureFieldTagCandidates,
		dashRawTagCandidates,
		sentinelTagCandidates,
	)

	expected := slices.Compact(slices.Sorted(slices.Values(union)))

	actual := slices.Sorted(slices.Values(allCongaTagCandidates))

	assert.Equal(t, expected, actual)
}
