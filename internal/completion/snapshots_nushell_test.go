package completion_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/nushell"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
)

//go:embed testdata/snapshots/nushell/*.json
var nushellSnapshotFixtures embed.FS

// nushellSnapshot mirrors one fork-observation fixture: the formatter-shaped
// input captured by tools/snapshot-completion and the fork's observed output
// (docs/completion-wire-protocol.md §9).
type nushellSnapshot struct {
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

// patchSnapshot mirrors one fork-observation fixture for the nushell Patch
// seam.
type patchSnapshot struct {
	Description string   `json:"description"`
	Input       []string `json:"input"`
	Observed    []string `json:"observed"`
}

// TestSnapshotEquivalence_Nushell replays the fork-observation fixtures
// through the native nushell formatter at the formatter seam (directives +
// candidates, the fork's ActionRawValues contract): for these cases the §7.7
// framing is byte-equivalent to the fork's output. The engine-bit derivation
// (wire.FromResult), the documented "=" nospace delta, and the rgb() hex
// normalization are covered by the corpus conformance and unit tests
// respectively.
//
//nolint:paralleltest // t.Setenv pins the description length for all subtests; the environment is process-wide
func TestSnapshotEquivalence_Nushell(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "80")

	entries, err := fs.ReadDir(nushellSnapshotFixtures, "testdata/snapshots/nushell")
	assert.NoError(t, err)

	assert.True(t, len(entries) >= 13, "expected the full observation corpus")

	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := nushellSnapshotFixtures.ReadFile("testdata/snapshots/nushell/" + entry.Name())
			assert.NoError(t, err)

			if strings.HasPrefix(entry.Name(), "patch-") {
				var fixture patchSnapshot

				assert.NoError(t, json.Unmarshal(data, &fixture))
				assert.Equal(t, fixture.Observed, nushell.Patch(append([]string{}, fixture.Input...)))

				return
			}

			var fixture nushellSnapshot

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

			// The fork's message collection sorts lexicographically (nushell
			// framing drops messages either way).
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

			formatted := nushell.Format(fixture.Input.CurrentWord, directives, candidates)

			expected := fixture.Observed

			// Documented delta (§7.7): the fork passed rgb() tokens through
			// verbatim, which nushell cannot parse; the native driver
			// normalizes them to hex. Fixture 06 is the only observation
			// carrying an rgb() style.
			if strings.HasSuffix(entry.Name(), "06-style-vocabulary.json") {
				expected = strings.Replace(expected, `"fg":"rgb(10,20,30)"`, `"fg":"#0a141e"`, 1)
			}

			assert.Equal(t, expected, formatted)
		})
	}
}
