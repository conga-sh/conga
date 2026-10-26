package fish_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/fish"
	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestFormat_ValueDescriptionLines(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service to a cluster", Style: "blue", Tag: "commands"},
		{Value: "logs", Display: "logs", Description: "Stream service logs"},
	}

	output := fish.Format("", wire.Directives{}, candidates)

	assert.Equal(t, "deploy\tDeploy service to a cluster\nlogs\tStream service logs", output)
}

func TestFormat_DescriptionOmitted(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service"},
		{Value: "logs", Display: "logs"},
	}

	output := fish.Format("", wire.Directives{}, candidates)

	assert.Equal(t, "deploy\tDeploy service\nlogs", output)
	assert.False(t, strings.Contains(output, "\t\n"), "no line may end with a bare tab separator")
}

func TestFormat_NoTrailingNewline(t *testing.T) {
	t.Parallel()

	output := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.False(t, strings.HasSuffix(output, "\n"), "the feed must not end with a trailing newline")
}

func TestFormat_EmptyCandidateList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", fish.Format("", wire.Directives{}, nil))
}

func TestFormat_SanitizesTabsAndCarriageReturns(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "cluster-alpha", Display: "cluster-alpha", Description: "First\tcluster\ttabs"},
		{Value: "cluster-beta", Display: "cluster-beta", Description: "Second\rcluster"},
		{Value: "value\twith\ttabs", Display: "value\twith\ttabs"},
	}

	output := fish.Format("", wire.Directives{}, candidates)

	assert.Equal(t,
		"cluster-alpha\tFirst cluster tabs\ncluster-beta\tSecond cluster\nvalue with tabs",
		output)
}

func TestFormat_MultiLineDescriptionKeepsFirstLine(t *testing.T) {
	t.Parallel()

	candidates := []wire.Candidate{
		{Value: "cluster-gamma", Display: "cluster-gamma", Description: "First line\nSecond line"},
	}

	assert.Equal(t, "cluster-gamma\tFirst line", fish.Format("", wire.Directives{}, candidates))
}

func TestFormat_DropsDirectivesMessagesStylesAndTags(t *testing.T) {
	t.Parallel()

	var directives wire.Directives
	directives.NoSpace.Add('*')
	directives.NoPrefix.Add('-')
	directives.KeepOrder = true
	directives.Messages = []string{"usage: app <command>"}
	directives.Usage = "usage: app <command>"

	candidates := []wire.Candidate{
		{Value: "deploy", Display: "deploy", Description: "Deploy service", Style: "blue", Tag: "commands"},
	}

	assert.Equal(t, "deploy\tDeploy service", fish.Format("", directives, candidates))
}

func TestFormat_IgnoresToComplete(t *testing.T) {
	t.Parallel()

	bare := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})
	prefixed := fish.Format("de", wire.Directives{}, []wire.Candidate{{Value: "deploy", Display: "deploy"}})

	assert.Equal(t, bare, prefixed)
}

func TestFormat_DescriptionClampedToDefaultLength(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 100)

	output := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "item", Display: "item", Description: long}})

	assert.Equal(t, "item\t"+strings.Repeat("x", 77)+"...", output)
}

func TestFormat_DescriptionClampCustomLength(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "5")

	output := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "item", Display: "item", Description: "abcdef"}})

	assert.Equal(t, "item\tab...", output)
}

func TestFormat_DescriptionClampHardCutAtShortLimit(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "3")

	output := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "item", Display: "item", Description: "abcdef"}})

	assert.Equal(t, "item\tabc", output)
}

func TestFormat_DescriptionClampHardCutBelowShortLimit(t *testing.T) {
	t.Setenv("CONGA_DESCRIPTION_LENGTH", "2")

	output := fish.Format("", wire.Directives{}, []wire.Candidate{{Value: "item", Display: "item", Description: "abcdef"}})

	assert.Equal(t, "item\tab", output)
}
