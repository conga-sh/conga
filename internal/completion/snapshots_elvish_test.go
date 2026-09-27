package completion_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/elvish"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
)

//go:embed testdata/snapshots/elvish/*.json
var elvishSnapshotFixtures embed.FS

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

// elvishSnapshot mirrors one fork-observation fixture: the formatter-shaped
// input captured by tools/snapshot-completion and the fork's observed output
// (docs/completion-wire-protocol.md §9).
type elvishSnapshot struct {
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

// TestSnapshotEquivalence_Elvish replays the fork-observation fixtures
// through the native elvish formatter at the formatter seam (directives +
// candidates, the fork's ActionRawValues contract): for these cases the §7.6
// framing is byte-equivalent to the fork's output. The engine-bit derivation
// (wire.FromResult) and the documented "=" nospace delta are covered by the
// corpus conformance and unit tests respectively.
//
//nolint:paralleltest // t.Setenv pins the description length for all subtests; the environment is process-wide
func TestSnapshotEquivalence_Elvish(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "80")

	entries, err := fs.ReadDir(elvishSnapshotFixtures, "testdata/snapshots/elvish")
	assert.NoError(t, err)

	assert.True(t, len(entries) >= 12, "expected the full observation corpus")

	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := elvishSnapshotFixtures.ReadFile("testdata/snapshots/elvish/" + entry.Name())
			assert.NoError(t, err)

			var fixture elvishSnapshot

			assert.NoError(t, json.Unmarshal(data, &fixture))

			directives := wire.Directives{
				KeepOrder: fixture.Input.Directives.KeepOrder,
				Usage:     fixture.Input.Directives.Usage,
			}

			for _, r := range fixture.Input.Directives.NoSpace {
				directives.NoSpace.Add(r)
			}

			for _, r := range fixture.Input.Directives.NoPrefix {
				directives.NoPrefix.Add(r)
			}

			// The fork's message collection sorts lexicographically.
			directives.Messages = append(directives.Messages, fixture.Input.Directives.Messages...)
			slices.Sort(directives.Messages)

			candidates := slicex.Map(fixture.Input.Candidates, func(candidate snapshotCandidate) wire.Candidate {
				return wire.Candidate{
					Value:       candidate.Value,
					Display:     candidate.Display,
					Description: candidate.Description,
					Style:       candidate.Style,
					Tag:         candidate.Tag,
				}
			})

			formatted := elvish.Format(fixture.Input.CurrentWord, directives, candidates)

			expected := fixture.Observed

			// Documented delta (§7.6): the fork passed truecolor tokens
			// through verbatim, which elvish's styling parser rejects; the
			// native driver normalizes them to the nearest indexed color.
			// Fixture 06 is the only observation carrying truecolor styles.
			if strings.HasSuffix(entry.Name(), "06-style-vocabulary.json") {
				expected = strings.Replace(expected, `"style":"#ff8800"`, `"style":"color208"`, 1)
				expected = strings.Replace(expected, `"style":"rgb(10,20,30)"`, `"style":"color233"`, 1)
			}

			assert.Equal(t, expected, formatted)
		})
	}
}
