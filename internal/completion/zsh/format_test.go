package zsh_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/completion/zsh"
)

func plainCandidates() []wire.Candidate {
	return []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service to a cluster", Style: "blue", Tag: "commands"},
		{Value: "logs", Display: "logs"},
	}
}

func TestFormat_RecordStream(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	output := zsh.Format("", wire.Directives{}, plainCandidates())

	assert.Equal(t,
		"D\x1fnoprefix\x1f\n"+
			"C\x1fcommands\x1fdeploy\x1fDeploy service to a cluster\x1fdeploy \x1f\n"+
			"C\x1f\x1flogs\x1f\x1flogs \x1f",
		output)
}

func TestFormat_DirectivesRecord(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	var directives wire.Directives

	directives.KeepOrder = true
	directives.NoSpace.Add('*')
	directives.Usage = "usage: app <command>"

	output := zsh.Format("", directives, nil)

	// No candidates: the noprefix rule does not apply, and the usage string
	// still folds into the message stream (ActiveHelp parity).
	assert.Equal(t, "D\x1fkeeporder,nospace\x1fusage: app <command>\nM\x1fusage: app <command>", output)
}

func TestFormat_NoprefixOnlyOnEmptyWord(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	empty := zsh.Format("", wire.Directives{}, plainCandidates())
	assert.Contains(t, empty, "D\x1fnoprefix\x1f", "an empty word makes candidates complete replacements")

	prefixed := zsh.Format("de", wire.Directives{}, plainCandidates())
	assert.NotContains(t, prefixed, "noprefix", "a typed prefix must not suppress auto-insertion")
}

func TestFormat_UsageFoldsIntoMessageStream(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	directives := wire.Directives{Usage: "Target deployment environment"}

	output := zsh.Format("", directives, plainCandidates())

	assert.Contains(t, output, "M\x1f\x1b[2mTarget deployment environment\x1b[39m")
}

func TestFormat_ErrorUsageNotDuplicated(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	// FromResult folds the usage into Messages on DirectiveError; the fold
	// must not emit the usage string twice.
	directives := wire.Directives{Usage: "usage: app <command>"}
	directives.Messages = append(directives.Messages, "usage: app <command>")

	output := zsh.Format("", directives, nil)

	assert.Equal(t, 1, strings.Count(output, "M\x1f"), "usage already present in messages must not repeat")
}

func TestFormat_MessagesCarryUsageStyle(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	var directives wire.Directives

	directives.Messages = append(directives.Messages, "usage: app <command>")

	output := zsh.Format("", directives, nil)

	assert.Contains(t, output, "M\x1fusage: app <command>")
	assert.NotContains(t, output, "\x1b[")
}

func TestFormat_MergeFlags(t *testing.T) {
	t.Run("default-on", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		candidates := []wire.Candidate{
			{Value: "--env", Display: "--env", Tag: "longhand flags"},
			{Value: "-e", Display: "-e", Tag: "shorthand flags"},
		}

		output := zsh.Format("", wire.Directives{}, candidates)

		assert.Contains(t, output, "C\x1fflags\x1f--env")
		assert.Contains(t, output, "C\x1fflags\x1f-e")
	})

	t.Run("explicit false keeps separate tags", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")
		t.Setenv("CONGA_MERGE_FLAGS", "false")

		candidates := []wire.Candidate{
			{Value: "--env", Display: "--env", Tag: "longhand flags"},
			{Value: "-e", Display: "-e", Tag: "shorthand flags"},
		}

		output := zsh.Format("", wire.Directives{}, candidates)

		assert.Contains(t, output, "C\x1flonghand flags\x1f--env")
		assert.Contains(t, output, "C\x1fshorthand flags\x1f-e")
	})

	t.Run("explicit true merges", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")
		t.Setenv("CONGA_MERGE_FLAGS", "true")

		candidates := []wire.Candidate{
			{Value: "--env", Display: "--env", Tag: "longhand flags"},
		}

		output := zsh.Format("", wire.Directives{}, candidates)

		assert.Contains(t, output, "C\x1fflags\x1f--env")
	})
}

func TestFormat_TrailingSpace(t *testing.T) {
	t.Run("appended by default", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := zsh.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

		assert.Contains(t, output, "\x1fdeploy \x1f")
	})

	t.Run("suppressed for equals suffix", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := zsh.Format("", wire.Directives{}, []wire.Candidate{{Value: "team=", Display: "team="}})

		assert.Contains(t, output, "\x1fteam=\x1f")
	})

	t.Run("suppressed for slash suffix", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := zsh.Format("", wire.Directives{}, []wire.Candidate{{Value: "subdir/", Display: "subdir/"}})

		assert.Contains(t, output, "\x1fsubdir/\x1f")
	})

	t.Run("suppressed by the nospace directive", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		var directives wire.Directives
		directives.NoSpace.Add('*')

		output := zsh.Format("", directives, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

		assert.Contains(t, output, "\x1fdeploy\x1f")
	})
}

func TestFormat_ValueEscaping(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	candidates := []wire.Candidate{
		{Value: "my app.yaml", Display: "my app.yaml"},
		{Value: "it's", Display: "it's"},
		{Value: "a$b", Display: "a$b"},
	}

	output := zsh.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, "\x1fmy\\ app.yaml \x1f")
	assert.Contains(t, output, "\x1fit\\'s \x1f")
	assert.Contains(t, output, "\x1fa\\$b \x1f")
}

func TestFormat_ValueEscapingPreservesAttachedAndTilde(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	candidates := []wire.Candidate{
		{Value: "--env=staging", Display: "staging"},
		{Value: "~/bin/tool", Display: "~/bin/tool"},
	}

	output := zsh.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, "\x1f--env=staging \x1f")
	assert.Contains(t, output, "\x1f~/bin/tool \x1f")
}

func TestFormat_NamedDirectoriesPassThrough(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	output := zsh.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "/home/user/docs/x", Display: "/home/user/docs/x"},
	})

	assert.Contains(t, output, "\x1f/home/user/docs/x \x1f")
}

func TestFormat_TagGroupingStable(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	candidates := []wire.Candidate{
		{Value: "a", Display: "a", Tag: "commands"},
		{Value: "f", Display: "f", Tag: "flags"},
		{Value: "b", Display: "b", Tag: "commands"},
		{Value: "u", Display: "u"},
	}

	output := zsh.Format("", wire.Directives{}, candidates)

	assert.Equal(t,
		"D\x1fnoprefix\x1f\n"+
			"C\x1fcommands\x1fa\x1f\x1fa \x1f\n"+
			"C\x1fcommands\x1fb\x1f\x1fb \x1f\n"+
			"C\x1fflags\x1ff\x1f\x1ff \x1f\n"+
			"C\x1f\x1fu\x1f\x1fu \x1f",
		output)
}

func TestFormat_FieldSanitization(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	output := zsh.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "a\nb", Display: "x\ty", Description: "d\r\nc\x1fsep"},
	})

	assert.Contains(t, output, "C\x1f\x1fx y\x1fd  c sep\x1f")
	assert.Contains(t, output, "\x1fa\\ b \x1f")
}

func TestFormat_ListColors(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	candidates := []wire.Candidate{
		{Value: "--env", Display: "--env", Tag: "flags", Style: "blue"},
		{Value: "--tag", Display: "--tag", Tag: "flags", Style: "magenta"},
		{Value: "--dry-run", Display: "--dry-run", Tag: "flags"},
		{Value: "deploy", Display: "deploy", Tag: "commands"},
	}

	output := zsh.Format("", wire.Directives{}, candidates)

	assert.Contains(t, output, "L\x1f")
	assert.Contains(t, output, "=(#b)(--env)([ ]## -- *)=0=34=2")
	assert.Contains(t, output, "=(#b)(--env)=0=34")
	assert.Contains(t, output, "=(#b)(--tag)=0=35")
	assert.Contains(t, output, "=(#b)(--dry-run)=0=")
	assert.Contains(t, output, "=(#b)(deploy)=0=")
	assert.Contains(t, output, "=(#b)(-- *)=0=2")
}

func TestFormat_ListColorsPatternEscaping(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	output := zsh.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "my:file", Display: "my:file"},
		{Value: "a*b", Display: "a*b"},
	})

	assert.Contains(t, output, "=(#b)(my\\:file)=0=")
	assert.Contains(t, output, "=(#b)(a\\*b)=0=")
}

func TestFormat_ListColorsAbsentWhenColorDisabled(t *testing.T) {
	t.Run("CONGA_COLOR=0", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := zsh.Format("", wire.Directives{}, plainCandidates())

		assert.NotContains(t, output, "L\x1f")
		assert.NotContains(t, output, "\x1b[")
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		output := zsh.Format("", wire.Directives{}, plainCandidates())

		assert.NotContains(t, output, "L\x1f")
	})

	t.Run("CLICOLOR=0", func(t *testing.T) {
		t.Setenv("CLICOLOR", "0")

		output := zsh.Format("", wire.Directives{}, plainCandidates())

		assert.NotContains(t, output, "L\x1f")
	})
}

func TestFormat_ListColorsPresentWithoutCandidates(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	output := zsh.Format("", wire.Directives{}, nil)

	assert.Equal(t, "D\x1f\x1f\nL\x1f=(#b)(-- *)=0=2", output)
}

func TestFormat_StyleLimit(t *testing.T) {
	t.Run("caps candidate entries", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "1")
		t.Setenv("CONGA_ZSH_STYLE_LIMIT", "2")

		candidates := []wire.Candidate{
			{Value: "a", Display: "a", Style: "blue"},
			{Value: "b", Display: "b", Style: "blue"},
			{Value: "c", Display: "c", Style: "blue"},
		}

		output := zsh.Format("", wire.Directives{}, candidates)

		assert.Equal(t, 5, strings.Count(output, "=(#b)("), "two candidate entry pairs plus the catch-all")
	})

	t.Run("negative limit is unlimited", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "1")
		t.Setenv("CONGA_ZSH_STYLE_LIMIT", "-1")

		candidates := []wire.Candidate{
			{Value: "a", Display: "a", Style: "blue"},
			{Value: "b", Display: "b", Style: "blue"},
			{Value: "c", Display: "c", Style: "blue"},
		}

		output := zsh.Format("", wire.Directives{}, candidates)

		assert.Equal(t, 7, strings.Count(output, "=(#b)("))
	})
}

func TestFormat_SgrFieldFollowsColor(t *testing.T) {
	t.Run("color enabled", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "1")

		output := zsh.Format("", wire.Directives{}, []wire.Candidate{
			{Value: "deploy", Display: "deploy", Tag: "commands", Style: "blue"},
			{Value: "logs", Display: "logs"},
		})

		assert.Contains(t, output, "C\x1fcommands\x1fdeploy\x1f\x1fdeploy \x1f34")
		assert.Contains(t, output, "C\x1f\x1flogs\x1f\x1flogs \x1f")
	})

	t.Run("color disabled", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "0")

		output := zsh.Format("", wire.Directives{}, []wire.Candidate{
			{Value: "deploy", Display: "deploy", Tag: "commands", Style: "blue"},
		})

		assert.Contains(t, output, "C\x1fcommands\x1fdeploy\x1f\x1fdeploy \x1f")
	})
}

func TestFormat_DescriptionNotClamped(t *testing.T) {
	t.Setenv("CONGA_COLOR", "0")

	long := strings.Repeat("x", 100)

	output := zsh.Format("", wire.Directives{}, []wire.Candidate{
		{Value: "item", Display: "item", Description: long},
	})

	assert.Contains(t, output, "\x1fitem\x1f"+long+"\x1f")
}
