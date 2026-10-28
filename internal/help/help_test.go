package help_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

var (
	// Expected help renderings for grouped and ungrouped section layouts.
	//go:embed testdata/grouped_flags.golden
	goldenGroupedFlags string

	//go:embed testdata/grouped_subcommands.golden
	goldenGroupedSubcommands string

	//go:embed testdata/ungrouped_flags.golden
	goldenUngroupedFlags string
)

type ExistingFile string

type ExistingDir string

type HTTPMethod string

type Port uint16

// ByteSize implements Help() with a pointer receiver to exercise the
// pointer-receiver detection path.
type ByteSize int64

func (b *ByteSize) Help() string { return "Byte size (e.g., 10MB, 500KiB, 2GB)" }

// SelfDescribing implements Help() with a value receiver.
type SelfDescribing string

func (s SelfDescribing) Help() string { return "A self-describing value" }

// PlainType does not implement Help().
type PlainType string

// TestFormat_MetavarFromType verifies that value slots derive their metavar
// from the flag's type (never its name): smart-cased named types, Go-kind
// metavars for anonymous primitives, and element-type metavars for slices.
func TestFormat_MetavarFromType(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "types",
		Flags: []*spec.Flag{
			{Name: "net-port", TargetType: reflect.TypeFor[Port]()},
			{Name: "input", TargetType: reflect.TypeFor[ExistingFile]()},
			{Name: "output", TargetType: reflect.TypeFor[ExistingDir]()},
			{Name: "method", TargetType: reflect.TypeFor[HTTPMethod]()},
			{Name: "retries", TargetType: reflect.TypeFor[int]()},
			{Name: "name", TargetType: reflect.TypeFor[string]()},
			{Name: "enable", TargetType: reflect.TypeFor[bool](), IsBool: true},
			{Name: "ports", IsSlice: true, TargetType: reflect.TypeFor[[]Port]()},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--net-port=PORT")
	assert.Contains(t, formatted, "--input=EXISTING-FILE")
	assert.Contains(t, formatted, "--output=EXISTING-DIR")
	assert.Contains(t, formatted, "--method=HTTP-METHOD")
	assert.Contains(t, formatted, "--retries=INT")
	assert.Contains(t, formatted, "--name=STRING")
	assert.Contains(t, formatted, "--ports=PORT,...")
	assert.NotContains(t, formatted, "--name=NAME")
	assert.NotContains(t, formatted, "--net-port=NET-PORT")
}

func TestFormat_MetavarFallbackForUnknownType(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "unknown",
		Flags: []*spec.Flag{
			{Name: "widget"},
			{Name: "blob", TargetType: reflect.TypeFor[struct{ Any string }]()},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--widget=VALUE")
	assert.Contains(t, formatted, "--blob=VALUE")
}

// TestFormat_WhyFlagFormats verifies that the built-in --why row advertises a
// value slot and format list only when the app enabled the JSON output format,
// and that the advertised formats are read from the chain root so synthesized
// leaf commands (e.g. the completion help screen) inherit them.
func TestFormat_WhyFlagFormats(t *testing.T) {
	t.Parallel()

	t.Run("plain toggle without WhyJSON", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app"}})

		assert.Contains(t, formatted, "    --why")
		assert.NotContains(t, formatted, "--why[=FORMAT]")
		assert.NotContains(t, formatted, "[enum: text")
	})

	t.Run("value slot and format list with WhyJSON", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app", WhyJSONEnabled: true}})

		assert.Contains(t, formatted, "    --why[=FORMAT]")
		assert.Contains(t, formatted, "[enum: text, json]")
	})

	t.Run("leaf without WhyJSON inherits the root formats", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "app", WhyJSONEnabled: true}
		leaf := &spec.Command{Name: "completion"}

		formatted := help.Format([]*spec.Command{root, leaf})

		assert.Contains(t, formatted, "    --why[=FORMAT]")
		assert.Contains(t, formatted, "[enum: text, json]")
	})
}

// TestFormat_BuiltinVisibilityToggles verifies each built-in flag row is
// gated by its command toggle at the renderer level, so an unpropagated
// app-wide option cannot leak a row into the Flags section.
func TestFormat_BuiltinVisibilityToggles(t *testing.T) {
	t.Parallel()

	t.Run("renders every builtin row by default", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app"}})

		assert.Contains(t, formatted, "--help")
		assert.Contains(t, formatted, "--version")
		assert.Contains(t, formatted, "--why")
		assert.Contains(t, formatted, "--completion")
	})

	t.Run("omits help row when help is disabled", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app", DisableHelp: true}})

		assert.NotContains(t, formatted, "--help")
	})

	t.Run("omits version row when version is disabled", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app", DisableVersion: true}})

		assert.NotContains(t, formatted, "--version")
	})

	t.Run("omits why row when why is disabled", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app", DisableWhy: true}})

		assert.NotContains(t, formatted, "--why")
	})

	t.Run("omits completion row when completion is disabled", func(t *testing.T) {
		t.Parallel()

		formatted := help.Format([]*spec.Command{{Name: "app", DisableCompletion: true}})

		assert.NotContains(t, formatted, "--completion")
	})

	t.Run("omits completion row on a command with user subcommands", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{
			Name:        "app",
			Subcommands: []*spec.Command{{Name: "deploy"}},
		}

		formatted := help.Format([]*spec.Command{root})

		assert.Contains(t, formatted, "--help")
		assert.NotContains(t, formatted, "--completion")
	})
}

// TestFormat_TypeProvidedHelpFallback verifies that flags without an explicit
// help tag inherit the default help text contributed by their value type: an
// explicit tag always wins, pointer and value Help() receivers are both
// detected, slices unwrap to their element type, and the built-in time.Duration
// special case renders a format hint.
func TestFormat_TypeProvidedHelpFallback(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "typed",
		Flags: []*spec.Flag{
			{Name: "size", TargetType: reflect.TypeFor[ByteSize]()},
			{Name: "label", TargetType: reflect.TypeFor[SelfDescribing]()},
			{Name: "sizes", IsSlice: true, TargetType: reflect.TypeFor[[]ByteSize]()},
			{Name: "timeout", TargetType: reflect.TypeFor[time.Duration]()},
			{Name: "plain", TargetType: reflect.TypeFor[PlainType]()},
			{Name: "explicit", Help: "Custom text", TargetType: reflect.TypeFor[ByteSize]()},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "A self-describing value")
	assert.Contains(t, formatted, "Duration (e.g., 300ms, 2m45s, 1h)")

	// The byte-size help is rendered for both the direct flag and the slice flag.
	assert.Equal(t, 2, strings.Count(formatted, "Byte size (e.g., 10MB, 500KiB, 2GB)"))

	// An explicit help tag overrides the type-provided help without appending it.
	assert.Contains(t, formatted, "Custom text")
	assert.NotContains(t, formatted, "Custom text Byte size")

	// A type without Help() contributes nothing to the help column.
	assert.Equal(t, "--plain=PLAIN-TYPE", strings.TrimSpace(lineContaining(formatted, "--plain=")))
}

func TestHelpFormatting_Basic(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "testapp",
		Summary: "A test CLI application",
		Flags: []*spec.Flag{
			{
				Name:    "verbose",
				Short:   'v',
				Help:    "Verbose logging",
				IsBool:  true,
				Default: "false",
			},
			{
				Name:        "port",
				Short:       'p',
				Help:        "Server port",
				Default:     "8080",
				Placeholder: "PORT",
				EnvVars:     []string{"TESTAPP_PORT"},
			},
		},
		Args: []*spec.Arg{
			{
				Name:     "<target>",
				Help:     "Target host",
				Required: true,
			},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})
	assert.Contains(t, formatted, "A test CLI application")
	assert.Contains(t, formatted, "Usage:\n  testapp <target> [flags...]")
	assert.Contains(t, formatted, "-v, --verbose")
	assert.Contains(t, formatted, "-p, --port=PORT")
	assert.Contains(t, formatted, "[default: 8080]")
	assert.Contains(t, formatted, "[env: TESTAPP_PORT]")
	assert.Contains(t, formatted, "<target>")
	assert.NotContains(t, formatted, "[required]")
}

func TestFormat_RequiredFlagsInUsageLine(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "deploy",
		Summary: "Deploy subcommand",
		Flags: []*spec.Flag{
			{
				Name:       "target",
				Help:       "Deploy target",
				TargetType: reflect.TypeFor[string](),
				Required:   true,
				Handle:     testContainerHandle{},
			},
			{
				Name:       "profile",
				Help:       "Build profile",
				Default:    "dev",
				TargetType: reflect.TypeFor[string](),
				Required:   true,
				Handle:     testContainerHandle{},
			},
			{
				Name:    "replicas",
				Help:    "Replica count",
				Default: "1",
			},
		},
		Args: []*spec.Arg{
			{
				Name:     "<name>",
				Help:     "Deployment name",
				Required: true,
			},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  deploy --target=STRING --profile=\"dev\" <name> [flags...]")
	assert.Contains(t, formatted, "--target=STRING")
	assert.Contains(t, formatted, "Deploy target")
	assert.Contains(t, formatted, "--profile=\"dev\"")
	assert.Contains(t, formatted, "Build profile")
	assert.Contains(t, formatted, "[default: 1]")
	assert.NotContains(t, formatted, "[required]")
}

func TestFormat_RequiredFlagPlaceholderInUsageLine(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "server",
		Summary: "Server subcommand",
		Flags: []*spec.Flag{
			{
				Name:        "server-port",
				Help:        "Server port",
				Placeholder: "PORT",
				TargetType:  reflect.TypeFor[int](),
				Required:    true,
				Handle:      testContainerHandle{},
			},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Usage:\n  server --server-port=PORT")
	assert.Contains(t, formatted, "--server-port=PORT  Server port")
}

// TestFormat_TableAlignmentCountsRunes verifies that column widths are
// measured in runes, so a multi-byte label does not push the help column.
func TestFormat_TableAlignmentCountsRunes(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Args: []*spec.Arg{
			{Name: "<café>", Help: "Coffee", Required: true},
			{Name: "<ab>", Help: "Short", Required: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Arguments:\n  <café>  Coffee\n  <ab>    Short")
}

// TestFormat_EmptyHelpCellHasNoTrailingWhitespace verifies that a table row
// with an empty help cell ends at its label: no padding is emitted when there
// is nothing to align.
func TestFormat_EmptyHelpCellHasNoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{
			{Name: "empty"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Commands:\n  empty\n\nFlags:")
}

// TestFormat_WidthWrapsCells pins the wrap contract: cells fold at space
// boundaries and continuations align under the help column.
func TestFormat_WidthWrapsCells(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:              "app",
		DisableHelp:       true,
		DisableVersion:    true,
		DisableWhy:        true,
		DisableCompletion: true,
		Flags: []*spec.Flag{
			{
				Name:        "config",
				Help:        "Path to the configuration file used by the application",
				Placeholder: "FILE",
				TargetType:  reflect.TypeFor[string](),
			},
		},
	}

	formatted := help.FormatWith([]*spec.Command{cmd}, help.Options{Width: 60})

	assert.Contains(t, formatted, "      --config=FILE  Path to the configuration file used by\n")
	assert.Contains(t, formatted, "                     the application")
}

// TestFormat_WrapKeepsHintGroupIntact pins that a bracketed hint group is never
// split: the space inside "[env: NAME]" must not become a wrap point, so the
// closing bracket is never orphaned onto its own line.
func TestFormat_WrapKeepsHintGroupIntact(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:              "app",
		DisableHelp:       true,
		DisableVersion:    true,
		DisableWhy:        true,
		DisableCompletion: true,
		Flags: []*spec.Flag{
			{
				Name:       "secret-key",
				Help:       "API secret (config key remapped, explicit env name)",
				EnvVars:    []string{"KITCHEN_SECRET"},
				TargetType: reflect.TypeFor[string](),
			},
		},
	}

	formatted := help.FormatWith([]*spec.Command{cmd}, help.Options{Width: 63})

	assert.Contains(t, formatted, "[env: KITCHEN_SECRET]")
	assert.NotContains(t, formatted, "[env:\n")
}

// TestFormat_WidthStacksLongLabels pins the narrow-width contract: a label too
// wide for the help column (capped at half the width) renders on its own line,
// while shorter labels keep the inline layout.
func TestFormat_WidthStacksLongLabels(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "very-long-flag-name", Help: "Explains the long flag", IsBool: true},
			{Name: "short", Help: "Explains the short flag", IsBool: true},
		},
	}

	formatted := help.FormatWith([]*spec.Command{cmd}, help.Options{Width: 40})

	assert.Contains(t, formatted, "      --very-long-flag-name\n                    Explains the long")
	assert.Contains(t, formatted, "      --short       Explains the short")
}

// TestFormat_WidthWrapsDescription pins that overlong description lines wrap
// to the width while the author's explicit line breaks survive.
func TestFormat_WidthWrapsDescription(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:              "app",
		Description:       "First paragraph stays short.\nA second paragraph that is long enough to fold at the wrap width.",
		DisableHelp:       true,
		DisableVersion:    true,
		DisableWhy:        true,
		DisableCompletion: true,
	}

	formatted := help.FormatWith([]*spec.Command{cmd}, help.Options{Width: 40})

	assert.Contains(t, formatted, "First paragraph stays short.\nA second paragraph that is long enough\nto fold at the wrap width.\n\nUsage:")
}

// TestUsage_UserDeclaredBuiltinShadows verifies that user flags reusing the
// freed --help/--version names and aliases stay first-class: only the shared
// engine built-in rows are excluded from the synopsis and Global Flags.
func TestUsage_UserDeclaredBuiltinShadows(t *testing.T) {
	t.Parallel()

	t.Run("required help shadow renders in the synopsis", func(t *testing.T) {
		t.Parallel()

		cmd := &spec.Command{
			Name: "app",
			Flags: []*spec.Flag{
				{
					Name:        "help",
					Help:        "Custom help topic",
					Placeholder: "TOPIC",
					TargetType:  reflect.TypeFor[string](),
					Required:    true,
					Handle:      testContainerHandle{},
				},
			},
		}

		usage := help.Usage([]*spec.Command{cmd})

		assert.Equal(t, "Usage:\n  app --help=TOPIC", usage)
	})

	t.Run("ancestor alias shadow stays in the leaf synopsis and Global Flags", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{
			Name: "app",
			Flags: []*spec.Flag{
				{Name: "assist", Aliases: []string{"help"}, Help: "Show assistance", IsBool: true, Required: true},
			},
			Subcommands: []*spec.Command{{Name: "build"}},
		}

		formatted := help.Format([]*spec.Command{root, root.Subcommands[0]})

		assert.Contains(t, formatted, "Usage:\n  app build --assist")
		assert.Contains(t, formatted, "Global Flags:\n      --assist  Show assistance")
	})

	t.Run("shared engine builtin row stays out of the synopsis", func(t *testing.T) {
		t.Parallel()

		cmd := &spec.Command{
			Name:  "app",
			Flags: []*spec.Flag{spec.BuiltinHelpFlag},
		}

		usage := help.Usage([]*spec.Command{cmd})

		assert.Equal(t, "Usage:\n  app", usage)
	})
}

func TestUsage_And_Hint(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "mycmd",
		Flags: []*spec.Flag{
			{Name: "flag1"},
		},
		Subcommands: []*spec.Command{
			{Name: "sub"},
		},
	}

	usage := help.Usage([]*spec.Command{cmd})
	assert.Equal(t, "Usage:\n  mycmd <command> [flags...]", usage)
	assert.Equal(t, "Run 'mycmd --help' for more information.", help.Hint([]*spec.Command{cmd}))
}

func TestUsage_RequiredGlobalFlagsInSubcommandUsageLine(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{
				Name:       "api-key",
				Help:       "API key",
				TargetType: reflect.TypeFor[string](),
				Required:   true,
				Handle:     testContainerHandle{},
			},
		},
		Subcommands: []*spec.Command{
			{
				Name: "build",
				Flags: []*spec.Flag{
					{
						Name:       "profile",
						Help:       "Build profile",
						Default:    "dev",
						TargetType: reflect.TypeFor[string](),
						Required:   true,
						Handle:     testContainerHandle{},
					},
				},
				Args: []*spec.Arg{
					{Name: "<artifact>", Help: "Artifact name", Required: true},
				},
			},
		},
	}

	usage := help.Usage([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  app build --api-key=STRING --profile=\"dev\" <artifact>", usage)
}

func TestUsage_OptionalGlobalFlagsAppendFlagsPlaceholder(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "behave-conga",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbosity count", IsBool: true},
			{Name: "port", Short: 'p', Help: "Server port", Default: "8080"},
		},
		Subcommands: []*spec.Command{
			{
				Name: "build",
				Flags: []*spec.Flag{
					{
						Name:       "profile",
						Help:       "Build profile",
						Default:    "dev",
						TargetType: reflect.TypeFor[string](),
						Required:   true,
						Handle:     testContainerHandle{},
					},
				},
				Args: []*spec.Arg{
					{Name: "<artifact>", Help: "Artifact name", Required: true},
				},
			},
		},
	}

	usage := help.Usage([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  behave-conga build --profile=\"dev\" <artifact> [flags...]", usage)
}

func TestUsage_MixedGlobalFlagsRenderRequiredAndPlaceholder(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "api-key", Help: "API key", TargetType: reflect.TypeFor[string](), Required: true, Handle: testContainerHandle{}},
			{Name: "verbose", Help: "Verbose logging", IsBool: true},
		},
		Subcommands: []*spec.Command{
			{Name: "build"},
		},
	}

	usage := help.Usage([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  app build --api-key=STRING [flags...]", usage)
}

func TestUsage_RequiredFlagsOnly_OmitsFlagsPlaceholder(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "deploy",
		Flags: []*spec.Flag{
			{Name: "target", Help: "Deploy target", TargetType: reflect.TypeFor[string](), Required: true, Handle: testContainerHandle{}},
		},
	}

	usage := help.Usage([]*spec.Command{cmd})

	assert.Equal(t, "Usage:\n  deploy --target=STRING", usage)
}

// TestUsage_ArgTokensFromSchema verifies that the usage synopsis and the
// Arguments section derive an argument's repetition and optionality from the
// schema while preserving author-supplied brackets: bracketed names keep a
// single bracket pair, and the repetition marker lands inside the placeholder.
func TestUsage_ArgTokensFromSchema(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		arg       *spec.Arg
		wantToken string
	}{
		{
			name:      "required scalar keeps the placeholder",
			arg:       &spec.Arg{Name: "<target>", Help: "Target", Required: true},
			wantToken: "<target>",
		},
		{
			name:      "optional scalar is bracketed",
			arg:       &spec.Arg{Name: "<target>", Help: "Target"},
			wantToken: "[<target>]",
		},
		{
			name:      "author brackets are preserved",
			arg:       &spec.Arg{Name: "[target]", Help: "Target"},
			wantToken: "[target]",
		},
		{
			name:      "required slice gains the marker inside the placeholder",
			arg:       &spec.Arg{Name: "<file>", Help: "File", Required: true, IsSlice: true},
			wantToken: "<file...>",
		},
		{
			name:      "optional slice gains the marker and brackets",
			arg:       &spec.Arg{Name: "<file>", Help: "File", IsSlice: true},
			wantToken: "[<file...>]",
		},
		{
			name:      "author-bracketed slice keeps one bracket pair",
			arg:       &spec.Arg{Name: "[file]", Help: "File", IsSlice: true},
			wantToken: "[file...]",
		},
		{
			name:      "author-bracketed placeholder slice",
			arg:       &spec.Arg{Name: "[<file>]", Help: "File", IsSlice: true},
			wantToken: "[<file...>]",
		},
		{
			name:      "author ellipsis is preserved",
			arg:       &spec.Arg{Name: "[file...]", Help: "File", IsSlice: true},
			wantToken: "[file...]",
		},
		{
			name:      "optional map argument",
			arg:       &spec.Arg{Name: "<key=value>", Help: "Entry", IsMap: true},
			wantToken: "[<key=value...>]",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{Name: "app", Args: []*spec.Arg{testCase.arg}}

			usage := help.Usage([]*spec.Command{cmd})
			formatted := help.Format([]*spec.Command{cmd})

			assert.Equal(t, "Usage:\n  app "+testCase.wantToken, usage)
			assert.Contains(t, formatted, "Arguments:\n  "+testCase.wantToken+"  "+testCase.arg.Help)
		})
	}
}

func TestUsage_HiddenGlobalFlagsDoNotAffectUsageLine(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "secret", Help: "Hidden flag", Hidden: true, TargetType: reflect.TypeFor[string](), Required: true, Handle: testContainerHandle{}},
			{Name: "verbose", Help: "Verbose logging", Hidden: true, IsBool: true},
		},
		Subcommands: []*spec.Command{
			{Name: "build"},
		},
	}

	usage := help.Usage([]*spec.Command{root, root.Subcommands[0]})

	assert.Equal(t, "Usage:\n  app build", usage)
}

func TestUsage_BuiltinLeavesExcludeGlobalFlags(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "api-key", Help: "API key", TargetType: reflect.TypeFor[string](), Required: true, Handle: testContainerHandle{}},
			{Name: "verbose", Help: "Verbose logging", IsBool: true},
		},
		Subcommands: []*spec.Command{
			{Name: "help", IsHelpCmd: true, Args: []*spec.Arg{{Name: "[<command>...]", IsSlice: true}}},
			{Name: "version", IsVersionCmd: true},
		},
	}

	helpUsage := help.Usage([]*spec.Command{root, subcommandByName(root, "help")})
	assert.Equal(t, "Usage:\n  app help [<command>...]", helpUsage)

	versionUsage := help.Usage([]*spec.Command{root, subcommandByName(root, "version")})
	assert.Equal(t, "Usage:\n  app version", versionUsage)
}

func TestFormat_GlobalFlags(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
			spec.BuiltinHelpFlag,
		},
		Subcommands: []*spec.Command{
			{
				Name:    "server",
				Summary: "Server subcommand",
				Flags: []*spec.Flag{
					{Name: "host", Help: "Server host", Default: "localhost"},
				},
			},
		},
	}

	formatted := help.Format([]*spec.Command{root, root.Subcommands[0]})

	// Root help screen must not render a Global Flags section.
	rootFormatted := help.Format([]*spec.Command{root})
	assert.NotContains(t, rootFormatted, "Global Flags")

	// Subcommand help lists inherited ancestor flags under Global Flags.
	assert.Contains(t, formatted, "Global Flags:")
	assert.Contains(t, formatted, "-v, --verbose")
	assert.Contains(t, formatted, "Verbose logging")

	// The built-in --help flag is excluded from the Global Flags section.
	if index := strings.Index(formatted, "Global Flags:"); index >= 0 {
		assert.NotContains(t, formatted[index:], "--help")
	}
}

func TestFormat_NoGlobalFlagsForBuiltinCommands(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
		},
		Subcommands: []*spec.Command{
			{
				Name:            "completion",
				Summary:         "Generate shell completion scripts",
				IsCompletionCmd: true,
			},
			{
				Name:      "help",
				Summary:   "Show help for a command",
				IsHelpCmd: true,
			},
			{
				Name:         "version",
				Summary:      "Show version information",
				IsVersionCmd: true,
			},
			{
				Name:    "server",
				Summary: "Server subcommand",
			},
		},
	}

	// Built-in infrastructure commands inherit no root flags: their help
	// screens must never render a Global Flags section.
	for _, builtin := range []string{"completion", "help", "version"} {
		formatted := help.Format([]*spec.Command{root, subcommandByName(root, builtin)})

		assert.NotContains(t, formatted, "Global Flags", "built-in %s help must omit root flags", builtin)
	}

	// User subcommands still list inherited ancestor flags under Global Flags.
	userFormatted := help.Format([]*spec.Command{root, subcommandByName(root, "server")})

	assert.Contains(t, userFormatted, "Global Flags:")
	assert.Contains(t, userFormatted, "-v, --verbose")
}

func TestFormat_HelpAndVersionLast(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "testapp",
		Version: "1.0.0",
		Flags: []*spec.Flag{
			{Name: "signal", Short: 'S', Help: "POSIX signal", IsBool: true},
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--version")
	assert.Contains(t, formatted, "Show version")

	// Built-in flags must be visually aligned with long-only flags (6 spaces
	// before the name, like "      --verbose").
	for _, label := range []string{"--help", "--version"} {
		line := lineContaining(formatted, label)
		assert.True(t, strings.HasPrefix(line, "      "+label), "built-in %s must be column-aligned: %q", label, line)
	}

	// --help and --version must close the Flags section, after every declared flag.
	assert.True(t, strings.Index(formatted, "--verbose") < strings.Index(formatted, "--help"))
	assert.True(t, strings.Index(formatted, "--help") < strings.Index(formatted, "--version"))
}

func TestFormat_ShowsVersionWithoutVersionString(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--version")
	assert.Contains(t, formatted, "--help")
}

func TestFormat_UserDeclaredHelpVersionKeptAtEnd(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
			{Name: "help", Short: 'h', Help: "Custom help", IsBool: true},
			{Name: "version", Help: "Custom version", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "-h, --help")
	assert.Contains(t, formatted, "Custom help")
	assert.Contains(t, formatted, "--version")
	assert.Contains(t, formatted, "Custom version")

	// User-declared help and version flags keep their declarations and are not duplicated.
	assert.Equal(t, 1, strings.Count(formatted, "--help"))
	assert.Equal(t, 1, strings.Count(formatted, "--version"))

	// They still come last, after regular declared flags.
	assert.True(t, strings.Index(formatted, "--verbose") < strings.Index(formatted, "--help"))
	assert.True(t, strings.Index(formatted, "--help") < strings.Index(formatted, "--version"))
}

func TestFormat_RootDeprecationBanner(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:       "testapp",
		Summary:    "A test CLI application",
		Deprecated: "use the new unified CLI",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "WARNING: testapp is deprecated: use the new unified CLI")
	assert.Contains(t, formatted, "A test CLI application")
	assert.True(t, strings.Index(formatted, "WARNING:") < strings.Index(formatted, "A test CLI application"))
}

// TestFormat_SubcommandDeprecationBanner pins that a deprecated command's own
// screen carries the runtime warning wording, with the command path resolved
// from the chain, above the screen content.
func TestFormat_SubcommandDeprecationBanner(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{
			{Name: "legacy", Summary: "Legacy command", Deprecated: "use deploy instead"},
		},
	}

	formatted := help.Format([]*spec.Command{root, root.Subcommands[0]})

	assert.Contains(t, formatted, "WARNING: command 'app legacy' is deprecated: use deploy instead\n\nLegacy command\n\nUsage:")
}

// TestFormat_NestedSubcommandDeprecationBanner pins the command path of a
// banner for a deprecation deeper in the chain.
func TestFormat_NestedSubcommandDeprecationBanner(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{
			{
				Name: "admin",
				Subcommands: []*spec.Command{
					{Name: "legacy", Summary: "Legacy admin command", Deprecated: "use admin migrate"},
				},
			},
		},
	}

	formatted := help.Format([]*spec.Command{root, root.Subcommands[0], root.Subcommands[0].Subcommands[0]})

	assert.Contains(t, formatted, "WARNING: command 'app admin legacy' is deprecated: use admin migrate\n\n")
}

func TestFormat_NoDeprecationBannerWhenNotDeprecated(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "testapp",
		Summary: "A test CLI application",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.NotContains(t, formatted, "WARNING:")
	assert.NotContains(t, formatted, "deprecated")
}

func TestFormat_Examples(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:     "deploy",
		Summary:  "Deploy a container",
		Examples: "\n  # Deploy a single container\n  deploy --image nginx:alpine\n\n  # Deploy with replicas\n  deploy --image nginx --replicas 3\n",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Examples:")
	assert.Contains(t, formatted, "  # Deploy a single container")
	assert.Contains(t, formatted, "  deploy --image nginx:alpine")
	assert.Contains(t, formatted, "  # Deploy with replicas")
	assert.Contains(t, formatted, "  deploy --image nginx --replicas 3")
	assert.NotContains(t, formatted, "\n  \n")
}

func TestFormat_WhitespaceOnlyDescriptionFallback(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:        "status",
		Summary:     "Show status",
		Description: "   \n\t  \n",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Show status")
}

func TestFormat_ExamplesOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "deploy",
		Summary: "Deploy a container",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.NotContains(t, formatted, "Examples:")
}

func TestFormat_HelpFooter(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name:       "app",
		Summary:    "Application root",
		HelpFooter: "Documentation: https://example.com/docs\nReport issues: https://example.com/issues",
		Subcommands: []*spec.Command{
			{Name: "deploy", Summary: "Deploy a container"},
		},
	}

	rootFormatted := help.Format([]*spec.Command{root})
	assert.True(t, strings.HasSuffix(rootFormatted, "Report issues: https://example.com/issues"))
	assert.Contains(t, rootFormatted, "Documentation: https://example.com/docs")

	subFormatted := help.Format([]*spec.Command{root, root.Subcommands[0]})
	assert.True(t, strings.HasSuffix(subFormatted, "Report issues: https://example.com/issues"))
	assert.Contains(t, subFormatted, "Documentation: https://example.com/docs")
}

func TestFormat_HelpFooterOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:    "app",
		Summary: "Application root",
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.NotContains(t, formatted, "Documentation:")
}

func TestFormat_SubcommandDeprecationMarker(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{
			{Name: "legacy", Summary: "Legacy command", Deprecated: "use deploy instead"},
			{Name: "deploy", Summary: "Deploy a container"},
		},
	}

	formatted := help.Format([]*spec.Command{root})

	assert.Contains(t, formatted, "legacy")
	assert.Contains(t, formatted, "[deprecated: use deploy instead]")
	assert.NotContains(t, formatted, "Deploy a container [deprecated:")
}

func TestFormat_FlagDeprecationMarker(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "old", Help: "Old flag", Deprecated: "use --new"},
			{Name: "new", Help: "New flag"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "[deprecated: use --new]")
	assert.NotContains(t, formatted, "New flag [deprecated:")
}

// testContainerHandle is a non-nil opaque engine handle for help fixtures:
// help rendering distinguishes container-backed flags from synthetic built-ins
// by handle presence.
type testContainerHandle struct{}

func lineContaining(haystack, needle string) string {
	for line := range strings.Lines(haystack) {
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.Contains(trimmed, needle) {
			return trimmed
		}
	}

	return ""
}

// subcommandByName returns the direct subcommand of cmd with the given name.
func subcommandByName(cmd *spec.Command, name string) *spec.Command {
	for _, subcommand := range cmd.Subcommands {
		if subcommand.Name == name {
			return subcommand
		}
	}

	return nil
}

func TestFormat_GroupedFlags_Sections(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
			{Name: "repo", Group: "Source", Help: "Repository path", TargetType: reflect.TypeFor[string]()},
			{Name: "quiet", Short: 'q', Group: "Output", Help: "Suppress output", IsBool: true},
			{Name: "color", Short: 'c', Group: "Output", Help: "Colorize output", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Equal(t, goldenGroupedFlags, formatted)
}

func TestFormat_GroupedSubcommands_Sections(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "tool",
		Subcommands: []*spec.Command{
			{Name: "serve", Summary: "Start server"},
			{Name: "commit", Group: "VCS", Summary: "Commit changes"},
			{Name: "status", Group: "VCS", Summary: "Show status"},
			{Name: "build", Group: "Pipeline", Summary: "Build project"},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Equal(t, goldenGroupedSubcommands, formatted)
}

// TestFormat_UngroupedFlagsRegression verifies that schemas without any group
// tags render byte-identical to the single-table layout used before grouped
// sections were introduced.
func TestFormat_UngroupedFlagsRegression(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
			{Name: "repo", Help: "Repository path", TargetType: reflect.TypeFor[string]()},
			{Name: "quiet", Short: 'q', Help: "Suppress output", IsBool: true},
			{Name: "color", Short: 'c', Help: "Colorize output", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Equal(t, goldenUngroupedFlags, formatted)
}

// TestFormat_AllFlagsGrouped_BuiltinsKeepOwnSection verifies that the built-in
// --completion and --help flags close the screen under a trailing "Flags"
// heading when every user flag carries a group.
func TestFormat_AllFlagsGrouped_BuiltinsKeepOwnSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "repo", Group: "Source", Help: "Repository path", TargetType: reflect.TypeFor[string]()},
			{Name: "quiet", Short: 'q', Group: "Output", Help: "Suppress output", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	sourceIndex := strings.Index(formatted, "Source:")
	outputIndex := strings.Index(formatted, "Output:")
	flagsIndex := strings.LastIndex(formatted, "\nFlags:")
	completionIndex := strings.LastIndex(formatted, "--completion")
	helpIndex := strings.LastIndex(formatted, "--help")

	assert.True(t, sourceIndex < outputIndex, "groups must render in first-appearance order")
	assert.True(t, flagsIndex > outputIndex, "the trailing Flags section must follow the groups")
	assert.True(t, completionIndex > flagsIndex, "--completion must open the trailing built-in block")
	assert.True(t, helpIndex > completionIndex, "--help must close the trailing built-in block")
}

// TestFormat_GroupedHiddenFlag_OmitsEmptyGroup verifies that a group whose
// only members are hidden renders no section at all.
func TestFormat_GroupedHiddenFlag_OmitsEmptyGroup(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "secret", Group: "Hidden group", Help: "Internal", Hidden: true},
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.NotContains(t, formatted, "Hidden group")
	assert.Contains(t, formatted, "Flags:")
	assert.Contains(t, formatted, "-v, --verbose")
}

// TestFormat_GroupedConfigFileFlag verifies that a ConfigFile flag's group tag
// drives its own titled section like any other flag.
func TestFormat_GroupedConfigFileFlag(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "testapp",
		Flags: []*spec.Flag{
			{Name: "config", Group: "Configuration", Help: "Path to config file", IsConfigFile: true, Placeholder: "PATH"},
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "Configuration:")
	assert.Contains(t, formatted, "    --config=PATH")
	assert.Contains(t, formatted, "Flags:")
	assert.Contains(t, formatted, "-v, --verbose")
}

// TestFormat_GroupedGlobalFlags verifies that inherited ancestor flags group
// into titled sections within the Global Flags area.
func TestFormat_GroupedGlobalFlags(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "verbose", Short: 'v', Help: "Verbose logging", IsBool: true},
			{Name: "profile", Group: "Connection", Help: "Server profile", TargetType: reflect.TypeFor[string]()},
		},
		Subcommands: []*spec.Command{
			{Name: "server", Summary: "Server subcommand"},
		},
	}

	formatted := help.Format([]*spec.Command{root, root.Subcommands[0]})

	assert.Contains(t, formatted, "Global Flags:")
	assert.Contains(t, formatted, "Connection:")
	assert.Contains(t, formatted, "    --profile=STRING")

	// Ungrouped global flags still render under the default title.
	globalIndex := strings.Index(formatted, "Global Flags:")
	connectionIndex := strings.Index(formatted, "Connection:")

	assert.True(t, globalIndex < connectionIndex, "ungrouped global flags must render first")
}

// TestFormat_BoolDefaultHint verifies the transparency rule for bool defaults:
// a plain bool declaring a true default carries the [default: true] hint while
// the false zero value stays implicit.
func TestFormat_BoolDefaultHint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		flag     *spec.Flag
		wantHint bool
	}{
		{
			name:     "true default is advertised",
			flag:     &spec.Flag{Name: "color", Help: "Colorize output", IsBool: true, Default: "true"},
			wantHint: true,
		},
		{
			name: "false default stays implicit",
			flag: &spec.Flag{Name: "debug", Help: "Debug mode", IsBool: true, Default: "false"},
		},
		{
			name: "non-canonical false default stays implicit",
			flag: &spec.Flag{Name: "verbose", Help: "Verbose mode", IsBool: true, Default: "0"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{Name: "app", Flags: []*spec.Flag{testCase.flag}}

			formatted := help.Format([]*spec.Command{cmd})

			if testCase.wantHint {
				assert.Contains(t, formatted, "[default: true]")

				return
			}

			assert.NotContains(t, formatted, "[default:")
		})
	}
}

// TestFormat_DefaultReferenceDropsValueSlotDefault verifies that referencing
// {{default}} in the help text removes the default from the flag's value slot,
// so the default is stated once — in the help column.
func TestFormat_DefaultReferenceDropsValueSlotDefault(t *testing.T) {
	t.Parallel()

	profile := &spec.Flag{
		Name:       "profile",
		Help:       "Build profile (dev)",
		Default:    "dev",
		TargetType: reflect.TypeFor[string](),
		Handle:     testContainerHandle{},
	}
	profile.HelpVars.Default = true

	cmd := &spec.Command{Name: "deploy", Flags: []*spec.Flag{profile}}

	formatted := help.Format([]*spec.Command{cmd})

	assert.Contains(t, formatted, "--profile=STRING")
	assert.NotContains(t, formatted, `--profile="dev"`)
	assert.Contains(t, formatted, "Build profile (dev)")
	assert.NotContains(t, formatted, "[default:")
}

// TestFormat_HelpVariablesSuppressHints verifies that a help tag referencing a
// constraint variable suppresses exactly that automatic hint. Each subtest
// covers one suppression rule with its own fixture carrying the help text the
// schema builder produces (variables already expanded) plus the help-variable
// references the renderer consults.
func TestFormat_HelpVariablesSuppressHints(t *testing.T) {
	t.Parallel()

	t.Run("range reference suppresses the range hint", func(t *testing.T) {
		t.Parallel()

		port := &spec.Flag{
			Name:       "port",
			Help:       "Port number (1..10)",
			Range:      &spec.Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"},
			TargetType: reflect.TypeFor[int](),
		}
		port.HelpVars.Range = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{port}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Port number")
		assert.Contains(t, line, "Port number (1..10)")
		assert.NotContains(t, line, "[1..10]")
	})

	t.Run("enum reference suppresses the enum hint", func(t *testing.T) {
		t.Parallel()

		env := &spec.Flag{
			Name:       "env",
			Help:       "Deployment environment (dev, prod)",
			Enum:       []string{"dev", "prod"},
			Default:    "dev",
			TargetType: reflect.TypeFor[string](),
		}
		env.HelpVars.Enum = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{env}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Deployment environment")
		assert.Contains(t, line, "Deployment environment (dev, prod) [default: dev]")
		assert.NotContains(t, line, "[enum:")
	})

	t.Run("default reference suppresses the default hint", func(t *testing.T) {
		t.Parallel()

		timeout := &spec.Flag{
			Name:       "timeout",
			Help:       "Request timeout (default 2s)",
			Default:    "2s",
			TargetType: reflect.TypeFor[string](),
		}
		timeout.HelpVars.Default = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{timeout}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Request timeout")
		assert.Contains(t, line, "Request timeout (default 2s)")
		assert.NotContains(t, line, "[default:")
	})

	t.Run("env reference suppresses the env hint", func(t *testing.T) {
		t.Parallel()

		region := &spec.Flag{
			Name:       "region",
			Help:       "Region (AWS_REGION)",
			EnvVars:    []string{"AWS_REGION"},
			TargetType: reflect.TypeFor[string](),
		}
		region.HelpVars.Env = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{region}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Region (")
		assert.Contains(t, line, "Region (AWS_REGION)")
		assert.NotContains(t, line, "[env:")
	})

	t.Run("items reference suppresses the items hint", func(t *testing.T) {
		t.Parallel()

		tags := &spec.Flag{
			Name:       "tags",
			Help:       "Tags (2..3)",
			Items:      &spec.Range{HasMin: true, HasMax: true, MinStr: "2", MaxStr: "3"},
			IsSlice:    true,
			TargetType: reflect.TypeFor[[]string](),
		}
		tags.HelpVars.Items = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{tags}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Tags (")
		assert.Contains(t, line, "Tags (2..3)")
		assert.NotContains(t, line, "[items:")
	})

	t.Run("unreferenced constraint keeps its hint", func(t *testing.T) {
		t.Parallel()

		username := &spec.Flag{
			Name:       "username",
			Help:       "Authentication username",
			Len:        &spec.Range{HasMin: true, HasMax: true, MinStr: "3", MaxStr: "20"},
			TargetType: reflect.TypeFor[string](),
		}

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{username}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Authentication username")
		assert.Contains(t, line, "Authentication username [len: 3..20]")
	})

	t.Run("suppression is per hint", func(t *testing.T) {
		t.Parallel()

		mixed := &spec.Flag{
			Name:       "mixed",
			Help:       "Mixed (1..10)",
			Range:      &spec.Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"},
			Len:        &spec.Range{HasMin: true, MinStr: "3"},
			TargetType: reflect.TypeFor[string](),
		}
		mixed.HelpVars.Range = true

		cmd := &spec.Command{Name: "testapp", Flags: []*spec.Flag{mixed}}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Mixed (")
		assert.Contains(t, line, "Mixed (1..10) [len: >= 3]")
	})
}

// TestFormat_ArgEnumHint verifies that a positional argument renders its enum
// values as the [enum: ...] hint unless the help tag references {{enum}}.
func TestFormat_ArgEnumHint(t *testing.T) {
	t.Parallel()

	t.Run("enum hint rendered when unreferenced", func(t *testing.T) {
		t.Parallel()

		channel := &spec.Arg{
			Name:       "<channel>",
			Help:       "Target channel",
			Enum:       []string{"slack", "email"},
			TargetType: reflect.TypeFor[string](),
		}

		cmd := &spec.Command{
			Name: "notify",
			Args: []*spec.Arg{channel},
		}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Target channel")
		assert.Contains(t, line, "Target channel [enum: slack, email]")
	})

	t.Run("enum hint suppressed when referenced", func(t *testing.T) {
		t.Parallel()

		topic := &spec.Arg{
			Name:       "[topic]",
			Help:       "Topic (news, alerts)",
			Enum:       []string{"news", "alerts"},
			TargetType: reflect.TypeFor[string](),
		}
		topic.HelpVars.Enum = true

		cmd := &spec.Command{
			Name: "notify",
			Args: []*spec.Arg{topic},
		}

		formatted := help.Format([]*spec.Command{cmd})

		line := lineContaining(formatted, "Topic (")
		assert.Contains(t, line, "Topic (news, alerts)")
		assert.NotContains(t, line, "[enum:")
	})
}

func TestFormat_FeaturesSection(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		FeatureFields: []*spec.FeatureField{
			{Name: "http3", Help: "Experimental HTTP/3 listener", Stage: spec.FeatureStageAlpha},
			{Name: "syntax", Help: "Syntax version", Stage: spec.FeatureStageBeta, IsInt: true, Enum: []string{"1", "2"}, Default: "2"},
			{Name: "stable", Help: "Stable core", Stage: spec.FeatureStageGA},
			{Name: "legacy", Help: "Legacy mode", Stage: spec.FeatureStageDeprecated},
		},
	}

	formatted := help.FormatWith([]*spec.Command{root}, help.Options{FeaturesSection: help.RenderFeatures})

	assert.Contains(t, formatted, "Features:")
	assert.Contains(t, formatted, "Experimental HTTP/3 listener [alpha]")
	assert.Contains(t, formatted, "Syntax version [beta] [levels: 1, 2] [default: 2]")
	assert.NotContains(t, formatted, "Stable core")
	assert.NotContains(t, formatted, "Legacy mode")
}

func TestFormat_FeaturesSection_EnvHints(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		FeatureFields: []*spec.FeatureField{
			{Name: "http3", Help: "Experimental HTTP/3 listener", EnvVars: []string{"APP_HTTP3"}},
			{Name: "syntax", Help: "Syntax version", IsInt: true, Enum: []string{"1", "2"}, Default: "2", EnvVars: []string{"APP_SYNTAX", "LEGACY_SYNTAX"}},
			{Name: "cli-only", Help: "CLI only"},
			{Name: "stable", Help: "Stable core", Stage: spec.FeatureStageGA, EnvVars: []string{"APP_STABLE"}},
		},
	}

	formatted := help.FormatWith([]*spec.Command{root}, help.Options{FeaturesSection: help.RenderFeatures})

	assert.Contains(t, formatted, "Experimental HTTP/3 listener [env: APP_HTTP3]")
	assert.Contains(t, formatted, "Syntax version [levels: 1, 2] [default: 2] [env: APP_SYNTAX, LEGACY_SYNTAX]")
	assert.Contains(t, lineContaining(formatted, "CLI only"), "CLI only")
	assert.NotContains(t, lineContaining(formatted, "CLI only"), "[env:")
	assert.NotContains(t, formatted, "APP_STABLE")
}

func TestFormat_FeaturesSection_OmittedWhenAllHidden(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		FeatureFields: []*spec.FeatureField{
			{Name: "stable", Help: "Stable core", Stage: spec.FeatureStageGA},
			{Name: "legacy", Help: "Legacy mode", Stage: spec.FeatureStageDeprecated},
		},
	}

	formatted := help.FormatWith([]*spec.Command{root}, help.Options{FeaturesSection: help.RenderFeatures})

	assert.NotContains(t, formatted, "Features:")
}

func TestFormat_FeaturesSection_RootScreenOnly(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		FeatureFields: []*spec.FeatureField{
			{Name: "http3", Help: "Experimental HTTP/3 listener", Stage: spec.FeatureStageAlpha},
		},
		Subcommands: []*spec.Command{
			{Name: "serve"},
		},
	}

	options := help.Options{FeaturesSection: help.RenderFeatures}

	subFormatted := help.FormatWith([]*spec.Command{root, root.Subcommands[0]}, options)

	assert.NotContains(t, subFormatted, "Features:")
}

func TestFormat_FeaturesSection_AbsentWithoutFeatures(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "verbose", Help: "Verbose logging", IsBool: true},
		},
	}

	formatted := help.FormatWith([]*spec.Command{root}, help.Options{FeaturesSection: help.RenderFeatures})

	assert.NotContains(t, formatted, "Features:")
}

func TestFormat_FeaturesSection_AbsentWithoutHook(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		FeatureFields: []*spec.FeatureField{
			{Name: "http3", Help: "Experimental HTTP/3 listener", Stage: spec.FeatureStageAlpha},
		},
	}

	formatted := help.Format([]*spec.Command{root})

	assert.NotContains(t, formatted, "Features:")
	assert.NotContains(t, formatted, "http3")
}

func TestFormat_EnumHintHiddenSuppressesHint(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "format", Help: "Output format", Enum: []string{"json", "yaml"}, EnumHintHidden: true, TargetType: reflect.TypeFor[string]()},
		},
		Args: []*spec.Arg{
			{Name: "<verb>", Help: "Verb", Enum: []string{"get", "post"}, EnumHintHidden: true, TargetType: reflect.TypeFor[string]()},
		},
	}

	formatted := help.Format([]*spec.Command{cmd})

	assert.NotContains(t, lineContaining(formatted, "Output format"), "[enum:")
	assert.NotContains(t, lineContaining(formatted, "Verb"), "[enum:")
}
