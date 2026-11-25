package help_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

func TestSynopsis_OneOfGroupRendersAlternation(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	synopsis := help.Synopsis([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--json | --yaml) <target>", synopsis)
}

func TestSynopsis_OneOfGroupRendersValueSlots(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "input-file", TargetType: reflect.TypeFor[string](), Placeholder: "PATH", OneOfGroup: "source"},
			{Name: "input-url", TargetType: reflect.TypeFor[string](), Placeholder: "URL", OneOfGroup: "source"},
		},
	}

	synopsis := help.Synopsis([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--input-file=PATH | --input-url=URL)", synopsis)
}

func TestSynopsis_OneOfGroupOmitsDeprecatedMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "oneofdep",
		Flags: []*spec.Flag{
			{Name: "replicas", TargetType: reflect.TypeFor[int](), OneOfGroup: "scale", Deprecated: "use --count"},
			{Name: "count", TargetType: reflect.TypeFor[int](), OneOfGroup: "scale"},
		},
	}

	assert.Equal(t, "Usage:\n  oneofdep --count=INT", help.Synopsis([]*spec.Command{cmd}))
}

func TestSynopsis_OneOfGroupOmitsDeprecatedMemberFromAlternation(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "xml", IsBool: true, OneOfGroup: "format", Deprecated: "use --yaml"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
		},
	}

	assert.Equal(t, "Usage:\n  app (--json | --yaml)", help.Synopsis([]*spec.Command{cmd}))
}

func TestSynopsis_OneOfGroupKeepsMembersWhenAllDeprecated(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format", Deprecated: "use --output"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format", Deprecated: "use --output"},
		},
	}

	assert.Equal(t, "Usage:\n  app (--json | --yaml)", help.Synopsis([]*spec.Command{cmd}))
}

func TestFormat_OneOfGroupListsDeprecatedMemberInFlags(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "oneofdep",
		Flags: []*spec.Flag{
			{Name: "replicas", TargetType: reflect.TypeFor[int](), OneOfGroup: "scale", Deprecated: "use --count", Help: "Replica count"},
			{Name: "count", TargetType: reflect.TypeFor[int](), OneOfGroup: "scale", Help: "Replica count"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  oneofdep --count=INT\n")
	assert.Contains(t, formatted, "--replicas=INT      Replica count [deprecated: use --count]")
	assert.Contains(t, formatted, "--count=INT         Replica count\n")
}

func TestSynopsis_OneOfGroupSingleVisibleMemberRendersPlainToken(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, Hidden: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	synopsis := help.Synopsis([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app --json <target>", synopsis)
}

func TestSynopsis_OneOfGroupAllMembersHiddenRendersNothing(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Hidden: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, Hidden: true, OneOfGroup: "format"},
		},
		Args: []*spec.Arg{{Name: "<target>", Required: true}},
	}

	synopsis := help.Synopsis([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app <target>", synopsis)
}

// TestFormat_OneOfGroupAllMembersHiddenOmitsRowsAndHint pins the documented
// behavior: a fully hidden exactly-one group is still enforced at runtime, but
// the help screen neither lists its members nor advertises the group.
func TestFormat_OneOfGroupAllMembersHiddenOmitsRowsAndHint(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Hidden: true, Help: "JSON output", OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, Hidden: true, Help: "YAML output", OneOfGroup: "format"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  app")
	assert.NotContains(t, formatted, "--json")
	assert.NotContains(t, formatted, "--yaml")
}

func TestSynopsis_OneOfGroupDoesNotSetFlagsPlaceholder(t *testing.T) {
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

	synopsis := help.Synopsis([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  app (--json | --yaml) [flags] <target>", synopsis)
}

func TestSynopsis_OneOfGroupOnAncestor(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, OneOfGroup: "format"},
			{Name: "yaml", IsBool: true, OneOfGroup: "format"},
		},
		Subcommands: []*spec.Command{{Name: "build"}},
	}

	synopsis := help.Synopsis([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  app build (--json | --yaml)", synopsis)
}

func TestSynopsis_FlagsPlaceholderPrecedesCollectors(t *testing.T) {
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
				RawArgs: &spec.RawArgs{Name: "[args...]"},
			},
			want: "Usage:\n  app [flags] [args...]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, help.Synopsis([]*spec.Command{test.cmd}))
		})
	}
}
