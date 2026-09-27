// Package fish renders completion results in fish's native completion feed
// and generates the fish registration snippet for the wire protocol
// (docs/completion-wire-protocol.md).
package fish

import (
	"strings"
	"sync"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
)

//nolint:gochecknoglobals // one-time lazy construction of the shared replacer below
var fieldSanitizerOnce sync.Once

//nolint:gochecknoglobals // immutable after fieldSanitizerOnce completes
var fieldSanitizerReplacer *strings.Replacer

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
		value := fieldSanitizer().Replace(candidate.Value)
		description := fieldSanitizer().Replace(candidate.TrimmedDescription())

		if description == "" {
			return value
		}

		return value + "\t" + description
	})

	return strings.Join(lines, "\n")
}

// fieldSanitizer normalizes the characters that would corrupt fish's
// tab-separated record parser: every tab, newline, and carriage return
// becomes a single space. Construction is deferred to first use so the
// driver package stays free of package initializers and drops at link time
// when fish completion is disabled.
func fieldSanitizer() *strings.Replacer {
	fieldSanitizerOnce.Do(func() {
		fieldSanitizerReplacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	})

	return fieldSanitizerReplacer
}
