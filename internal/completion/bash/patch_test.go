package bash //nolint:testpackage // exercises the unexported patch-to-format presentation state channel

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/sentinel"
)

func TestPatch_StashesWordbreakPrefixFromRawLine(t *testing.T) {
	resetPresentationState(t)

	line := `cloudctl deploy --env="pro`
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))

	patched, err := Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched)
	assert.True(t, presentationState.active)
	assert.Equal(t, `--env="`, presentationState.wordbreakPrefix)
	assert.Equal(t, "", presentationState.compType, "unset COMP_TYPE must stash the empty raw value")
	assert.Equal(t, "", os.Getenv("COMP_LINE"))
}

func TestPatch_StashesCompType(t *testing.T) {
	resetPresentationState(t)

	line := "cloudctl deploy --labels team="
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))
	t.Setenv("COMP_TYPE", "63")

	patched, err := Patch([]string{"deploy", "--labels", "team="})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--labels", "team="}, patched)
	assert.Equal(t, "63", presentationState.compType)
	assert.Equal(t, "team=", presentationState.wordbreakPrefix)
}

func TestPatch_UnparseableCompPointFallsBackToLineEnd(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_LINE", "cloudctl deploy --env ")
	t.Setenv("COMP_POINT", "not-a-number")

	patched, err := Patch([]string{"deploy", "--env", ""})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env", ""}, patched)
}

func TestPatch_EmptyLinePassesThrough(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_LINE", "")
	t.Setenv("COMP_POINT", "0")

	args := []string{"deploy"}

	patched, err := Patch(args)

	assert.NoError(t, err)
	assert.Equal(t, args, patched)
	assert.False(t, presentationState.active)
}

func TestPatch_MissingPointPassesThrough(t *testing.T) {
	resetPresentationState(t)
	t.Setenv("COMP_LINE", "cloudctl deploy ")

	args := []string{"deploy", ""}

	patched, err := Patch(args)

	assert.NoError(t, err)
	assert.Equal(t, args, patched)
	assert.False(t, presentationState.active)
}

func TestPatch_RedirectPositionUnsetsCompletionEnvironment(t *testing.T) {
	resetPresentationState(t)

	line := "cloudctl deploy > "
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))

	patched, err := Patch([]string{"deploy", ">", ""})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, presentationState.active)

	_, compLineSet := os.LookupEnv("COMP_LINE")
	assert.False(t, compLineSet, "COMP_LINE must be consumed even on the redirect-rejection path so a repeated Patch never re-lexes it")

	_, compPointSet := os.LookupEnv("COMP_POINT")
	assert.False(t, compPointSet, "COMP_POINT must be consumed even on the redirect-rejection path")
}

func TestPatch_RedirectPositionWithoutTrailingSpace(t *testing.T) {
	resetPresentationState(t)

	line := "cloudctl deploy >"
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))

	patched, err := Patch([]string{"deploy", ">"})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, presentationState.active)
}

func TestPatch_RedirectPositionLeavesPresentationStateClean(t *testing.T) {
	resetPresentationState(t)

	// Pre-pollute presentationState to verify Patch zeroes it at entry.
	presentationState.active = true
	presentationState.compType = "63"
	presentationState.wordbreakPrefix = "--stale="

	line := "cloudctl deploy > "
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))

	patched, err := Patch([]string{"deploy", ">", ""})

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrRedirectTarget)
	assert.Zero(t, patched)
	assert.False(t, presentationState.active, "presentationState.active must be false after redirect error")
	assert.Equal(t, "", presentationState.compType, "presentationState.compType must be empty after redirect error")
	assert.Equal(t, "", presentationState.wordbreakPrefix, "presentationState.wordbreakPrefix must be empty after redirect error")
}

func TestPatch_SequentialCyclesDoNotBleedState(t *testing.T) {
	resetPresentationState(t)

	// Cycle 1: flag with equals sign wordbreak
	line1 := "cloudctl deploy --env=pro"
	t.Setenv("COMP_LINE", line1)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line1)))

	patched1, err := Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched1)
	assert.Equal(t, "--env=", presentationState.wordbreakPrefix)

	out1 := Format("--env=pro", wire.Directives{}, []wire.Candidate{{Value: "--env=production"}})

	assert.Contains(t, out1, "C\tproduction")

	// Cycle 2: colon wordbreak without equals
	line2 := "cloudctl deploy user:al"
	t.Setenv("COMP_LINE", line2)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line2)))

	patched2, err := Patch([]string{"deploy", "user:al"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "user:al"}, patched2)
	assert.Equal(t, "user:", presentationState.wordbreakPrefix)

	out2 := Format("user:al", wire.Directives{}, []wire.Candidate{{Value: "user:alice"}})

	assert.Contains(t, out2, "C\talice")
	assert.False(t, strings.Contains(out2, "--env="), "cycle 2 must not bleed cycle 1 prefix")

	// Cycle 3: clean invocation with no COMP_LINE must clear active state
	patched3, err := Patch([]string{"deploy", "plain"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "plain"}, patched3)
	assert.False(t, presentationState.active, "un-instrumented cycle 3 must leave presentationState inactive")
	assert.Equal(t, "", presentationState.wordbreakPrefix)
}

func TestPatch_CustomCompWordbreaksWithoutEquals(t *testing.T) {
	resetPresentationState(t)

	line := "cloudctl deploy --env=pro"
	t.Setenv("COMP_LINE", line)
	t.Setenv("COMP_POINT", strconv.Itoa(len(line)))
	t.Setenv("COMP_WORDBREAKS", " \t\n\"'><;|&(:")

	patched, err := Patch([]string{"deploy", "--env=pro"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"deploy", "--env=pro"}, patched)
	assert.Equal(t, "", presentationState.wordbreakPrefix, "prefix should be empty when '=' is not in COMP_WORDBREAKS")

	out := Format("--env=pro", wire.Directives{}, []wire.Candidate{{Value: "--env=production"}})

	assert.Contains(t, out, "C\t--env=production", "candidate should retain full flag when '=' is not a wordbreak")
}
