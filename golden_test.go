package conga_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
)

var (
	// Golden help/usage screens rendered by the canonical goldenCLI.
	//go:embed testdata/help/root.golden
	goldenHelpRoot string

	//go:embed testdata/help/session.golden
	goldenHelpSession string

	//go:embed testdata/help/subcommand.golden
	goldenHelpSubcommand string

	//go:embed testdata/help/usage.golden
	goldenHelpUsage string

	// Golden stderr screens for each usage-on-error mode and parse failure.
	//go:embed testdata/errors/invalid_value.golden
	goldenErrorsInvalidValue string

	//go:embed testdata/errors/missing_oneof_flag.golden
	goldenErrorsMissingOneOfFlag string

	//go:embed testdata/errors/missing_required_flag.golden
	goldenErrorsMissingRequiredFlag string

	//go:embed testdata/errors/subcommand_required.golden
	goldenErrorsSubcommandRequired string

	//go:embed testdata/errors/usage_default.golden
	goldenErrorsUsageDefault string

	//go:embed testdata/errors/usage_full.golden
	goldenErrorsUsageFull string

	//go:embed testdata/errors/usage_hint.golden
	goldenErrorsUsageHint string

	//go:embed testdata/errors/usage_silence.golden
	goldenErrorsUsageSilence string

	// Golden completion scripts generated through the app-driven completion command.
	//go:embed testdata/completion/command_bash.golden
	goldenCompletionCommandBash string

	//go:embed testdata/completion/command_powershell.golden
	goldenCompletionCommandPowershell string

	//go:embed testdata/completion/command_zsh.golden
	goldenCompletionCommandZsh string

	// Golden --why provenance reports covering the representative report shapes.
	//go:embed testdata/why/config_paths_loaded.golden
	goldenWhyConfigPathsLoaded string

	//go:embed testdata/why/config_paths_missing.golden
	goldenWhyConfigPathsMissing string

	//go:embed testdata/why/defaults.golden
	goldenWhyDefaults string

	//go:embed testdata/why/map_cascade_merge.golden
	goldenWhyMapCascadeMerge string

	//go:embed testdata/why/map_entry_width.golden
	goldenWhyMapEntryWidth string

	//go:embed testdata/why/map_merge.golden
	goldenWhyMapMerge string

	//go:embed testdata/why/redaction_badges.golden
	goldenWhyRedactionBadges string

	//go:embed testdata/why/scalar_cascade_merge.golden
	goldenWhyScalarCascadeMerge string

	//go:embed testdata/why/subcommand_report.golden
	goldenWhySubcommandReport string
)

// updateGolden rewrites generated golden files (completion corpus and
// completion scripts) from current output when the suite runs with -update.
// Designed goldens (help, errors, --why) have no update mode and are edited by
// hand. Review regenerated snapshots manually; the embedded copy is refreshed
// on the next compilation.
//
//nolint:gochecknoglobals // standard -update golden-refresh flag seam
var updateGolden = flag.Bool("update", false, "rewrite generated golden files from current output")

// goldenCLI is the canonical reference application: one struct exercises every
// rendering axis (description, footer, examples, sectioned and unsectioned
// flags, negatable bool, counter, map, enum, range, env, hidden, deprecated,
// and a nested subcommand) so the help, usage, and error screens share one
// fixture.
type goldenCLI struct {
	noopRunner

	Name    conga.OptionalFlag[string]            `default:"demo"                     help:"Application name"          short:"n"`
	Verbose conga.Counter                         `help:"Verbosity level"             short:"v"`
	Color   conga.OptionalFlag[bool]              `help:"Colorize output"             negatable:"true"`
	Tags    conga.OptionalFlag[map[string]string] `help:"Arbitrary tags"              short:"t"`
	Port    conga.OptionalFlag[int]               `default:"8080"                     help:"Listen port"               range:"1..65535"`
	Format  conga.OptionalFlag[string]            `enum:"json,yaml"                   help:"Output format"`
	Token   conga.OptionalFlag[string]            `env:"CONGA_TEST_GOLDEN_TOKEN"      help:"API token"`
	Output  conga.OptionalFlag[string]            `help:"Output path"                 section:"Output"`
	Legacy  conga.OptionalFlag[bool]              `deprecated:"use --name"            help:"Legacy toggle"`
	Hidden  conga.OptionalFlag[string]            `help:"Internal knob"               hidden:"true"`
	Deploy  conga.Command[goldenDeployCmd]        `section:"Lifecycle"                summary:"Deploy the application"`
	Status  conga.Command[goldenStatusCmd]        `summary:"Show status"`
	Help    conga.HelpCommand                     `summary:"Show help for a command"`
	Version conga.VersionCommand                  `summary:"Show version information"`
}

type goldenDeployCmd struct {
	Target   conga.RequiredArg[string]  `help:"Deployment target"     name:"<target>"`
	Replicas conga.OptionalFlag[int]    `default:"1"                  help:"Replica count" range:"1..100"`
	DryRun   conga.OptionalFlag[bool]   `help:"Preview changes only"`
	Reason   conga.RequiredFlag[string] `help:"Reason for deployment"`
}

func (c *goldenDeployCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

type goldenStatusCmd struct {
	Detailed conga.OptionalFlag[bool] `help:"Show detailed status"`
}

func (c *goldenStatusCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

func newGoldenApp() *conga.App[goldenCLI] {
	return configureGoldenApp(conga.New[goldenCLI]())
}

// configureGoldenApp applies the goldenCLI documentation identity to an app,
// so both the zero-value and constructor forms render identical screens.
func configureGoldenApp(app *conga.App[goldenCLI]) *conga.App[goldenCLI] {
	return app.
		Name("golden").
		Description("Golden reference application.").
		HelpFooter("Documentation: https://example.com/golden").
		Examples("# Deploy the api target\ngolden deploy api --reason rollout").
		Version("1.2.3")
}

// assertGolden compares got against the embedded generated golden expected,
// or rewrites the file at relativePath when the suite runs with -update. The
// path is resolved against this file's directory so tests that change the
// working directory still refresh the committed fixture. Designed goldens use
// assert.Equal directly.
func assertGolden(t *testing.T, relativePath, expected, got string) {
	t.Helper()

	if *updateGolden {
		_, file, _, _ := runtime.Caller(0)
		assert.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(file), relativePath), []byte(got), 0o644))

		return
	}

	assert.Equal(t, expected, got)
}

// normalizeWhyGolden substitutes environment-specific fragments (temp paths,
// env-derived paths) with stable placeholders before golden comparison. The
// replacements are alternating from/to pairs.
func normalizeWhyGolden(output string, replacements ...string) string {
	for i := 0; i+1 < len(replacements); i += 2 {
		output = strings.ReplaceAll(output, replacements[i], replacements[i+1])
	}

	return output
}
