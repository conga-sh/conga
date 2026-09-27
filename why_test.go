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
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
)

// --- Part 1: builtin hoist (help/version never fail on resolution errors) ---

type hoistConfigCLI struct {
	Config conga.ConfigFile `help:"Path to config file"`
}

type hoistSub struct{}

func (s *hoistSub) Run(_ context.Context, _ *conga.Session) error { return nil }

type hoistSubCLI struct {
	Config conga.ConfigFile        `help:"Path to config file"`
	Sub    conga.Command[hoistSub] `name:"sub"                 summary:"Subcommand"`
}

type hoistHelpCLI struct {
	Config conga.ConfigFile  `help:"Path to config file"`
	Help   conga.HelpCommand `summary:"Show help for any command"`
}

type hoistDeprecatedCLI struct {
	OldFlag conga.OptionalFlag[bool] `deprecated:"use --new-flag"         help:"Legacy toggle"`
	Help    conga.HelpCommand        `summary:"Show help for any command"`
}

func (c *hoistDeprecatedCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

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
	assert.Contains(t, buffer.String(), "Usage:")
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
	assert.Contains(t, buffer.String(), "Usage:")
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
	assert.Contains(t, buffer.String(), "Usage:")
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

// --- Part 3: --why builtin ---

type whyRunner struct{ ran bool }

func (r *whyRunner) Run(_ context.Context, _ *conga.Session) error {
	r.ran = true

	return nil
}

type whyCLI struct {
	Port    conga.OptionalFlag[int]               `help:"Listen port"`
	Token   conga.OptionalFlag[string]            `env:"WHY_TOKEN"    help:"API token"`
	Shown   conga.OptionalFlag[string]            `help:"Shown value" safe:"true"`
	Region  conga.OptionalFlag[string]            `config:"region"    help:"Deployment region"`
	Headers conga.OptionalFlag[map[string]string] `help:"Headers"     safe:"true"`
	Secret  conga.OptionalFlag[string]            `help:"Hidden knob" hidden:"true"`
	Config  conga.ConfigFile                      `env:"WHY_CONFIG"   help:"Config path"`
	Sub     conga.Command[whyRunner]              `name:"sub"         summary:"Subcommand"`
}

func TestWhy_ReportRedactionAndBadges(t *testing.T) {
	t.Setenv("WHY_TOKEN", "s3cr3t-token")

	configPath := filepath.Join(t.TempDir(), "why-config.json")
	assert.NoError(t, os.WriteFile(configPath, []byte(`{"region": "eu-west-1"}`), 0o600))
	t.Setenv("WHY_CONFIG", configPath)

	var buffer bytes.Buffer

	cli := new(whyCLI)
	app := conga.New(cli).
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--port", "8080", "--headers", "z=1,a=2", "--shown", "visible-value", "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)

	// Redaction default: every non-safe value renders the fixed-width block,
	// regardless of source tier or value length.
	assert.Contains(t, output, "██████")
	assert.NotContains(t, output, "8080")
	assert.NotContains(t, output, "s3cr3t-token")

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--port") {
			assert.True(t, strings.Contains(line, "██████"), "value column must be redacted: %q", line)
			assert.True(t, strings.Contains(line, " cli "), "CLI source word: %q", line)
		}

		if strings.Contains(line, "--token") {
			assert.True(t, strings.Contains(line, " env "), "env source word: %q", line)
			assert.True(t, strings.Contains(line, "WHY_TOKEN"), "env column: %q", line)
		}

		if strings.Contains(line, "--region") {
			assert.True(t, strings.Contains(line, " config "), "config source word: %q", line)
			assert.True(t, strings.Contains(line, " region"), "config path column: %q", line)
		}

		if strings.Contains(line, "--shown") {
			assert.True(t, strings.Contains(line, "visible-value"), "safe field renders the real value: %q", line)
		}

		if strings.Contains(line, "--config") {
			assert.True(t, strings.Contains(line, configPath), "config-file path is always displayed: %q", line)
		}
	}

	// Safe opt-in: the map renders with sorted keys.
	assert.Contains(t, output, "a=2, z=1")

	// Footer hint advertises the safe tag.
	assert.Contains(t, output, `values are redacted — tag fields safe:"true" to display`)

	// The command never ran: --why short-circuits execution.
	assert.False(t, cli.Sub.Get().ran)
}

func TestWhy_UnsetRows(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("whyapp").
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

	output := buffer.String()

	assert.Contains(t, output, "super-secret-level")

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "--level") {
			assert.Contains(t, line, " default")
			assert.Contains(t, line, "super-secret-level")
		}
	}
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
		Name("whyapp").
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
		Name("whyapp").
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
		Name("whyapp").
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
		Name("whyapp").
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
			Name("whyapp").
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
		Name("whyapp").
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
		Name("whyapp").
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "error: invalid integer value")
}

func TestWhy_ConfigFileReadFailureDiagnosticRow(t *testing.T) {
	t.Setenv("WHY_CONFIG", "missing-why-config.json")

	var buffer bytes.Buffer

	app := conga.New[whyCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
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
		Name("decoderless").
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
			Name("whyapp").
			RegisterDecoder(".json", json.RawDecoder).
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
			Name("whyapp").
			RegisterDecoder(".json", json.RawDecoder).
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

type whySearchPathCLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port"`
}

type whyExplicitConfigCLI struct {
	ConfigFile conga.ConfigFile        `help:"Explicit configuration file"`
	Port       conga.OptionalFlag[int] `default:"8080"                     help:"Listen port"`
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
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(localDir, userDir), config.Cascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Config paths checked")
	assert.Contains(t, output, localDir)
	assert.Contains(t, output, userDir)

	inSection := false

	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "Config paths checked"):
			inSection = true
		case strings.HasPrefix(line, "Create a configuration file"):
			inSection = false
		case inSection && strings.HasPrefix(strings.TrimSpace(line), localDir):
			assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "no config file"), "empty tier must report the absence: %q", line)
		}
	}
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
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(dirTier, stemPath), config.Cascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "loaded: "+dirFile)
	assert.Contains(t, output, "loaded: "+stemFile)
	assert.NotContains(t, output, "no config file")
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
			Name("whyapp").
			RegisterDecoder(".json", json.RawDecoder).
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
			Name("whyapp").
			RegisterDecoder(".json", json.RawDecoder).
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

func TestWhy_ConfigPathsChecked_OmittedWithoutConfigResolution(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
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
		assert.Contains(t, buffer.String(), "Usage:")
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
			Name("whyapp").
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
			Name("whyapp").
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
			Name("whyapp").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"--completion", "zsh", "--help", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Generate the shell completion script for the zsh shell.")
	})
}

type whyFlatCLI struct {
	Port conga.OptionalFlag[int] `help:"Listen port"`
}

// TestWhy_Collision pins the collision guard on the builtin --why name (every
// command offers the builtin, so a user flag named "why" collides anywhere).
func TestWhy_Collision(t *testing.T) {
	t.Parallel()

	type collidingCLI struct {
		Why conga.OptionalFlag[bool] `help:"Explain reasoning"`
	}

	app := conga.New[collidingCLI]().Name("whyapp").EnableWhy()

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Contains(t, err.Error(), "built-in why (enabled by app.EnableWhy())")
}

type whySubCollisionRunner struct {
	Why conga.OptionalFlag[bool] `help:"Explain reasoning"`
}

func (r *whySubCollisionRunner) Run(_ context.Context, _ *conga.Session) error { return nil }

type whySubCollisionCLI struct {
	Sub conga.Command[whySubCollisionRunner] `name:"sub" summary:"Subcommand"`
}

func TestWhy_Collision_SubcommandUserFlag(t *testing.T) {
	t.Parallel()

	app := conga.New[whySubCollisionCLI]().Name("whyapp").EnableWhy()

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
		Name("whyapp").
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
		Name("whyapp").
		Args([]string{"sub", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Sub.Get().Why.Get())
	assert.True(t, cli.Sub.Get().ran)
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
	Sub conga.Command[whySubTakeoverRunner] `name:"sub" summary:"Subcommand"`
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
	CLIOnly conga.OptionalFlag[string]   `config:"-"         help:"CLI-only override"`
	Sub     conga.Command[whyLeafRunner] `name:"sub"         summary:"Subcommand"`
}

func TestWhy_OnSubcommands(t *testing.T) {
	t.Parallel()

	t.Run("subcommand help lists --why", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyCLI]().
			Name("whyapp").
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

		output := buffer.String()

		assert.NoError(t, err)
		assert.Contains(t, output, `Value provenance for "whyapp sub":`)

		// Column headers render in both tables, in tier-preference order.
		assert.Contains(t, output, "FLAG")
		assert.Contains(t, output, "ENV")
		assert.Contains(t, output, "CONFIG PATH")

		// Leaf rows precede the Global Flags section; ancestor rows follow it.
		assert.True(t, strings.Index(output, "--depth") < strings.Index(output, "Global Flags:"), "leaf rows must precede Global Flags")
		assert.True(t, strings.Index(output, "Global Flags:") < strings.Index(output, "--port"), "ancestor rows must follow Global Flags")

		for line := range strings.SplitSeq(output, "\n") {
			switch {
			case strings.Contains(line, "--depth"):
				assert.True(t, strings.Contains(line, " cli "), "source word: %q", line)
				assert.True(t, strings.Contains(line, "WHYAPP_DEPTH"), "derived env var: %q", line)
				assert.True(t, strings.Contains(line, "sub.depth"), "canonical config path: %q", line)
			case strings.Contains(line, "--region"):
				assert.True(t, strings.Contains(line, "sub.region"), "explicit config tag namespaced under the command path: %q", line)
			case strings.Contains(line, "--cli-only"):
				assert.True(t, strings.Contains(line, " -"), "config-excluded flag renders a dash: %q", line)
				assert.True(t, strings.Contains(line, "WHYAPP_CLI_ONLY"), "derived env var for config-excluded flag: %q", line)
			case strings.Contains(line, "--port"):
				assert.True(t, strings.Contains(line, "WHYAPP_PORT"), "derived env var: %q", line)
				assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "port"), "root-owned flag keeps its bare config path under Global Flags: %q", line)
				assert.False(t, strings.Contains(line, "sub.port"), "root-owned flag must not inherit the active subcommand path: %q", line)
			}
		}

		assert.Contains(t, output, "██████")
		assert.NotContains(t, output, "8080")
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
		assert.Contains(t, buffer.String(), "Usage:")
	})

	t.Run("root help lists --why before the closing --help line", func(t *testing.T) {
		t.Parallel()

		var (
			buffer bytes.Buffer
		)

		app := conga.New[whyCLI]().
			Name("whyapp").
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
		Name("whyapp").
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
			RegisterDecoder(".json", json.RawDecoder).
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
			case strings.HasPrefix(line, "Create a configuration file"):
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
			RegisterDecoder(".json", json.RawDecoder).
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
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(filepath.Join(t.TempDir(), "conf")), config.Cascading()).
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
		Name("whyapp").
		ConfigOptions(config.Paths(filepath.Join(t.TempDir(), "conf")), config.Cascading()).
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
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(missingTier), config.Cascading()).
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
		RegisterDecoder(".toml", toml.RawDecoder).
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(dirTier), config.Cascading()).
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
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(dirTier), config.Cascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "loaded: "+configPath)
	assert.NotContains(t, output, "looked for:")
	assert.NotContains(t, output, "Create a configuration file")
}

func TestWhy_ConfigExamples_SuggestedWhenNothingLoaded(t *testing.T) {
	t.Parallel()

	missingTier := filepath.Join(t.TempDir(), "conf", "whyapp")

	var buffer bytes.Buffer

	app := conga.New[whySearchPathCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Paths(missingTier), config.Cascading()).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Create a configuration file at one of:")
	assert.Contains(t, output, "  "+filepath.Join(missingTier, "config.json"))
	assert.Contains(t, output, "  "+filepath.Join(missingTier, "whyapp.json"))
	assert.Contains(t, output, "  "+missingTier+".json")
}

func TestWhy_ProbedFiles_ExplicitPathOmitted(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.json")

	var buffer bytes.Buffer

	app := conga.New[whyExplicitConfigCLI]().
		Name("whyapp").
		RegisterDecoder(".json", json.RawDecoder).
		Stdout(&buffer).
		EnableWhy().
		Args([]string{"--config-file", missingPath, "--why"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, missingPath)
	assert.NotContains(t, output, "looked for:")
	assert.NotContains(t, output, "Create a configuration file")
}
