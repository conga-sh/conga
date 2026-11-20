package help_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

func TestFormat_XorGroupNamesOtherMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "export",
		Flags: []*spec.Flag{
			{Name: "gzip", IsBool: true, Help: "Compress with gzip", XorGroup: "compression"},
			{Name: "zstd", IsBool: true, Help: "Compress with zstd", XorGroup: "compression"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--gzip              Compress with gzip [xor: --zstd]\n")
	assert.Contains(t, formatted, "--zstd              Compress with zstd [xor: --gzip]\n")
}

func TestFormat_XorGroupNamesEveryOtherMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "report",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Help: "Output in JSON format", XorGroup: "format"},
			{Name: "yaml", IsBool: true, Help: "Output in YAML format", XorGroup: "format"},
			{Name: "table", IsBool: true, Help: "Output in Table format", XorGroup: "format"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Output in JSON format [xor: --yaml, --table]\n")
	assert.Contains(t, formatted, "Output in YAML format [xor: --json, --table]\n")
	assert.Contains(t, formatted, "Output in Table format [xor: --json, --yaml]\n")
}

func TestFormat_AndGroupNamesOtherMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "export",
		Flags: []*spec.Flag{
			{Name: "username", TargetType: reflect.TypeFor[string](), Help: "Proxy user name", AndGroup: "auth"},
			{Name: "password", TargetType: reflect.TypeFor[string](), Help: "Proxy password", AndGroup: "auth"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Proxy user name [and: --password]\n")
	assert.Contains(t, formatted, "Proxy password [and: --username]\n")
}

func TestFormat_FlagInXorAndAndGroupsShowsBothHints(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "export",
		Flags: []*spec.Flag{
			{Name: "gzip", IsBool: true, Help: "Compress with gzip", XorGroup: "compression", AndGroup: "tuning"},
			{Name: "zstd", IsBool: true, Help: "Compress with zstd", XorGroup: "compression"},
			{Name: "level", TargetType: reflect.TypeFor[int](), Help: "Compression level", AndGroup: "tuning"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Compress with gzip [xor: --zstd] [and: --level]\n")
	assert.Contains(t, formatted, "Compression level [and: --gzip]\n")
}

func TestFormat_GroupHintLeavesOutHiddenMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "report",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Help: "Output in JSON format", XorGroup: "format"},
			{Name: "yaml", IsBool: true, Help: "Output in YAML format", XorGroup: "format"},
			{Name: "table", IsBool: true, Help: "Output in Table format", XorGroup: "format", Hidden: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Output in JSON format [xor: --yaml]\n")
	assert.Contains(t, formatted, "Output in YAML format [xor: --json]\n")
	assert.NotContains(t, formatted, "--table")
}

func TestFormat_GroupHintOmittedWhenOtherMembersHidden(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "report",
		Flags: []*spec.Flag{
			{Name: "json", IsBool: true, Help: "Output in JSON format", XorGroup: "format"},
			{Name: "yaml", IsBool: true, Help: "Output in YAML format", XorGroup: "format", Hidden: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--json              Output in JSON format\n")
	assert.NotContains(t, formatted, "[xor:")
}

func TestFormat_XorHintLeavesOutDeprecatedMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "deploy",
		Flags: []*spec.Flag{
			{Name: "replicas", TargetType: reflect.TypeFor[int](), Help: "Number of replicas", XorGroup: "replicas"},
			{Name: "count", TargetType: reflect.TypeFor[int](), Help: "Number of replicas", XorGroup: "replicas", Deprecated: "use --replicas"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--replicas=INT      Number of replicas\n")
	assert.Contains(t, formatted, "--count=INT         Number of replicas [xor: --replicas] [deprecated: use --replicas]\n")
}

func TestFormat_AndHintNamesDeprecatedMember(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "export",
		Flags: []*spec.Flag{
			{Name: "username", TargetType: reflect.TypeFor[string](), Help: "Proxy user name", AndGroup: "auth"},
			{Name: "password", TargetType: reflect.TypeFor[string](), Help: "Proxy password", AndGroup: "auth", Deprecated: "use --token"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Proxy user name [and: --password]\n")
	assert.Contains(t, formatted, "Proxy password [and: --username] [deprecated: use --token]\n")
}

func TestFormat_GroupHintFollowsEnumHint(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "report",
		Flags: []*spec.Flag{
			{Name: "format", TargetType: reflect.TypeFor[string](), Help: "Output format", Enum: []string{"json", "yaml"}, XorGroup: "output"},
			{Name: "raw", IsBool: true, Help: "Print raw records", XorGroup: "output"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Output format [enum: json, yaml] [xor: --raw]\n")
}

func TestFormat_GroupHintOnGlobalFlags(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "quiet", IsBool: true, Help: "Print nothing", XorGroup: "verbosity"},
			{Name: "verbose", IsBool: true, Help: "Print each step", XorGroup: "verbosity"},
		},
	}
	deploy := &spec.Command{Name: "deploy"}
	root.Subcommands = []*spec.Command{deploy}

	formatted := help.Format([]*spec.Command{root, deploy})

	assert.Contains(t, formatted, "Global Flags:\n")
	assert.Contains(t, formatted, "Print nothing [xor: --verbose]\n")
	assert.Contains(t, formatted, "Print each step [xor: --quiet]")
}

func TestFormat_OneOfGroupHasNoHint(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "export",
		Flags: []*spec.Flag{
			{Name: "to-file", TargetType: reflect.TypeFor[string](), Help: "Write to a file", OneOfGroup: "destination"},
			{Name: "to-url", TargetType: reflect.TypeFor[string](), Help: "Upload to a URL", OneOfGroup: "destination"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  export (--to-file=STRING | --to-url=STRING)\n")
	assert.Contains(t, formatted, "Write to a file\n")
	assert.Contains(t, formatted, "Upload to a URL\n")
}

func TestFormat_GroupHintOnShadowedBuiltinFlag(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "tool",
		Flags: []*spec.Flag{
			{Name: "help", IsBool: true, Help: "Print a cheat sheet", XorGroup: "docs"},
			{Name: "manual", IsBool: true, Help: "Open the manual", XorGroup: "docs"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Print a cheat sheet [xor: --manual]\n")
	assert.Contains(t, formatted, "Open the manual [xor: --help]\n")
}
