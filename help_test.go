package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/yaml"
	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

type TestHelpRootCLI struct {
	Verbose conga.OptionalFlag[bool]          `help:"Verbose logging"     short:"v"`
	Cluster conga.Command[TestHelpClusterCmd] `summary:"Cluster commands"`
	Help    conga.HelpCommand
}

type TestHelpClusterCmd struct {
	Node conga.Command[TestHelpNodeCmd] `summary:"Node commands"`
}

type TestHelpNodeCmd struct {
	Drain conga.Command[TestHelpDrainCmd] `summary:"Drain node"`
}

type TestHelpDrainCmd struct {
	NodeID conga.RequiredArg[string]  `help:"Node ID"`
	Reason conga.RequiredFlag[string] `help:"Reason for drain"`
}

func (d *TestHelpDrainCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type TestCustomHCLI struct {
	Host conga.OptionalFlag[string] `help:"Server host" short:"h"`
}

func (c *TestCustomHCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type SpecialFlagsHelpCLI struct {
	Verbose  conga.Counter                         `help:"Verbosity level"  short:"v"`
	Location conga.OptionalFlag[bool]              `help:"Follow redirects" negatable:"true" short:"L"`
	Tags     conga.OptionalFlag[map[string]string] `help:"Tags"             short:"t"`
}

type ConfigFileHelpCLI struct {
	ConfigFile conga.ConfigFile `short:"c"`
}

type ConfigFileDefaultHelpCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile
}

type MultiConfigFileDefaultHelpCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile `multi:"true"`
}

type UserHelpFlagCLI struct {
	Help conga.OptionalFlag[string] `help:"Custom help argument"`
}

func (u *UserHelpFlagCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type TestGroupTagsFlagsCLI struct {
	Verbose conga.OptionalFlag[bool]   `help:"Verbose logging" short:"v"`
	Repo    conga.OptionalFlag[string] `group:"Source control" help:"Repository path"`
	Quiet   conga.OptionalFlag[bool]   `group:"Output"         help:"Suppress output" short:"q"`
	Color   conga.OptionalFlag[bool]   `group:"Output"         help:"Colorize output"`
	Config  conga.ConfigFile           `group:"Configuration"`
}

type TestGroupTagsServerCmd struct{}

func (c *TestGroupTagsServerCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

type TestGroupTagsSubcommandsCLI struct {
	Deploy conga.Command[TestGroupTagsServerCmd] `group:"Lifecycle"     summary:"Deploy the application"`
	Status conga.Command[plainSubcommand]        `summary:"Show status"`
}

func TestHelpFlag_Root(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := newGoldenApp().
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/help/root.golden", goldenHelpRoot, buffer.String())
}

func TestHelpFlag_Subcommand(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := newGoldenApp().
		Args([]string{"deploy", "--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/help/subcommand.golden", goldenHelpSubcommand, buffer.String())
}

func TestHelpFlag_PrecedenceOverRequired(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	// Calling drain without <node-id> and without --reason, but with --help
	app := conga.New[TestHelpRootCLI]().
		Name("testapp").
		Args([]string{"cluster", "node", "drain", "--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  testapp cluster node drain")
}

func TestHelpCommand_Root(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := newGoldenApp().
		Args([]string{"help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/help/root.golden", goldenHelpRoot, buffer.String())
}

type TestHelpAliasCLI struct {
	Help conga.HelpCommand `aliases:"h"`
}

func TestHelpCommand_AliasRouting(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestHelpAliasCLI]().
		Name("testapp").
		Args([]string{"h"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  testapp")
}

func TestHelpCommand_SubcommandRouting(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestHelpRootCLI]().
		Name("testapp").
		Args([]string{"help", "cluster", "node", "drain"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  testapp cluster node drain")
}

func TestHelpCommand_Path(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(TestHelpRootCLI)
	app := conga.New(cli).Stdout(&buffer).Args([]string{"help", "cluster", "node", "drain"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"cluster", "node", "drain"}, cli.Help.Path())
}

func TestHelpCommand_Path_DefensiveCopy(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(TestHelpRootCLI)
	app := conga.New(cli).Stdout(&buffer).Args([]string{"help", "cluster", "node", "drain"})

	err := app.Run()

	assert.NoError(t, err)

	returned := cli.Help.Path()
	returned[0] = "mutated"

	assert.Equal(t, []string{"cluster", "node", "drain"}, cli.Help.Path())
}

func TestHelpCommand_PathNilReceiver(t *testing.T) {
	t.Parallel()

	var command *conga.HelpCommand

	assert.Zero(t, command.Path())
}

func TestHelpCommand_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	app := conga.New[TestHelpRootCLI]().
		Args([]string{"help", "cluster", "bogus"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
}

func TestShortH_Unreserved(t *testing.T) {
	t.Parallel()

	app := conga.New[TestHelpRootCLI]().
		Args([]string{"-h"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
}

func TestShortH_CustomFieldBinding(t *testing.T) {
	t.Parallel()

	cli := new(TestCustomHCLI)
	app := conga.New(cli).
		Args([]string{"-h", "example.com"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "example.com", cli.Host.Get())
}

func TestHelpFlag_RendersToConfiguredStdout(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[TestHelpRootCLI]().
		Description("A test app").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "A test app")
	assert.Contains(t, output, "--help")
}

func TestHelpFormatting_SpecialFlags(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[SpecialFlagsHelpCLI]().
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	helpText := buffer.String()
	assert.Contains(t, helpText, "-v, --verbose")
	assert.Contains(t, helpText, "-L, --[no-]location")
	assert.Contains(t, helpText, "-t, --tags=KEY=VALUE,...")
}

func TestHelpFormatting_ConfigFileFlag(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[ConfigFileHelpCLI]().
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	helpText := buffer.String()
	assert.Contains(t, helpText, "-c, --config-file=PATH")
	assert.Contains(t, helpText, "Path to config file")
	assert.NotContains(t, helpText, "[default:")
}

func TestHelpFormatting_ConfigFileDefaultHelp(t *testing.T) {
	t.Parallel()

	t.Run("single flag with decoder", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[ConfigFileDefaultHelpCLI]().
			RegisterDecoder(".json", json.Decode).
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--config-file=PATH")
		assert.Contains(t, buffer.String(), "Path to config file [decoders: .json]")
		assert.NotContains(t, buffer.String(), "config file(s)")
		assert.NotContains(t, buffer.String(), "no configuration decoder registered")
	})

	t.Run("multi flag with decoder", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[MultiConfigFileDefaultHelpCLI]().
			RegisterDecoder(".json", json.Decode).
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--config-file=PATH,...")
		assert.Contains(t, buffer.String(), "Path to config file(s) [decoders: .json]")
	})

	t.Run("multiple decoders sorted", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[ConfigFileDefaultHelpCLI]().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".json", json.Decode).
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Path to config file [decoders: .json, .yaml]")
	})

	t.Run("single flag without decoder appends warning", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[ConfigFileDefaultHelpCLI]().
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Path to config file (no configuration decoder registered)")
	})
}

func TestDescriptionProvider_Fallback(t *testing.T) {
	t.Parallel()

	t.Run("subcommand omitting Description falls back to summary", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Deploy conga.Command[plainSubcommand] `summary:"One-line synopsis"`
		}

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Args([]string{"deploy", "--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "One-line synopsis")
	})

	t.Run("subcommand returning empty Description falls back to summary", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Deploy conga.Command[emptyDescSubcommand] `summary:"Fallback synopsis"`
		}

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Args([]string{"deploy", "--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Fallback synopsis")
	})
}

func TestDescriptionProvider_Rich(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Deploy conga.Command[richSubcommand] `summary:"Concise deploy synopsis"`
	}

	t.Run("subcommand screen renders full description", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Args([]string{"deploy", "--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Deploy a container image.\n\nSupports rolling updates and rollbacks.")
	})

	t.Run("parent screen renders summary not description", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()
		assert.Contains(t, output, "Concise deploy synopsis")
		assert.NotContains(t, output, "Supports rolling updates and rollbacks.")
	})
}

func TestExampleProvider_Formatting(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Deploy conga.Command[exampleSubcommand] `summary:"Deploy a container"`
	}

	var buffer bytes.Buffer

	app := conga.New[RootCLI]().
		Args([]string{"deploy", "--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Examples:\n  # Deploy a single container\n  app deploy --image nginx:alpine")
}

func TestExampleProvider_OmittedOnError(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Deploy conga.Command[exampleSubcommand] `summary:"Deploy a container"`
	}

	var (
		buffer   bytes.Buffer
		exitCode = -1
	)

	app := conga.New[RootCLI]().
		Args([]string{"deploy", "--unknown"}).
		Stderr(&buffer).
		ExitFunc(func(code int) {
			exitCode = code
		})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)

	// Exit handles usage errors by rendering error and synopsis to stderr and calling exit(2).
	app.Exit(err)

	assert.Equal(t, 2, exitCode)
	assert.NotContains(t, buffer.String(), "Examples:")
}

func TestApp_Examples_HelpFormatting(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		noopRunner
	}

	var buffer bytes.Buffer

	app := conga.New[RootCLI]().
		Examples("\n\n# Deploy application\napp deploy --image nginx\n\n").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Examples:\n  # Deploy application\n  app deploy --image nginx")
}

func TestApp_Deprecated_HelpFormatting(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Deploy conga.Command[plainSubcommand] `summary:"Deploy command"`
	}

	t.Run("root screen renders deprecation banner at top", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Deprecated("use new-app instead").
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "WARNING: app is deprecated: use new-app instead\n\nUsage:")
	})

	t.Run("subcommand screen renders root deprecation banner", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			Deprecated("use new-app instead").
			Args([]string{"deploy", "--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "WARNING: app is deprecated: use new-app instead\n\nDeploy command\n\nUsage:")
	})
}

func TestHelpFooter_GlobalInheritance(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Deploy conga.Command[plainSubcommand] `summary:"Deploy a container"`
	}

	const footer = "Documentation: https://example.com/docs"

	t.Run("root screen renders footer", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			HelpFooter(footer).
			Args([]string{"--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), footer)
	})

	t.Run("subcommand screen inherits footer", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[RootCLI]().
			HelpFooter(footer).
			Args([]string{"deploy", "--help"}).
			Stdout(&buffer)

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), footer)
	})
}

func TestHelp_CollisionWithoutDisableHelpFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[UserHelpFlagCLI]().
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
}

func TestHelp_CustomFlagWithDisableHelpFlag(t *testing.T) {
	t.Parallel()

	cli := new(UserHelpFlagCLI)
	app := conga.New(cli).
		DisableHelpFlag().
		Args([]string{"--help", "custom-topic"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "custom-topic", cli.Help.Get())
}

// requiredHelpShadowCLI reuses the freed help name on a required flag; its
// runner renders the active screen so tests can pin the synopsis and row.
type requiredHelpShadowCLI struct {
	Help conga.RequiredFlag[string] `help:"Custom help topic" placeholder:"TOPIC"`
}

func (c *requiredHelpShadowCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

// aliasHelpShadowCLI reaches the built-in help classification through an alias
// on an ancestor flag, which must stay a regular flag on subcommand screens.
type aliasHelpShadowCLI struct {
	Assist conga.RequiredFlag[bool]               `aliases:"help"              help:"Show assistance"`
	Build  conga.Command[aliasHelpShadowBuildCmd] `summary:"Build the project"`
}

// aliasHelpShadowBuildCmd renders its own screen so the test can pin what a
// subcommand help screen shows for an ancestor flag carrying the help alias.
type aliasHelpShadowBuildCmd struct{}

func (c *aliasHelpShadowBuildCmd) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

// TestHelp_RequiredHelpShadowStaysFirstClass pins that a required user flag
// reusing the freed help name stays a regular flag: it renders in both the
// usage synopsis and the Flags table.
func TestHelp_RequiredHelpShadowStaysFirstClass(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[requiredHelpShadowCLI]().
		Name("app").
		DisableHelpFlag().
		Args([]string{"--help", "topic"}).
		Stdout(&buffer)

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  app --help=TOPIC")
	assert.Contains(t, buffer.String(), "--help=TOPIC  Custom help topic")
}

// TestHelp_AliasShadowStaysVisibleOnSubcommandScreen pins that an ancestor
// flag carrying the help alias is not treated as the built-in: it stays in the
// leaf synopsis and the Global Flags section.
func TestHelp_AliasShadowStaysVisibleOnSubcommandScreen(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[aliasHelpShadowCLI]().
		Name("app").
		DisableHelpFlag().
		Args([]string{"--assist", "build"}).
		Stdout(&buffer)

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  app build --assist")
	assert.Contains(t, buffer.String(), "Global Flags:\n      --assist  Show assistance")
}

// TestHelpScreen_GroupTags_Flags verifies that the group struct tag drives
// titled sections on the rendered help screen: ungrouped flags first, then
// group sections in first-appearance order, with the built-in --help flag
// closing the ungrouped Flags section.
func TestHelpScreen_GroupTags_Flags(t *testing.T) {
	t.Parallel()

	cmd, err := spec.Build(&TestGroupTagsFlagsCLI{}, "testapp", conga.SpecOptionsWithContainers(spec.Options{}))
	assert.NoError(t, err)

	formatted := help.Format([]*spec.Command{cmd})

	flagsIndex := strings.Index(formatted, "\nFlags:")
	sourceIndex := strings.Index(formatted, "Source control:")
	outputIndex := strings.Index(formatted, "Output:")
	configIndex := strings.Index(formatted, "Configuration:")
	helpIndex := strings.LastIndex(formatted, "--help")

	assert.True(t, flagsIndex >= 0 && flagsIndex < sourceIndex, "ungrouped flags must render before groups")
	assert.True(t, sourceIndex < outputIndex, "groups must render in first-appearance order")
	assert.True(t, outputIndex < configIndex, "groups must render in first-appearance order")
	assert.True(t, helpIndex > flagsIndex && helpIndex < sourceIndex, "built-in --help must close the ungrouped Flags section")
}

// TestHelpScreen_GroupTags_Subcommands verifies that a grouped subcommand
// renders under its own titled Commands section, after ungrouped commands.
func TestHelpScreen_GroupTags_Subcommands(t *testing.T) {
	t.Parallel()

	cmd, err := spec.Build(&TestGroupTagsSubcommandsCLI{}, "testapp", conga.SpecOptionsWithContainers(spec.Options{}))
	assert.NoError(t, err)

	formatted := help.Format([]*spec.Command{cmd})

	commandsIndex := strings.Index(formatted, "\nCommands:")
	lifecycleIndex := strings.Index(formatted, "Lifecycle:")

	assert.True(t, commandsIndex >= 0 && commandsIndex < lifecycleIndex, "ungrouped commands must render before groups")
	assert.Contains(t, formatted, "Lifecycle:\n  deploy  Deploy the application")
}

func TestHelpScreen_EnvPrefixDerived(t *testing.T) {
	t.Parallel()

	type SubCmd struct {
		Target conga.RequiredFlag[string] `help:"Deployment target"`
	}

	type AppCLI struct {
		ConfigFile conga.ConfigFile           `help:"Config file"`
		Port       conga.OptionalFlag[int]    `default:"8080"              env:"CONGA_TEST_CUSTOM_PORT" help:"Port"`
		Worker     conga.OptionalFlag[int]    `default:"4"                 help:"Workers"`
		Secret     conga.OptionalFlag[string] `env:"-"                     help:"Internal secret"`
		Deploy     conga.Command[SubCmd]      `summary:"Deploy subcommand"`
	}

	var rootBuffer bytes.Buffer

	app := conga.New[AppCLI]().
		EnvPrefix("CONGA_TEST").
		Args([]string{"--help"}).
		Stdout(&rootBuffer)

	err := app.Run()
	assert.NoError(t, err)

	rootHelp := rootBuffer.String()
	assert.Contains(t, rootHelp, "[env: CONGA_TEST_CONFIG]")
	assert.Contains(t, rootHelp, "[env: CONGA_TEST_CUSTOM_PORT]")
	assert.Contains(t, rootHelp, "[env: CONGA_TEST_WORKER]")
	assert.NotContains(t, rootHelp, "CONGA_TEST_SECRET")
	assert.NotContains(t, rootHelp, "[env: -]")

	var subBuffer bytes.Buffer

	appSub := conga.New[AppCLI]().
		EnvPrefix("CONGA_TEST").
		Args([]string{"deploy", "--help"}).
		Stdout(&subBuffer)

	err = appSub.Run()
	assert.NoError(t, err)

	subHelp := subBuffer.String()
	assert.Contains(t, subHelp, "[env: CONGA_TEST_TARGET]")
	assert.Contains(t, subHelp, "[env: CONGA_TEST_CUSTOM_PORT]")
	assert.Contains(t, subHelp, "[env: CONGA_TEST_WORKER]")
}

type plainSubcommand struct{}

func (c *plainSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type emptyDescSubcommand struct{}

func (c *emptyDescSubcommand) Description() string { return "" }

func (c *emptyDescSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type richSubcommand struct{}

func (c *richSubcommand) Description() string {
	return "Deploy a container image.\n\nSupports rolling updates and rollbacks."
}

func (c *richSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type exampleSubcommand struct{}

func (c *exampleSubcommand) Examples() string {
	return "# Deploy a single container\napp deploy --image nginx:alpine"
}

func (c *exampleSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

// TestHelp_WidthHonorsColumns pins the width plumbing: COLUMNS reaches the
// help renderer through RunContext and folds an overlong help cell under a
// continuation line aligned to the help column. It cannot run in parallel
// because t.Setenv mutates the process environment.
func TestHelp_WidthHonorsColumns(t *testing.T) {
	t.Setenv("COLUMNS", "48")

	type columnsCLI struct {
		noopRunner

		Config conga.OptionalFlag[string] `help:"Path to the configuration file used by every command" placeholder:"FILE"`
	}

	var buffer bytes.Buffer

	app := conga.New[columnsCLI]().
		Name("app").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "      --config=FILE  Path to the configuration\n")
	assert.Contains(t, buffer.String(), "\n                     file used by every command\n")
	assert.NotContains(t, buffer.String(), "Path to the configuration file used by every command")
}
