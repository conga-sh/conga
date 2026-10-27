package conga_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	stdjson "encoding/json"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

func TestBuiltinHoist_HelpWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		EnvPrefix("CONGA_TEST_HOIST_").
		Stdout(&buffer).
		Args([]string{"--help"})

	t.Setenv("CONGA_TEST_HOIST_CONFIG", "missing-from-env-help.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_VersionWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		EnvPrefix("CONGA_TEST_HOIST_").
		Version("1.2.3").
		Stdout(&buffer).
		Args([]string{"--version"})

	t.Setenv("CONGA_TEST_HOIST_CONFIG", "missing-from-env-version.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "cloudctl has version 1.2.3")
}

func TestBuiltinHoist_ExplicitMissingConfigPathWithHelp(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		Args([]string{"--config-file", "missing-explicit.yaml", "--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_SubcommandHelpWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistSubCLI]().
		Name("cloudctl").
		EnvPrefix("CONGA_TEST_HOIST_").
		Stdout(&buffer).
		Args([]string{"sub", "--help"})

	t.Setenv("CONGA_TEST_HOIST_CONFIG", "missing-from-env-subhelp.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl sub")
}

func TestBuiltinHoist_HelpSubcommandWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistHelpCLI]().
		Name("cloudctl").
		EnvPrefix("CONGA_TEST_HOIST_").
		Stdout(&buffer).
		Args([]string{"help"})

	t.Setenv("CONGA_TEST_HOIST_CONFIG", "missing-from-env-helpcmd.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_CompletionScriptWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		EnableCompletion().
		Name("cloudctl").
		EnvPrefix("CONGA_TEST_HOIST_").
		Stdout(&buffer).
		Args([]string{"--completion", "zsh"})

	t.Setenv("CONGA_TEST_HOIST_CONFIG", "missing-from-env-completion.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "#compdef cloudctl")
}

func TestBuiltinHoist_VersionSkipsConfigResolutionWithoutFeatures(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(configPath, []byte("{}"), 0o600)
	assert.NoError(t, err)

	decodeCount := 0
	decode := func(_ []byte) (map[string]any, error) {
		decodeCount++

		return map[string]any{}, nil
	}

	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		Version("1.2.3").
		Stdout(&buffer).
		RegisterDecoder(".json", decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--version"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Equal(t, 0, decodeCount)
	assert.Contains(t, buffer.String(), "cloudctl has version 1.2.3")
}

func TestBuiltinHoist_HelpSkipsConfigResolutionWithoutFeatures(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(configPath, []byte("{}"), 0o600)
	assert.NoError(t, err)

	decodeCount := 0
	decode := func(_ []byte) (map[string]any, error) {
		decodeCount++

		return map[string]any{}, nil
	}

	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		Stdout(&buffer).
		RegisterDecoder(".json", decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--help"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Equal(t, 0, decodeCount)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_WhyResolvesConfigOnceWithoutFeatures(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(configPath, []byte("{}"), 0o600)
	assert.NoError(t, err)

	decodeCount := 0
	decode := func(_ []byte) (map[string]any, error) {
		decodeCount++

		return map[string]any{}, nil
	}

	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		RegisterDecoder(".json", decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--why"})

	err = app.Run()
	assert.NoError(t, err)
	assert.Equal(t, 1, decodeCount)
	assert.Contains(t, buffer.String(), "Value provenance")
}

func TestBuiltinHoist_DeprecationNotices(t *testing.T) {
	t.Parallel()

	t.Run("still emitted on normal runs", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[hoistDeprecatedCLI]().
			Name("cloudctl").
			Stderr(&buffer).
			Args([]string{"--old-flag"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "WARNING: flag '--old-flag' is deprecated")
	})

	t.Run("not emitted when deprecated flag accompanies a meta command", func(t *testing.T) {
		t.Parallel()

		var (
			stdoutBuffer bytes.Buffer
			stderrBuffer bytes.Buffer
		)

		app := conga.New[hoistDeprecatedCLI]().
			Name("cloudctl").
			Stdout(&stdoutBuffer).
			Stderr(&stderrBuffer).
			Args([]string{"--old-flag", "help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, stderrBuffer.String(), "deprecated")
	})
}

func TestWhy_ReportRedactionAndBadges(t *testing.T) {
	t.Setenv("CONGA_TEST_WHY_TOKEN", "s3cr3t-env-token")
	t.Chdir(t.TempDir())

	const configPath = "why-config.json"

	assert.NoError(t, os.WriteFile(configPath, []byte(`{"region": "eu-west-1"}`), 0o600))
	t.Setenv("CONGA_TEST_WHY_CONFIG", configPath)

	var buffer bytes.Buffer

	cli := new(whyCLI)
	app := conga.New(cli).
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--port", "8080", "--headers", "z=1,a=2", "--shown", "visible-value", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/why/redaction-badges.golden", goldenWhyRedactionBadges, buffer.String())

	// The command never ran: --why short-circuits execution.
	assert.False(t, cli.Sub.Get().ran)
}

type whyMergeFeatures struct {
	Alpha bool `help:"Alpha gate"`
	Beta  bool `default:"true"    help:"Beta gate"`
}

type whyMergeCLI struct {
	noopRunner

	Features conga.Features[whyMergeFeatures]      `config:"experiments"  env:"CONGA_TEST_WHY_MERGE_FEATURES" flag:"feature-gates"`
	Headers  conga.OptionalFlag[map[string]string] `help:"Headers"        safe:"true"`
	Secrets  conga.OptionalFlag[map[string]string] `help:"Secret headers"`
}

// TestWhy_MapMergePerKeyProvenance pins the per-key source attribution of a
// map flag merged across the config, environment, and CLI tiers. Relative
// config paths keep the reported origins stable across machines.
//
//nolint:paralleltest // Swaps the process working directory via t.Chdir.
func TestWhy_MapMergePerKeyProvenance(t *testing.T) {
	t.Chdir(t.TempDir())

	configPath := "why-merge.json"

	assert.NoError(t, os.WriteFile(configPath, []byte(`{
		"experiments": {"alpha": true},
		"headers": {"region": "eu-west-1", "shared": "cfg"},
		"secrets": {"token": "abc"}
	}`), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whyMergeCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Stdout(&buffer).
		EnableFeatures().
		EnableWhy().
		Args([]string{"--headers", "shared=cli,team=core", "--secrets", "clikey=xyz", "--feature-gates", "beta=false", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/why/map-merge.golden", goldenWhyMapMerge, buffer.String())
}

// whyCascadeCLI exercises a map flag loaded from two cascading files so the
// report can attribute each key to its contributing file.
type whyCascadeCLI struct {
	noopRunner

	Config conga.ConfigFile                      `help:"Config file" multi:"true"`
	Tags   conga.OptionalFlag[map[string]string] `help:"Tags"        safe:"true"`
}

// TestWhy_CascadingMapOriginsPerKey pins that a map flag merged across
// cascading files reports each key's contributing file in the source cell.
//
//nolint:paralleltest // Swaps the process working directory via t.Chdir.
func TestWhy_CascadingMapOriginsPerKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	sysPath := "sys.json"
	localPath := "local.json"

	assert.NoError(t, os.WriteFile(sysPath, []byte(`{"tags": {"region": "eu-west-1", "shared": "sys"}}`), 0o600))
	assert.NoError(t, os.WriteFile(localPath, []byte(`{"tags": {"shared": "local", "team": "core"}}`), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whyCascadeCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := normalizeWhyGolden(buffer.String(), tmpDir, "<TMP>")
	assertGolden(t, "testdata/why/map-cascade-merge.golden", goldenWhyMapCascadeMerge, output)
}

// whyCascadeScalarCLI exercises scalar flags loaded from two cascading files
// so the report can attribute each value to its contributing file.
type whyCascadeScalarCLI struct {
	noopRunner

	Config conga.ConfigFile           `help:"Config file" multi:"true"`
	Region conga.OptionalFlag[string] `help:"Region"      safe:"true"`
	Team   conga.OptionalFlag[string] `help:"Team"        safe:"true"`
}

// TestWhy_CascadingScalarOriginsPerFile pins that a config-sourced scalar
// reports the contributing file in the source cell: the nearest file for a
// key set by several files, and the lower file for a key it alone sets.
//
//nolint:paralleltest // Swaps the process working directory via t.Chdir.
func TestWhy_CascadingScalarOriginsPerFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	sysPath := "sys.json"
	localPath := "local.json"

	assert.NoError(t, os.WriteFile(sysPath, []byte(`{"region": "eu-west-1", "team": "core"}`), 0o600))
	assert.NoError(t, os.WriteFile(localPath, []byte(`{"region": "us-east-1"}`), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whyCascadeScalarCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localPath, sysPath), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := normalizeWhyGolden(buffer.String(), tmpDir, "<TMP>")
	assertGolden(t, "testdata/why/scalar-cascade-merge.golden", goldenWhyScalarCascadeMerge, output)
}

type whyMapEntryWidthCLI struct {
	noopRunner

	Secrets conga.OptionalFlag[map[string]string] `help:"Secret headers"`
}

func TestWhy_MapEntryWidensValueColumn(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyMapEntryWidthCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--secrets", "token=abc", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/why/map-entry-width.golden", goldenWhyMapEntryWidth, buffer.String())
}

type whyNoCLIFeatures struct {
	Alpha bool `help:"Alpha gate"`
}

type whyNoCLIFeaturesCLI struct {
	noopRunner

	Features conga.Features[whyNoCLIFeatures] `config:"experiments"`
}

func TestWhy_NoCLIFeaturesExcludedFromFlagTable(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyNoCLIFeaturesCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableFeatures().
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	flagSection, featuresSection, found := strings.Cut(buffer.String(), "Features:")
	assert.True(t, found)
	assert.NotContains(t, flagSection, "--features")
	assert.Contains(t, featuresSection, "alpha")
}

// TestWhy_FieldEnvNoDetail pins that a per-feature env variable renders the
// same provenance row as a combined-map env key: source "env" with no detail,
// so the variable name never appears in the report.
func TestWhy_FieldEnvNoDetail(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[featureFieldEnvCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableFeatures().
		EnableWhy().
		Args([]string{"--why"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)

	_, featuresSection, found := strings.Cut(buffer.String(), "Features:")
	assert.True(t, found)

	featuresTable, _, _ := strings.Cut(featuresSection, "\n\n")

	assert.Contains(t, featuresTable, "gentle-force")

	for line := range strings.SplitSeq(featuresTable, "\n") {
		if strings.Contains(line, "gentle-force") {
			assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "env"))
			assert.NotContains(t, line, "CONGA_TEST_TASK_X_")
		}
	}
}

// TestWhy_FieldEnvOverridesConfigClearsConfigDetail pins that an env override
// of a config-sourced key leaves no stale configuration detail: the row
// reports the env tier only.
func TestWhy_FieldEnvOverridesConfigClearsConfigDetail(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "experiments.yml")
	err := os.WriteFile(configPath, fixtureFeatureGentleForce, 0o600)
	assert.NoError(t, err)

	var buffer bytes.Buffer

	app := conga.New[featureFieldEnvCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableFeatures().
		EnableWhy().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		ConfigOptions(conga.ConfigExplicit(configPath)).
		Args([]string{"--why"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "false")

	err = app.Run()

	assert.NoError(t, err)

	_, featuresSection, found := strings.Cut(buffer.String(), "Features:")
	assert.True(t, found)

	featuresTable, _, _ := strings.Cut(featuresSection, "\n\n")

	assert.Contains(t, featuresTable, "gentle-force")
	assert.Contains(t, featuresTable, "false")
	assert.Contains(t, featuresTable, "env")
	assert.NotContains(t, featuresTable, "config")
	assert.NotContains(t, featuresTable, configPath)
	assert.NotContains(t, featuresTable, "CONGA_TEST_TASK_X_")
}

func TestWhy_UnsetRows(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NotContains(t, output, "(unset)")
	assert.Contains(t, output, "CONFIG PATH")

	// Root invocations render no Global Flags section.
	assert.NotContains(t, output, "Global Flags:")

	portRow := ""

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--port") {
			portRow = line

			break
		}
	}

	assert.True(t, portRow != "", "report must contain the --port row")
	assert.Contains(t, portRow, " unset")
}

func TestWhy_DefaultValuesDisplayed(t *testing.T) {
	t.Parallel()

	type defaultCLI struct {
		Level conga.OptionalFlag[string] `default:"super-secret-level" help:"Log level"`
	}

	var buffer bytes.Buffer

	app := conga.New[defaultCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assertGolden(t, "testdata/why/defaults.golden", goldenWhyDefaults, buffer.String())
}

func TestWhy_BoolAndCounterVisibleByDefault(t *testing.T) {
	t.Parallel()

	type toggleCLI struct {
		Verbose conga.OptionalFlag[bool]   `help:"Verbose output"`
		Loud    conga.Counter              `help:"Verbosity level"`
		Token   conga.OptionalFlag[string] `help:"API token"`
	}

	var buffer bytes.Buffer

	app := conga.New[toggleCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--verbose", "--loud", "--loud", "--token", "s3cr3t-token", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.NotContains(t, output, "s3cr3t-token")

	var verboseRow, loudRow, tokenRow string

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--verbose") {
			verboseRow = line
		}

		if strings.Contains(line, "--loud") {
			loudRow = line
		}

		if strings.Contains(line, "--token") {
			tokenRow = line
		}
	}

	assert.True(t, verboseRow != "", "report must contain the --verbose row")
	assert.True(t, loudRow != "", "report must contain the --loud row")
	assert.True(t, tokenRow != "", "report must contain the --token row")
	assert.Contains(t, verboseRow, "true")
	assert.Contains(t, loudRow, "2")
	assert.Contains(t, tokenRow, "██████")

	// The redacted string row still drives the footer hint.
	assert.Contains(t, output, `values are redacted — tag fields safe:"true" to display`)
}

func TestWhy_BoolSafeFalseOverrideRedacts(t *testing.T) {
	t.Parallel()

	type overrideCLI struct {
		Telemetry conga.OptionalFlag[bool] `help:"Enable telemetry" safe:"false"`
	}

	var buffer bytes.Buffer

	app := conga.New[overrideCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--telemetry", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)

	telemetryRow := ""

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--telemetry") {
			telemetryRow = line

			break
		}
	}

	assert.True(t, telemetryRow != "", "report must contain the --telemetry row")
	assert.Contains(t, telemetryRow, "██████")

	assert.Contains(t, output, `values are redacted — tag fields safe:"true" to display`)
}

func TestWhy_AllVisibleRowsOmitFooter(t *testing.T) {
	t.Parallel()

	type visibleCLI struct {
		Verbose conga.OptionalFlag[bool] `help:"Verbose output"`
		Loud    conga.Counter            `help:"Verbosity level"`
	}

	var buffer bytes.Buffer

	app := conga.New[visibleCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--verbose", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.NotContains(t, output, "██████")
	assert.NotContains(t, output, `values are redacted — tag fields safe:"true" to display`)
}

func TestWhy_NegatedBoolDisplaysFalse(t *testing.T) {
	t.Parallel()

	type negatableCLI struct {
		Color conga.OptionalFlag[bool] `help:"Colorize output" negatable:"true"`
	}

	var buffer bytes.Buffer

	app := conga.New[negatableCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--no-color", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)

	colorRow := ""

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--color") {
			colorRow = line

			break
		}
	}

	assert.True(t, colorRow != "", "report must contain the --color row")
	assert.Contains(t, colorRow, "false")
}

func TestWhy_MapDeterminism(t *testing.T) {
	t.Parallel()

	render := func() string {
		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--headers", "z=1,a=2,m=3", "--why"})

		err := app.Run()

		assert.NoError(t, err)

		return buffer.String()
	}

	assert.Equal(t, render(), render(), "repeated runs must be byte-identical")
}

func TestWhy_HiddenFlagMarked(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--secret", "hidden-value", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NotContains(t, output, "hidden-value")

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--secret") {
			assert.Contains(t, line, "(hidden)")
		}
	}
}

func TestWhy_ScalarCoercionErrorFailsFast(t *testing.T) {
	t.Setenv("CONGA_TEST_WHY_PORT", "not-a-number")

	type envPortCLI struct {
		Port conga.OptionalFlag[int] `env:"CONGA_TEST_WHY_PORT" help:"Listen port"`
	}

	app := conga.New[envPortCLI]().
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid integer value")
}

func TestWhy_MapCoercionErrorFailsFast(t *testing.T) {
	t.Parallel()

	type mapCLI struct {
		Headers conga.OptionalFlag[map[string]string] `help:"Headers" safe:"true" sep:"none"`
	}

	var buffer bytes.Buffer

	app := conga.New[mapCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--headers", "bad1", "--headers", "bad2", "--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid key-value pair "bad1"`)
}

func TestWhy_ConfigFileReadFailureError(t *testing.T) {
	t.Setenv("CONGA_TEST_WHY_CONFIG", "missing-why-config.json")

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		RegisterDecoder(".json", json.Decode).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `cannot read config file "missing-why-config.json"`)
}

func TestWhy_DecoderlessEnvConfigVarError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv("CONGA_TEST_DECODERLESS_CONFIG", missingPath)

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		EnvPrefix("CONGA_TEST_DECODERLESS_").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CONGA_TEST_DECODERLESS_CONFIG")
	assert.Contains(t, err.Error(), "requires a registered config decoder")
}

func TestWhy_ConfigFileRowsRender(t *testing.T) {
	t.Parallel()

	t.Run("single config file row", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "app.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{"port": 9000}`), 0o600))

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--config-file", configPath, "--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "--config-file")
		assert.Contains(t, output, configPath)
		assert.NotContains(t, output, "error:")
	})

	t.Run("multi config files row", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		firstPath := filepath.Join(tmpDir, "a.json")
		assert.NoError(t, os.WriteFile(firstPath, []byte(`{}`), 0o600))

		secondPath := filepath.Join(tmpDir, "b.json")
		assert.NoError(t, os.WriteFile(secondPath, []byte(`{}`), 0o600))

		type multiCLI struct {
			Configs conga.ConfigFile `help:"Config paths" multi:"true" name:"config-files"`
		}

		var buffer bytes.Buffer

		app := conga.New[multiCLI]().
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--config-files", firstPath, "--config-files", secondPath, "--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "--config-files")
		assert.Contains(t, output, "(2 files)")
		assert.Contains(t, output, firstPath)
		assert.Contains(t, output, secondPath)
		assert.NotContains(t, output, "error:")
	})
}

func TestWhy_SliceValuesRenderedVertically(t *testing.T) {
	t.Parallel()

	type sliceCLI struct {
		noopRunner

		Tags  conga.OptionalFlag[[]string] `help:"Tags"  safe:"true"`
		Flags conga.OptionalFlag[[]bool]   `help:"Flags" safe:"true"`
	}

	var buffer bytes.Buffer

	app := conga.New[sliceCLI]().
		Name("whyapp").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--tags", "go,cli,conga", "--flags", "true,false", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.Contains(t, output, "--tags")
	assert.Contains(t, output, "(3 values)")
	assert.Contains(t, output, "├─ go")
	assert.Contains(t, output, "├─ cli")
	assert.Contains(t, output, "└─ conga")
	assert.Contains(t, output, "--flags")
	assert.Contains(t, output, "(2 values)")
	assert.Contains(t, output, "├─ true")
	assert.Contains(t, output, "└─ false")
}

func TestWhy_ConfigPathsChecked_NothingFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	localDir := filepath.Join(tmpDir, "local")
	userDir := filepath.Join(tmpDir, "user")

	assert.NoError(t, os.MkdirAll(localDir, 0o700))
	assert.NoError(t, os.MkdirAll(userDir, 0o700))

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(localDir, userDir), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := normalizeWhyGolden(buffer.String(), tmpDir, "<TMP>")
	assertGolden(t, "testdata/why/config-paths-missing.golden", goldenWhyConfigPathsMissing, output)
}

// TestWhy_ConfigPathsChecked_LoadedTiers pins that a loaded search tier
// reports the file that resolved it. Relative tier paths keep the reported
// origins stable across machines.
//
//nolint:paralleltest // Swaps the process working directory via t.Chdir.
func TestWhy_ConfigPathsChecked_LoadedTiers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Stem tier: "<stem>.json" is probed when the stem path does not exist.
	stemPath := filepath.Join("user", "whyapp")
	assert.NoError(t, os.MkdirAll(filepath.Dir(stemPath), 0o700))

	assert.NoError(t, os.WriteFile(stemPath+".json", []byte(`{"port": 9000}`), 0o600))

	// Directory tier: candidate config names are probed inside the directory.
	dirTier := "system"
	assert.NoError(t, os.MkdirAll(dirTier, 0o700))

	assert.NoError(t, os.WriteFile(filepath.Join(dirTier, "config.json"), []byte(`{}`), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(dirTier, stemPath), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := normalizeWhyGolden(buffer.String(), tmpDir, "<TMP>")
	assertGolden(t, "testdata/why/config-paths-loaded.golden", goldenWhyConfigPathsLoaded, output)
}

func TestWhy_ConfigPathsChecked_ExplicitPath(t *testing.T) {
	t.Parallel()

	t.Run("loaded explicit path", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "app.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{}`), 0o600))

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyExplicitConfigCLI]().
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--config-file", configPath, "--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Config paths")
		assert.Contains(t, output, configPath)
		assert.Contains(t, output, "loaded")
		assert.NotContains(t, output, "loaded: "+configPath)
	})

	t.Run("failing explicit path returns error", func(t *testing.T) {
		t.Parallel()

		missingPath := filepath.Join(t.TempDir(), "missing.json")

		var buffer bytes.Buffer

		app := conga.New[whyExplicitConfigCLI]().
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--config-file", missingPath, "--why"})

		err := app.Run()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read config file")
		assert.Contains(t, err.Error(), missingPath)
	})
}

func TestWhy_ConfigFileRowUnset(t *testing.T) {
	t.Parallel()

	missingTier := filepath.Join(t.TempDir(), "missing")

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(missingTier)).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	configRow := ""

	for line := range strings.SplitSeq(buffer.String(), "\n") {
		if strings.Contains(line, "--config-file") {
			configRow = line

			break
		}
	}

	assert.True(t, configRow != "", "report must contain the config-file row")
	assert.Contains(t, configRow, "unset")
	assert.NotContains(t, configRow, "default")
}

func TestWhy_ConfigPathsChecked_OmittedWithoutConfigResolution(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.NotContains(t, buffer.String(), "Config paths")
}

func TestWhy_BuiltinPriority(t *testing.T) {
	t.Parallel()

	t.Run("--why --help renders help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			Name("whyapp").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--why", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Usage:\n  whyapp")
	})

	t.Run("--why --version renders version", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			Name("whyapp").
			Version("1.0.0").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--why", "--version"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "whyapp has version 1.0.0")
	})

	t.Run("--why --completion zsh renders the why report", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--why", "--completion", "zsh"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Value provenance")
	})

	t.Run("--completion --why renders bare completion help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--completion", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Generate shell completion script")
	})

	t.Run("--completion zsh --help --why renders completion help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--completion", "zsh", "--help", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Generate the shell completion script for the zsh shell.")
	})
}

// TestWhy_Collision pins the collision guard on the builtin --why name (every
// command offers the builtin, so a user flag named "why" collides anywhere).
func TestWhy_Collision(t *testing.T) {
	t.Parallel()

	type collidingCLI struct {
		Why conga.OptionalFlag[bool] `help:"Explain reasoning"`
	}

	app := conga.New[collidingCLI]().EnableWhy()

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), "built-in why (enabled by app.EnableWhy())")
}

func TestWhy_Collision_SubcommandUserFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[whySubCollisionCLI]().EnableWhy()

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), "built-in why (enabled by app.EnableWhy())")
}

func TestWhy_UserFlagWithoutOptIn(t *testing.T) {
	t.Setenv("CONGA_TEST_TAKEOVER_TOKEN", "plain")

	var buffer bytes.Buffer

	cli := new(whyTakeoverCLI)
	app := conga.New(cli).
		Stdout(&buffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Why.Get())
	assert.Equal(t, "plain", cli.Token.Get())
	assert.True(t, cli.ran)
	assert.NotContains(t, buffer.String(), "Value provenance")
}

func TestWhy_UserFlagWithoutOptIn_Subcommand(t *testing.T) {
	t.Parallel()

	cli := new(whySubTakeoverCLI)
	app := conga.New(cli).
		Args([]string{"sub", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Sub.Get().Why.Get())
	assert.True(t, cli.Sub.Get().ran)
}

func TestWhy_OnSubcommands(t *testing.T) {
	t.Parallel()

	t.Run("subcommand help lists --why", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyCLI]().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"sub", "--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "--why")
	})

	t.Run("--why on a subcommand renders the invocation-wide report", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
		)

		cli := new(whyLeafCLI)
		app := conga.New(cli).
			Name("whyapp").
			EnvPrefix("CONGA_TEST_WHYAPP_").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--port", "8080", "sub", "--depth", "3", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assertGolden(t, "testdata/why/subcommand-report.golden", goldenWhySubcommandReport, buffer.String())
		assert.False(t, cli.Sub.Get().ran)
	})

	t.Run("subcommand --why --help renders help", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			Name("whyapp").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"sub", "--why", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Usage:\n  whyapp sub")
	})

	t.Run("root help lists --why before the closing --help line", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyCLI]().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "--why")
		assert.True(t, strings.Index(output, "--why") < strings.Index(output, "--help"), "--why must render before --help")
	})
}

// TestWhy_SynthesizedCompletionSubcommand pins that the auto-mounted
// completion subcommand inherits the root's built-in --why visibility: the
// flag is offered only when the app enabled app.EnableWhy(), matching the
// binder, help renderer, and completion engine.
func TestWhy_SynthesizedCompletionSubcommand(t *testing.T) {
	t.Parallel()

	t.Run("help omits --why when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			EnableCompletion().
			Stdout(&buffer).
			Args([]string{"completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("--why is a usage error when why is disabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[whyCLI]().
			EnableCompletion().
			Args([]string{"completion", "--why"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
	})

	t.Run("help lists --why when why is enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			EnableCompletion().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--why")
	})

	t.Run("--why renders the report when why is enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			EnableCompletion().
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"completion", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Value provenance")
	})
}

// TestWhy_FlatCompletionHelpVisibility pins that the ephemeral flat-CLI
// completion help screens inherit the root's built-in flag visibility: --why
// appears only when app.EnableWhy() was called, and --help only when the help
// flag was not disabled.
func TestWhy_FlatCompletionHelpVisibility(t *testing.T) {
	t.Parallel()

	t.Run("bare completion omits --why when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("completion help omits --why when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("per-shell completion help omits --why when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("bare completion lists --why when why is enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--why")
	})

	t.Run("bare completion omits --help when the help flag is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			DisableHelpFlag().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "Show help for command")
	})
}

func TestWhy_NoPositionalArgRows(t *testing.T) {
	t.Parallel()

	type argCLI struct {
		Target conga.RequiredArg[string] `help:"Deployment target"`
		Port   conga.OptionalFlag[int]   `help:"Listen port"`
	}

	var buffer bytes.Buffer

	app := conga.New[argCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"prod", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.Contains(t, output, "--port")
	assert.NotContains(t, output, "Deployment target")
}

func TestWhy_ConfigPathsChecked_DefaultXDGTier(t *testing.T) {
	t.Run("missing app directory reports absence", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		tierPath := filepath.Join(tmpDir, "whyapp")

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whySearchPathCLI]().
			Name("whyapp").
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Config paths")
		assert.Contains(t, output, tierPath)

		inSection := false
		found := false

		for line := range strings.SplitSeq(output, "\n") {
			switch {
			case strings.HasPrefix(line, "Config paths"):
				inSection = true
			case strings.HasPrefix(line, "Create a config file"):
				inSection = false
			case inSection && strings.HasPrefix(strings.TrimSpace(line), tierPath):
				found = true

				assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "directory does not exist"), "missing default tier must report the absence: %q", line)
			}
		}

		assert.True(t, found, "config paths section must list the missing default tier")
	})

	t.Run("loaded app directory reports the file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		appDir := filepath.Join(tmpDir, "whyapp")
		assert.NoError(t, os.MkdirAll(appDir, 0o700))

		configPath := filepath.Join(appDir, "config.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{"port": 9000}`), 0o600))

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whySearchPathCLI]().
			Name("whyapp").
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "loaded: "+configPath)
		assert.Contains(t, output, "config")
	})
}

func TestWhy_ConfigDecoders_HeaderSuffixListsRegisteredDecoders(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(filepath.Join(t.TempDir(), "conf")), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Config decoders: .json")
	assert.Contains(t, output, "Config paths")
}

func TestWhy_ConfigDecoders_OmittedWithoutRegisteredDecoders(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		ConfigOptions(conga.ConfigPaths(filepath.Join(t.TempDir(), "conf")), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Config paths")
	assert.NotContains(t, output, "Config decoders:")
}

func TestWhy_ProbedFiles_MissingTierProbesAppStem(t *testing.T) {
	t.Parallel()

	missingTier := filepath.Join(t.TempDir(), "conf", "whyapp")

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(missingTier), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, missingTier+"   directory does not exist\n    looked for: "+missingTier+".json")
}

func TestWhy_ProbedFiles_DirectoryTierProbesConfigAndAppStems(t *testing.T) {
	t.Parallel()

	dirTier := filepath.Join(t.TempDir(), "conf")
	assert.NoError(t, os.MkdirAll(dirTier, 0o700))

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(dirTier), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, dirTier+"   no config file")
	assert.Contains(t, output, "    looked for: "+filepath.Join(dirTier, "config.json"))
	assert.Contains(t, output, "    looked for: "+filepath.Join(dirTier, "config.toml"))
	assert.Contains(t, output, "    looked for: "+filepath.Join(dirTier, "whyapp.json"))
	assert.Contains(t, output, "    looked for: "+filepath.Join(dirTier, "whyapp.toml"))
}

func TestWhy_ProbedFiles_LoadedTierOmitted(t *testing.T) {
	t.Parallel()

	dirTier := filepath.Join(t.TempDir(), "conf")
	assert.NoError(t, os.MkdirAll(dirTier, 0o700))

	configPath := filepath.Join(dirTier, "config.json")
	assert.NoError(t, os.WriteFile(configPath, []byte(`{}`), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(dirTier), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "loaded: "+configPath)
	assert.NotContains(t, output, "looked for:")
	assert.NotContains(t, output, "Create a config file")
}

func TestWhy_ConfigExamples_SuggestedWhenNothingLoaded(t *testing.T) {
	t.Parallel()

	missingTier := filepath.Join(t.TempDir(), "conf", "whyapp")

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigPaths(missingTier), conga.ConfigCascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Create a config file at one of:")
	assert.Contains(t, output, "  "+filepath.Join(missingTier, "config.json"))
	assert.Contains(t, output, "  "+filepath.Join(missingTier, "whyapp.json"))
	assert.Contains(t, output, "  "+missingTier+".json")
}

func TestWhy_ProbedFiles_ExplicitPathOmitted(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "app.json")
	assert.NoError(t, os.WriteFile(configPath, []byte("{}"), 0o600))

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--config-file", configPath, "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, configPath)
	assert.NotContains(t, output, "looked for:")
	assert.NotContains(t, output, "Create a config file")
}

type hoistConfigCLI struct {
	Config conga.ConfigFile
}

type hoistSub struct{}

func (s *hoistSub) Run(_ context.Context, _ *conga.Session) error { return nil }

type hoistSubCLI struct {
	Config conga.ConfigFile
	Sub    conga.Command[hoistSub] `summary:"Subcommand"`
}

type hoistHelpCLI struct {
	Config conga.ConfigFile
	Help   conga.HelpCommand `summary:"Show help for any command"`
}

type hoistDeprecatedCLI struct {
	OldFlag conga.OptionalFlag[bool] `deprecated:"use --new-flag"         help:"Legacy toggle"`
	Help    conga.HelpCommand        `summary:"Show help for any command"`
}

func (c *hoistDeprecatedCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type whyRunner struct{ ran bool }

func (r *whyRunner) Run(_ context.Context, _ *conga.Session) error {
	r.ran = true

	return nil
}

type whyCLI struct {
	Port    conga.OptionalFlag[int]               `help:"Listen port"`
	Token   conga.OptionalFlag[string]            `env:"CONGA_TEST_WHY_TOKEN"  help:"API token"`
	Shown   conga.OptionalFlag[string]            `help:"Shown value"          safe:"true"`
	Region  conga.OptionalFlag[string]            `config:"region"             help:"Deployment region"`
	Headers conga.OptionalFlag[map[string]string] `help:"Headers"              safe:"true"`
	Secret  conga.OptionalFlag[string]            `help:"Hidden knob"          hidden:"true"`
	Config  conga.ConfigFile                      `env:"CONGA_TEST_WHY_CONFIG" help:"Config path"`
	Sub     conga.Command[whyRunner]              `summary:"Subcommand"`
}

type whySearchPathCLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port"`
}

type whyExplicitConfigCLI struct {
	ConfigFile conga.ConfigFile        `help:"Explicit configuration file"`
	Port       conga.OptionalFlag[int] `default:"8080"                     help:"Listen port"`
}

type whyFlatCLI struct {
	Port conga.OptionalFlag[int] `help:"Listen port"`
}

type whySubCollisionRunner struct {
	Why conga.OptionalFlag[bool] `help:"Explain reasoning"`
}

func (r *whySubCollisionRunner) Run(_ context.Context, _ *conga.Session) error { return nil }

type whySubCollisionCLI struct {
	Sub conga.Command[whySubCollisionRunner] `summary:"Subcommand"`
}

type whySubTakeoverRunner struct {
	Why conga.OptionalFlag[bool] `help:"Explain reasoning"`
	ran bool
}

func (r *whySubTakeoverRunner) Run(_ context.Context, _ *conga.Session) error {
	r.ran = true

	return nil
}

type whySubTakeoverCLI struct {
	Sub conga.Command[whySubTakeoverRunner] `summary:"Subcommand"`
}

type whyTakeoverCLI struct {
	Why   conga.OptionalFlag[bool]   `help:"Explain reasoning"`
	Token conga.OptionalFlag[string] `env:"CONGA_TEST_TAKEOVER_TOKEN" help:"Token" safe:"true"`

	ran bool
}

func (c *whyTakeoverCLI) Run(_ context.Context, _ *conga.Session) error {
	c.ran = true

	return nil
}

type whyLeafRunner struct {
	Depth  conga.OptionalFlag[int]    `help:"Nesting depth"`
	Region conga.OptionalFlag[string] `config:"region"      help:"Deployment region"`
	ran    bool
}

func (r *whyLeafRunner) Run(_ context.Context, _ *conga.Session) error {
	r.ran = true

	return nil
}

type whyLeafCLI struct {
	Port    conga.OptionalFlag[int]      `help:"Listen port"`
	CLIOnly conga.OptionalFlag[string]   `config:"-"           help:"CLI-only override"`
	Sub     conga.Command[whyLeafRunner] `summary:"Subcommand"`
}

// TestBuiltinWhy_HelpAdvertisesFormats pins the conditional --why help row:
// the optional value slot and format list appear only when the app enabled
// WhyOptionJSON, mirroring the completion candidates.
func TestBuiltinWhy_HelpAdvertisesFormats(t *testing.T) {
	t.Parallel()

	t.Run("text-only app renders a plain toggle", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			Name("cloudctl").
			EnableWhy().
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "    --why")
		assert.NotContains(t, buffer.String(), "--why[=FORMAT]")
		assert.NotContains(t, buffer.String(), "[enum: text")
	})

	t.Run("json app renders the value slot and format list", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCLI]().
			Name("cloudctl").
			EnableWhy(conga.WhyOptionJSON()).
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "    --why[=FORMAT]")
		assert.Contains(t, buffer.String(), "[enum: text, json]")
	})
}

func TestBuiltinWhy_JSONFormat(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--shown", "visible-value", "--why=json"})

	err := app.Run()

	assert.NoError(t, err)

	type mapEntryPayload struct {
		Key      string `json:"key"`
		Value    any    `json:"value"`
		Source   string `json:"source"`
		Detail   string `json:"detail"`
		Redacted bool   `json:"redacted"`
	}

	type flagPayload struct {
		Name       string            `json:"name"`
		Type       string            `json:"type"`
		Value      any               `json:"value"`
		Source     string            `json:"source"`
		Detail     string            `json:"detail"`
		Env        string            `json:"env"`
		ConfigPath string            `json:"configPath"`
		Hidden     bool              `json:"hidden"`
		Redacted   bool              `json:"redacted"`
		Entries    []mapEntryPayload `json:"entries"`
	}

	type featurePayload struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Source string `json:"source"`
		Detail string `json:"detail"`
	}

	type configPathPayload struct {
		Path        string   `json:"path"`
		Status      string   `json:"status"`
		ProbedFiles []string `json:"probedFiles"`
	}

	type configPayload struct {
		Decoders []string            `json:"decoders"`
		Paths    []configPathPayload `json:"paths"`
		Examples []string            `json:"examples"`
	}

	type whyJSONPayload struct {
		Schema           string           `json:"$schema"`
		SchemaVersion    int              `json:"schemaVersion"`
		Command          string           `json:"command"`
		Version          string           `json:"version"`
		Timestamp        string           `json:"timestamp"`
		WorkingDirectory string           `json:"workingDirectory"`
		Flags            []flagPayload    `json:"flags"`
		GlobalFlags      []flagPayload    `json:"globalFlags"`
		Features         []featurePayload `json:"features"`
		Config           configPayload    `json:"config"`
		Redacted         bool             `json:"redacted"`
	}

	var payload whyJSONPayload

	err = stdjson.Unmarshal(buffer.Bytes(), &payload)

	assert.NoError(t, err)

	assert.Equal(t, "https://conga.sh/schemas/why/v1.json", payload.Schema)
	assert.Equal(t, 1, payload.SchemaVersion)
	assert.Equal(t, "cloudctl", payload.Command)
	assert.True(t, payload.Timestamp != "")
	assert.True(t, payload.WorkingDirectory != "")
	assert.True(t, filepath.IsAbs(payload.WorkingDirectory))

	var foundPort, foundShown, foundSecret bool

	for _, flag := range payload.Flags {
		switch flag.Name {
		case "port":
			foundPort = true

			assert.True(t, flag.Redacted)
			assert.Zero(t, flag.Value)
			assert.Equal(t, "cli", flag.Source)
			assert.Equal(t, "int", flag.Type)
		case "shown":
			foundShown = true

			assert.False(t, flag.Redacted)
			assert.Equal(t, "visible-value", flag.Value)
			assert.Equal(t, "cli", flag.Source)
			assert.Equal(t, "string", flag.Type)
		case "secret":
			foundSecret = true

			assert.True(t, flag.Hidden)
		}
	}

	assert.True(t, foundPort)
	assert.True(t, foundShown)
	assert.True(t, foundSecret)
}

func TestBuiltinWhy_JSONMapAndFeatures(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyMergeCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		EnableFeatures().
		Stdout(&buffer).
		Args([]string{"--headers", "region=eu-central-1,env=prod", "--secrets", "api=xyz", "--feature-gates", "alpha=true", "--why=json"})

	err := app.Run()

	assert.NoError(t, err)

	var raw map[string]any

	err = stdjson.Unmarshal(buffer.Bytes(), &raw)

	assert.NoError(t, err)

	schemaVersion, versionOk := raw["schemaVersion"].(float64)
	assert.True(t, versionOk)
	assert.Equal(t, float64(1), schemaVersion)
	assert.Equal(t, "https://conga.sh/schemas/why/v1.json", raw["$schema"])

	flags, ok := raw["flags"].([]any)
	assert.True(t, ok)

	var foundHeaders, foundSecrets bool

	for _, flagRaw := range flags {
		flagMap, mapOk := flagRaw.(map[string]any)
		if !mapOk {
			continue
		}

		switch flagMap["name"] {
		case "headers":
			foundHeaders = true

			assert.Equal(t, "map", flagMap["type"])
			entries, entriesOk := flagMap["entries"].([]any)
			assert.True(t, entriesOk)
			assert.Equal(t, 2, len(entries))
		case "secrets":
			foundSecrets = true

			flagRedacted, flagRedactedOk := flagMap["redacted"].(bool)
			assert.True(t, flagRedactedOk)
			assert.True(t, flagRedacted)
			assert.Zero(t, flagMap["value"])
			entries, entriesOk := flagMap["entries"].([]any)
			assert.True(t, entriesOk)
			assert.Equal(t, 1, len(entries))
			entryMap, entryOk := entries[0].(map[string]any)
			assert.True(t, entryOk)
			assert.Equal(t, "api", entryMap["key"])
			entryRedacted, entryRedactedOk := entryMap["redacted"].(bool)
			assert.True(t, entryRedactedOk)
			assert.True(t, entryRedacted)
			assert.Zero(t, entryMap["value"])
		}
	}

	assert.True(t, foundHeaders)
	assert.True(t, foundSecrets)

	features, ok := raw["features"].([]any)
	assert.True(t, ok)
	assert.True(t, len(features) >= 2)
}

// TestBuiltinWhy_FieldEnvJSONSource pins that the JSON why report attributes
// a field-env feature key to the env tier.
func TestBuiltinWhy_FieldEnvJSONSource(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[featureFieldEnvCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		EnableFeatures().
		Stdout(&buffer).
		Args([]string{"--why=json"})

	t.Setenv("CONGA_TEST_TASK_X_GENTLE_FORCE", "1")

	err := app.Run()

	assert.NoError(t, err)

	var raw map[string]any

	err = stdjson.Unmarshal(buffer.Bytes(), &raw)

	assert.NoError(t, err)

	features, ok := raw["features"].([]any)
	assert.True(t, ok)

	found := false

	for _, featureRaw := range features {
		featureMap, mapOk := featureRaw.(map[string]any)
		if !mapOk || featureMap["name"] != "gentle-force" {
			continue
		}

		found = true

		assert.Equal(t, "env", featureMap["source"])
	}

	assert.True(t, found)
}

func TestBuiltinWhy_TextFormatExplicit(t *testing.T) {
	t.Parallel()

	var bareBuffer, explicitBuffer bytes.Buffer

	app1 := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&bareBuffer).
		Args([]string{"--port", "8080", "--why"})

	err1 := app1.Run()

	assert.NoError(t, err1)

	app2 := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&explicitBuffer).
		Args([]string{"--port", "8080", "--why=text"})

	err2 := app2.Run()

	assert.NoError(t, err2)

	assert.Equal(t, bareBuffer.String(), explicitBuffer.String())
}

func TestBuiltinWhy_InvalidFormat(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--why=banana"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid value "banana" (must be one of: text, json) for --why`)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))

	var usage conga.UsageError

	assert.True(t, errors.As(err, &usage))
	assert.Equal(t, "cloudctl", usage.CommandPath())
}

func TestBuiltinWhy_FalseFlagRejected(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"sub", "--why=false"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid value "false" (must be one of: text, json) for --why`)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

func TestBuiltinWhy_LastValueWinsOverInvalid(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--why=banana", "--why=json"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"command":"cloudctl"`)
}

func TestBuiltinWhy_EmptyAttachedValue(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--why="})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Value provenance")
}

func TestBuiltinWhy_RepeatedFormatLastValueWins(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--why=json", "--why=text"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Value provenance")
}

func TestBuiltinWhy_HelpWinsOverInvalidFormat(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--help", "--why=banana"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinWhy_VersionWinsOverInvalidFormat(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		Version("1.2.3").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--version", "--why=banana"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "cloudctl has version 1.2.3")
}

func TestBuiltinWhy_TrueFlag(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--why=true"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Value provenance")
}

func TestBuiltinWhy_NonFormatBoolsRejected(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{"True", "0", "1", "FALSE", "TRUE"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			var buffer bytes.Buffer

			app := conga.New[whyCLI]().
				Name("cloudctl").
				EnableWhy().
				Stdout(&buffer).
				Args([]string{"--why=" + invalid})

			err := app.Run()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), `invalid value "`+invalid+`" (must be one of: text, json) for --why`)
			assert.IsError(t, err, conga.ErrValidation)
			assert.True(t, conga.IsUsageError(err))
		})
	}
}

func TestBuiltinWhy_JSONWithoutOptionRejected(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("cloudctl").
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--port", "8080", "--why=json"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid value "json" for --why: JSON output format is not enabled on this application (enable with app.EnableWhy(conga.WhyOptionJSON()))`)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

type whySessionCLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port" safe:"true"`

	preText string
	preJSON string
	runText string
	runJSON string
}

func (c *whySessionCLI) PreRun(_ context.Context, session *conga.Session) error {
	c.preText = session.Why()
	c.preJSON = session.WhyJSON()

	return nil
}

func (c *whySessionCLI) Run(_ context.Context, session *conga.Session) error {
	c.runText = session.Why()
	c.runJSON = session.WhyJSON()

	return nil
}

func TestSessionWhy_InsideLifecycleHooks(t *testing.T) {
	t.Parallel()

	var (
		buffer bytes.Buffer
		cli    whySessionCLI
	)

	app := conga.New(&cli).
		Name("whyapp").
		Stdout(&buffer).
		EnableWhy(conga.WhyOptionJSON()).
		Args([]string{"--port", "9000"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, cli.preText, "Value provenance")
	assert.Contains(t, cli.preText, "--port")
	assert.Contains(t, cli.preText, "9000")
	assert.Contains(t, cli.preJSON, `"schemaVersion":1`)
	assert.Contains(t, cli.preJSON, `"name":"port"`)
	assert.Contains(t, cli.runText, "Value provenance")
	assert.Contains(t, cli.runText, "--port")
	assert.Contains(t, cli.runText, "9000")
	assert.Contains(t, cli.runJSON, `"schemaVersion":1`)
}

type whyDisabledSessionCLI struct {
	text string
	json string
}

func (c *whyDisabledSessionCLI) Run(_ context.Context, session *conga.Session) error {
	c.text = session.Why()
	c.json = session.WhyJSON()

	return nil
}

func TestSessionWhy_Disabled(t *testing.T) {
	t.Parallel()

	t.Run("EnableWhy absent", func(t *testing.T) {
		t.Parallel()

		var cli whyDisabledSessionCLI

		app := conga.New(&cli).
			Name("whyapp").
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "", cli.text)
		assert.Equal(t, "", cli.json)
	})

	t.Run("WhyOptionJSON option absent", func(t *testing.T) {
		t.Parallel()

		var cli whyDisabledSessionCLI

		app := conga.New(&cli).
			Name("whyapp").
			EnableWhy().
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, cli.text, "Value provenance")
		assert.Equal(t, "", cli.json)
	})
}

type whyJSONTypesCLI struct {
	Int32    conga.OptionalFlag[int32]          `help:"Int32 value"    safe:"true"`
	Uint16   conga.OptionalFlag[uint16]         `help:"Uint16 value"   safe:"true"`
	Float32  conga.OptionalFlag[float32]        `help:"Float32 value"  safe:"true"`
	Duration conga.OptionalFlag[time.Duration]  `help:"Duration value" safe:"true"`
	Size     conga.OptionalFlag[conga.ByteSize] `help:"Size value"     safe:"true"`
	Flags    conga.OptionalFlag[[]bool]         `help:"Bool slice"     safe:"true"`
}

func (c *whyJSONTypesCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func jsonValue[T any](t *testing.T, value any) T {
	t.Helper()

	typed, ok := value.(T)
	assert.True(t, ok)

	return typed
}

func TestBuiltinWhy_JSONSerializesScalarAndDomainTypes(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyJSONTypesCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		Stdout(&buffer).
		Args([]string{
			"--int32", "42",
			"--uint16", "65000",
			"--float32", "3.14",
			"--duration", "5s",
			"--size", "1KiB",
			"--flags", "true,false",
			"--why=json",
		})

	err := app.Run()

	assert.NoError(t, err)

	var raw map[string]any

	assert.NoError(t, stdjson.Unmarshal(buffer.Bytes(), &raw))

	flags, ok := raw["flags"].([]any)
	assert.True(t, ok)

	byName := make(map[string]map[string]any, len(flags))

	for _, flagRaw := range flags {
		flagMap, mapOk := flagRaw.(map[string]any)
		if !mapOk {
			continue
		}

		name, nameOk := flagMap["name"].(string)
		if nameOk {
			byName[name] = flagMap
		}
	}

	assert.Equal(t, "int", byName["int32"]["type"])
	assert.Equal(t, float64(42), jsonValue[float64](t, byName["int32"]["value"]))
	assert.Equal(t, "uint", byName["uint16"]["type"])
	assert.Equal(t, float64(65000), jsonValue[float64](t, byName["uint16"]["value"]))
	assert.Equal(t, "float", byName["float32"]["type"])
	assert.Equal(t, float64(3.14), jsonValue[float64](t, byName["float32"]["value"]))
	assert.Equal(t, "string", byName["duration"]["type"])
	assert.Equal(t, "5s", byName["duration"]["value"])
	assert.Equal(t, "string", byName["size"]["type"])
	assert.Equal(t, "1KiB", byName["size"]["value"])
	assert.Equal(t, "bool[]", byName["flags"]["type"])
	assert.Equal(t, []any{true, false}, jsonValue[[]any](t, byName["flags"]["value"]))
}

func TestBuiltinWhy_JSONFeatureFlagValue(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyMergeCLI]().
		Name("cloudctl").
		EnableWhy(conga.WhyOptionJSON()).
		EnableFeatures().
		Stdout(&buffer).
		Args([]string{"--feature-gates", "alpha=true", "--why=json"})

	err := app.Run()

	assert.NoError(t, err)

	var raw map[string]any

	assert.NoError(t, stdjson.Unmarshal(buffer.Bytes(), &raw))

	flags, ok := raw["flags"].([]any)
	assert.True(t, ok)

	var featureFlag map[string]any

	for _, flagRaw := range flags {
		flagMap, mapOk := flagRaw.(map[string]any)
		if mapOk && flagMap["name"] == "feature-gates" {
			featureFlag = flagMap
		}
	}

	assert.True(t, featureFlag != nil)
	assert.Equal(t, map[string]any{"alpha": "true"}, jsonValue[map[string]any](t, featureFlag["value"]))
}

type whyVersionMetaCLI struct {
	Port    conga.OptionalFlag[int] `default:"8080" help:"Listen port" safe:"true"`
	Version conga.VersionCommand
}

type whyCompletionMetaCLI struct {
	Port       conga.OptionalFlag[int] `default:"7070" help:"Listen port" safe:"true"`
	Completion conga.CompletionCommand
}

func TestWhy_MetaCommandsBindFlags(t *testing.T) {
	t.Parallel()

	t.Run("version command with engine-owned flag", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyVersionMetaCLI]().
			Name("whyapp").
			Version("1.0.0").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"version", "--json", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Value provenance")
		assert.Contains(t, buffer.String(), "--port")
		assert.Contains(t, buffer.String(), "8080")
		assert.Contains(t, buffer.String(), "default")
	})

	t.Run("completion command", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyCompletionMetaCLI]().
			Name("whyapp").
			Stdout(&buffer).
			EnableCompletion().
			EnableWhy().
			Args([]string{"completion", "bash", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Value provenance")
		assert.Contains(t, buffer.String(), "--port")
		assert.Contains(t, buffer.String(), "7070")
		assert.Contains(t, buffer.String(), "default")
	})
}

type whyPointerTextValue struct {
	text string
}

func (v *whyPointerTextValue) UnmarshalText(text []byte) error {
	v.text = string(text)

	return nil
}

func (v *whyPointerTextValue) MarshalText() ([]byte, error) {
	return []byte(v.text), nil
}

type whyPointerTextCLI struct {
	Value conga.OptionalFlag[whyPointerTextValue] `help:"Value" safe:"true"`
}

// whyBootstrapCLI supplies its feature gate document from code through a
// Bootstrapper.
type whyBootstrapCLI struct {
	noopRunner

	Features conga.Features[whyMergeFeatures] `config:"experiments" flag:"feature-gates"`
}

func (c *whyBootstrapCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	document := &conga.ConfigDocument{}
	document.Set("experiments", map[string]any{"alpha": true}, "pipeline.yaml")
	b.SetConfig(document)

	return nil
}

func TestBuiltinWhy_FeatureFlagRowReportsConfigOrigin(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyBootstrapCLI]().
		Name("whyapp").
		EnableFeatures().
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	var featureFlagLine string

	for line := range strings.SplitSeq(buffer.String(), "\n") {
		if strings.Contains(line, "--feature-gates") {
			featureFlagLine = line
		}
	}

	assert.Contains(t, featureFlagLine, "config (pipeline.yaml)")
}

func TestBuiltinWhy_JSONPointerReceiverTextMarshaler(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyPointerTextCLI]().
		Name("whyapp").
		EnableWhy(conga.WhyOptionJSON()).
		Stdout(&buffer).
		Args([]string{"--value", "hello", "--why=json"})

	err := app.Run()

	assert.NoError(t, err)

	var payload struct {
		Flags []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Value any    `json:"value"`
		} `json:"flags"`
	}

	err = stdjson.Unmarshal(buffer.Bytes(), &payload)

	assert.NoError(t, err)

	var found bool

	for _, flag := range payload.Flags {
		if flag.Name != "value" {
			continue
		}

		found = true

		assert.Equal(t, "string", flag.Type)
		assert.Equal(t, "hello", flag.Value)
	}

	assert.True(t, found)
}
