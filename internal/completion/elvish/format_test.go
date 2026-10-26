package elvish_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/elvish"
	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestFormat_SingleLineJSONObject(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "serve", Display: "serve", Description: "start web server"}})

	assert.False(t, strings.Contains(output, "\n"), "elvish framing must be a single line")

	var parsed map[string]any

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed), "elvish framing must be valid JSON")
}

func TestFormat_CarriesAllCandidateFields(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "serve", Display: "serve", Description: "start web server", Style: "blue", Tag: "commands"},
	})

	var parsed struct {
		Usage            string           `json:"usage"`
		Messages         []string         `json:"messages"`
		DescriptionStyle string           `json:"descriptionStyle"`
		Candidates       []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))

	assert.Equal(t, "", parsed.Usage)
	assert.Equal(t, []string{}, parsed.Messages)
	assert.Equal(t, "dim", parsed.DescriptionStyle)

	assert.Equal(t, 1, len(parsed.Candidates))

	candidate := parsed.Candidates[0]
	assert.Equal(t, any("serve"), candidate["value"])
	assert.Equal(t, any("serve"), candidate["display"])
	assert.Equal(t, any("start web server"), candidate["description"])
	assert.Equal(t, any(" "), candidate["codeSuffix"])
	assert.Equal(t, any("blue"), candidate["style"])
}

func TestFormat_EmptyStyleFallsBackToDefault(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "serve", Display: "serve"}})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("default"), parsed.Candidates[0]["style"])
}

func TestFormat_EmptyDisplayDefaultsToValue(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "serve"}})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("serve"), parsed.Candidates[0]["display"])
}

func TestFormat_NospaceDirectiveClearsCodeSuffix(t *testing.T) {
	t.Parallel()

	var directives wire.Directives
	directives.NoSpace.Add('*')

	output := elvish.Format("", directives, []wire.Candidate{{Value: "serve", Display: "serve"}})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any(""), parsed.Candidates[0]["codeSuffix"])
}

func TestFormat_TrailingEqualsImpliesNospace(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "--env=", Display: "--env="}})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any(""), parsed.Candidates[0]["codeSuffix"])
}

func TestFormat_NospaceMatcherMatchesBySuffixOnly(t *testing.T) {
	t.Parallel()

	var directives wire.Directives
	directives.NoSpace.Add('/')

	output := elvish.Format("", directives, []wire.Candidate{
		{Value: "dir/", Display: "dir/"},
		{Value: "file.txt", Display: "file.txt"},
	})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any(""), parsed.Candidates[0]["codeSuffix"])
	assert.Equal(t, any(" "), parsed.Candidates[1]["codeSuffix"])
}

func TestFormat_UsageClearedWhenCandidatesExist(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{Usage: "app [flags]"}

	withCandidates := elvish.Format("", directives, []wire.Candidate{{Value: "serve", Display: "serve"}})
	withoutCandidates := elvish.Format("", directives, nil)

	var parsed struct {
		Usage string `json:"usage"`
	}

	assert.NoError(t, json.Unmarshal([]byte(withCandidates), &parsed))
	assert.Equal(t, "", parsed.Usage, "usage must be cleared when candidates exist")

	assert.NoError(t, json.Unmarshal([]byte(withoutCandidates), &parsed))
	assert.Equal(t, "app [flags]", parsed.Usage, "usage is sent for the zero-candidate case")
}

func TestFormat_CarriesMessages(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{Messages: []string{"unknown command \"invalid\"", "usage: app [flags]"}}

	var parsed struct {
		Messages []string `json:"messages"`
	}

	assert.NoError(t, json.Unmarshal([]byte(elvish.Format("", directives, nil)), &parsed))
	assert.Equal(t, directives.Messages, parsed.Messages)
}

func TestFormat_SanitizesControlCharacters(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "va\tlue", Display: "dis\rplay\nname", Description: "desc\nwith\nnewlines\tand\ttabs"},
	})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("value"), parsed.Candidates[0]["value"])
	assert.Equal(t, any("displayname"), parsed.Candidates[0]["display"])
	// Descriptions take the first line (TrimmedDescription) before the
	// sanitizer strips control characters.
	assert.Equal(t, any("desc"), parsed.Candidates[0]["description"])
}

func TestFormat_StyleVocabularyPassesThrough(t *testing.T) {
	t.Parallel()

	for _, styleToken := range []string{
		"red", "bright-red bold", "bg-bright-blue italic", "color123",
		"bold dim italic underlined blink inverse color9 bg-color21", "fg-default",
	} {
		output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "v", Display: "v", Style: styleToken}})

		var parsed struct {
			Candidates []map[string]any `json:"candidates"`
		}

		assert.NoError(t, json.Unmarshal([]byte(output), &parsed), "token %q must produce valid JSON", styleToken)
		assert.Equal(t, any(styleToken), parsed.Candidates[0]["style"], "token %q must pass through", styleToken)
	}
}

func TestFormat_TruecolorNormalizesToNearestIndexed(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "hex", Display: "hex", Style: "#ff0000"},
		{Value: "func", Display: "func", Style: "rgb(255,0,0)"},
		{Value: "bg", Display: "bg", Style: "bg-#00ff00"},
		{Value: "gray", Display: "gray", Style: "#808080"},
	})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))

	// #ff0000 and rgb(255,0,0) duplicate base-16 red: nearest is color9.
	assert.Equal(t, any("color9"), parsed.Candidates[0]["style"])
	assert.Equal(t, any("color9"), parsed.Candidates[1]["style"])
	assert.Equal(t, any("bg-color10"), parsed.Candidates[2]["style"])
	// #808080 is base-16 gray (8) and ramp entry 244: nearest is color8.
	assert.Equal(t, any("color8"), parsed.Candidates[3]["style"])
}

func TestFormat_MultiLineDescriptionClamped(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "10")

	output := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "v", Display: "v", Description: "first\nsecond"}})

	var parsed struct {
		Candidates []map[string]any `json:"candidates"`
	}

	assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
	assert.Equal(t, any("first"), parsed.Candidates[0]["description"])
}

func TestFormat_IgnoresToComplete(t *testing.T) {
	t.Parallel()

	bare := elvish.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})
	prefixed := elvish.Format("de", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.Equal(t, bare, prefixed)
}

func TestFormat_EmptyCandidateList(t *testing.T) {
	t.Parallel()

	output := elvish.Format("", wire.Directives{}, nil)

	assert.Equal(t, `{"usage":"","messages":[],"descriptionStyle":"dim","candidates":[]}`, output)
}

func TestFormat_ColorDisabledVariants(t *testing.T) {
	candidates := []wire.Candidate{
		{Value: "serve", Display: "serve", Description: "start web server", Style: "blue"},
	}

	t.Run("CONGA_COLOR=0", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := elvish.Format("", wire.Directives{}, candidates)

		var parsed struct {
			DescriptionStyle string           `json:"descriptionStyle"`
			Candidates       []map[string]any `json:"candidates"`
		}

		assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
		assert.Equal(t, "default", parsed.DescriptionStyle, "descriptionStyle must be default when color is disabled")
		assert.Equal(t, any("default"), parsed.Candidates[0]["style"], "candidate style must be default when color is disabled")
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		output := elvish.Format("", wire.Directives{}, candidates)

		var parsed struct {
			DescriptionStyle string           `json:"descriptionStyle"`
			Candidates       []map[string]any `json:"candidates"`
		}

		assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
		assert.Equal(t, "default", parsed.DescriptionStyle)
		assert.Equal(t, any("default"), parsed.Candidates[0]["style"])
	})

	t.Run("CLICOLOR=0", func(t *testing.T) {
		t.Setenv("CLICOLOR", "0")

		output := elvish.Format("", wire.Directives{}, candidates)

		var parsed struct {
			DescriptionStyle string           `json:"descriptionStyle"`
			Candidates       []map[string]any `json:"candidates"`
		}

		assert.NoError(t, json.Unmarshal([]byte(output), &parsed))
		assert.Equal(t, "default", parsed.DescriptionStyle)
		assert.Equal(t, any("default"), parsed.Candidates[0]["style"])
	})
}
