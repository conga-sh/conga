package help_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

func TestUsage_ImplicitCommandBracketsCommandToken(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "tool",
		Flags: []*spec.Flag{{Name: "verbose"}},
		Subcommands: []*spec.Command{
			{Name: "run", Implicit: true},
		},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  tool [<command>] [flags]", usage)
}

func TestFormat_ImplicitCommandMarksDefaultInDeclarationOrder(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "tool",
		Subcommands: []*spec.Command{
			{Name: "build", Summary: "Build the project"},
			{Name: "run", Summary: "Execute a file", Implicit: true},
			{Name: "install", Summary: "Install dependencies"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "build    Build the project")
	assert.Contains(t, formatted, "run      Execute a file (default)")
	assert.Contains(t, formatted, "install  Install dependencies")
	assert.True(t, strings.Index(formatted, "build") < strings.Index(formatted, "run"))
	assert.True(t, strings.Index(formatted, "run") < strings.Index(formatted, "install"))
}
