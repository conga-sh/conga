package conga_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/yaml"
)

var (
	//go:embed testdata/fixtures/feature_camel_case.yml
	fixtureFeatureCamelCase []byte

	// Cascade payloads splitting the feature gates across a system and a
	// local config tier.
	//go:embed testdata/fixtures/feature_cascade_local.yml
	fixtureFeatureCascadeLocal []byte

	//go:embed testdata/fixtures/feature_cascade_sys.yml
	fixtureFeatureCascadeSys []byte

	//go:embed testdata/fixtures/feature_gentle_force.yml
	fixtureFeatureGentleForce []byte

	//go:embed testdata/fixtures/feature_inactive_flag.yml
	fixtureFeatureInactiveFlag []byte

	//go:embed testdata/fixtures/feature_invalid_value.yml
	fixtureFeatureInvalidValue []byte

	//go:embed testdata/fixtures/feature_multi_active.yml
	fixtureFeatureMultiActive []byte

	//go:embed testdata/fixtures/feature_packages.yml
	fixtureFeaturePackages []byte

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

	//go:embed testdata/fixtures/feature_snake_case.yml
	fixtureFeatureSnakeCase []byte

	//go:embed testdata/fixtures/feature_typo.yml
	fixtureFeatureTypo []byte
)

type featureExperiments struct {
	GentleForce bool `default:"false"                 help:"Forces all dependent tasks"`
	Packages    bool `help:"Manage Taskfile packages"`
	Syntax      int  `enum:"1,2"                      help:"Syntax version (0 = off)"`
}

type featureTestCLI struct {
	Features conga.Features[featureExperiments] `config:"experiments" env:"CONGA_TEST_TASK_X_FEATURES" flag:"feature-gates"`

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

	Features conga.Features[featureExperiments] `config:"experiments" env:"CONGA_TEST_TASK_X_FEATURES" flag:"feature-gates"`

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
		EnableFeatures().
		Args([]string{"--feature-gates", "gentle-force=true", "--force-all"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "gentle-force=false")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.ActiveGentle)
}

func TestFeature_MergesSourcesPerKey(t *testing.T) {
	t.Parallel()

	writeConfig := func(t *testing.T, fixture []byte) string {
		t.Helper()

		configPath := filepath.Join(t.TempDir(), ".taskrc.yml")
		assert.NoError(t, os.WriteFile(configPath, fixture, 0o600))

		return configPath
	}

	t.Run("config and CLI contribute different gates", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureGentleForce)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "packages=true"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Features.Get().GentleForce)
		assert.True(t, cli.Features.Get().Packages)
	})

	t.Run("CLI overrides config per key", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureGentleForce)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "gentle-force=false"})

		err := app.Run()

		assert.NoError(t, err)
		assert.False(t, cli.Features.Get().GentleForce)
	})

	t.Run("config snake_case key loses to the CLI spelling", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureSnakeCase)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "gentle-force=false"})

		err := app.Run()

		assert.NoError(t, err)
		assert.False(t, cli.Features.Get().GentleForce)
		assert.Equal(t, conga.SourceCLI, cli.Features.Source())
	})

	t.Run("config camelCase key loses to the CLI spelling", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureCamelCase)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "gentle-force=false"})

		err := app.Run()

		assert.NoError(t, err)
		assert.False(t, cli.Features.Get().GentleForce)
		assert.Equal(t, conga.SourceCLI, cli.Features.Source())
	})

	t.Run("CLI same-tier later token overrides earlier token in input sequence", func(t *testing.T) {
		t.Parallel()

		var cli1 featureTestCLI

		app1 := conga.New(&cli1).
			Name("task").
			EnableFeatures().
			Args([]string{"--feature-gates", "gentle_force=false,gentle-force=true"})

		err1 := app1.Run()

		assert.NoError(t, err1)
		assert.True(t, cli1.Features.Get().GentleForce)

		var cli2 featureTestCLI

		app2 := conga.New(&cli2).
			Name("task").
			EnableFeatures().
			Args([]string{"--feature-gates", "gentle-force=true,gentle_force=false"})

		err2 := app2.Run()

		assert.NoError(t, err2)
		assert.False(t, cli2.Features.Get().GentleForce)
	})

	t.Run("why table attributes the canonical key to the CLI tier", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureSnakeCase)

		var buffer bytes.Buffer

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Stdout(&buffer).
			EnableFeatures().
			EnableWhy().
			Args([]string{"--feature-gates", "gentle-force=false", "--why"})

		err := app.Run()

		assert.NoError(t, err)

		_, featuresSection, found := strings.Cut(buffer.String(), "Features:")
		assert.True(t, found)

		for line := range strings.SplitSeq(featuresSection, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "gentle-force") {
				assert.Contains(t, line, "false")
				assert.Contains(t, line, "cli")
			}
		}
	})

	t.Run("CLI override of a config key masks its invalid value", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureInvalidValue)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "syntax=1"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, 1, cli.Features.Get().Syntax)
	})

	t.Run("unknown config key errors even when CLI sets the flag", func(t *testing.T) {
		t.Parallel()

		configPath := writeConfig(t, fixtureFeatureTypo)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--feature-gates", "packages=true"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrConfig)
		assert.Contains(t, err.Error(), `unknown feature "GNTLE_FORCCE" (did you mean "gentle-force"?)`)
	})
}

func TestFeature_MergesEnvAboveConfig(t *testing.T) {
	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "packages=true")

	configPath := filepath.Join(t.TempDir(), ".taskrc.yml")
	assert.NoError(t, os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600))

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().GentleForce)
	assert.True(t, cli.Features.Get().Packages)
}

// featureCascadeCLI exercises feature gates against cascading config files.
type featureCascadeCLI struct {
	ConfigFile conga.ConfigFile `help:"Config file" multi:"true"`

	Features conga.Features[featureExperiments] `config:"experiments" env:"CONGA_TEST_TASK_X_FEATURES" flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran            bool
	ActiveGentle   bool
	ActivePackages bool
}

func (cli *featureCascadeCLI) Run(_ context.Context, _ *conga.Session) error {
	cli.Ran = true
	cli.ActiveGentle = cli.Features.Get().GentleForce
	cli.ActivePackages = cli.Features.Get().Packages

	return nil
}

// TestFeature_CascadingConfigMergesPerKey pins that feature gates merge key
// by key across cascading config files: a gate enabled only in a lower file
// survives alongside gates from higher files.
func TestFeature_CascadingConfigMergesPerKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	localPath := filepath.Join(tempDir, "local.yml")

	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))
	assert.NoError(t, os.WriteFile(localPath, fixtureFeatureCascadeLocal, 0o600))

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.ActiveGentle)
	assert.True(t, cli.ActivePackages)
}

// TestFeature_CascadingConfigNearestFileWinsGate pins that a gate declared by
// two cascading files resolves to the nearest file.
func TestFeature_CascadingConfigNearestFileWinsGate(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))

	// The local tier re-declares the sys file's gate with the opposite value.
	localPath := writeConfigFile(t, `{"experiments": {"gentle-force": "false", "packages": "true"}}`)

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.False(t, cli.ActiveGentle)
	assert.True(t, cli.ActivePackages)
}

// TestFeature_CascadingConfigEnvOverridesPerKey pins that the feature
// environment variable overrides one cascaded gate key without dropping the
// files' remaining gates.
func TestFeature_CascadingConfigEnvOverridesPerKey(t *testing.T) {
	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "gentle-force=false")

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	localPath := filepath.Join(tempDir, "local.yml")

	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))
	assert.NoError(t, os.WriteFile(localPath, fixtureFeatureCascadeLocal, 0o600))

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.False(t, cli.ActiveGentle)
	assert.True(t, cli.ActivePackages)
}

// TestFeature_CascadingConfigCLIOverridesPerKey pins that the feature CLI flag
// overrides one cascaded gate key without dropping the files' remaining gates.
func TestFeature_CascadingConfigCLIOverridesPerKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	localPath := filepath.Join(tempDir, "local.yml")

	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))
	assert.NoError(t, os.WriteFile(localPath, fixtureFeatureCascadeLocal, 0o600))

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{"--feature-gates", "packages=false"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.ActiveGentle)
	assert.False(t, cli.ActivePackages)
}

// TestFeature_CascadingConfigGateOffPrunesFlag pins that a gate turned off by
// the nearest cascading file prunes the gated flag's CLI invocation.
func TestFeature_CascadingConfigGateOffPrunesFlag(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))

	// The local tier turns the gate off.
	localPath := writeConfigFile(t, `{"experiments": {"gentle-force": "false"}}`)

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{"--force-all"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires feature")
}

// TestFeature_CascadingConfigRestrictsPrunedConfigKey pins that a gated-off
// flag's config key in a cascading file still fails the run instead of being
// silently ignored.
func TestFeature_CascadingConfigRestrictsPrunedConfigKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	sysPath := filepath.Join(tempDir, "sys.yml")
	assert.NoError(t, os.WriteFile(sysPath, fixtureFeatureCascadeSys, 0o600))

	// The local tier turns the gate off and declares the gated flag's key.
	localPath := writeConfigFile(t, `{"experiments": {"gentle-force": "false"}, "force-all": true}`)

	var cli featureCascadeCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), "requires feature")
}

func TestFeature_EnvSpellingLosesToCLI(t *testing.T) {
	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "gentle_force=true")

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{"--feature-gates", "gentle-force=false"})

	err := app.Run()

	assert.NoError(t, err)
	assert.False(t, cli.Features.Get().GentleForce)
	assert.Equal(t, conga.SourceCLI, cli.Features.Source())
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
		EnableFeatures().
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

	app := conga.New[featureDuplicateCLI]().Name("task").
		EnableFeatures()

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `feature "dup" (already declared`)
}

func TestFeature_FlagGating_Disabled(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
		Args([]string{"--force-all"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "gentle-force=true")

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
		EnableFeatures().
		Args([]string{"package", "--name", "lib"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "packages=true")

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

// TestFeature_AppEnvPrefixNeverTouchesFeatures pins the env-tier isolation of
// the Features container: the application-level EnvPrefix applies to normal
// flags only, so a derived combined map variable like CONGA_TEST_TASK_X_FEATURE_GATES
// never reaches feature resolution, and a gated flag stays inactive.
func TestFeature_AppEnvPrefixNeverTouchesFeatures(t *testing.T) {
	var cli featureDerivedEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		EnvPrefix("CONGA_TEST_TASK_X").
		Args([]string{"--force-all"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURE_GATES", "gentle-force=true")

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `flag '--force-all' requires feature "gentle-force"`)
	assert.False(t, cli.ForceAll.IsSet())
	assert.False(t, cli.ForceAll.Get())
}

// TestFeature_EnvForInactiveGatedFlag_Ignored pins the environment-tier
// asymmetry: pruning removes a gated flag from the schema before environment
// binding, so a variable targeting it is silently ignored while a config key
// naming it is rejected.
func TestFeature_EnvForInactiveGatedFlag_Ignored(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		EnvPrefix("CONGA_TEST_TASK_X").
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_FORCE_ALL", "true")

	err := app.Run()
	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.ForceAll.IsSet())
	assert.False(t, cli.ForceAll.Get())
}

// fieldEnvExperiments mirrors task's experiment surface: plain feature fields
// whose environment variables derive from the container's envprefix.
type fieldEnvExperiments struct {
	GentleForce   bool `help:"Forces all dependent tasks"`
	Packages      bool `help:"Manage Taskfile packages"`
	EnvPrecedence bool `help:"Taskfile env entries override the inherited environment"`
	Syntax        int  `enum:"1,2"                                                     help:"Syntax version (0 = off)"`
}

type featureFieldEnvCLI struct {
	Features conga.Features[fieldEnvExperiments] `config:"experiments" envprefix:"CONGA_TEST_TASK_X" flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran              bool
	ActiveGentle     bool
	ActivePrecedence bool
	ResolvedSyntax   int
}

func (c *featureFieldEnvCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true
	c.ActiveGentle = c.Features.Get().GentleForce
	c.ActivePrecedence = c.Features.Get().EnvPrecedence
	c.ResolvedSyntax = c.Features.Get().Syntax

	return nil
}

// fieldEnvOverrideSet mixes explicit env tags, an excluded field, and an
// untagged field that keeps deriving from the container prefix.
type fieldEnvOverrideSet struct {
	GentleForce bool `env:"CONGA_TEST_CUSTOM_FORCE"   help:"Forces all dependent tasks"`
	Excluded    bool `env:"-"                         help:"CLI and config only"`
	Packages    bool `help:"Manage Taskfile packages"`
}

type featureFieldEnvOverrideCLI struct {
	Features conga.Features[fieldEnvOverrideSet] `envprefix:"CONGA_TEST_TASK_X" flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran            bool
	ActiveGentle   bool
	ActivePackages bool
	ActiveExcluded bool
}

func (c *featureFieldEnvOverrideCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true
	c.ActiveGentle = c.Features.Get().GentleForce
	c.ActivePackages = c.Features.Get().Packages
	c.ActiveExcluded = c.Features.Get().Excluded

	return nil
}

type featureFieldEnvStandaloneCLI struct {
	Features conga.Features[fieldEnvOverrideSet] `flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran          bool
	ActiveGentle bool
}

func (c *featureFieldEnvStandaloneCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true
	c.ActiveGentle = c.Features.Get().GentleForce

	return nil
}

// featureFieldEnvSplitPrefixCLI splits the prefixes: the application prefix
// drives normal flags while the container envprefix drives features, so the
// two derivation namespaces coexist without colliding.
type featureFieldEnvSplitPrefixCLI struct {
	Features conga.Features[fieldEnvExperiments] `envprefix:"CONGA_TEST_TASK_X" flag:"feature-gates"`

	Port     conga.OptionalFlag[int]  `help:"Port"`
	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`

	Ran          bool
	ActiveGentle bool
	PortValue    int
}

func (c *featureFieldEnvSplitPrefixCLI) Run(ctx context.Context, session *conga.Session) error {
	c.Ran = true
	c.ActiveGentle = c.Features.Get().GentleForce
	c.PortValue = c.Port.Get()

	return nil
}

// featureCombinedEnvCLI declares multiple combined-map variables so the
// winning variable can be distinguished from the declaration list.
type featureCombinedEnvCLI struct {
	Features conga.Features[featureExperiments] `env:"CONGA_TEST_GATES_A,CONGA_TEST_GATES_B" flag:"feature-gates"`
}

func (c *featureCombinedEnvCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// TestFeature_FieldEnv_DerivedFromContainerEnvPrefix pins that the container
// envprefix derives one variable per feature with no field tags at all.
func TestFeature_FieldEnv_DerivedFromContainerEnvPrefix(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{"--force-all"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ActiveGentle)
	assert.True(t, cli.ForceAll.Get())
}

// TestFeature_FieldEnv_IntLevelViaEnv pins that a leveled integer feature
// resolves through its derived variable.
func TestFeature_FieldEnv_IntLevelViaEnv(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_SYNTAX", "2")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.Equal(t, 2, cli.ResolvedSyntax)
}

// TestFeature_FieldEnv_ExplicitOverride pins that an explicit field env tag
// replaces the derived variable and the derived name stops binding.
func TestFeature_FieldEnv_ExplicitOverride(t *testing.T) {
	t.Run("explicit variable binds", func(t *testing.T) {
		var cli featureFieldEnvOverrideCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			Args([]string{"--force-all"})

		t.Setenv("CONGA_TEST_CUSTOM_FORCE", "1")

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.ActiveGentle)
		assert.True(t, cli.ForceAll.Get())
	})

	t.Run("derived name no longer binds", func(t *testing.T) {
		var cli featureFieldEnvOverrideCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			Args([]string{})

		t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Ran)
		assert.False(t, cli.ActiveGentle)
	})
}

// TestFeature_FieldEnv_ExcludedOptOut pins that env:"-" on a feature field
// removes its variable while untagged siblings keep deriving.
func TestFeature_FieldEnv_ExcludedOptOut(t *testing.T) {
	var cli featureFieldEnvOverrideCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_EXCLUDED", "1")
	t.Setenv("CONGA_TEST_TASK_X_PACKAGES", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ActivePackages)
	assert.False(t, cli.ActiveExcluded)
}

// TestFeature_FieldEnv_Standalone pins that field env tags bind without any
// container tag: explicit names work and untagged fields stay env-free.
func TestFeature_FieldEnv_Standalone(t *testing.T) {
	t.Run("explicit variable binds", func(t *testing.T) {
		var cli featureFieldEnvStandaloneCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			Args([]string{"--force-all"})

		t.Setenv("CONGA_TEST_CUSTOM_FORCE", "1")

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.ActiveGentle)
		assert.True(t, cli.ForceAll.Get())
	})

	t.Run("untagged field binds nothing", func(t *testing.T) {
		var cli featureFieldEnvStandaloneCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
			Args([]string{})

		t.Setenv("CONGA_TEST_TASK_X_PACKAGES", "1")

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Ran)
		assert.False(t, cli.ActiveGentle)
	})
}

// TestFeature_EnvSurfaceAbsentByDefault pins that a container without env or
// envprefix binds no environment variable: the combined-map spelling is
// ignored and the gate stays off.
func TestFeature_EnvSurfaceAbsentByDefault(t *testing.T) {
	var cli featureDerivedEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{"--force-all"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURE_GATES", "gentle-force=true")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `flag '--force-all' requires feature "gentle-force"`)
	assert.False(t, cli.ForceAll.Get())
}

// TestFeature_ContainerEnvAndPrefixConflict pins the env/envprefix
// exclusivity: the two tags name different env surfaces and fail schema build.
func TestFeature_ContainerEnvAndPrefixConflict(t *testing.T) {
	t.Parallel()

	type conflictingSet struct {
		Enabled bool
	}

	var cli struct {
		Features conga.Features[conflictingSet] `env:"CONGA_TEST_FEATURES" envprefix:"CONGA_TEST_"`
	}

	app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), `tag "env" and tag "envprefix" cannot both be declared on a conga.Features container (env names the combined map variable; envprefix derives one variable per feature)`)
}

// TestFeature_FieldEnvWithContainerEnvConflict pins the one-env-surface rule:
// field env tags under a combined-map container fail schema build.
func TestFeature_FieldEnvWithContainerEnvConflict(t *testing.T) {
	t.Parallel()

	type conflictingSet struct {
		Enabled bool `env:"CONGA_TEST_TASK_X_ENABLED"`
	}

	var cli struct {
		Features conga.Features[conflictingSet] `env:"CONGA_TEST_FEATURES"`
	}

	app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), `feature field Enabled declares env while the Features container declares env (the combined map variable); declare one env surface per container`)
}

// TestFeature_FieldEnv_MultipleNamesFirstSetWins pins the flag-convention
// multi-name rule: the first set variable wins and an empty variable falls
// through to the next name.
func TestFeature_FieldEnv_MultipleNamesFirstSetWins(t *testing.T) {
	type multiNameSet struct {
		GentleForce bool `env:"CONGA_TEST_PRIMARY_FORCE,CONGA_TEST_SECOND_FORCE"`
	}

	t.Run("first set variable wins", func(t *testing.T) {
		var cli struct {
			noopRunner

			Features conga.Features[multiNameSet]
		}

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		t.Setenv("CONGA_TEST_PRIMARY_FORCE", "false")
		t.Setenv("CONGA_TEST_SECOND_FORCE", "true")

		err := app.Run()

		assert.NoError(t, err)
		assert.False(t, cli.Features.Get().GentleForce)
	})

	t.Run("empty first variable falls through", func(t *testing.T) {
		var cli struct {
			noopRunner

			Features conga.Features[multiNameSet]
		}

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		t.Setenv("CONGA_TEST_PRIMARY_FORCE", "")
		t.Setenv("CONGA_TEST_SECOND_FORCE", "true")

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Features.Get().GentleForce)
	})
}

// TestFeature_FieldEnv_OverridesConfig pins the ladder: the per-feature env
// tier overrides the configuration tier for the same key.
func TestFeature_FieldEnv_OverridesConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600)
	assert.NoError(t, err)

	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "false")

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.ActiveGentle)
}

// TestFeature_FieldEnv_SparseMergeWithConfig pins the key-by-key merge: the
// env tier enables one key while the config tier enables another.
func TestFeature_FieldEnv_SparseMergeWithConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeaturePackages, 0o600)
	assert.NoError(t, err)

	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ActiveGentle)
}

// TestFeature_FieldEnv_EmptyValueFallsThroughToConfig pins that an empty env
// variable is unset: the configuration tier's value survives.
func TestFeature_FieldEnv_EmptyValueFallsThroughToConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".taskrc.yml")
	err := os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600)
	assert.NoError(t, err)

	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "")

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ActiveGentle)
}

// TestFeature_FieldEnv_CliBeatsEnv pins the top of the ladder: a command-line
// map key overrides the per-feature variable for that key.
func TestFeature_FieldEnv_CliBeatsEnv(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{"--feature-gates", "gentle-force=false"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.ActiveGentle)
}

// TestFeature_FieldEnv_CliBeatsEnvWithRegisteredStringMap pins that a custom
// map parser registered for map[string]string does not divert the engine-owned
// feature map from its native merge, where CLI keys stay ahead of env values.
func TestFeature_FieldEnv_CliBeatsEnvWithRegisteredStringMap(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		RegisterMap[map[string]string]().
		Args([]string{"--feature-gates", "gentle-force=false"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.False(t, cli.ActiveGentle)
}

// TestFeature_FieldEnv_InvalidValueError pins the hard-failure contract with
// the variable named: an invalid per-feature env value aborts with a usage
// error that identifies the offending variable.
func TestFeature_FieldEnv_InvalidValueError(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_SYNTAX", "3")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax" (from environment variable CONGA_TEST_TASK_X_SYNTAX): must be one of: 0, 1, 2`)
}

// TestFeature_CombinedEnv_InvalidValueError pins that an invalid value from a
// combined-map environment variable names the variable in the diagnostic.
func TestFeature_CombinedEnv_InvalidValueError(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "syntax=3")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
	assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax" (from environment variable CONGA_TEST_TASK_X_FEATURES): must be one of: 0, 1, 2`)
}

// TestFeature_CombinedEnv_InvalidValueErrorUnderWhy pins the same attribution
// when the run also enables --why reporting.
func TestFeature_CombinedEnv_InvalidValueErrorUnderWhy(t *testing.T) {
	var cli featureTestCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		EnableWhy().
		Args([]string{"--why"})

	t.Setenv("CONGA_TEST_TASK_X_FEATURES", "syntax=3")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax" (from environment variable CONGA_TEST_TASK_X_FEATURES): must be one of: 0, 1, 2`)
}

// TestFeature_CombinedEnv_InvalidValueNamesWinningVariable pins that the
// diagnostic names the environment variable that supplied the map, not every
// variable the container declares.
func TestFeature_CombinedEnv_InvalidValueNamesWinningVariable(t *testing.T) {
	var cli featureCombinedEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_GATES_A", "")
	t.Setenv("CONGA_TEST_GATES_B", "syntax=3")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Contains(t, err.Error(), `invalid value "3" for feature "syntax" (from environment variable CONGA_TEST_GATES_B): must be one of: 0, 1, 2`)
	assert.NotContains(t, err.Error(), "CONGA_TEST_GATES_A")
}

// TestFeature_FieldEnv_SourceReporting pins the container accessors when only
// per-feature env variables supply values.
func TestFeature_FieldEnv_SourceReporting(t *testing.T) {
	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Args([]string{})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceEnv, cli.Features.Source())
	assert.Equal(t, "CONGA_TEST_TASK_X_GENTLE_FORCE", cli.Features.SourceDetail())
	assert.True(t, cli.Features.IsSet())
	assert.True(t, cli.Features.Changed())
}

// TestFeature_FieldEnv_CoexistsWithAppPrefix pins that the application prefix
// and the container envprefix coexist: normal flags derive from the
// application prefix, features from the container prefix, and no collision is
// reported.
func TestFeature_FieldEnv_CoexistsWithAppPrefix(t *testing.T) {
	var cli featureFieldEnvSplitPrefixCLI

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		EnvPrefix("CONGA_TEST").
		Args([]string{"--force-all"})

	t.Setenv("CONGA_TEST_PORT", "8080")
	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Ran)
	assert.True(t, cli.ActiveGentle)
	assert.Equal(t, 8080, cli.PortValue)
}

// featureEnvSingleSet carries one explicitly env-tagged feature for the
// feature-vs-flag collision tests.
type featureEnvSingleSet struct {
	Enabled bool `env:"CONGA_TEST_SHARED_VAR"`
}

type featureEnvFlagDupCLI struct {
	Features conga.Features[featureEnvSingleSet]

	Port conga.OptionalFlag[int] `env:"CONGA_TEST_SHARED_VAR" help:"Port"`
}

func (c *featureEnvFlagDupCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// featureEnvDerivedFlagSet names the variable the plain --port flag derives
// from the application prefix.
type featureEnvDerivedFlagSet struct {
	Enabled bool `env:"CONGA_TEST_TASK_X_PORT"`
}

type featureEnvDerivedFlagCLI struct {
	Features conga.Features[featureEnvDerivedFlagSet]

	Port conga.OptionalFlag[int] `help:"Port"`
}

func (c *featureEnvDerivedFlagCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type featureEnvMapDupCLI struct {
	Features conga.Features[featureExperiments] `env:"CONGA_TEST_SHARED_MAP_VAR" flag:"feature-gates"`

	Port conga.OptionalFlag[int] `env:"CONGA_TEST_SHARED_MAP_VAR" help:"Port"`
}

func (c *featureEnvMapDupCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type featureEnvSubSet struct {
	Enabled bool `env:"CONGA_TEST_SUB_CONGA_TEST_SHARED_VAR"`
}

type featureEnvSubcommand struct {
	Name conga.OptionalFlag[string] `env:"CONGA_TEST_SUB_CONGA_TEST_SHARED_VAR" help:"Name"`
}

func (c *featureEnvSubcommand) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type featureEnvSubCLI struct {
	Features conga.Features[featureEnvSubSet]

	Sub conga.Command[featureEnvSubcommand] `summary:"Sub"`
}

func (c *featureEnvSubCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// TestFeature_FieldEnv_CollisionTests pins the env-name collision validator:
// two feature fields resolving to one variable, or a feature variable a flag
// also binds, fail schema build with a self-describing error.
func TestFeature_FieldEnv_CollisionTests(t *testing.T) {
	t.Parallel()

	t.Run("duplicate fields in one container", func(t *testing.T) {
		t.Parallel()

		var cli struct {
			Features conga.Features[struct {
				First  bool `env:"CONGA_TEST_SHARED_VAR"`
				Second bool `env:"CONGA_TEST_SHARED_VAR"`
			}]
		}

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `environment variable "CONGA_TEST_SHARED_VAR" is bound by both feature "first" and feature "second"`)
	})

	t.Run("explicit field variable collides with a derived sibling", func(t *testing.T) {
		t.Parallel()

		var cli struct {
			Features conga.Features[struct {
				First  bool `env:"CONGA_TEST_TASK_X_SECOND"`
				Second bool
			}] `envprefix:"CONGA_TEST_TASK_X_"`
		}

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `environment variable "CONGA_TEST_TASK_X_SECOND" is bound by both feature "first" and feature "second"`)
	})

	t.Run("field variable collides with a flag variable", func(t *testing.T) {
		t.Parallel()

		var cli featureEnvFlagDupCLI

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct featureEnvFlagDupCLI field Features: schema error: environment variable "CONGA_TEST_SHARED_VAR" is bound by both feature "enabled" and flag --port`, err.Error())
	})

	t.Run("field variable collides with a derived flag variable", func(t *testing.T) {
		t.Parallel()

		var cli featureEnvDerivedFlagCLI

		app := conga.New(&cli).Name("task").EnableFeatures().EnvPrefix("CONGA_TEST_TASK_X").Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `environment variable "CONGA_TEST_TASK_X_PORT" is bound by both feature "enabled" and flag --port`)
	})

	t.Run("combined map variable collides with a flag variable", func(t *testing.T) {
		t.Parallel()

		var cli featureEnvMapDupCLI

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `environment variable "CONGA_TEST_SHARED_MAP_VAR" is bound by both the feature map flag --feature-gates and flag --port`)
	})

	t.Run("field variable collides with a subcommand flag variable", func(t *testing.T) {
		t.Parallel()

		var cli featureEnvSubCLI

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `environment variable "CONGA_TEST_SUB_CONGA_TEST_SHARED_VAR" is bound by both feature "enabled" and flag --name`)
	})

	t.Run("flag variables do not collide with each other", func(t *testing.T) {
		t.Parallel()

		var cli struct {
			noopRunner

			Features conga.Features[struct{}]

			First  conga.OptionalFlag[int] `env:"CONGA_TEST_SHARED_FLAG" help:"First"`
			Second conga.OptionalFlag[int] `env:"CONGA_TEST_SHARED_FLAG" help:"Second"`
		}

		app := conga.New(&cli).Name("task").EnableFeatures().Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
	})
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
		Stdout(&buffer).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--version"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "has version")
}

func TestFeature_ConfigTypo_WhyFails(t *testing.T) {
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
		EnableFeatures().
		EnableWhy().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--why"})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown feature")
}

// TestFeature_Why_RejectedValueErrors pins that invalid feature values or unknown
// feature names return errors even when --why is passed.
func TestFeature_Why_RejectedValueErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			EnableFeatures().
			EnableWhy().
			Args([]string{"--feature-gates", "syntax=9", "--why"})

		err := app.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid value")
	})

	t.Run("unknown key returns error", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
			cli    featureTestCLI
		)

		app := conga.New(&cli).
			Name("task").
			Stdout(&buffer).
			EnableFeatures().
			EnableWhy().
			Args([]string{"--feature-gates", "gentle-forse=true", "--why"})

		err := app.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown feature")
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
			EnableFeatures().
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
			EnableFeatures().
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

func TestFeature_HelpFeaturesSection(t *testing.T) {
	t.Parallel()

	var (
		buffer bytes.Buffer
		cli    featureTestCLI
	)

	app := conga.New(&cli).
		Name("task").
		EnableFeatures().
		Stdout(&buffer).
		Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.Contains(t, output, "Features:")
	assert.Contains(t, output, "gentle-force  Forces all dependent tasks [default: false]")
	assert.Contains(t, output, "syntax        Syntax version (0 = off) [levels: 1, 2]")
	assert.NotContains(t, output, "[enum: gentle-force")
}

func TestFeature_MultiLevel(t *testing.T) {
	t.Parallel()

	t.Run("runs at level two and materializes the value", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableFeatures().
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
			EnableFeatures().
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
			EnableFeatures().
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
			EnableCompletion().
			EnableFeatures()

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
			EnableFeatures().
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
			EnableCompletion().
			EnableFeatures()

		for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
			_, formatted := app.CompleteForShell(shell, []string{"--feature-gates=gentle-force=true", "--"})
			assert.Contains(t, formatted, "--force-all")
			assert.NotContains(t, formatted, "--syntax")
		}
	})

	t.Run("feature key completion carries help descriptions", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			EnableFeatures()

		result, _ := app.CompleteForShell("bash", []string{"--feature-gates", ""})

		assert.Equal(t, 3, len(result.Items))
		assert.Equal(t, "gentle-force=", result.Items[0].Value)
		assert.Equal(t, "Forces all dependent tasks", result.Items[0].Description)
		assert.Equal(t, "packages=", result.Items[1].Value)
		assert.Equal(t, "Manage Taskfile packages", result.Items[1].Description)
		assert.Equal(t, "syntax=", result.Items[2].Value)
		assert.Equal(t, "Syntax version (0 = off)", result.Items[2].Description)
	})

	t.Run("CLI feature gates drive subcommand completion", func(t *testing.T) {
		t.Parallel()

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			EnableFeatures()

		_, formatted := app.CompleteForShell("bash", []string{"--feature-gates", "packages=true", "package", "--"})
		assert.Contains(t, formatted, "--name")
	})

	t.Run("config gates survive unrelated CLI gates", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, ".taskrc.yml")
		err := os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600)
		assert.NoError(t, err)

		var cli featureTestCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			EnableFeatures().
			RegisterDecoder(".yaml", yaml.Decode).
			RegisterDecoder(".yml", yaml.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath))

		_, formatted := app.CompleteForShell("bash", []string{"--feature-gates", "packages=true", "--"})
		assert.Contains(t, formatted, "--force-all")
	})
}

// TestFeature_Completion_FieldEnvInactiveGatePruned pins that a gated flag
// stays absent from completion candidates while its feature is off.
func TestFeature_Completion_FieldEnvInactiveGatePruned(t *testing.T) {
	t.Parallel()

	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		EnableFeatures()

	_, inactive := app.CompleteForShell("bash", []string{"--"})

	assert.NotContains(t, inactive, "--force-all")
}

// TestFeature_Completion_FieldEnvUnprunesGates pins that per-feature env
// variables are visible during completion pruning: a gate enabled through its
// derived variable un-prunes the gated flag's candidates.
func TestFeature_Completion_FieldEnvUnprunesGates(t *testing.T) {
	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	var cli featureFieldEnvCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		EnableFeatures()

	_, active := app.CompleteForShell("bash", []string{"--"})

	assert.Contains(t, active, "--force-all")
}

// TestFeature_Completion_FieldEnvCLIBeatsEnv pins completion precedence: a
// feature key supplied on the command line keeps its value when the matching
// per-feature environment variable is also set.
func TestFeature_Completion_FieldEnvCLIBeatsEnv(t *testing.T) {
	t.Run("env enables the gate", func(t *testing.T) {
		t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

		var cli featureFieldEnvCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			EnableFeatures()

		_, enabled := app.CompleteForShell("bash", []string{"--"})

		assert.Contains(t, enabled, "--force-all")
	})

	t.Run("CLI key wins over env", func(t *testing.T) {
		t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

		var cli featureFieldEnvCLI

		app := conga.New(&cli).
			Name("task").
			EnableCompletion().
			EnableFeatures()

		_, cliWins := app.CompleteForShell("bash", []string{"--feature-gates", "gentle-force=false", "--"})

		assert.NotContains(t, cliWins, "--force-all")
	})
}

// featureDotenvSet backs the dotenv-in-completion tests; SetenvFunc(t.Setenv)
// restores loaded variables when each test finishes.
type featureDotenvSet struct {
	GentleForce bool `help:"Forces all dependent tasks"`
}

type featureDotenvCLI struct {
	Features conga.Features[featureDotenvSet] `envprefix:"CONGA_TEST_TASK_DOTENV_" flag:"feature-gates"`

	ForceAll conga.OptionalFlag[bool] `feature:"gentle-force" help:"Forces all dependent tasks"`
}

// TestFeature_Completion_DotenvFieldEnv pins that App.Dotenv values reach
// completion: a .env file enabling a feature un-prunes its gated flag.
//
//nolint:paralleltest // dotenv.LoadWith(t.Setenv) mutates the process environment.
func TestFeature_Completion_DotenvFieldEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	err := os.WriteFile(envPath, []byte("CONGA_TEST_TASK_DOTENV_GENTLE_FORCE=1\n"), 0o600)
	assert.NoError(t, err)

	var cli featureDotenvCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		EnableFeatures().
		Dotenv(envPath).
		SetenvFunc(t.Setenv)

	_, formatted := app.CompleteForShell("bash", []string{"--"})

	assert.Contains(t, formatted, "--force-all")
}

// TestFeature_Completion_DotenvErrorTolerated pins that a malformed .env does
// not abort the completion: the failure is tolerated like a loader failure
// and candidates render from the remaining tiers.
//
//nolint:paralleltest // dotenv.LoadWith(t.Setenv) mutates the process environment.
func TestFeature_Completion_DotenvErrorTolerated(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	err := os.WriteFile(envPath, []byte("this is not dotenv\n"), 0o600)
	assert.NoError(t, err)

	var cli featureDotenvCLI

	app := conga.New(&cli).
		Name("task").
		EnableCompletion().
		EnableFeatures().
		Dotenv(envPath).
		SetenvFunc(t.Setenv)

	_, formatted := app.CompleteForShell("bash", []string{"--"})

	assert.Contains(t, formatted, "--feature-gates")
	assert.NotContains(t, formatted, "--force-all")
}

func TestFeature_Completion_ExplicitConfigFile(t *testing.T) {
	t.Parallel()

	newApp := func() *conga.App[featureConfigCLI] {
		return conga.New[featureConfigCLI]().
			Name("task").
			EnableCompletion().
			EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableCompletion().
		EnableFeatures()

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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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
		EnableFeatures().
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

	app := conga.New(&cli).Name("task").
		EnableFeatures().
		Args([]string{})

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

	app := conga.New(&cli).Name("app").
		EnableFeatures().
		Args([]string{"--foo", "--bar", "notanumber"})

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
			EnableFeatures().
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
		EnableFeatures().
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

	app := conga.New(&cli).Name("app").
		EnableFeatures().
		Args([]string{"package", "--config"})

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
			EnableFeatures().
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
		EnableFeatures().
		RegisterDecoder(".yaml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"cluster", "node"})

	err = app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file declares flag '--wait' which requires feature "experimental"`)
}

func TestFeature_RequiresEnableFeatures(t *testing.T) {
	t.Parallel()

	var cli featureTestCLI

	app := conga.New(&cli).Name("task")

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), "conga.Features declared on root command requires app.EnableFeatures()")
}
