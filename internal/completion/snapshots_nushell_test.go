package completion_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/nushell"
)

//go:embed testdata/snapshots/nushell/*.json
var nushellSnapshotFixtures embed.FS

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

			var fixture snapshotFixture

			assert.NoError(t, json.Unmarshal(data, &fixture))

			directives, candidates := fixture.toWire()

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
