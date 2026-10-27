package completion_test

import (
	"cmp"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

//go:embed testdata/corpus/*.json
var corpusFixtures embed.FS

// updateGolden rewrites the golden snapshots from the current formatter
// output when the suite runs with -update. Review regenerated snapshots
// manually; the shells themselves are exercised end-to-end by test/shell.
//
//nolint:gochecknoglobals // standard -update golden-refresh flag seam
var updateGolden = flag.Bool("update", false, "rewrite conformance golden files from current formatter output")

// goldenModes enumerates every framing replayed by the conformance suite:
// the shell passed to NewSession plus the sideband environment the
// mode pins. The suite runs sequentially because t.Setenv pins the
// process-wide environment.
//
//nolint:gochecknoglobals // immutable mode matrix shared by the conformance tests
var goldenModes = []struct {
	name  string
	shell string
	env   map[string]string
}{
	{name: "bash", shell: "bash"},
	{name: "bash-successive-tabs", shell: "bash", env: map[string]string{"COMP_TYPE": "63"}},
	{name: "fish", shell: "fish"},
	{name: "elvish", shell: "elvish"},
	{name: "nushell", shell: "nushell"},
	{name: "export", shell: "export"},
	{name: "powershell", shell: "powershell", env: map[string]string{"CONGA_COLOR": "0"}},
	{name: "powershell-styled", shell: "powershell", env: map[string]string{"CONGA_COLOR": "1"}},
	{name: "powershell-tooltip", shell: "powershell", env: map[string]string{"CONGA_COLOR": "0", "CONGA_TOOLTIP": "1"}},
	{name: "zsh", shell: "zsh", env: map[string]string{"CONGA_COLOR": "0"}},
	{name: "zsh-styled", shell: "zsh", env: map[string]string{"CONGA_COLOR": "1"}},
}

// singleLineFramings lists the JSON framings whose documents must never span
// multiple lines.
//
//nolint:gochecknoglobals // immutable framing classification shared by the conformance tests
var singleLineFramings = map[string]bool{
	"export":     true,
	"powershell": true,
	"elvish":     true,
	"nushell":    true,
}

type conformanceItem struct {
	Value       string `json:"value"`
	Display     string `json:"display"`
	Description string `json:"description,omitempty"`
	Style       string `json:"style,omitempty"`
	Tag         string `json:"tag,omitempty"`
}

type conformanceDirectives struct {
	Error      bool `json:"error"`
	KeepOrder  bool `json:"keeporder"`
	NoFileComp bool `json:"nofilecomp"`
	NoSpace    bool `json:"nospace"`
}

type conformanceFixture struct {
	Items      []conformanceItem     `json:"items"`
	Directives conformanceDirectives `json:"directives"`
	Usage      string                `json:"usage"`
}

// TestConformance_GoldenReplay replays every corpus fixture through every
// framing mode via the dispatch seam and compares the formatted output
// byte-for-byte against the golden snapshots in
// testdata/golden/<mode>/<fixture>.golden. Structural invariants that a
// snapshot cannot express (the export round-trip, field-presence rules) are
// covered separately.
//

func TestConformance_GoldenReplay(t *testing.T) {
	fixtures := loadCorpusFixtures(t)

	for _, mode := range goldenModes {
		for fixtureName, fixture := range fixtures {
			t.Run(mode.name+"/"+fixtureName, func(t *testing.T) {
				for key, value := range mode.env {
					t.Setenv(key, value)
				}

				formatted := completion.NewSession(mode.shell).Format("", resultFromFixture(fixture))

				if singleLineFramings[mode.shell] {
					assert.False(t, strings.Contains(formatted, "\n"), mode.name+" framing must be a single line")
				}

				goldenPath := filepath.Join("testdata", "golden", mode.name, fixtureName+".golden")

				if *updateGolden {
					assert.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
					assert.NoError(t, os.WriteFile(goldenPath, []byte(formatted), 0o644))
				}

				expected, err := os.ReadFile(goldenPath)
				assert.NoError(t, err)

				assert.Equal(t, string(expected), formatted)
			})
		}
	}
}

// TestConformance_ExportReplay replays every corpus fixture through the
// native export formatter via the dispatch seam: the JSON document must
// round-trip to the recorded candidates and directives modulo the §7.1 schema.
//
//nolint:paralleltest // the replay reads fixture state shared with the golden suite
func TestConformance_ExportReplay(t *testing.T) {
	for name, fixture := range loadCorpusFixtures(t) {
		t.Run(name, func(t *testing.T) {
			assertExportFraming(t, fixture, completion.NewSession("export").Format("", resultFromFixture(fixture)))
		})
	}
}

// TestConformance_MergeFlags guards the per-shell merge-flags contract on one
// fixture: shells without a tag channel render identical output for merge
// disabled and enabled, while zsh merges flag tags by default and keeps the
// engine's separate tags under CONGA_MERGE_FLAGS=false
// (docs/completion-wire-protocol.md §4.2).
//

func TestConformance_MergeFlags(t *testing.T) {
	fixture, ok := loadCorpusFixtures(t)["deploy-bare"]
	assert.True(t, ok)

	result := resultFromFixture(fixture)

	for _, mergeCase := range []struct {
		shell string
		env   map[string]string
	}{
		{shell: "bash"},
		{shell: "fish"},
		{shell: "powershell", env: map[string]string{"CONGA_COLOR": "0"}},
		{shell: "elvish"},
		{shell: "nushell"},
	} {
		t.Run(mergeCase.shell, func(t *testing.T) {
			for key, value := range mergeCase.env {
				t.Setenv(key, value)
			}

			baseline := completion.NewSession(mergeCase.shell).Format("", result)

			t.Setenv("CONGA_MERGE_FLAGS", "false")

			unmerged := completion.NewSession(mergeCase.shell).Format("", result)

			t.Setenv("CONGA_MERGE_FLAGS", "true")

			merged := completion.NewSession(mergeCase.shell).Format("", result)

			assert.Equal(t, baseline, unmerged)
			assert.Equal(t, baseline, merged)
		})
	}

	t.Run("zsh", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		baseline := completion.NewSession("zsh").Format("", result)
		assert.Contains(t, baseline, "\x1fflags\x1f", "merging is default-on for zsh")

		t.Setenv("CONGA_MERGE_FLAGS", "false")

		unmerged := completion.NewSession("zsh").Format("", result)
		assert.Contains(t, unmerged, "\x1flonghand flags\x1f")
		assert.Contains(t, unmerged, "\x1fshorthand flags\x1f")
		assert.NotContains(t, unmerged, "\x1fflags\x1f")

		t.Setenv("CONGA_MERGE_FLAGS", "true")

		merged := completion.NewSession("zsh").Format("", result)

		assert.Equal(t, baseline, merged)
	})
}

// loadCorpusFixtures reads every snapshot fixture from the embedded corpus.
func loadCorpusFixtures(t *testing.T) map[string]conformanceFixture {
	t.Helper()

	entries, err := fs.ReadDir(corpusFixtures, "testdata/corpus")
	assert.NoError(t, err)

	fixtures := make(map[string]conformanceFixture, len(entries))
	for _, entry := range entries {
		data, err := corpusFixtures.ReadFile("testdata/corpus/" + entry.Name())
		assert.NoError(t, err)

		var fixture conformanceFixture
		assert.NoError(t, json.Unmarshal(data, &fixture))

		fixtures[strings.TrimSuffix(entry.Name(), ".json")] = fixture
	}

	return fixtures
}

// resultFromFixture reconstructs the recorded engine result, mapping the
// fixture's directive bits back onto the engine vocabulary.
func resultFromFixture(fixture conformanceFixture) predict.Result {
	items := slicex.Map(fixture.Items, func(item conformanceItem) predict.Item {
		return predict.Item{
			Value:       item.Value,
			Display:     item.Display,
			Description: item.Description,
			Style:       item.Style,
			Tag:         item.Tag,
		}
	})

	var directive predict.Directive
	if fixture.Directives.Error {
		directive |= predict.DirectiveError
	}

	if fixture.Directives.KeepOrder {
		directive |= predict.DirectiveKeepOrder
	}

	if fixture.Directives.NoFileComp {
		directive |= predict.DirectiveNoFileCompletion
	}

	if fixture.Directives.NoSpace {
		directive |= predict.DirectiveNoSpace
	}

	return predict.Result{Items: items, Directive: directive, Usage: fixture.Usage}
}

// assertExportFraming verifies one export rendering against its fixture:
// single line, candidate field presence per §7.1 (value/display always,
// description/style/tag omitted when empty, display defaulted), and the
// always-complete directives object plus usage.
func assertExportFraming(t *testing.T, fixture conformanceFixture, formatted string) {
	t.Helper()

	assert.False(t, strings.Contains(formatted, "\n"), "export framing must be a single line")
	assert.False(t, strings.HasSuffix(formatted, "\n"), "export framing must not end with a newline")

	var decoded struct {
		Candidates []map[string]any `json:"candidates"`
		Directives struct {
			KeepOrder bool     `json:"keeporder"`
			Messages  []string `json:"messages"`
			NoPrefix  bool     `json:"noprefix"`
			NoSpace   bool     `json:"nospace"`
		} `json:"directives"`
		Usage string `json:"usage"`
	}

	assert.NoError(t, json.Unmarshal([]byte(formatted), &decoded))
	assert.Equal(t, len(fixture.Items), len(decoded.Candidates))

	for i, item := range fixture.Items {
		candidate := decoded.Candidates[i]

		assert.Equal(t, any(item.Value), candidate["value"])

		expectedDisplay := item.Display
		if expectedDisplay == "" {
			expectedDisplay = item.Value
		}

		assert.Equal(t, any(expectedDisplay), candidate["display"])

		assertOptionalJSONField(t, candidate, "description", item.Description)
		assertOptionalJSONField(t, candidate, "style", item.Style)
		assertOptionalJSONField(t, candidate, "tag", item.Tag)
	}

	assert.Equal(t, fixture.Directives.KeepOrder, decoded.Directives.KeepOrder)
	assert.Equal(t, fixture.Directives.NoSpace, decoded.Directives.NoSpace)
	assert.False(t, decoded.Directives.NoPrefix)

	expectedMessages := []string{}
	if fixture.Directives.Error {
		expectedMessages = append(expectedMessages, cmp.Or(fixture.Usage, "ERROR"))
	}

	assert.Equal(t, expectedMessages, decoded.Directives.Messages)
	assert.Equal(t, fixture.Usage, decoded.Usage)
}

// assertOptionalJSONField verifies the §7.1 field-presence rule for one
// optional candidate field: omitted when empty, equal to the value otherwise.
func assertOptionalJSONField(t *testing.T, candidate map[string]any, field string, value string) {
	t.Helper()

	if value == "" {
		_, present := candidate[field]
		assert.False(t, present, "field %q must be omitted when empty", field)

		return
	}

	assert.Equal(t, any(value), candidate[field])
}
