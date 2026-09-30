package spec_test

import (
	"context"
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

type SentinelInvalidTagCLI struct {
	_    struct{}      `help:"App description"`
	Port mockFlag[int] `help:"Port"`
}

type AllowedForeignTagsCLI struct {
	Name mockFlag[string] `db:"user_name" doc:"User name" help:"User name" json:"name" toml:"name" validate:"required" yaml:"name"`
}

type DirectiveIgnoreCLI struct {
	Port   mockFlag[int]    `default:"8080" help:"Listen port"`
	Secret mockFlag[string] `conga:"-"      unknown_weird_tag:"val"`
}

type DirectiveFieldAllowUnknownCLI struct {
	Metric mockFlag[string] `conga:"allow-unknown-tags" help:"Metric" prometheus:"gauge"`
}

type DirectiveCommandAllowUnknownCLI struct {
	Subcommand mockCmd[SubcommandWithCustomTags] `conga:"allow-unknown-tags" summary:"Subcommand with custom tags"`
}

type SubcommandWithCustomTags struct {
	Worker mockFlag[string] `customtag:"worker1" help:"Worker name"`
}

func (s *SubcommandWithCustomTags) Run(ctx context.Context) error {
	return nil
}

type DirectiveRecurseCLI struct {
	Parent mockCmd[ParentCmdWithRecurse] `conga:"allow-unknown-tags,recurse" summary:"Parent"`
}

type ParentCmdWithRecurse struct {
	Child mockCmd[ChildCmdWithCustomTags] `summary:"Child"`
}

type ChildCmdWithCustomTags struct {
	Token mockFlag[string] `deepcustom:"true" help:"Token"`
}

func (c *ChildCmdWithCustomTags) Run(ctx context.Context) error {
	return nil
}

type StructSentinelAllowUnknownCLI struct {
	_      struct{}         `conga:"allow-unknown-tags"`
	Worker mockFlag[string] `customtag:"worker1"        help:"Worker name"`
}

type LegacyShorthandDirectiveCLI struct {
	Verbose mockFlag[bool] `conga:"-v, --verbose" help:"Verbose output"`
}

type AppLevelCustomTagCLI struct {
	Rate mockFlag[float64] `datadog:"rate" help:"Sample rate"`
}

type RequiredCongaTagCLI struct {
	Port mockFlag[int] `conga:"required"`
}

type RequiredStandaloneTagCLI struct {
	Port mockFlag[int] `required:"true"`
}

func TestStrictTagLinter_TypoDetection(t *testing.T) {
	t.Parallel()

	t.Run("typo in short tag suggests short", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagFlagCLI field Verbose: schema error: invalid struct tag: unknown struct tag "shrot" (did you mean "short"?). Use conga:"allow-unknown-tags" or app.AllowTags("shrot") to permit external tags`, err.Error())
	})

	t.Run("typo in default tag suggests default", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagDefaultCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagDefaultCLI field Port: schema error: invalid struct tag: unknown struct tag "defualt" (did you mean "default"?). Use conga:"allow-unknown-tags" or app.AllowTags("defualt") to permit external tags`, err.Error()) //nolint:misspell // intentionally misspelled in expected test error
	})

	t.Run("typo in help tag suggests help", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoTagHelpCLI field Debug: schema error: invalid struct tag: unknown struct tag "hlepp" (did you mean "help"?). Use conga:"allow-unknown-tags" or app.AllowTags("hlepp") to permit external tags`, err.Error())
	})
}

func TestStrictTagLinter_ContextValidation(t *testing.T) {
	t.Parallel()

	t.Run("short tag on positional arg errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidArgShortCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidArgShortCLI field Path: schema error: invalid struct tag: tag "short" is invalid on positional argument (tag "short" is only valid on flags)`, err.Error())
	})

	t.Run("xor tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidCommandXorCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidCommandXorCLI field Subcommand: schema error: invalid struct tag: tag "xor" is invalid on subcommand (tag "xor" is only valid on flags)`, err.Error())
	})

	t.Run("negatable tag on non-bool flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidNegatableNonBoolCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidNegatableNonBoolCLI field Port: schema error: invalid struct tag: tag "negatable" is only valid on boolean flags, got int`, err.Error())
	})

	t.Run("help tag on dash args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidDashArgsHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidDashArgsHelpCLI field Extra: schema error: invalid struct tag: tag "help" is invalid on dash args (tag "help" is only valid on other CONGA constructs)`, err.Error())
	})

	t.Run("help tag on raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextInvalidRawArgsHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextInvalidRawArgsHelpCLI field Raw: schema error: invalid struct tag: tag "help" is invalid on raw args (tag "help" is only valid on other CONGA constructs)`, err.Error())
	})

	t.Run("counter tag on flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ObsoleteCounterTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ObsoleteCounterTagCLI field Count: schema error: invalid struct tag: unknown struct tag "counter". Use conga:"allow-unknown-tags" or app.AllowTags("counter") to permit external tags`, err.Error())
	})

	t.Run("sep tag on config files is permitted", func(t *testing.T) {
		t.Parallel()

		var cli ConfigFilesWithSepCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

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
		assert.Equal(t, `struct ConfigFileDefaultTagCLI field Config: schema error: invalid struct tag: tag "default" is not supported on config-file flags; declare search locations with app.ConfigOptions(config.Paths(...)) or config.Explicit(...), or show an example value with placeholder:"..."`, err.Error())
	})

	t.Run("help tag on sentinel field errors", func(t *testing.T) {
		t.Parallel()

		var cli SentinelInvalidTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SentinelInvalidTagCLI field _: schema error: invalid struct tag: tag "help" is invalid on sentinel field (tag "help" is only valid on other CONGA constructs)`, err.Error())
	})
}

func TestStrictTagLinter_ForeignEcosystemTags(t *testing.T) {
	t.Parallel()

	t.Run("allowed foreign tags pass without panic", func(t *testing.T) {
		t.Parallel()

		var cli AllowedForeignTagsCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Flags))
		assert.Equal(t, "name", s.Flags[0].Name)
	})
}

func TestStrictTagLinter_CongaDirectives(t *testing.T) {
	t.Parallel()

	t.Run("conga ignore directive skips field", func(t *testing.T) {
		t.Parallel()

		var cli DirectiveIgnoreCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Flags))
		assert.Equal(t, "port", s.Flags[0].Name)
	})

	t.Run("conga allow-unknown-tags on field permits custom tags", func(t *testing.T) {
		t.Parallel()

		var cli DirectiveFieldAllowUnknownCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Flags))
		assert.Equal(t, "metric", s.Flags[0].Name)
	})

	t.Run("conga allow-unknown-tags on subcommand permits custom tags in child struct", func(t *testing.T) {
		t.Parallel()

		var cli DirectiveCommandAllowUnknownCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Subcommands))
		assert.Equal(t, 1, len(s.Subcommands[0].Flags))
		assert.Equal(t, "worker", s.Subcommands[0].Flags[0].Name)
	})

	t.Run("conga allow-unknown-tags with recurse permits custom tags across hierarchy", func(t *testing.T) {
		t.Parallel()

		var cli DirectiveRecurseCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Subcommands))
		assert.Equal(t, 1, len(s.Subcommands[0].Subcommands))
		assert.Equal(t, "token", s.Subcommands[0].Subcommands[0].Flags[0].Name)
	})

	t.Run("conga allow-unknown-tags on struct sentinel permits custom tags", func(t *testing.T) {
		t.Parallel()

		var cli StructSentinelAllowUnknownCLI

		s, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(s.Flags))
		assert.Equal(t, "worker", s.Flags[0].Name)
	})

	t.Run("legacy conga naming shorthand errors with directive guidance", func(t *testing.T) {
		t.Parallel()

		var cli LegacyShorthandDirectiveCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct LegacyShorthandDirectiveCLI field Verbose: schema error: invalid struct tag: unrecognized conga directive "-v". Note: flag names and short flags must be declared using name:"..." and short:"..."`, err.Error())
	})
}

func TestStrictTagLinter_OptionsConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("AllowedTags permits custom registered tag", func(t *testing.T) {
		t.Parallel()

		var cli AppLevelCustomTagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			StrictTagValidation: true,
			AllowedTags:         []string{"datadog"},
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Flags))
		assert.Equal(t, "rate", cmdSpec.Flags[0].Name)
	})

	t.Run("StrictTagValidation false disables validation without panic", func(t *testing.T) {
		t.Parallel()

		var cli TypoTagFlagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			StrictTagValidation: false,
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Flags))
		assert.Equal(t, "verbose", cmdSpec.Flags[0].Name)
	})
}

func TestStrictTagLinter_RequiredTagRejected(t *testing.T) {
	t.Parallel()

	t.Run("conga:required tag is rejected with error", func(t *testing.T) {
		t.Parallel()

		var cli RequiredCongaTagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct RequiredCongaTagCLI field Port: schema error: invalid struct tag: unrecognized conga directive "required". Note: flag names and short flags must be declared using name:"..." and short:"..."`, err.Error())
	})

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
