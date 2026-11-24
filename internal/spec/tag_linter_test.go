package spec_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/spec"
)

type TypoTagFlagCLI struct {
	Verbose mockFlag[bool] `help:"Enable verbose" shrot:"v"`
}

type TypoTagDefaultCLI struct {
	Port mockFlag[int] `defualt:"8080" help:"Listen port"` //nolint:misspell // intentionally misspelled for test
}

type TypoTagHelpCLI struct {
	Debug mockFlag[bool] `hlepp:"Debug mode"`
}

type ContextInvalidArgShortCLI struct {
	Path mockArg[string] `help:"File path" short:"p"`
}

type ContextInvalidCommandXorCLI struct {
	Subcommand mockCmd[LeafTestCmd] `summary:"Subcommand" xor:"group1"`
}

type ContextInvalidCommandOneOfCLI struct {
	Subcommand mockCmd[LeafTestCmd] `oneof:"group1" summary:"Subcommand"`
}

type ContextInvalidConfigFileOneOfCLI struct {
	Config mockConfigFile `name:"config" oneof:"group1"`
}

type LeafTestCmd struct {
	Data mockFlag[string] `help:"Data payload"`
}

func (l *LeafTestCmd) Run(ctx context.Context) error {
	return nil
}

type ContextInvalidNegatableNonBoolCLI struct {
	Port mockFlag[int] `help:"Port" negatable:"true"`
}

type ContextDashArgsHelpCLI struct {
	Extra mockDashArgs `help:"Trailing args"`
}

type ContextRawArgsHelpCLI struct {
	Raw mockRawArgs `help:"Raw args"`
}

type ContextDashArgsHelpVariableCLI struct {
	Extra mockDashArgs `help:"Trailing args {{enum}}"`
}

type ContextRawArgsHelpEscapeCLI struct {
	Raw mockRawArgs `help:"Raw {{{{args}}"`
}

type ObsoleteCounterTagCLI struct {
	Count mockFlag[int] `counter:"true" help:"Count"`
}

type ConfigFilesWithSepCLI struct {
	Configs mockMultiConfigFile `help:"Config files" multi:"true" sep:";"`
}

type ConfigFileDefaultTagCLI struct {
	Config mockConfigFile `default:"config.yaml" help:"Config file path"`
}

type CounterDefaultTagCLI struct {
	Verbose mockCounter `default:"2" help:"Verbosity level"`
}

type BlankFieldInvalidTagCLI struct {
	_    struct{}      `help:"App description"`
	Port mockFlag[int] `help:"Port"`
}

type BlankFieldEnvPrefixCLI struct {
	_    struct{}      `envprefix:"APP_"`
	Port mockFlag[int] `help:"Port"`
}

type BlankFieldForeignTagCLI struct {
	_    struct{}      `json:"-"`
	Port mockFlag[int] `help:"Port"`
}

type AllowedForeignTagsCLI struct {
	Name mockFlag[string] `db:"user_name" doc:"User name" help:"User name" json:"name" toml:"name" validate:"required" yaml:"name"`
}

type RemovedCongaTagCLI struct {
	Subcommand mockCmd[LeafTestCmd] `conga:"ignore" summary:"Subcommand"`
}

type AppLevelCustomTagCLI struct {
	Rate mockFlag[float64] `datadog:"rate" help:"Sample rate"`
}

type RequiredStandaloneTagCLI struct {
	Port mockFlag[int] `required:"true"`
}

func TestStrictTagLinter_TypoDetection(t *testing.T) {
	t.Parallel()

	t.Run("typo in short tag suggests short", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct TypoTagFlagCLI field Verbose: unknown struct tag "shrot"`, err.Error())
		assert.Equal(t, `did you mean "short"? Use app.AllowTags("shrot") to permit external tags`, errlines.Hint(err))
	})

	t.Run("typo in default tag suggests default", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagDefaultCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct TypoTagDefaultCLI field Port: unknown struct tag "defualt"`, err.Error()) //nolint:misspell // intentionally misspelled in expected test error
		assert.Equal(t, "", errlines.Note(err))
		assert.Equal(t, `did you mean "default"? Use app.AllowTags("defualt") to permit external tags`, errlines.Hint(err))
	})

	t.Run("typo in help tag suggests help", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct TypoTagHelpCLI field Debug: unknown struct tag "hlepp"`, err.Error())
		assert.Equal(t, `did you mean "help"? Use app.AllowTags("hlepp") to permit external tags`, errlines.Hint(err))
	})
}

func TestStrictTagLinter_ContextValidation(t *testing.T) {
	t.Parallel()

	t.Run("short tag on positional arg errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidArgShortCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextInvalidArgShortCLI field Path: tag "short" is invalid on positional argument`, err.Error())
		assert.Equal(t, `tag "short" is only valid on flags and config file flags`, errlines.Note(err))
	})

	t.Run("xor tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidCommandXorCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextInvalidCommandXorCLI field Subcommand: tag "xor" is invalid on subcommand`, err.Error())
		assert.Equal(t, `tag "xor" is only valid on flags`, errlines.Note(err))
	})

	t.Run("oneof tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidCommandOneOfCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextInvalidCommandOneOfCLI field Subcommand: tag "oneof" is invalid on subcommand`, err.Error())
		assert.Equal(t, `tag "oneof" is only valid on flags`, errlines.Note(err))
	})

	t.Run("oneof tag on config file flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidConfigFileOneOfCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextInvalidConfigFileOneOfCLI field Config: tag "oneof" is invalid on config file flag`, err.Error())
		assert.Equal(t, `tag "oneof" is only valid on flags`, errlines.Note(err))
	})

	t.Run("negatable tag on non-bool flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidNegatableNonBoolCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextInvalidNegatableNonBoolCLI field Port: tag "negatable" is only valid on boolean flags, got int`, err.Error())
	})

	t.Run("help tag on dash args is stored", func(t *testing.T) {
		t.Parallel()

		var cli ContextDashArgsHelpCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, "Trailing args", cmdSpec.DashArgs.Help)
	})

	t.Run("help tag on raw args is stored", func(t *testing.T) {
		t.Parallel()

		var cli ContextRawArgsHelpCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, "Raw args", cmdSpec.RawArgs.Help)
	})

	t.Run("help variable on dash args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextDashArgsHelpVariableCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ContextDashArgsHelpVariableCLI field Extra: tag "help" references {{enum}}, but the field provides no value for that variable`, err.Error())
	})

	t.Run("escaped braces in raw args help are unescaped", func(t *testing.T) {
		t.Parallel()

		var cli ContextRawArgsHelpEscapeCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, "Raw {{args}}", cmdSpec.RawArgs.Help)
	})

	t.Run("counter tag on flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ObsoleteCounterTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ObsoleteCounterTagCLI field Count: unknown struct tag "counter"`, err.Error())
		assert.Equal(t, `use app.AllowTags("counter") to permit external tags`, errlines.Hint(err))
	})

	t.Run("sep tag on config files is permitted", func(t *testing.T) {
		t.Parallel()

		var cli ConfigFilesWithSepCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Flags))
		assert.Equal(t, ";", cmdSpec.Flags[0].Sep)
	})

	t.Run("default tag on config file errors", func(t *testing.T) {
		t.Parallel()

		var cli ConfigFileDefaultTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ConfigFileDefaultTagCLI field Config: tag "default" is not supported on config-file flags`, err.Error())
		assert.Equal(t, `declare search locations with app.ConfigOptions(conga.ConfigPaths(...)) or conga.ConfigExplicit(...), or show an example value with placeholder:"..."`, errlines.Hint(err))
	})

	t.Run("default tag on counter errors", func(t *testing.T) {
		t.Parallel()

		var cli CounterDefaultTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct CounterDefaultTagCLI field Verbose: tag "default" is not supported on counter flags`, err.Error())
		assert.Equal(t, `counters count command-line occurrences starting at 0`, errlines.Note(err))
		assert.Equal(t, `seed a base level with an env or config-file binding, or use OptionalFlag[int] for a numeric default`, errlines.Hint(err))
	})

	t.Run("help tag on blank field errors", func(t *testing.T) {
		t.Parallel()

		var cli BlankFieldInvalidTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct BlankFieldInvalidTagCLI field _: tag "help" is invalid on blank field`, err.Error())
		assert.Equal(t, `tag "help" is only valid on flags, positional arguments, config file flags, Features containers, feature fields, and trailing-argument collectors`, errlines.Note(err))
	})

	t.Run("envprefix tag on blank field errors", func(t *testing.T) {
		t.Parallel()

		var cli BlankFieldEnvPrefixCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct BlankFieldEnvPrefixCLI field _: tag "envprefix" is not supported on blank (_) fields`, err.Error())
		assert.Equal(t, `set the root prefix with app.EnvPrefix(...) and a subcommand prefix with the envprefix tag on its Command[T] field`, errlines.Hint(err))
	})
}

func TestStrictTagLinter_ForeignEcosystemTags(t *testing.T) {
	t.Parallel()

	t.Run("allowed foreign tags pass without panic", func(t *testing.T) {
		t.Parallel()

		var cli AllowedForeignTagsCLI

		s, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Flags))
		assert.Equal(t, "name", s.Flags[0].Name)
	})
}

func TestStrictTagLinter_BlankFieldForeignTagAccepted(t *testing.T) {
	t.Parallel()

	var cli BlankFieldForeignTagCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
}

func TestValidateFieldTags_BlankFieldEnvPrefixRejectedWhenUnknownKeysSkipped(t *testing.T) {
	t.Parallel()

	err := spec.ValidateFieldTags(reflect.StructTag(`envprefix:"APP_"`), "blank field", nil, spec.ValidationOptions{SkipUnknownTagKeyCheck: true})

	assert.Error(t, err)
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `tag "envprefix" is not supported on blank (_) fields`, err.Error())
	assert.Equal(t, `set the root prefix with app.EnvPrefix(...) and a subcommand prefix with the envprefix tag on its Command[T] field`, errlines.Hint(err))
}

func TestValidateFieldTags_BlankFieldHelpAcceptedWhenUnknownKeysSkipped(t *testing.T) {
	t.Parallel()

	err := spec.ValidateFieldTags(reflect.StructTag(`help:"App description"`), "blank field", nil, spec.ValidationOptions{SkipUnknownTagKeyCheck: true})

	assert.NoError(t, err)
}

func TestStrictTagLinter_CongaTagRejected(t *testing.T) {
	t.Parallel()

	var cli RemovedCongaTagCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct RemovedCongaTagCLI field Subcommand: unknown struct tag "conga"`, err.Error())
	assert.Equal(t, `use app.AllowTags("conga") to permit external tags`, errlines.Hint(err))
}

func TestStrictTagLinter_OptionsConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("AllowedTags permits custom registered tag", func(t *testing.T) {
		t.Parallel()

		var cli AppLevelCustomTagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			AllowedTags: []string{"datadog"},
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Flags))
		assert.Equal(t, "rate", cmdSpec.Flags[0].Name)
	})
}

func TestStrictTagLinter_RequiredTagRejected(t *testing.T) {
	t.Parallel()

	t.Run("standalone required:true tag is rejected with error", func(t *testing.T) {
		t.Parallel()

		var cli RequiredStandaloneTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct RequiredStandaloneTagCLI field Port: tag "required" is not supported`, err.Error())
		assert.Equal(t, `use conga.RequiredFlag[T] or conga.RequiredArg[T] instead`, errlines.Hint(err))
	})
}

type contextConfigOnArgCLI struct {
	Path mockReqArg[string] `config:"path" help:"Path"`
}

type contextPlaceholderOnSubcommandSub struct{}

type contextPlaceholderOnSubcommandCLI struct {
	Sub mockCmd[contextPlaceholderOnSubcommandSub] `placeholder:"X" summary:"Sub"`
}

type contextDefaultOnSubcommandSub struct{}

type contextDefaultOnSubcommandCLI struct {
	Sub mockCmd[contextDefaultOnSubcommandSub] `default:"x" summary:"Sub"`
}

// The "only valid on" list comes from the per-role tag tables.
func TestStrictTagLinter_ContextListFromRoleTables(t *testing.T) {
	t.Parallel()

	t.Run("config on a positional argument", func(t *testing.T) {
		t.Parallel()

		var cli contextConfigOnArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct contextConfigOnArgCLI field Path: tag "config" is invalid on positional argument`, err.Error())
		assert.Equal(t, `tag "config" is only valid on flags, config file flags, and Features containers`, errlines.Note(err))
	})

	t.Run("placeholder on a subcommand", func(t *testing.T) {
		t.Parallel()

		var cli contextPlaceholderOnSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct contextPlaceholderOnSubcommandCLI field Sub: tag "placeholder" is invalid on subcommand`, err.Error())
		assert.Equal(t, `tag "placeholder" is only valid on flags and config file flags`, errlines.Note(err))
	})

	t.Run("default on a subcommand", func(t *testing.T) {
		t.Parallel()

		var cli contextDefaultOnSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct contextDefaultOnSubcommandCLI field Sub: tag "default" is invalid on subcommand`, err.Error())
		assert.Equal(t, `tag "default" is only valid on flags, positional arguments, and feature fields`, errlines.Note(err))
	})
}

type allowedTagTypoCLI struct {
	Replicas mockFlag[int] `help:"Replicas" histroy:"replicas"`
}

type foreignTagCLI struct {
	Replicas mockFlag[int] `bson:"replicas" help:"Replicas"`
}

// A misspelt allowed key is suggested by its allowed name, without the
// advice to allow the misspelling (#127).
func TestStrictTagLinter_AllowedTagSuggestion(t *testing.T) {
	t.Parallel()

	t.Run("misspelt allowed key", func(t *testing.T) {
		t.Parallel()

		var cli allowedTagTypoCLI

		_, err := buildSchema(&cli, "app", spec.Options{AllowedTags: []string{"history"}})

		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct allowedTagTypoCLI field Replicas: unknown struct tag "histroy"`, err.Error())
		assert.Equal(t, `did you mean "history"?`, errlines.Hint(err))
	})

	t.Run("foreign key with no near CONGA tag", func(t *testing.T) {
		t.Parallel()

		var cli foreignTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct foreignTagCLI field Replicas: unknown struct tag "bson"`, err.Error())
		assert.Equal(t, `use app.AllowTags("bson") to permit external tags`, errlines.Hint(err))
	})
}
