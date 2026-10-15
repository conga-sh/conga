package completion_test

import (
	"slices"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
)

// snapshotCandidate mirrors wire.Candidate with JSON tags for fixture
// unmarshaling; the wire type itself carries no tags (its JSON key mapping is
// case-insensitive field-name matching, asserted by the conformance suite).
type snapshotCandidate struct {
	Value       string `json:"value"`
	Display     string `json:"display"`
	Description string `json:"description"`
	Style       string `json:"style"`
	Tag         string `json:"tag"`
}

// snapshotFixture mirrors one fork-observation fixture: the formatter-shaped
// input captured by tools/snapshot-completion and the fork's observed output
// (docs/completion-wire-protocol.md §9).
type snapshotFixture struct {
	Description string `json:"description"`
	Input       struct {
		CurrentWord string `json:"currentWord"`
		Directives  struct {
			NoSpace   string   `json:"nospace"`
			NoPrefix  string   `json:"noprefix"`
			KeepOrder bool     `json:"keeporder"`
			Messages  []string `json:"messages"`
			Usage     string   `json:"usage"`
		} `json:"directives"`
		Candidates []snapshotCandidate `json:"candidates"`
	} `json:"input"`
	Observed string `json:"observed"`
}

// toWire converts the fixture's input directives and candidates into their
// wire representations.
func (f snapshotFixture) toWire() (wire.Directives, []wire.Candidate) {
	directives := wire.Directives{
		KeepOrder: f.Input.Directives.KeepOrder,
		Usage:     f.Input.Directives.Usage,
	}

	for _, r := range f.Input.Directives.NoSpace {
		directives.NoSpace.Add(r)
	}

	for _, r := range f.Input.Directives.NoPrefix {
		directives.NoPrefix.Add(r)
	}

	// The fork's message collection sorts lexicographically.
	directives.Messages = append(directives.Messages, f.Input.Directives.Messages...)
	slices.Sort(directives.Messages)

	candidates := slicex.Map(f.Input.Candidates, func(candidate snapshotCandidate) wire.Candidate {
		return wire.Candidate{
			Value:       candidate.Value,
			Display:     candidate.Display,
			Description: candidate.Description,
			Style:       candidate.Style,
			Tag:         candidate.Tag,
		}
	})

	return directives, candidates
}
