// Package fish renders completion results in fish's native completion feed
// and generates the fish registration snippet for the wire protocol
// (docs/completion-wire-protocol.md).
package fish

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
)

const controlCutset = "\t\n\r"

// Format renders candidates as fish's native completion feed: one line per
// candidate in engine order, the value and description separated by a tab
// (the tab and description are omitted when the candidate has no
// description). Descriptions are clamped per CONGA_DESCRIPTION_LENGTH
// (wire.Candidate.TrimmedDescription). Tabs, newlines, and carriage returns
// inside values and descriptions are normalized to single spaces. Fish has no
// channel for directives, messages, styles, or tags: they are silently
// dropped. toComplete is accepted for seam symmetry and ignored: fish
// pre-filters candidates engine-side.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	lines := slicex.Map(candidates, func(candidate wire.Candidate) string {
		value := sanitizeField(candidate.Value)
		description := sanitizeField(candidate.TrimmedDescription())

		if description == "" {
			return value
		}

		return value + "\t" + description
	})

	return strings.Join(lines, "\n")
}

func sanitizeField(s string) string {
	return wire.ReplaceControlWithSpace(s, controlCutset)
}
