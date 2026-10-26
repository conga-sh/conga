package nushell_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/nushell"
	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestFormat_BareRecordArray(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "commit", Display: "commit", Description: "record changes"},
		{Value: "logs", Display: "logs"},
	})

	assert.False(t, strings.Contains(output, "\n"), "nushell framing must be a single line")

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed), "nushell framing must be valid JSON")
	assert.Equal(t, 2, len(parsed))
	assert.Equal(t, any("commit "), parsed[0]["value"])
	assert.Equal(t, any("commit"), parsed[0]["display"])
	assert.Equal(t, any("record changes"), parsed[0]["description"])
	assert.Equal(t, any("logs "), parsed[1]["value"])
}

func TestFormat_DescriptionOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{{Value: "logs", Display: "logs"}})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	_, present := parsed[0]["description"]
	assert.False(t, present, "description must be omitted when empty")
}

func TestFormat_MetacharacterQuoting(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "my file.txt", Display: "my file.txt"},
		{Value: `a"b`, Display: `a"b`},
		{Value: `a\b`, Display: `a\b`},
		{Value: "$(pwd)", Display: "$(pwd)"},
		{Value: "x;y|z", Display: "x;y|z"},
		{Value: "{brace}[bracket]", Display: "{brace}[bracket]"},
		{Value: "#hash", Display: "#hash"},
		{Value: "a&b<c>d", Display: "a&b<c>d"},
		{Value: "plain", Display: "plain"},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any(`"my file.txt" `), parsed[0]["value"])
	assert.Equal(t, any(`"a\"b" `), parsed[1]["value"])
	assert.Equal(t, any(`"a\\b" `), parsed[2]["value"])
	assert.Equal(t, any(`"$(pwd)" `), parsed[3]["value"])
	assert.Equal(t, any(`"x;y|z" `), parsed[4]["value"])
	assert.Equal(t, any(`"{brace}[bracket]" `), parsed[5]["value"])
	assert.Equal(t, any(`"#hash" `), parsed[6]["value"])
	assert.Equal(t, any(`"a&b<c>d" `), parsed[7]["value"])
	assert.Equal(t, any("plain "), parsed[8]["value"])
}

func TestFormat_TildeStaysOutsideQuotes(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "~/config", Display: "~/config"},
		{Value: "~my dir/x", Display: "~my dir/x"},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("~/config "), parsed[0]["value"], "tilde alone is not a metacharacter")
	assert.Equal(t, any(`~"my dir/x" `), parsed[1]["value"])
}

func TestFormat_NospaceBakesTrailingSpaceRule(t *testing.T) {
	t.Parallel()

	var directives wire.Directives
	directives.NoSpace.Add('*')

	output := nushell.Format("", directives, []wire.Candidate{
		{Value: "one", Display: "one"},
		{Value: "two=", Display: "two="},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("one"), parsed[0]["value"], "nospace removes the trailing space from the value")
	assert.Equal(t, any("two="), parsed[1]["value"])
}

func TestFormat_TrailingEqualsAndSlashImpliedNospace(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "--env=", Display: "--env="},
		{Value: "dir/", Display: "dir/"},
		{Value: "file.txt", Display: "file.txt"},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("--env="), parsed[0]["value"])
	assert.Equal(t, any("dir/"), parsed[1]["value"])
	assert.Equal(t, any("file.txt "), parsed[2]["value"])
}

func TestFormat_SanitizesCarriageReturnsAndNewlinesKeepsTabs(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "va\tlue", Display: "dis\rplay\nname", Description: "desc\nwith\rnewlines"},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("va\tlue "), parsed[0]["value"], "tabs are kept")
	assert.Equal(t, any("displayname"), parsed[0]["display"])
	// Descriptions are flattened (sanitized before clamping, unlike fish's
	// and elvish's first-line cut).
	assert.Equal(t, any("descwithnewlines"), parsed[0]["description"])
}

func TestFormat_StyleRecords(t *testing.T) {
	t.Parallel()

	output := nushell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "plain", Display: "plain"},
		{Value: "red", Display: "red", Style: "red"},
		{Value: "bright", Display: "bright", Style: "bright-red bold"},
		{Value: "graybright", Display: "graybright", Style: "bright-black"},
		{Value: "whitebright", Display: "whitebright", Style: "bright-white"},
		{Value: "background", Display: "background", Style: "bg-bright-blue italic"},
		{Value: "indexed", Display: "indexed", Style: "color123"},
		{Value: "hex", Display: "hex", Style: "#ff8800"},
		{Value: "func", Display: "func", Style: "rgb(10,20,30)"},
		{Value: "everything", Display: "everything", Style: "bold dim italic underlined blink inverse"},
	})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))

	_, present := parsed[0]["style"]
	assert.False(t, present, "unstyled candidates omit the style field")
	assert.Equal(t, any(map[string]any{"fg": "red"}), parsed[1]["style"])
	assert.Equal(t, any(map[string]any{"fg": "light_red", "attr": "b"}), parsed[2]["style"])
	assert.Equal(t, any(map[string]any{"fg": "dark_gray"}), parsed[3]["style"])
	assert.Equal(t, any(map[string]any{"fg": "white"}), parsed[4]["style"])
	assert.Equal(t, any(map[string]any{"bg": "light_blue", "attr": "i"}), parsed[5]["style"])
	assert.Equal(t, any(map[string]any{"fg": "#87ffff"}), parsed[6]["style"])
	assert.Equal(t, any(map[string]any{"fg": "#ff8800"}), parsed[7]["style"])
	assert.Equal(t, any(map[string]any{"fg": "#0a141e"}), parsed[8]["style"])
	assert.Equal(t, any(map[string]any{"attr": "lbdiru"}), parsed[9]["style"])
}

func TestFormat_EmptyCandidateList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "[]", nushell.Format("", wire.Directives{}, nil))
}

func TestFormat_IgnoresToComplete(t *testing.T) {
	t.Parallel()

	bare := nushell.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})
	prefixed := nushell.Format("de", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.Equal(t, bare, prefixed)
}

func TestFormat_DescriptionClamped(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "10")

	// Multi-line descriptions flatten before the clamp applies.
	output := nushell.Format("", wire.Directives{}, []wire.Candidate{{Value: "v", Display: "v", Description: "first\nsecond"}})

	var parsed []map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("firstse..."), parsed[0]["description"])
}

func TestFormat_ColorDisabledVariants(t *testing.T) {
	candidates := []wire.Candidate{
		{Value: "red", Display: "red", Style: "red"},
		{Value: "bright", Display: "bright", Style: "bright-red bold"},
	}

	t.Run("CONGA_COLOR=0", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := nushell.Format("", wire.Directives{}, candidates)

		assert.False(t, strings.Contains(output, `"style":`), "color-disabled output must not carry style records")
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		output := nushell.Format("", wire.Directives{}, candidates)

		assert.False(t, strings.Contains(output, `"style":`), "color-disabled output must not carry style records")
	})

	t.Run("CLICOLOR=0", func(t *testing.T) {
		t.Setenv("CLICOLOR", "0")

		output := nushell.Format("", wire.Directives{}, candidates)

		assert.False(t, strings.Contains(output, `"style":`), "color-disabled output must not carry style records")
	})
}
