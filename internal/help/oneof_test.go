package help_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

func TestUsage_OneOfGroupRendersAlternation(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--json | --yaml) <target>", usage)
}

func TestUsage_OneOfGroupRendersValueSlots(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "input-file", TargetType: reflect.TypeFor[string](), Placeholder: "PATH", OneOfGroup: "source"},
			{Name: "input-url", TargetType: reflect.TypeFor[string](), Placeholder: "URL", OneOfGroup: "source"},
		},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--input-file=PATH | --input-url=URL)", usage)
}

func TestUsage_OneOfGroupSingleVisibleMemberRendersPlainToken(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, Hidden: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app --json <target>", usage)
}

func TestUsage_OneOfGroupAllMembersHiddenRendersNothing(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Hidden: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, Hidden: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app <target>", usage)
}

func TestUsage_OneOfGroupDoesNotSetFlagsPlaceholder(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
			{Name: "verbose", IsBool: true},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--json | --yaml) [flags] <target>", usage)
}

func TestUsage_OneOfGroupOnAncestor(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
		},
		Subcommands: []*spec.Command{{Name: "build"}},
	}

	usage := help.Usage([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  app build (--json | --yaml)", usage)
}

func TestUsage_FlagsPlaceholderPrecedesCollectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *spec.Command
		want string
	}{
		{
			name: "dash args",
			cmd: &spec.Command{
				Name:     "app",
				Flags:    []*spec.Flag{{Name: "verbose", IsBool: true}},
				Args:     []*spec.Arg{{Name: "<target>", Required: true}},
				DashArgs: &spec.DashArgs{Name: "[-- <args...>]"},
			},
			want: "Usage:\n  app [flags] <target> [-- <args...>]",
		},
		{
			name: "raw args",
			cmd: &spec.Command{
				Name:    "app",
				Flags:   []*spec.Flag{{Name: "verbose", IsBool: true}},
				RawArgs: &spec.RawArgs{Name: "[command] [args...]"},
			},
			want: "Usage:\n  app [flags] [command] [args...]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, help.Usage([]*spec.Command{test.cmd}))
		})
	}
}
