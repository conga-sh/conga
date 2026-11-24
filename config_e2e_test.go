package conga_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
	"github.com/conga-sh/conga/internal/errlines"
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

	cfgPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"port": 9090, "host": "10.0.0.1"}`), 0o600)
	assert.NoError(t, err)

	cli := new(AppCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
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

	cfgPath := filepath.Join(t.TempDir(), "custom.yaml")
	err := os.WriteFile(cfgPath, []byte("port: 7070\n"), 0o600)
	assert.NoError(t, err)

	cli := new(CustomCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
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
		Port   conga.OptionalFlag[int] `config:"port" default:"8000" env:"CONGA_TEST_PRECEDENCE_PORT" name:"port"`
	}

	cfgPath := filepath.Join(t.TempDir(), "precedence.toml")
	err := os.WriteFile(cfgPath, []byte("port = 8080\n"), 0o600)
	assert.NoError(t, err)

	// Scenario A: Config file overrides default
	cliA := new(PrecedenceCLI)
	appA := conga.New(cliA).
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{"-c", cfgPath})
	err = appA.Run()
	assert.NoError(t, err)
	assert.Equal(t, 8080, cliA.Port.Get())
	assert.Equal(t, conga.SourceConfig, cliA.Port.Source())

	// Scenario B: Env var overrides config file
	t.Setenv("CONGA_TEST_PRECEDENCE_PORT", "8081")

	cliB := new(PrecedenceCLI)
	appB := conga.New(cliB).
		RegisterDecoder(".toml", toml.Decode).
		Args([]string{"-c", cfgPath})
	err = appB.Run()
	assert.NoError(t, err)
	assert.Equal(t, 8081, cliB.Port.Get())
	assert.Equal(t, conga.SourceEnv, cliB.Port.Source())

	// Scenario C: CLI flag overrides env var and config file
	cliC := new(PrecedenceCLI)
	appC := conga.New(cliC).
		RegisterDecoder(".toml", toml.Decode).
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

	cfgPath := filepath.Join(t.TempDir(), "real.json")
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
		RegisterDecoder(".json", json.Decode).
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

	cfgPath := filepath.Join(t.TempDir(), "app.yaml")
	err := os.WriteFile(cfgPath, fixtureSubcommandNamespace, 0o600)
	assert.NoError(t, err)

	// Execute subcommand with config file on root: app --config-file app.yaml server --port 1234
	cli := new(RootCLI)
	app := conga.New(cli).
		RegisterDecoder(".yaml", yaml.Decode).
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
	assert.Equal(t, "struct SubcommandWithConfig field Config: ConfigFile can only be declared on the root command", err.Error())
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
	assert.Equal(t, "struct DuplicateConfigCLI field Config2: duplicate ConfigFile field", err.Error())
	assert.Equal(t, "already declared by field Config1", errlines.Note(err))
}

func TestConfigFile_SchemaValidation_CascadingRequiresMulti(t *testing.T) {
	t.Parallel()

	type SingleConfigCLI struct {
		noopRunner

		Config conga.ConfigFile `name:"config"`
	}

	app := conga.New[SingleConfigCLI]().
		ConfigOptions(conga.ConfigCascading()).
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, `the root config-file flag "config" must declare multi:"true"`, err.Error())
	assert.Equal(t, `conga.ConfigCascading() merges multiple config files`, errlines.Note(err))
	assert.Equal(t, `add multi:"true" or drop conga.ConfigCascading() from app.ConfigOptions(...)`, errlines.Hint(err))
}

func TestConfigFile_SchemaValidation_CascadingWithMultiAccepted(t *testing.T) {
	t.Parallel()

	type MultiConfigCLI struct {
		noopRunner

		Config conga.ConfigFile `multi:"true" name:"config"`
	}

	app := conga.New[MultiConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigCascading()).
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
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigCascading()).
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
		assert.Equal(t, `struct PseudoTagCLI field Config: struct tag "~explicit" is reserved for CONGA's app-level configuration settings`, err.Error())
		assert.Equal(t, `declare it via app.ConfigOptions(...) instead`, errlines.Hint(err))
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

	cfgPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"verbose": 3}`), 0o600)
	assert.NoError(t, err)

	cli := new(CounterConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
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

	cfgPath := filepath.Join(t.TempDir(), "app.json")
	err := os.WriteFile(cfgPath, []byte(`{"verbose": 5}`), 0o600)
	assert.NoError(t, err)

	cli := new(CounterOverrideCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		Args([]string{"--config", cfgPath, "-vv"})

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
		RegisterDecoder(".json", json.Decode).
		Args([]string{"--help"}).
		Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--config-file=PATH")
	assert.Contains(t, buffer.String(), "Config file [decoders: .json]")
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
		Args([]string{"--config", cfgPath})

	err = app.Run()

	assertNotUsageError(t, err)
	assert.Equal(t, "flag --config requires a registered config decoder", err.Error())
	assert.Equal(t, "call App.RegisterDecoder with a format decoder (e.g. decoder/json)", errlines.Hint(err))
}

func TestConfig_EnvConfigVar_IgnoredWithoutConfigSupport(t *testing.T) {
	t.Setenv("CONGA_TEST_DECODERLESS_CONFIG", filepath.Join(t.TempDir(), "app.json"))

	type EnvConfigCLI struct {
		noopRunner

		Value conga.OptionalFlag[string] `name:"value"`
	}

	cli := new(EnvConfigCLI)
	app := conga.New(cli).
		EnvPrefix("CONGA_TEST_DECODERLESS_").
		Args([]string{"--value", "x"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "x", cli.Value.Get())
}

func TestConfig_EnvConfigVar_FlagBindsItWithoutConfigSupport(t *testing.T) {
	t.Setenv("CONGA_TEST_NOCFG_CONFIG", "profile-a")

	type ConfigNameCLI struct {
		noopRunner

		Config conga.OptionalFlag[string] `help:"Profile name"`
	}

	cli := new(ConfigNameCLI)
	app := conga.New(cli).
		EnvPrefix("CONGA_TEST_NOCFG_").
		Args(nil)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "profile-a", cli.Config.Get())
	assert.Equal(t, conga.SourceEnv, cli.Config.Source())
}

func TestConfig_EnvConfigVar_ConfigFileWithoutDecoderStillRequiresDecoder(t *testing.T) {
	t.Setenv("CONGA_TEST_CFGNODEC_CONFIG", filepath.Join(t.TempDir(), "app.json"))

	type ConfigFileOnlyCLI struct {
		noopRunner

		Config conga.ConfigFile `help:"Config file"`
	}

	app := conga.New[ConfigFileOnlyCLI]().
		EnvPrefix("CONGA_TEST_CFGNODEC_").
		Args(nil)

	err := app.Run()

	assertNotUsageError(t, err)
	assert.Equal(t, "environment variable CONGA_TEST_CFGNODEC_CONFIG requires a registered config decoder", err.Error())
	assert.Equal(t, "call App.RegisterDecoder with a format decoder (e.g. decoder/json)", errlines.Hint(err))
}

func TestConfig_EnvConfigVar_ConfigFileEnvExcluded(t *testing.T) {
	t.Setenv("CONGA_TEST_CFGEXCL_CONFIG", filepath.Join(t.TempDir(), "missing.json"))

	type ExcludedConfigCLI struct {
		noopRunner

		Config conga.ConfigFile `env:"-" help:"Config file"`
	}

	app := conga.New[ExcludedConfigCLI]().
		EnvPrefix("CONGA_TEST_CFGEXCL_").
		RegisterDecoder(".json", json.Decode).
		Args(nil)

	err := app.Run()

	assert.NoError(t, err)
}

func TestConfig_EnvConfigVar_ConfigFileEnvTagReplacesPrefixVariable(t *testing.T) {
	configPath := writeConfigFile(t, `{"value": "from-file"}`)
	t.Setenv("CONGA_TEST_CFGTAG_CFG", configPath)
	t.Setenv("CONGA_TEST_CFGTAG_CONFIG", "profile-b")

	type TaggedConfigCLI struct {
		noopRunner

		ConfigFile conga.ConfigFile           `env:"CONGA_TEST_CFGTAG_CFG" help:"Config file"`
		Config     conga.OptionalFlag[string] `help:"Profile name"`
		Value      conga.OptionalFlag[string] `help:"Value"`
	}

	cli := new(TaggedConfigCLI)
	app := conga.New(cli).
		EnvPrefix("CONGA_TEST_CFGTAG_").
		RegisterDecoder(".json", json.Decode).
		Args(nil)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "from-file", cli.Value.Get())
	assert.Equal(t, "profile-b", cli.Config.Get())
}

type DefaultTagConfigFileCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile `default:"/etc/decoderless/app.json"`
}

func TestConfigFile_DefaultTagRejected(t *testing.T) {
	t.Parallel()

	app := conga.New[DefaultTagConfigFileCLI]().
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, `struct DefaultTagConfigFileCLI field ConfigFile: tag "default" is not supported on config-file flags`, err.Error())
	assert.Equal(t, `declare search locations with app.ConfigOptions(conga.ConfigPaths(...)) or conga.ConfigExplicit(...), or show an example value with placeholder:"..."`, errlines.Hint(err))
}

type configEnvDerivedCLI struct {
	noopRunner

	Config conga.OptionalFlag[string] `help:"Profile name"`
}

type configEnvSubcommandLeaf struct {
	noopRunner

	Name conga.OptionalFlag[string] `env:"PC_CONFIG" help:"Name"`
}

type configEnvSubcommandCLI struct {
	Deploy conga.Command[configEnvSubcommandLeaf]
}

type configEnvConfigFileCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile           `help:"Config file"`
	Config     conga.OptionalFlag[string] `help:"Profile name"`
}

type configEnvFeatureSet struct {
	Config bool `help:"Config feature"`
}

type configEnvFeatureCLI struct {
	noopRunner

	Features conga.Features[configEnvFeatureSet] `envprefix:"PC_" flag:"feature-gates"`
}

func TestSchema_ConfigEnvCollision(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		run      func() error
		expected string
		note     string
		hint     string
	}{
		{
			name: "derived flag name",
			run: func() error {
				return conga.New[configEnvDerivedCLI]().EnvPrefix("PC").RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvDerivedCLI field Config: environment variable "PC_CONFIG" is bound by both the config file variable and flag --config`,
			hint:     `give the flag another env tag or exclude it with env:"-"`,
		},
		{
			name: "explicit env tag on a subcommand flag",
			run: func() error {
				return conga.New[configEnvSubcommandCLI]().EnvPrefix("PC").RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvSubcommandLeaf field Name: environment variable "PC_CONFIG" is bound by both the config file variable and flag --name`,
			hint:     `give the flag another env tag or exclude it with env:"-"`,
		},
		{
			name: "flag colliding with a ConfigFile",
			run: func() error {
				return conga.New[configEnvConfigFileCLI]().EnvPrefix("PC").RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvConfigFileCLI field Config: environment variable "PC_CONFIG" is bound by both flag --config-file and flag --config`,
			hint:     `give the flag another env tag or exclude it with env:"-"`,
		},
		{
			name: "feature variable",
			run: func() error {
				return conga.New[configEnvFeatureCLI]().EnvPrefix("PC").EnableFeatures().RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvFeatureCLI field Features: environment variable "PC_CONFIG" is bound by both the config file variable and feature "config"`,
			hint:     `give the feature another env tag or exclude it with env:"-"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
			assert.Equal(t, testCase.note, errlines.Note(err))
			assert.Equal(t, testCase.hint, errlines.Hint(err))
		})
	}
}

type configEnvMapNoFlagCLI struct {
	noopRunner

	Features conga.Features[configEnvFeatureSet] `env:"PC_CONFIG"`
}

type configEnvMapFlagCLI struct {
	noopRunner

	Features conga.Features[configEnvFeatureSet] `env:"PC_CONFIG" flag:"feature-gates"`
}

type configEnvConfigFileFeatureCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile                    `help:"Config file"`
	Features   conga.Features[configEnvFeatureSet] `envprefix:"PC_"    flag:"feature-gates"`
}

type configEnvConfigFileNoDecoderCLI struct {
	noopRunner

	ConfigFile conga.ConfigFile           `help:"Config file"`
	Config     conga.OptionalFlag[string] `help:"Profile name"`
}

func TestSchema_ConfigEnvCollisionFeaturesAndConfigFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		run      func() error
		expected string
		note     string
		hint     string
	}{
		{
			name: "Features container without a flag",
			run: func() error {
				return conga.New[configEnvMapNoFlagCLI]().EnvPrefix("PC").EnableFeatures().RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvMapNoFlagCLI field Features: environment variable "PC_CONFIG" is bound by both the config file variable and the Features container`,
			hint:     `give the Features container another env tag`,
		},
		{
			name: "Features container with a flag",
			run: func() error {
				return conga.New[configEnvMapFlagCLI]().EnvPrefix("PC").EnableFeatures().RegisterDecoder(".json", json.Decode).Args(nil).Run()
			},
			expected: `struct configEnvMapFlagCLI field Features: environment variable "PC_CONFIG" is bound by both the config file variable and the feature map flag --feature-gates`,
			hint:     `give the Features container another env tag`,
		},
		{
			name: "feature colliding with a ConfigFile",
			run: func() error {
				return conga.New[configEnvConfigFileFeatureCLI]().EnvPrefix("PC").EnableFeatures().Args(nil).Run()
			},
			expected: `struct configEnvConfigFileFeatureCLI field Features: environment variable "PC_CONFIG" is bound by both flag --config-file and feature "config"`,
			hint:     `give the feature another env tag or exclude it with env:"-"`,
		},
		{
			name: "ConfigFile without a decoder still reserves its variable",
			run: func() error {
				return conga.New[configEnvConfigFileNoDecoderCLI]().EnvPrefix("PC").Args(nil).Run()
			},
			expected: `struct configEnvConfigFileNoDecoderCLI field Config: environment variable "PC_CONFIG" is bound by both flag --config-file and flag --config`,
			hint:     `give the flag another env tag or exclude it with env:"-"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
			assert.Equal(t, testCase.note, errlines.Note(err))
			assert.Equal(t, testCase.hint, errlines.Hint(err))
		})
	}
}

func TestConfig_EnvConfigVar_ConfigOptionsWithoutDecoderLeavesItToFlags(t *testing.T) {
	t.Setenv("CONGA_TEST_OPTSONLY_CONFIG", "profile-c")

	type OptionsOnlyCLI struct {
		noopRunner

		Config conga.OptionalFlag[string] `help:"Profile name"`
	}

	cli := new(OptionsOnlyCLI)
	app := conga.New(cli).
		EnvPrefix("CONGA_TEST_OPTSONLY_").
		ConfigOptions(conga.ConfigKeyCase(conga.SnakeCase)).
		Args(nil)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "profile-c", cli.Config.Get())
	assert.Equal(t, conga.SourceEnv, cli.Config.Source())
}
