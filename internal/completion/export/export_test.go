package export_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/export"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/predict"
)

func plainDirectives() wire.Directives {
	var directives wire.Directives
	directives.NoSpace.Add('*')
	directives.NoPrefix.Add('-')

	return directives
}

func TestFormat_PlainResult(t *testing.T) {
	t.Parallel()

	directives := plainDirectives()
	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
		{Value: "logs", Display: "logs"},
	}

	output := export.Format("deploy", directives, candidates)

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":"deploy","description":"Deploy service","style":"blue","tag":"commands"},`+
			`{"value":"logs","display":"logs"}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":true,"nospace":true},"usage":""}`,
		output)
}

func TestFormat_DisplayAlwaysPresent(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{}
	candidates := []wire.Candidate{{Value: "deploy", Display: ""}}

	output := export.Format("", directives, candidates)

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":""}],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_EmptyCandidateList(t *testing.T) {
	t.Parallel()

	output := export.Format("", wire.Directives{}, nil)

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_ErrorResultMessagesAndUsage(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{
		KeepOrder: true,
		Messages:  []string{"usage: app <command>"},
		Usage:     "usage: app <command>",
	}

	output := export.Format("", directives, nil)

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":true,"messages":["usage: app <command>"],"noprefix":false,"nospace":false},"usage":"usage: app <command>"}`,
		output)
}

func TestFormat_MultipleMessages(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{Messages: []string{"first", "second \"quoted\""}}

	output := export.Format("", directives, nil)

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":false,"messages":["first","second \"quoted\""],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_JsonEscaping(t *testing.T) {
	t.Parallel()

	// Built by concatenation so the source carries no raw control bytes and no
	// escape sequences that could be misinterpreted: the escaper must emit the
	// literal six-character JSON escapes.
	controlValue := "ctrl" + string(rune(0x01)) + string(rune(0x1f))
	controlEscaped := "\\" + "u0001" + "\\" + "u001f"
	invalidUTF8 := "bad" + string([]byte{0xff}) + "utf8"

	directives := wire.Directives{}
	candidates := []wire.Candidate{
		{Value: "quote\"back\\slash", Display: "d"},
		{Value: controlValue, Display: "d"},
		{Value: invalidUTF8, Display: "d"},
		{Value: "unié", Display: "d"},
	}

	output := export.Format("", directives, candidates)

	assert.Equal(t,
		`{"candidates":[`+
			`{"value":"quote\"back\\slash","display":"d"},`+
			`{"value":"ctrl`+controlEscaped+`","display":"d"},`+
			`{"value":"bad`+"\\"+`ufffdutf8","display":"d"},`+
			`{"value":"unié","display":"d"}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_SingleLineNoTrailingNewline(t *testing.T) {
	t.Parallel()

	output := export.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.False(t, strings.Contains(output, "\n"), "export framing must be a single line")
	assert.False(t, strings.HasSuffix(output, "\n"), "export framing must not end with a newline")
}

func TestFormat_IgnoresToComplete(t *testing.T) {
	t.Parallel()

	bare := export.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})
	prefixed := export.Format("--env=", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.Equal(t, bare, prefixed)
}

func TestMarshal_StyleResolver(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{}
	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Style: "blue"},
		{Value: "logs", Display: "logs"},
	}

	output := export.Marshal(directives, candidates, func(candidate wire.Candidate) string {
		if candidate.Style == "" {
			return ""
		}

		return "34"
	})

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":"deploy","style":"blue","sgr":"34"},{"value":"logs","display":"logs"}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestMarshal_NilResolverOmitsSgr(t *testing.T) {
	t.Parallel()

	directives := wire.Directives{}
	candidates := []wire.Candidate{{Value: "deploy", Display: "deploy", Style: "blue"}}

	output := export.Marshal(directives, candidates, nil)

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":"deploy","style":"blue"}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_SeamEquivalence(t *testing.T) {
	t.Parallel()

	result := predict.Result{
		Items: []predict.Item{
			{Value: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
		},
		Directive: predict.DirectiveNoSpace | predict.DirectiveError,
		Usage:     "usage: app <command>",
	}

	candidates, directives := wire.FromResult(result)

	output := export.Format("", directives, candidates)

	assert.Equal(t,
		`{"candidates":[{"value":"deploy","display":"deploy","description":"Deploy service","style":"blue","tag":"commands"}],`+
			`"directives":{"keeporder":false,"messages":["usage: app <command>"],"noprefix":false,"nospace":true},"usage":"usage: app <command>"}`,
		output)
}
