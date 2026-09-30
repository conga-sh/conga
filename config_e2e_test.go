package conga_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

//go:embed testdata/fixtures/subcommand_namespace.yaml
var fixtureSubcommandNamespace []byte

func TestConfigFile_DefaultFlagNameAndAccessors(t *testing.T) {
	t.Parallel()

	type AppCLI struct {
		noopRunner

		ConfigFile conga.ConfigFile
		Port       conga.OptionalFlag[int]    `default:"8000"      name:"port"`
		Host       conga.OptionalFlag[string] `default:"localhost" name:"host"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"port": 9090, "host": "10.0.0.1"}`), 0o600)
	assert.NoError(t, err)

	cli := new(AppCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"--config-file", cfgPath})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.ConfigFile.IsSet())
	assert.True(t, cli.ConfigFile.Changed())
	assert.Equal(t, cfgPath, cli.ConfigFile.Path())
	assert.Equal(t, conga.SourceCLI, cli.ConfigFile.Source())
	assert.Equal(t, 9090, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, "10.0.0.1", cli.Host.Get())
}

func TestConfigFile_CustomNameAndShortFlag(t *testing.T) {
	t.Parallel()

	type CustomCLI struct {
		noopRunner

		Config conga.ConfigFile        `help:"Custom config flag" name:"config" short:"c"`
		Port   conga.OptionalFlag[int] `default:"8000"            name:"port"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "custom.yaml")
	err := os.WriteFile(cfgPath, []byte("port: 7070\n"), 0o600)
	assert.NoError(t, err)

	cli := new(CustomCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.RawDecoder).
		Args([]string{"-c", cfgPath})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, cfgPath, cli.Config.Path())
	assert.Equal(t, conga.SourceCLI, cli.Config.Source())
	assert.Equal(t, 7070, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestConfigFile_FourTierPrecedence(t *testing.T) {
	type PrecedenceCLI struct {
		noopRunner

		Config conga.ConfigFile        `name:"config" short:"c"`
		Port   conga.OptionalFlag[int] `config:"port" default:"8000" env:"PRECEDENCE_PORT" name:"port"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "precedence.toml")
	err := os.WriteFile(cfgPath, []byte("port = 8080\n"), 0o600)
	assert.NoError(t, err)

	// Scenario A: Config file overrides default
	cliA := new(PrecedenceCLI)
	appA := conga.New(cliA).
		RegisterDecoder(".toml", toml.RawDecoder).
		Args([]string{"-c", cfgPath})
	err = appA.Run()
	assert.NoError(t, err)
	assert.Equal(t, 8080, cliA.Port.Get())
	assert.Equal(t, conga.SourceConfig, cliA.Port.Source())

	// Scenario B: Env var overrides config file
	t.Setenv("PRECEDENCE_PORT", "8081")

	cliB := new(PrecedenceCLI)
	appB := conga.New(cliB).
		RegisterDecoder(".toml", toml.RawDecoder).
		Args([]string{"-c", cfgPath})
	err = appB.Run()
	assert.NoError(t, err)
	assert.Equal(t, 8081, cliB.Port.Get())
	assert.Equal(t, conga.SourceEnv, cliB.Port.Source())

	// Scenario C: CLI flag overrides env var and config file
	cliC := new(PrecedenceCLI)
	appC := conga.New(cliC).
		RegisterDecoder(".toml", toml.RawDecoder).
		Args([]string{"-c", cfgPath, "--port", "8082"})
	err = appC.Run()
	assert.NoError(t, err)
	assert.Equal(t, 8082, cliC.Port.Get())
	assert.Equal(t, conga.SourceCLI, cliC.Port.Source())
}

func TestConfigFile_CollisionResistance(t *testing.T) {
	t.Parallel()

	type CollisionCLI struct {
		noopRunner

		ConfigFile conga.ConfigFile
		Message    conga.OptionalFlag[string] `name:"message"  short:"m"`
		Port       conga.OptionalFlag[int]    `default:"3000"  name:"port"`
		Target     conga.RequiredArg[string]  `name:"<target>"`
		Dash       conga.DashArgs
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "real.json")
	err := os.WriteFile(cfgPath, []byte(`{"port": 4545}`), 0o600)
	assert.NoError(t, err)

	// User passes flag value that literally contains "--config-file" and "--" arguments
	args := []string{
		"--message=--config-file",
		"--config-file", cfgPath,
		"my-target",
		"--",
		"--config-file", "ignored.json",
	}

	cli := new(CollisionCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args(args)

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, cfgPath, cli.ConfigFile.Path())
	assert.Equal(t, 4545, cli.Port.Get())
	assert.Equal(t, "--config-file", cli.Message.Get())
	assert.Equal(t, "my-target", cli.Target.Get())
	assert.Equal(t, []string{"--config-file", "ignored.json"}, cli.Dash.Get())
}

func TestConfigFile_SubcommandNamespaceLoading(t *testing.T) {
	t.Parallel()

	type ServerStartCmd struct {
		noopRunner

		Port conga.OptionalFlag[int]    `default:"8080"      name:"port"`
		Host conga.OptionalFlag[string] `default:"localhost" name:"host"`
	}

	type RootCLI struct {
		ConfigFile conga.ConfigFile
		Verbose    conga.OptionalFlag[bool]      `name:"verbose" short:"v"`
		Server     conga.Command[ServerStartCmd] `name:"server"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "app.yaml")
	err := os.WriteFile(cfgPath, fixtureSubcommandNamespace, 0o600)
	assert.NoError(t, err)

	// Execute subcommand with config file on root: app --config-file app.yaml server --port 1234
	cli := new(RootCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.RawDecoder).
		Args([]string{"--config-file", cfgPath, "server", "--port", "1234"})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.Get())
	assert.Equal(t, conga.SourceConfig, cli.Verbose.Source())
	assert.True(t, cli.Server.IsSelected())
	assert.Equal(t, 1234, cli.Server.Get().Port.Get())
	assert.Equal(t, conga.SourceCLI, cli.Server.Get().Port.Source())
	assert.Equal(t, "0.0.0.0", cli.Server.Get().Host.Get())
	assert.Equal(t, conga.SourceConfig, cli.Server.Get().Host.Source())
}

func TestConfigFile_SchemaValidation_SubcommandRejected(t *testing.T) {
	t.Parallel()

	type SubcommandWithConfig struct {
		Config conga.ConfigFile `name:"config"`
	}

	type InvalidRootCLI struct {
		Subcommand conga.Command[SubcommandWithConfig] `name:"sub"`
	}

	app := conga.New[InvalidRootCLI]().
		Args([]string{"sub"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "struct SubcommandWithConfig field Config: schema error: ConfigFile can only be declared on the root command", err.Error())
}

func TestConfigFile_SchemaValidation_DuplicateRejected(t *testing.T) {
	t.Parallel()

	type DuplicateConfigCLI struct {
		Config1 conga.ConfigFile `name:"config1"`
		Config2 conga.ConfigFile `name:"config2"`
	}

	app := conga.New[DuplicateConfigCLI]().
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, "struct DuplicateConfigCLI field Config2: schema error: duplicate flag: ConfigFile (already declared by config1)", err.Error())
}

func TestConfigFile_SchemaValidation_CascadingRequiresMulti(t *testing.T) {
	t.Parallel()

	type SingleConfigCLI struct {
		noopRunner

		Config conga.ConfigFile `name:"config"`
	}

	app := conga.New[SingleConfigCLI]().
		ConfigOptions(config.Cascading()).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `schema error: Cascading() merges multiple configuration files; the root config-file flag "config" must declare multi:"true" (or drop Cascading() from app.ConfigOptions(...))`, err.Error())
}

func TestConfigFile_SchemaValidation_CascadingWithMultiAccepted(t *testing.T) {
	t.Parallel()

	type MultiConfigCLI struct {
		noopRunner

		Config conga.ConfigFile `multi:"true" name:"config"`
	}

	app := conga.New[MultiConfigCLI]().
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Cascading()).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
}

func TestConfigFile_SchemaValidation_CascadingWithoutFlagAccepted(t *testing.T) {
	t.Parallel()

	type FlaglessCLI struct {
		noopRunner

		Port conga.OptionalFlag[int] `default:"8080"`
	}

	app := conga.New[FlaglessCLI]().
		RegisterDecoder(".json", json.RawDecoder).
		ConfigOptions(config.Cascading()).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
}

func TestConfigFile_SchemaValidation_ReservedPseudoTagsRejected(t *testing.T) {
	t.Parallel()

	t.Run("explicit pseudo-tag", func(t *testing.T) {
		t.Parallel()

		type PseudoTagCLI struct {
			noopRunner

			Config conga.ConfigFile `name:"config" ~explicit:"config.json"`
		}

		app := conga.New[PseudoTagCLI]().
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct PseudoTagCLI field Config: schema error: invalid struct tag: struct tag "~explicit" is reserved for CONGA's app-level configuration settings; declare it via app.ConfigOptions(...) instead`, err.Error())
	})

	t.Run("cascading pseudo-tag on plain flag", func(t *testing.T) {
		t.Parallel()

		type PseudoTagFlagCLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `default:"8080" ~cascading:"true"`
		}

		app := conga.New[PseudoTagFlagCLI]().
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `struct tag "~cascading" is reserved`)
	})
}

func TestConfigFile_CounterFromConfig(t *testing.T) {
	t.Parallel()

	type CounterConfigCLI struct {
		noopRunner

		Config  conga.ConfigFile `name:"config"`
		Verbose conga.Counter    `help:"Verbosity level" short:"v"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"verbose": 3}`), 0o600)
	assert.NoError(t, err)

	cli := new(CounterConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"--config", cfgPath})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.IsSet())
	assert.Equal(t, 3, cli.Verbose.Get())
	assert.Equal(t, conga.SourceConfig, cli.Verbose.Source())
}

func TestConfigFile_CounterCLIOverridesConfig(t *testing.T) {
	t.Parallel()

	type CounterOverrideCLI struct {
		noopRunner

		Config  conga.ConfigFile `name:"config"`
		Verbose conga.Counter    `help:"Verbosity level" short:"v"`
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"verbose": 5}`), 0o600)
	assert.NoError(t, err)

	cli := new(CounterOverrideCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"--config", cfgPath, "-v=2"})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.IsSet())
	assert.Equal(t, 2, cli.Verbose.Get())
	assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
}

func TestConfigFile_DecoderlessHelpWarns(t *testing.T) {
	t.Parallel()

	type DecoderlessHelpCLI struct {
		noopRunner

		ConfigFile conga.ConfigFile `help:"Config file"`
	}

	var buffer bytes.Buffer

	app := conga.New[DecoderlessHelpCLI]().
		Name("myapp").
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--config-file=PATH")
	assert.Contains(t, buffer.String(), "Config file (no configuration decoder registered)")
}

func TestConfigFile_WithDecoderHelpOmitsWarning(t *testing.T) {
	t.Parallel()

	type DecoderHelpCLI struct {
		noopRunner

		ConfigFile conga.ConfigFile `help:"Config file"`
	}

	var buffer bytes.Buffer

	app := conga.New[DecoderHelpCLI]().
		Name("myapp").
		RegisterDecoder(".json", json.RawDecoder).
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--config-file=PATH")
	assert.NotContains(t, buffer.String(), "no configuration decoder registered")
}

func TestConfigFile_DecoderlessExplicitTargetErrors(t *testing.T) {
	t.Parallel()

	type DecoderlessTargetCLI struct {
		noopRunner

		Config conga.ConfigFile `name:"config"`
	}

	cfgPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(cfgPath, []byte(`{}`), 0o600)
	assert.NoError(t, err)

	app := conga.New[DecoderlessTargetCLI]().
		Name("myapp").
		Args([]string{"--config", cfgPath})

	err = app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.Equal(t,
		"config error: flag --config requires a registered config decoder: call App.RegisterDecoder with a format decoder (e.g. decoder/json)",
		err.Error())
}

func TestConfig_EnvConfigVar_RequiresDecoder(t *testing.T) {
	t.Setenv("DECODERLESS_CONFIG", filepath.Join(t.TempDir(), "app.json"))

	type EnvConfigCLI struct {
		noopRunner

		Value conga.OptionalFlag[string] `name:"value"`
	}

	app := conga.New[EnvConfigCLI]().
		Name("decoderless").
		EnvPrefix("DECODERLESS_").
		Args([]string{})

	err := app.Run()

	assert.IsError(t, err, conga.ErrConfig)
	assert.Equal(t,
		"config error: environment variable DECODERLESS_CONFIG requires a registered config decoder: call App.RegisterDecoder with a format decoder (e.g. decoder/json)",
		err.Error())
}

type DefaultTagConfigFileCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile `default:"/etc/decoderless/app.json"`
}

func TestConfigFile_DefaultTagRejected(t *testing.T) {
	t.Parallel()

	app := conga.New[DefaultTagConfigFileCLI]().
		Name("defaulttag").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `struct DefaultTagConfigFileCLI field ConfigFile: schema error: invalid struct tag: tag "default" is not supported on config-file flags; declare search locations with app.ConfigOptions(config.Paths(...)) or config.Explicit(...), or show an example value with placeholder:"..."`, err.Error())
}
