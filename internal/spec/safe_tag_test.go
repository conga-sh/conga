package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type SafeFlagCLI struct {
	Token mockFlag[string] `help:"API token" safe:"true"`
}

type SafePresentFlagCLI struct {
	Token mockFlag[string] `help:"API token" safe:""`
}

type SafeFalseFlagCLI struct {
	Token mockFlag[string] `help:"API token" safe:"false"`
}

type SafeTypoFlagCLI struct {
	Token mockFlag[string] `help:"API token" safe:"ture"`
}

type SafeConfigFileCLI struct {
	Config mockConfigFile `help:"Config path" safe:"true"`
}

type SafeTypoConfigFileCLI struct {
	Config mockConfigFile `help:"Config path" safe:"ture"`
}

type SafeOnArgCLI struct {
	Target mockArg[string] `help:"Target" safe:"true"`
}

type SafeAbsentBoolCLI struct {
	Toggle mockFlag[bool] `help:"Toggle output"`
}

type SafeAbsentCounterCLI struct {
	Verbosity mockCounter `help:"Verbosity level"`
}

type SafeFalseBoolCLI struct {
	Toggle mockFlag[bool] `help:"Toggle output" safe:"false"`
}

type SafeFalseCounterCLI struct {
	Verbosity mockCounter `help:"Verbosity level" safe:"false"`
}

type SafeAbsentBoolSliceCLI struct {
	Toggles mockFlag[[]bool] `help:"Toggle list"`
}

type SafeAbsentBoolMapCLI struct {
	Toggles mockFlag[map[string]bool] `help:"Toggle map"`
}

// TestSafeTag_LintAcceptance verifies the strict linter accepts safe on
// ordinary flags, and still rejects it on positional arguments and config
// file flags.
func TestSafeTag_LintAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("accepted on ordinary flag", func(t *testing.T) {
		t.Parallel()

		var cli SafeFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
	})

	t.Run("rejected on config file flag", func(t *testing.T) {
		t.Parallel()

		var cli SafeConfigFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SafeConfigFileCLI field Config: schema error: invalid struct tag: tag "safe" is invalid on config file flag (tag "safe" is only valid on flags)`, err.Error())
	})

	t.Run("rejected on positional argument", func(t *testing.T) {
		t.Parallel()

		var cli SafeOnArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SafeOnArgCLI field Target: schema error: invalid struct tag: tag "safe" is invalid on positional argument (tag "safe" is only valid on flags)`, err.Error())
	})
}

// TestSafeTag_StrictBoolMatrix_Flag pins the strict bool parsing on the
// ordinary flag call site: only "", "true", and "false" are accepted.
func TestSafeTag_StrictBoolMatrix_Flag(t *testing.T) {
	t.Parallel()

	t.Run("safe true parses to true", func(t *testing.T) {
		t.Parallel()

		var cli SafeFlagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("bare safe presence parses to true", func(t *testing.T) {
		t.Parallel()

		var cli SafePresentFlagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("safe false parses to false", func(t *testing.T) {
		t.Parallel()

		var cli SafeFalseFlagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.False(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("safe typo is a schema error", func(t *testing.T) {
		t.Parallel()

		var cli SafeTypoFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SafeTypoFlagCLI field Token: schema error: invalid struct tag: tag "safe" expects "true" or "false", got "ture"`, err.Error())
	})
}

// TestSafeTag_RejectedOnConfigFile pins that safe is rejected on the config
// file flag call site: config-file paths are always displayed in the --why
// report, so the tag has no effect there.
func TestSafeTag_RejectedOnConfigFile(t *testing.T) {
	t.Parallel()

	t.Run("safe true is rejected", func(t *testing.T) {
		t.Parallel()

		var cli SafeConfigFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SafeConfigFileCLI field Config: schema error: invalid struct tag: tag "safe" is invalid on config file flag (tag "safe" is only valid on flags)`, err.Error())
	})

	t.Run("safe typo is rejected identically", func(t *testing.T) {
		t.Parallel()

		var cli SafeTypoConfigFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct SafeTypoConfigFileCLI field Config: schema error: invalid struct tag: tag "safe" is invalid on config file flag (tag "safe" is only valid on flags)`, err.Error())
	})
}

// TestSafeTag_AbsentDefaultsFalse verifies absence of the tag leaves Safe false.
func TestSafeTag_AbsentDefaultsFalse(t *testing.T) {
	t.Parallel()

	var cli struct {
		Token mockFlag[string] `help:"API token"`
	}

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

	assert.NoError(t, err)
	assert.False(t, cmdSpec.Flags[0].Safe)
}

// TestSafeTag_BoolAndCounterDefaultVisible pins the type-based default:
// boolean and counter flags are displayed in the --why report without an
// explicit safe tag, because their value domain can never carry credential
// material.
func TestSafeTag_BoolAndCounterDefaultVisible(t *testing.T) {
	t.Parallel()

	t.Run("boolean flag defaults to safe", func(t *testing.T) {
		t.Parallel()

		var cli SafeAbsentBoolCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("counter flag defaults to safe", func(t *testing.T) {
		t.Parallel()

		var cli SafeAbsentCounterCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.Flags[0].Safe)
	})
}

// TestSafeTag_ExplicitOverrideWins pins tag-present-wins semantics: an
// explicit safe:"false" re-redacts a boolean or counter flag despite the
// type-based default.
func TestSafeTag_ExplicitOverrideWins(t *testing.T) {
	t.Parallel()

	t.Run("safe false redacts a boolean flag", func(t *testing.T) {
		t.Parallel()

		var cli SafeFalseBoolCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.False(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("safe false redacts a counter flag", func(t *testing.T) {
		t.Parallel()

		var cli SafeFalseCounterCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.False(t, cmdSpec.Flags[0].Safe)
	})
}

// TestSafeTag_SliceAndMapOfBoolStayExplicit pins that the type-based default
// is restricted to scalar booleans: slices and maps of bool keep the opt-in
// redaction default, because map keys are rendered by the --why report.
func TestSafeTag_SliceAndMapOfBoolStayExplicit(t *testing.T) {
	t.Parallel()

	t.Run("bool slice defaults to redacted", func(t *testing.T) {
		t.Parallel()

		var cli SafeAbsentBoolSliceCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.False(t, cmdSpec.Flags[0].Safe)
	})

	t.Run("bool map defaults to redacted", func(t *testing.T) {
		t.Parallel()

		var cli SafeAbsentBoolMapCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.False(t, cmdSpec.Flags[0].Safe)
	})
}
