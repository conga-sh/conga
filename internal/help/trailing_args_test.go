package help_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

func TestFormat_DashArgsHelpRendersAfterPositionals(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:     "wrap",
		Args:     []*spec.Arg{{Name: "<tool>", Required: true, Help: "Tool to run"}},
		DashArgs: &spec.DashArgs{Name: "[-- <args...>]", Help: "Arguments passed to the tool"},
	}

	assert.Contains(t, help.Format([]*spec.Command{cmd}), "Arguments:\n  <tool>          Tool to run\n  [-- <args...>]  Arguments passed to the tool\n")
}

func TestFormat_RawArgsHelpUsesNameTag(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "exec",
		RawArgs: &spec.RawArgs{Name: "[command...]", Help: "Command and its arguments"},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  exec [command...]\n")
	assert.Contains(t, formatted, "Arguments:\n  [command...]  Command and its arguments\n")
}

func TestFormat_CollectorWithoutHelpHasNoArgumentsSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:     "wrap",
		DashArgs: &spec.DashArgs{Name: "[-- <args...>]"},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  wrap [-- <args...>]\n")
	assert.NotContains(t, formatted, "Arguments:")
}

func TestFormat_BranchDashArgsHelpRendersArgumentsSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:        "tool",
		IsRunnable:  true,
		Subcommands: []*spec.Command{{Name: "build", Summary: "Build the project"}},
		DashArgs:    &spec.DashArgs{Name: "[-- <args...>]", Help: "Arguments for the default action"},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Commands:\n  build  Build the project\n")
	assert.Contains(t, formatted, "Arguments:\n  [-- <args...>]  Arguments for the default action\n")
}
