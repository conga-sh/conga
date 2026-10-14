package conga_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
)

func TestBuiltinHoist_HelpWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		EnvPrefix("HOIST_").
		Stdout(&buffer).
		Args([]string{"--help"})

	t.Setenv("HOIST_CONFIG", "missing-from-env.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_VersionWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		Name("cloudctl").
		EnvPrefix("HOIST_").
		Version("1.2.3").
		Stdout(&buffer).
		Args([]string{"--version"})

	t.Setenv("HOIST_CONFIG", "missing-from-env.yaml")

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
		EnvPrefix("HOIST_").
		Stdout(&buffer).
		Args([]string{"sub", "--help"})

	t.Setenv("HOIST_CONFIG", "missing-from-env.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl sub")
}

func TestBuiltinHoist_HelpSubcommandWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistHelpCLI]().
		Name("cloudctl").
		EnvPrefix("HOIST_").
		Stdout(&buffer).
		Args([]string{"help"})

	t.Setenv("HOIST_CONFIG", "missing-from-env.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  cloudctl")
}

func TestBuiltinHoist_CompletionScriptWithEnvSourcedMissingConfigPath(t *testing.T) {
	var buffer bytes.Buffer

	app := conga.New[hoistConfigCLI]().
		EnableCompletion().
		Name("cloudctl").
		EnvPrefix("HOIST_").
		Stdout(&buffer).
		Args([]string{"--completion", "zsh"})

	t.Setenv("HOIST_CONFIG", "missing-from-env.yaml")

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "#compdef cloudctl")
}

func TestBuiltinHoist_VersionSkipsConfigResolutionWithoutFeatures(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "app.json")
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

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "app.json")
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

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "app.json")
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
	t.Setenv("WHY_TOKEN", "s3cr3t-token")
	t.Chdir(t.TempDir())

	const configPath = "why-config.json"

	assert.NoError(t, os.WriteFile(configPath, []byte(`{"region": "eu-west-1"}`), 0o600))
	t.Setenv("WHY_CONFIG", configPath)

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

	Features conga.Features[whyMergeFeatures]      `config:"experiments"  env:"WHY_MERGE_FEATURES" flag:"feature-gates"`
	Headers  conga.OptionalFlag[map[string]string] `help:"Headers"        safe:"true"`
	Secrets  conga.OptionalFlag[map[string]string] `help:"Secret headers"`
}

func TestWhy_MapMergePerKeyProvenance(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "why-merge.json")

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

	output := normalizeWhyGolden(buffer.String(), tempDir, "<TMP>")

	assertGolden(t, "testdata/why/map-merge.golden", goldenWhyMapMerge, output)
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

	output := buffer.String()

	flagSection, featuresSection, found := strings.Cut(output, "Features:")
	assert.True(t, found)
	assert.NotContains(t, flagSection, "--features")
	assert.Contains(t, featuresSection, "alpha")
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

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--port") {
			assert.Contains(t, line, " unset")
		}
	}
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

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--verbose") {
			assert.True(t, strings.Contains(line, "true"), "boolean value renders by default: %q", line)
		}

		if strings.Contains(line, "--loud") {
			assert.True(t, strings.Contains(line, "2"), "counter total renders by default: %q", line)
		}

		if strings.Contains(line, "--token") {
			assert.True(t, strings.Contains(line, "██████"), "string value stays redacted: %q", line)
		}
	}

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

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--telemetry") {
			assert.True(t, strings.Contains(line, "██████"), "safe:\"false\" re-redacts a boolean: %q", line)
		}
	}

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

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--color") {
			assert.True(t, strings.Contains(line, "false"), "negated boolean renders its value: %q", line)
		}
	}
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

func TestWhy_TolerantCoercionErrorRow(t *testing.T) {
	t.Setenv("WHY_PORT", "not-a-number")

	type envPortCLI struct {
		Port conga.OptionalFlag[int] `env:"WHY_PORT" help:"Listen port"`
	}

	var buffer bytes.Buffer

	app := conga.New[envPortCLI]().
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "error: invalid integer value")
}

func TestWhy_MapCoercionErrorUsesDisplaySeparator(t *testing.T) {
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

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "bad1 bad2")
	assert.NotContains(t, buffer.String(), "bad1nonebad2")
}

func TestWhy_ConfigFileReadFailureDiagnosticRow(t *testing.T) {
	t.Setenv("WHY_CONFIG", "missing-why-config.json")

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		RegisterDecoder(".json", json.Decode).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, `error: cannot read config file "missing-why-config.json"`)
	assert.Contains(t, output, "missing-why-config.json")
}

func TestWhy_DecoderlessEnvConfigVarDiagnosticRow(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv("DECODERLESS_CONFIG", missingPath)

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		EnvPrefix("DECODERLESS_").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.Contains(t, output, "DECODERLESS_CONFIG")
	assert.Contains(t, output, "requires a registered config decoder")
	assert.Contains(t, output, missingPath)
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
		assert.Contains(t, output, firstPath+","+secondPath)
		assert.NotContains(t, output, "error:")
	})
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

func TestWhy_ConfigPathsChecked_LoadedTiers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Stem tier: "<stem>.json" is probed when the stem path does not exist.
	stemPath := filepath.Join(tmpDir, "user", "whyapp")
	assert.NoError(t, os.MkdirAll(filepath.Dir(stemPath), 0o700))

	stemFile := stemPath + ".json"
	assert.NoError(t, os.WriteFile(stemFile, []byte(`{"port": 9000}`), 0o600))

	// Directory tier: candidate config names are probed inside the directory.
	dirTier := filepath.Join(tmpDir, "system")
	assert.NoError(t, os.MkdirAll(dirTier, 0o700))

	dirFile := filepath.Join(dirTier, "config.json")
	assert.NoError(t, os.WriteFile(dirFile, []byte(`{}`), 0o600))

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
		assert.Contains(t, output, "Config paths checked")
		assert.Contains(t, output, "loaded: "+configPath)
	})

	t.Run("failing explicit path marks the candidate as error", func(t *testing.T) {
		t.Parallel()

		missingPath := filepath.Join(t.TempDir(), "missing.json")

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyExplicitConfigCLI]().
			RegisterDecoder(".json", json.Decode).
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--config-file", missingPath, "--why"})

		err := app.Run()

		assert.NoError(t, err)

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, "Config paths checked")

		statusLine := ""

		inSection := false

		for line := range strings.SplitSeq(output, "\n") {
			switch {
			case strings.HasPrefix(line, "Config paths checked"):
				inSection = true
			case inSection && strings.Contains(line, missingPath):
				statusLine = line
			}
		}

		assert.True(t, statusLine != "", "section must list the failing candidate path")
		assert.True(t, strings.HasSuffix(strings.TrimSpace(statusLine), "error"), "failed candidate must report the error status: %q", statusLine)
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

	output := buffer.String()

	configRow := ""

	for line := range strings.SplitSeq(output, "\n") {
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

	output := buffer.String()

	assert.NoError(t, err)
	assert.NotContains(t, output, "Config paths checked")
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
	t.Setenv("TAKEOVER_TOKEN", "plain")

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
			EnvPrefix("WHYAPP_").
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
		assert.Contains(t, output, "Config paths checked")
		assert.Contains(t, output, tierPath)

		inSection := false

		for line := range strings.SplitSeq(output, "\n") {
			switch {
			case strings.HasPrefix(line, "Config paths checked"):
				inSection = true
			case strings.HasPrefix(line, "Create a config file"):
				inSection = false
			case inSection && strings.HasPrefix(strings.TrimSpace(line), tierPath):
				assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "directory does not exist"), "missing default tier must report the absence: %q", line)
			}
		}
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
	assert.Contains(t, output, "Config paths checked")
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
	assert.Contains(t, output, "Config paths checked")
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

	missingPath := filepath.Join(t.TempDir(), "missing.json")

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--config-file", missingPath, "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, missingPath)
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
	Token   conga.OptionalFlag[string]            `env:"WHY_TOKEN"      help:"API token"`
	Shown   conga.OptionalFlag[string]            `help:"Shown value"   safe:"true"`
	Region  conga.OptionalFlag[string]            `config:"region"      help:"Deployment region"`
	Headers conga.OptionalFlag[map[string]string] `help:"Headers"       safe:"true"`
	Secret  conga.OptionalFlag[string]            `help:"Hidden knob"   hidden:"true"`
	Config  conga.ConfigFile                      `env:"WHY_CONFIG"     help:"Config path"`
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
	Token conga.OptionalFlag[string] `env:"TAKEOVER_TOKEN"     help:"Token" safe:"true"`

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
