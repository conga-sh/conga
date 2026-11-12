package spec_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

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

type ContextInvalidDashArgsHelpCLI struct {
	Extra mockDashArgs `help:"Trailing args"`
}

type ContextInvalidRawArgsHelpCLI struct {
	Raw mockRawArgs `help:"Raw args"`
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
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagFlagCLI field Verbose: schema error: invalid struct tag: unknown struct tag "shrot" (did you mean "short"?). Use app.AllowTags("shrot") to permit external tags`, err.Error())
	})

	t.Run("typo in default tag suggests default", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagDefaultCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagDefaultCLI field Port: schema error: invalid struct tag: unknown struct tag "defualt" (did you mean "default"?). Use app.AllowTags("defualt") to permit external tags`, err.Error()) //nolint:misspell // intentionally misspelled in expected test error
	})

	t.Run("typo in help tag suggests help", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagHelpCLI field Debug: schema error: invalid struct tag: unknown struct tag "hlepp" (did you mean "help"?). Use app.AllowTags("hlepp") to permit external tags`, err.Error())
	})
}

func TestStrictTagLinter_ContextValidation(t *testing.T) {
	t.Parallel()

	t.Run("short tag on positional arg errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidArgShortCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidArgShortCLI field Path: schema error: invalid struct tag: tag "short" is invalid on positional argument (tag "short" is only valid on flags and config file flags)`, err.Error())
	})

	t.Run("xor tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidCommandXorCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidCommandXorCLI field Subcommand: schema error: invalid struct tag: tag "xor" is invalid on subcommand (tag "xor" is only valid on flags)`, err.Error())
	})

	t.Run("oneof tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidCommandOneOfCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidCommandOneOfCLI field Subcommand: schema error: invalid struct tag: tag "oneof" is invalid on subcommand (tag "oneof" is only valid on flags)`, err.Error())
	})

	t.Run("oneof tag on config file flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidConfigFileOneOfCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidConfigFileOneOfCLI field Config: schema error: invalid struct tag: tag "oneof" is invalid on config file flag (tag "oneof" is only valid on flags)`, err.Error())
	})

	t.Run("negatable tag on non-bool flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidNegatableNonBoolCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidNegatableNonBoolCLI field Port: schema error: invalid struct tag: tag "negatable" is only valid on boolean flags, got int`, err.Error())
	})

	t.Run("help tag on dash args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidDashArgsHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidDashArgsHelpCLI field Extra: schema error: invalid struct tag: tag "help" is invalid on dash args (tag "help" is only valid on flags, positional arguments, config file flags, Features containers, and feature fields)`, err.Error())
	})

	t.Run("help tag on raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidRawArgsHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidRawArgsHelpCLI field Raw: schema error: invalid struct tag: tag "help" is invalid on raw args (tag "help" is only valid on flags, positional arguments, config file flags, Features containers, and feature fields)`, err.Error())
	})

	t.Run("counter tag on flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ObsoleteCounterTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ObsoleteCounterTagCLI field Count: schema error: invalid struct tag: unknown struct tag "counter". Use app.AllowTags("counter") to permit external tags`, err.Error())
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
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ConfigFileDefaultTagCLI field Config: schema error: invalid struct tag: tag "default" is not supported on config-file flags; declare search locations with app.ConfigOptions(conga.ConfigPaths(...)) or conga.ConfigExplicit(...), or show an example value with placeholder:"..."`, err.Error())
	})

	t.Run("default tag on counter errors", func(t *testing.T) {
		t.Parallel()

		var cli CounterDefaultTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct CounterDefaultTagCLI field Verbose: schema error: invalid struct tag: tag "default" is not supported on counter flags; counters count command-line occurrences starting at 0. Seed a base level with an env or config-file binding, or use OptionalFlag[int] for a numeric default`, err.Error())
	})

	t.Run("help tag on blank field errors", func(t *testing.T) {
		t.Parallel()

		var cli BlankFieldInvalidTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct BlankFieldInvalidTagCLI field _: schema error: invalid struct tag: tag "help" is invalid on blank field (tag "help" is only valid on flags, positional arguments, config file flags, Features containers, and feature fields)`, err.Error())
	})

	t.Run("envprefix tag on blank field errors", func(t *testing.T) {
		t.Parallel()

		var cli BlankFieldEnvPrefixCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct BlankFieldEnvPrefixCLI field _: schema error: invalid struct tag: tag "envprefix" is not supported on blank (_) fields; set the root prefix with app.EnvPrefix(...) and a subcommand prefix with the envprefix tag on its Command[T] field`, err.Error())
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
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `schema error: invalid struct tag: tag "envprefix" is not supported on blank (_) fields; set the root prefix with app.EnvPrefix(...) and a subcommand prefix with the envprefix tag on its Command[T] field`, err.Error())
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
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RemovedCongaTagCLI field Subcommand: schema error: invalid struct tag: unknown struct tag "conga". Use app.AllowTags("conga") to permit external tags`, err.Error())
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
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct RequiredStandaloneTagCLI field Port: schema error: invalid struct tag: tag "required" is not supported. Use conga.RequiredFlag[T] or conga.RequiredArg[T] instead`, err.Error())
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

		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct contextConfigOnArgCLI field Path: schema error: invalid struct tag: tag "config" is invalid on positional argument (tag "config" is only valid on flags, config file flags, and Features containers)`, err.Error())
	})

	t.Run("placeholder on a subcommand", func(t *testing.T) {
		t.Parallel()

		var cli contextPlaceholderOnSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct contextPlaceholderOnSubcommandCLI field Sub: schema error: invalid struct tag: tag "placeholder" is invalid on subcommand (tag "placeholder" is only valid on flags and config file flags)`, err.Error())
	})

	t.Run("default on a subcommand", func(t *testing.T) {
		t.Parallel()

		var cli contextDefaultOnSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct contextDefaultOnSubcommandCLI field Sub: schema error: invalid struct tag: tag "default" is invalid on subcommand (tag "default" is only valid on flags, positional arguments, and feature fields)`, err.Error())
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

		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct allowedTagTypoCLI field Replicas: schema error: invalid struct tag: unknown struct tag "histroy" (did you mean "history"?)`, err.Error())
	})

	t.Run("foreign key with no near CONGA tag", func(t *testing.T) {
		t.Parallel()

		var cli foreignTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct foreignTagCLI field Replicas: schema error: invalid struct tag: unknown struct tag "bson". Use app.AllowTags("bson") to permit external tags`, err.Error())
	})
}
