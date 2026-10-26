package powershell_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/powershell"
	"github.com/conga-sh/conga/internal/completion/wire"
)

func plainCandidates() []wire.Candidate {
	return []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service to a cluster", Style: "blue", Tag: "commands"},
		{Value: "logs", Display: "logs"},
	}
}

func TestFormat_PlainPayload(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	output := powershell.Format("", wire.Directives{}, plainCandidates())

	assert.Equal(t,
		`{"candidates":[`+
			`{"value":"deploy ","display":"deploy (Deploy service to a cluster)","description":" ","style":"blue","tag":"commands"},`+
			`{"value":"logs ","display":"logs","description":" "}],`+
			`"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}`,
		output)
}

func TestFormat_TrailingSpace(t *testing.T) {
	t.Run("appended by default", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

		assert.Contains(t, output, `"value":"deploy "`)
	})

	t.Run("suppressed for slash suffix", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "dir/", Display: "dir/"}})

		assert.Contains(t, output, `"value":"dir/"`)
	})

	t.Run("suppressed for equals suffix", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "team=", Display: "team="}})

		assert.Contains(t, output, `"value":"team="`)
	})

	t.Run("suppressed by the nospace directive", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		var directives wire.Directives
		directives.NoSpace.Add('*')

		output := powershell.Format("", directives, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

		assert.Contains(t, output, `"value":"deploy"`)
	})
}

func TestFormat_MetaValueQuoting(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	candidates := []wire.Candidate{
		{Value: "foo bar", Display: "foo bar"},
		{Value: "it's", Display: "it's"},
		{Value: "a,b", Display: "a,b"},
	}

	output := powershell.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, `"value":"'foo bar' "`)
	assert.Contains(t, output, `"value":"'it''s' "`)
	assert.Contains(t, output, `"value":"'a,b' "`)
}

func TestFormat_SanitizesFramingCharacters(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	candidates := []wire.Candidate{
		{Value: "a\nb\tc", Display: "x\ny", Description: "First\tline\nSecond"},
	}

	output := powershell.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, `"value":"abc "`)
	assert.Contains(t, output, `"display":"xy (Firstline)"`)
}

func TestFormat_DropsEmptyValues(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	output := powershell.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "", Display: "ghost"},
		{Value: "deploy", Display: "deploy"},
	})

	assert.Contains(t, output, `"candidates":[{"value":"deploy "`)
	assert.False(t, strings.Contains(output, "ghost"), "empty-value candidates must be dropped")
}

func TestFormat_EmptyDisplayFallsBackToSpace(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	assert.Contains(t, powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy"}}), `"display":" "`)
}

func TestFormat_DescriptionClampedToDefaultLength(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	long := strings.Repeat("x", 100)

	output := powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "item", Display: "item", Description: long}})

	assert.Contains(t, output, `"display":"item (`+strings.Repeat("x", 77)+`...)"`)
}

func TestFormat_IgnoresToComplete(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	bare := powershell.Format("", wire.Directives{}, plainCandidates())
	prefixed := powershell.Format("de", wire.Directives{}, plainCandidates())

	assert.Equal(t, bare, prefixed)
}

func TestFormat_CarriesDirectivesAndUsage(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	var directives wire.Directives

	directives.KeepOrder = true
	directives.Messages = []string{"usage: app <command>"}
	directives.Usage = "usage: app <command>"

	output := powershell.Format("", directives, nil)

	assert.Equal(t,
		`{"candidates":[],"directives":{"keeporder":true,"messages":["usage: app <command>"],"noprefix":false,"nospace":false},"usage":"usage: app <command>"}`,
		output)
}

func TestFormat_ColorEnabled(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	output := powershell.Format("", wire.Directives{}, plainCandidates())

	assert.Contains(t, output, `"display":"`+"`e[34mdeploy`e[2m (Deploy service to a cluster)`e[0m\"")
	assert.Contains(t, output, `"display":"logs"`)
	assert.Contains(t, output, `"sgr":"34"`)
}

func TestFormat_ColorEnabledUnstyledCandidate(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	output := powershell.Format("", wire.Directives{}, []wire.Candidate{{Value: "logs", Display: "logs", Description: "Stream logs"}})

	assert.Contains(t, output, `"display":"logs`+"`e[2m (Stream logs)`e[0m\"")
	_, hasSgr := extractCandidateField(t, output, "sgr")
	assert.False(t, hasSgr, "unstyled candidates must not carry an sgr field")
}

func TestFormat_ColorDisabledVariants(t *testing.T) {
	t.Run("CONGA_COLOR=0", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := powershell.Format("", wire.Directives{}, plainCandidates())

		assert.False(t, strings.Contains(output, "`e["), "color-disabled output must not carry escape sequences")
		assert.False(t, strings.Contains(output, `"sgr":`), "color-disabled output must not carry sgr fields")
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		output := powershell.Format("", wire.Directives{}, plainCandidates())

		assert.False(t, strings.Contains(output, "`e["))
	})

	t.Run("CLICOLOR=0", func(t *testing.T) {
		t.Setenv("CLICOLOR", "0")

		output := powershell.Format("", wire.Directives{}, plainCandidates())

		assert.False(t, strings.Contains(output, "`e["))
	})
}

func TestFormat_TooltipMode(t *testing.T) {
	t.Run("color disabled", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")
		t.Setenv("CONGA_TOOLTIP", "1")

		output := powershell.Format("", wire.Directives{}, plainCandidates())

		assert.Contains(t, output, `"display":"deploy"`)
		assert.Contains(t, output, `"description":"Deploy service to a cluster"`)
		assert.Contains(t, output, `"display":"logs"`)
		assert.Contains(t, output, `"description":" "`)
	})

	t.Run("color enabled", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "1")
		t.Setenv("CONGA_TOOLTIP", "1")

		output := powershell.Format("", wire.Directives{}, plainCandidates())

		assert.Contains(t, output, `"display":"`+"`e[34mdeploy`e[0m\"")
		assert.Contains(t, output, `"description":"`+"`e[2mDeploy service to a cluster`e[0m\"")
	})
}

func TestFormat_TooltipDisabledKeepsDescriptionInListItemText(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")
	t.Setenv("CONGA_TOOLTIP", "0")

	output := powershell.Format("", wire.Directives{}, plainCandidates())

	assert.Contains(t, output, `"display":"deploy (Deploy service to a cluster)"`)
	assert.Contains(t, output, `"description":" "`)
}

func TestFormat_SgrVocabulary(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	candidates := []wire.Candidate{
		{Value: "a", Display: "a", Style: "bold red"},
		{Value: "b", Display: "b", Style: "#ff0000"},
		{Value: "c", Display: "c", Style: "bogus"},
	}

	output := powershell.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, `"sgr":"1;31"`)
	assert.Contains(t, output, `"sgr":"38;2;255;0;0"`)
	assert.False(t, strings.Contains(output, `"sgr":""`), "untranslatable styles must omit the sgr field")
}

// extractCandidateField reports whether any candidate object in the payload
// carries the named field.
func extractCandidateField(t *testing.T, payload string, field string) (string, bool) {
	t.Helper()

	marker := `"` + field + `":`

	_, after, ok := strings.Cut(payload, marker)
	if !ok {
		return "", false
	}

	rest := after
	end := strings.IndexAny(rest, ",}!")

	if end == -1 {
		return rest, true
	}

	return strings.Trim(rest[:end], `"`), true
}
