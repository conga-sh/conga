package conga_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/yaml"
)

var (
	//go:embed testdata/fixtures/feature_gentle_force.yml
	fixtureFeatureGentleForce []byte

	//go:embed testdata/fixtures/feature_inactive_flag.yml
	fixtureFeatureInactiveFlag []byte

	//go:embed testdata/fixtures/feature_invalid_value.yml
	fixtureFeatureInvalidValue []byte

	//go:embed testdata/fixtures/feature_multi_active.yml
	fixtureFeatureMultiActive []byte

	//go:embed testdata/fixtures/feature_pruned_subtree.yml
	fixtureFeaturePrunedSubtree []byte

	//go:embed testdata/fixtures/feature_scoped_config_key.yml
	fixtureFeatureScopedConfigKey []byte

	//go:embed testdata/fixtures/feature_scoped_flag.yml
	fixtureFeatureScopedFlag []byte

	//go:embed testdata/fixtures/feature_scoped_key.yml
	fixtureFeatureScopedKey []byte

	//go:embed testdata/fixtures/feature_sibling_config_key.yml
	fixtureFeatureSiblingConfigKey []byte

	//go:embed testdata/fixtures/feature_typo.yml
	fixtureFeatureTypo []byte
)

type featureExperiments struct {
	GentleForce bool `default:"false"                 help:"Forces all dependent tasks"`
	Packages    bool `help:"Manage Taskfile packages"`
	Syntax      int  `enum:"1,2"                      help:"Syntax version (0 = off)"`
}

type featureTestCLI struct {
	Features conga.Features[featureExperiments] `config:"experiments" env:"TASK_X_FEATURES" flag:"feature-gates"`

	Force    conga.OptionalFlag[bool]   `help:"Forces task execution" short:"f"`
	ForceAll conga.OptionalFlag[bool]   `feature:"gentle-force"       help:"Forces all dependent tasks"`
	V2Only   conga.OptionalFlag[string] `feature:"syntax"             help:"Syntax v2 configuration"    name:"syntax"`
	V1Only   conga.OptionalFlag[string] `feature:"syntax"             help:"Syntax v1 configuration"    name:"v1"`
	Legacy   conga.OptionalFlag[bool]   `feature:"!gentle-force"      help:"Legacy force behavior"`

	Package conga.Command[featurePackageSubcommand] `feature:"packages" summary:"Manage Taskfile packages"`

	Ran            bool
	ActiveGentle   bool
	ResolvedSyntax int
	ActivePackages bool
}

type featurePackageSubcommand struct {
	Name conga.OptionalFlag[string] `help:"Package name"`

	Ran bool
}

func (c *featureTestCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true
	c.ActiveGentle = c.Features.Get().GentleForce
	c.ResolvedSyntax = c.Features.Get().Syntax
	c.ActivePackages = c.Features.Get().Packages

	return nil
}

func (s *featurePackageSubcommand) Run(ctx context.Context, session *conga.Session) error {
	s.Ran = true

	return nil
}

type featureConfigCLI struct {
	ConfigFile conga.ConfigFile `help:"Config file"`

	Features conga.Features[featureExperiments] `config:"experiments" env:"TASK_X_FEATURES" flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool]   `feature:"gentle-force" help:"Forces all dependent tasks"`
	V2Only   conga.OptionalFlag[string] `feature:"syntax"       help:"Syntax v2 configuration"    name:"syntax"`
}

func (c *featureConfigCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_CLIOverridesEnv(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--feature-gates", "gentle-force=true", "--force-all"})

	t.Setenv("TASK_X_FEATURES", "gentle-force=false")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.ActiveGentle)
}

type featureRootOnlyCLI struct {
	Sub conga.Command[featureRootOnlySub] `summary:"Sub"`
}

type featureRootOnlySub struct {
	Features conga.Features[featureExperiments]
}

func (s *featureRootOnlySub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_RootOnly(t *testing.T) {
	t.Parallel()

	var cli featureRootOnlyCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"sub"})

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conga.Features must be declared on the root command struct")
}

type featureDuplicateCLI struct {
	Features conga.Features[featureDuplicateSet]
}

type featureDuplicateSet struct {
	First  bool `name:"dup"`
	Second bool `name:"dup"`
}

func TestFeature_DuplicateName(t *testing.T) {
	t.Parallel()

	app := conga.New[featureDuplicateCLI]().Name("task")

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `feature "dup" (already declared`)
}

func TestFeature_FlagGating_Disabled(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--force-all"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `flag '--force-all' requires feature "gentle-force"`)
}

func TestFeature_FlagGating_EnabledViaCLI(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--feature-gates", "gentle-force=true", "--force-all"})

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ForceAll.Get())
	assert.True(t, cli.ActiveGentle)
}

func TestFeature_FlagGating_EnabledViaEnv(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--force-all"})

	t.Setenv("TASK_X_FEATURES", "gentle-force=true")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ForceAll.Get())
	assert.True(t, cli.ActiveGentle)
}

func TestFeature_SubcommandGating_EnabledViaEnv(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"package", "--name", "lib"})

	t.Setenv("TASK_X_FEATURES", "packages=true")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Package.Get().Ran)
}

type featureDerivedEnvCLI struct {
	Features conga.Features[featureExperiments] `flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran bool
}

func (c *featureDerivedEnvCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true

	return nil
}

func TestFeature_FlagGating_EnabledViaDerivedEnvPrefix(t *testing.T) {
	var cli featureDerivedEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnvPrefix("TASK_X").
		Args([]string{"--force-all"})

	t.Setenv("TASK_X_FEATURE_GATES", "gentle-force=true")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ForceAll.Get())
}

// TestFeature_EnvForInactiveGatedFlag_Ignored pins the environment-tier
// asymmetry: pruning removes a gated flag from the schema before environment
// binding, so a variable targeting it is silently ignored while a config key
// naming it is rejected.
func TestFeature_EnvForInactiveGatedFlag_Ignored(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnvPrefix("TASK_X").
		Args([]string{})

	t.Setenv("TASK_X_FORCE_ALL", "true")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.ForceAll.IsSet())
	assert.False(t, cli.ForceAll.Get())
}

func TestFeature_FlagGating_EnabledViaConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600)
	assert.NoError(t, err)

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--force-all"})

	err = app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ForceAll.Get())
	assert.True(t, cli.ActiveGentle)
}

func TestFeature_Config_TypoRejection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureTypo, 0o600)
	assert.NoError(t, err)

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--force"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `unknown feature "GNTLE_FORCCE" (did you mean "gentle-force"?)`)
}

func TestFeature_ConfigTypo_HelpStillRenders(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureTypo, 0o600)
	assert.NoError(t, err)

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		Stdout(&buffer).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--help"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--force")
}

func TestFeature_ConfigTypo_VersionStillRenders(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureTypo, 0o600)
	assert.NoError(t, err)

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		Stdout(&buffer).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--version"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "has version")
}

func TestFeature_ConfigTypo_WhyStillRenders(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureTypo, 0o600)
	assert.NoError(t, err)

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		Stdout(&buffer).
		EnableWhy().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--why"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "gentle-force")
}

// TestFeature_Why_RejectedValueProvenance pins honest --why provenance: a
// CLI feature value the tolerant overlay drops must render its defaulted value
// under the "unset"/"default" source, never the CLI tier it was rejected from.
func TestFeature_Why_RejectedValueProvenance(t *testing.T) {
	t.Parallel()

	t.Run("invalid value falls back to unset", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--feature-gates", "syntax=9", "--why"})

		err := app.Run()
		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "  FEATURE        VALUE   SOURCE\n"+
			"  gentle-force   false   default\n"+
			"  packages       false   unset\n"+
			"  syntax         0       unset\n")
	})

	t.Run("unknown key leaves declared default", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--feature-gates", "gentle-forse=true", "--why"})

		err := app.Run()
		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "  gentle-force   false   default\n")
	})
}

func TestFeature_Config_InvalidValue(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureInvalidValue, 0o600)
	assert.NoError(t, err)

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--force"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax": must be one of: 0, 1, 2`)
}

func TestFeature_CLI_UnknownFeature(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--feature-gates", "gentle-forse=true"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `unknown feature "gentle-forse" (did you mean "gentle-force"?)`)
}

func TestFeature_CLI_InvalidValue(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"--feature-gates", "syntax=9"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Contains(t, err.Error(), `invalid value "9" for feature "syntax"`)
}

func TestFeature_SubcommandGating_Disabled(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"package"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `command "package" requires feature "packages"`)
}

func TestFeature_SubcommandGating_Enabled(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		Args([]string{"--feature-gates=packages=true", "package", "--name", "lib"})

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Package.IsSelected())
	assert.True(t, cli.Package.Get().Ran)
	assert.Equal(t, "lib", cli.Package.Get().Name.Get())
}

func TestFeature_HelpExclusion(t *testing.T) {
	t.Parallel()

	t.Run("omits inactive flags and subcommands", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()
		assert.NoError(t, err)

		output := buffer.String()

		assert.Contains(t, output, "--force")
		assert.NotContains(t, output, "--force-all")
		assert.NotContains(t, output, "--syntax")
		assert.NotContains(t, output, "Commands:")
		assert.Contains(t, output, "--legacy") // !gentle-force is active when gentle-force is off
	})

	t.Run("includes active flags and subcommands", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			Args([]string{"--feature-gates", "gentle-force=true,packages=true,syntax=2", "--help"})

		err := app.Run()
		assert.NoError(t, err)

		output := buffer.String()

		assert.Contains(t, output, "--force-all")
		assert.Contains(t, output, "--syntax")
		assert.Contains(t, output, "package")
		assert.NotContains(t, output, "--legacy") // negated flag excluded when feature is active
	})
}

func TestFeature_MultiLevel(t *testing.T) {
	t.Parallel()

	t.Run("runs at level two and materializes the value", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			Args([]string{"--feature-gates", "syntax=2", "--syntax", "experimental"})

		err := app.Run()
		assert.NoError(t, err)
		assert.True(t, cli.Ran)
		assert.Equal(t, 2, cli.ResolvedSyntax)
		assert.Equal(t, "experimental", cli.V2Only.Get())
	})

	t.Run("runs at level one and materializes the value", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			Args([]string{"--feature-gates", "syntax=1", "--v1", "legacy"})

		err := app.Run()
		assert.NoError(t, err)
		assert.True(t, cli.Ran)
		assert.Equal(t, 1, cli.ResolvedSyntax)
		assert.Equal(t, "legacy", cli.V1Only.Get())
	})

	t.Run("rejects value outside declared enum", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			Args([]string{"--feature-gates", "syntax=3"})

		err := app.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax": must be one of: 0, 1, 2`)
	})
}

func TestFeature_Completion(t *testing.T) {
	t.Parallel()

	t.Run("omits inactive flags and commands from completion", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion()

		_, formatted := app.CompleteForShell("bash", []string{"--"})
		assert.Contains(t, formatted, "--force")
		assert.NotContains(t, formatted, "--force-all")
		assert.NotContains(t, formatted, "--syntax")

		_, cmdFormatted := app.CompleteForShell("bash", []string{""})
		assert.NotContains(t, cmdFormatted, "package")
	})

	t.Run("includes active flags and commands in completion", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, ".taskrc.yml")
		err := os.WriteFile(configPath, fixtureFeatureMultiActive, 0o600)
		assert.NoError(t, err)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath))

		_, formatted := app.CompleteForShell("bash", []string{"--"})
		assert.Contains(t, formatted, "--force-all")
		assert.Contains(t, formatted, "--syntax")

		_, cmdFormatted := app.CompleteForShell("bash", []string{""})
		assert.Contains(t, cmdFormatted, "package")
	})

	t.Run("CLI feature gates drive flag completion", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion()

		for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
			_, formatted := app.CompleteForShell(shell, []string{"--feature-gates=gentle-force=true", "--"})
			assert.Contains(t, formatted, "--force-all")
			assert.NotContains(t, formatted, "--syntax")
		}
	})

	t.Run("CLI feature gates drive subcommand completion", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion()

		_, formatted := app.CompleteForShell("bash", []string{"--feature-gates", "packages=true", "package", "--"})
		assert.Contains(t, formatted, "--name")
	})
}

func TestFeature_Completion_ExplicitConfigFile(t *testing.T) {
	t.Parallel()

	newApp := func() *conga.App[featureConfigCLI] {
		return conga.New[featureConfigCLI]().
			Name("task").
			EnableCompletion().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode)
	}

	configPath := filepath.Join(t.TempDir(), ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureMultiActive, 0o600)
	assert.NoError(t, err)

	t.Run("detached --config-file drives flag completion", func(t *testing.T) {
		t.Parallel()

		_, formatted := newApp().CompleteForShell("bash", []string{"--config-file", configPath, "--"})

		assert.Contains(t, formatted, "--force-all")
		assert.Contains(t, formatted, "--syntax")
	})

	t.Run("attached --config-file drives flag completion", func(t *testing.T) {
		t.Parallel()

		_, formatted := newApp().CompleteForShell("bash", []string{"--config-file=" + configPath, "--"})

		assert.Contains(t, formatted, "--force-all")
		assert.Contains(t, formatted, "--syntax")
	})

	t.Run("omitted --config-file leaves gated flags pruned", func(t *testing.T) {
		t.Parallel()

		_, formatted := newApp().CompleteForShell("bash", []string{"--"})

		assert.NotContains(t, formatted, "--force-all")
		assert.NotContains(t, formatted, "--syntax")
	})
}

func TestFeature_WhyResolvesConfigOnce(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureMultiActive, 0o600)
	assert.NoError(t, err)

	var loads atomic.Int64

	decode := func(data []byte) (map[string]any, error) {
		loads.Add(1)

		return yaml.Decode(data)
	}

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableWhy().
		Stdout(&bytes.Buffer{}).
		RegisterDecoder(".yaml", decode).
		RegisterDecoder(".yml", decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--why"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), loads.Load())
	assert.True(t, cli.Features.Get().GentleForce)
}

func TestFeature_SubcommandHelp_Disabled(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		Args([]string{"package", "--help"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `command "package" requires feature "packages"`)
}

// TestFeature_FlagGating_HelpRejected pins the symmetry between a gated
// subcommand and a gated flag under --help: an inactive flag on the command
// line is a usage error even when help was requested.
func TestFeature_FlagGating_HelpRejected(t *testing.T) {
	t.Parallel()

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		Stdout(&buffer).
		Args([]string{"--force-all", "--help"})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `flag '--force-all' requires feature "gentle-force"`)
	assert.NotContains(t, buffer.String(), "Usage:")
}

// TestFeature_Completion_PartialGateTokenStillPrunes pins completion pruning
// under a malformed --feature-gates token: the tolerant fallback must still
// prune inactive features rather than abort and leak gated suggestions.
func TestFeature_Completion_PartialGateTokenStillPrunes(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion()

	_, flagFormatted := app.CompleteForShell("bash", []string{"--feature-gates", "gentle-force=true,ty", "--"})
	assert.NotContains(t, flagFormatted, "--force-all")

	_, commandFormatted := app.CompleteForShell("bash", []string{"--feature-gates", "gentle-force=true,ty", ""})
	assert.NotContains(t, commandFormatted, "package")
}

// TestFeature_Completion_NoStateLeak pins the completion observation contract:
// pruning a completion invocation must not bind the app's feature container,
// so a real execution that follows resolves features from its own tiers.
func TestFeature_Completion_NoStateLeak(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		Args([]string{})

	_, formatted := app.CompleteForShell("bash", []string{"--feature-gates", "gentle-force=true", "--"})
	assert.Contains(t, formatted, "--force-all")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.Features.Get().GentleForce)
	assert.False(t, cli.Features.IsSet())
}

// TestFeature_VersionSkipsConfigResolution pins the version-only hoist: a lone
// --version renders from app metadata without decoding any configuration file,
// even when the app declares feature toggles.
func TestFeature_VersionSkipsConfigResolution(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureMultiActive, 0o600)
	assert.NoError(t, err)

	var loads atomic.Int64

	decode := func(data []byte) (map[string]any, error) {
		loads.Add(1)

		return yaml.Decode(data)
	}

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		Version("1.2.3").
		Stdout(&buffer).
		RegisterDecoder(".yaml", decode).
		RegisterDecoder(".yml", decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--version"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), loads.Load())
	assert.Contains(t, buffer.String(), "has version")
}

func TestFeature_ConfigFile_InactiveFlagRejection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureInactiveFlag, 0o600)
	assert.NoError(t, err)

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--force"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--force-all' which requires feature "gentle-force"`)
}

type featureScopedConfigCLI struct {
	Features conga.Features[featureExperiments] `flag:"feature-gates"`

	Sub conga.Command[featureScopedConfigSubcommand] `feature:"packages" summary:"Package operations"`

	Ran bool
}

type featureScopedConfigSubcommand struct {
	Name conga.OptionalFlag[string] `help:"Package name"`
	Beta conga.OptionalFlag[string] `config:"beta-key"   feature:"syntax" help:"Beta toggle"`

	Ran bool
}

func (c *featureScopedConfigCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true

	return nil
}

func (s *featureScopedConfigSubcommand) Run(ctx context.Context, session *conga.Session) error {
	s.Ran = true

	return nil
}

func TestFeature_ConfigFile_InactiveScopedFlagRejection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureScopedFlag, 0o600)
	assert.NoError(t, err)

	var cli featureScopedConfigCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--feature-gates", "packages=true", "sub"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--beta' which requires feature "syntax"`)
}

func TestFeature_ConfigFile_InactiveSubcommandSubtreeRejection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeaturePrunedSubtree, 0o600)
	assert.NoError(t, err)

	var cli featureScopedConfigCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--name' which requires feature "packages"`)
}

func TestFeature_ConfigFile_ScopedFlagBindsWhenActive(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureScopedFlag, 0o600)
	assert.NoError(t, err)

	var cli featureScopedConfigCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--feature-gates", "packages=true,syntax=2", "sub"})

	err = app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Sub.Get().Ran)
	assert.Equal(t, "hello", cli.Sub.Get().Beta.Get())
}

type featureScopedKeyConfigCLI struct {
	Features conga.Features[featureExperiments] `flag:"feature-gates"`

	Wait conga.OptionalFlag[string] `config:"timeout" help:"Root wait timeout"`

	Admin conga.Command[featureScopedKeyAdminSubcommand] `feature:"packages" summary:"Admin operations"`

	Ran bool
}

type featureScopedKeyAdminSubcommand struct {
	Timeout conga.OptionalFlag[string] `help:"Admin timeout"`

	Ran bool
}

func (c *featureScopedKeyConfigCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true

	return nil
}

func (s *featureScopedKeyAdminSubcommand) Run(ctx context.Context, session *conga.Session) error {
	s.Ran = true

	return nil
}

// TestFeature_ConfigFile_PrunedSubcommandBareKey guards the bare-name config
// tier: a pruned subcommand flag must not claim the shared top-level key that a
// root flag owns, otherwise its own gating spuriously rejects the root's value.
func TestFeature_ConfigFile_PrunedSubcommandBareKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureScopedKey, 0o600)
	assert.NoError(t, err)

	var cli featureScopedKeyConfigCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	err = app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.Equal(t, "30s", cli.Wait.Get())
}

type featureSharedConfigKeyCLI struct {
	Features conga.Features[featureExperiments] `flag:"feature-gates"`

	Wait conga.OptionalFlag[string] `config:"custom-key" help:"Root wait timeout"`

	Admin conga.Command[featureSharedConfigKeyAdmin] `feature:"packages" summary:"Admin operations"`

	Ran bool
}

type featureSharedConfigKeyAdmin struct {
	Beta conga.OptionalFlag[string] `config:"custom-key" help:"Beta toggle"`

	Ran bool
}

func (c *featureSharedConfigKeyCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true

	return nil
}

func (s *featureSharedConfigKeyAdmin) Run(ctx context.Context, session *conga.Session) error {
	s.Ran = true

	return nil
}

// TestFeature_ConfigFile_PrunedSubcommandBareConfigKeyNotClaimed guards the
// shared explicit-config tier: a pruned subcommand flag declaring the same bare
// config key as an active root flag must not reject the root's value.
func TestFeature_ConfigFile_PrunedSubcommandBareConfigKeyNotClaimed(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureSiblingConfigKey, 0o600)
	assert.NoError(t, err)

	var cli featureSharedConfigKeyCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	err = app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.Equal(t, "root-value", cli.Wait.Get())
}

// TestFeature_ConfigFile_PrunedSubcommandScopedConfigKeyRejection keeps the
// rejection contract for command-scoped keys: only the namespaced key belongs
// to the pruned subcommand flag, so a value targeting it is not silently
// ignored.
func TestFeature_ConfigFile_PrunedSubcommandScopedConfigKeyRejection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureScopedConfigKey, 0o600)
	assert.NoError(t, err)

	var cli featureSharedConfigKeyCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--beta' which requires feature "packages"`)
}

type featureInvalidEnumTagCLI struct {
	Features conga.Features[featureExperiments]
	BadFlag  conga.OptionalFlag[string] `feature:"syntax=3" name:"bad"`
}

func (c *featureInvalidEnumTagCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_ValueComparisonSyntaxRejected(t *testing.T) {
	t.Parallel()

	var cli featureInvalidEnumTagCLI

	app := conga.New(&cli).Name("task").Args([]string{})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), `invalid feature tag syntax "syntax=3"`)
}

type flagConstraintCLI struct {
	Foo conga.OptionalFlag[bool] `xor:"group"`
	Bar conga.OptionalFlag[bool] `xor:"group"`
	Num conga.RequiredArg[int]
}

func (c *flagConstraintCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_FlagConstraintPrecedenceOverArgCoercion(t *testing.T) {
	t.Parallel()

	var cli flagConstraintCLI

	app := conga.New(&cli).Name("app").Args([]string{"--foo", "--bar", "notanumber"})

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `mutually exclusive`)
}

// TestFeature_GatedFlagErrorDeterministic pins the first-error contract when
// several inactive gated flags are supplied: targets are inspected in
// flag-name order, so the reported flag never depends on map iteration order.
func TestFeature_GatedFlagErrorDeterministic(t *testing.T) {
	t.Parallel()

	var firstErrMsg string

	for i := range 20 {
		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			Args([]string{"--syntax", "x", "--force-all", "--v1", "y"})

		err := app.Run()
		assert.Error(t, err)

		if i == 0 {
			firstErrMsg = err.Error()
		} else {
			assert.Equal(t, firstErrMsg, err.Error(), "error message must be deterministic across iterations")
		}
	}

	assert.Contains(t, firstErrMsg, `flag '--force-all' requires feature "gentle-force"`)
}

func TestFeature_ConfigErrorPrecedenceOverFeatureGate(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "broken.yaml")
	err := os.WriteFile(configPath, []byte("broken: [unclosed\n"), 0o600)
	assert.NoError(t, err)

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"package"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.NotContains(t, err.Error(), `requires feature`)
}

type badConfigFlagCLI struct {
	Features conga.Features[featureExperiments]
	Config   conga.ConfigFile                        `name:"config"`
	Package  conga.Command[featurePackageSubcommand] `feature:"packages"`
}

func (c *badConfigFlagCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_ExplicitConfigExtractionErrorHalts(t *testing.T) {
	t.Parallel()

	var cli badConfigFlagCLI

	app := conga.New(&cli).Name("app").Args([]string{"package", "--config"})

	err := app.Run()
	assert.Error(t, err)
}

func TestFeature_DeterministicErrorAcrossIterations(t *testing.T) {
	t.Parallel()

	var firstErrMsg string

	for i := range 20 {
		var cli featureTestCLI

		app := conga.New(&cli).
			Name("app").
			Args([]string{"--feature-gates", "zebra=true,alpha=true,beta=true"})

		err := app.Run()
		assert.Error(t, err)

		if i == 0 {
			firstErrMsg = err.Error()
		} else {
			assert.Equal(t, firstErrMsg, err.Error(), "error message must be deterministic across iterations")
		}
	}
}

type clusterExperiments struct {
	Experimental bool
}

type leafClusterCmd struct {
	Ran  bool
	Wait conga.OptionalFlag[string] `feature:"experimental" name:"wait"`
}

func (c *leafClusterCmd) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true

	return nil
}

type midClusterCmd struct {
	Node conga.Command[leafClusterCmd]
}

type nestedClusterCLI struct {
	Features conga.Features[clusterExperiments]
	Cluster  conga.Command[midClusterCmd]
}

func (c *nestedClusterCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func TestFeature_ConfigFile_PrunedCommandLocalKeyRejection(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte("cluster:\n  node:\n    wait: 30s\n"), 0o600)
	assert.NoError(t, err)

	var cli nestedClusterCLI

	app := conga.New(&cli).
		Name("task").
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"cluster", "node"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--wait' which requires feature "experimental"`)
}
