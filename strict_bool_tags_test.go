package conga_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type hiddenTypoFlagCLI struct {
	noopRunner

	Debug conga.OptionalFlag[bool] `help:"Debug" hidden:"yes"`
}

type hiddenTypoSubcommandCLI struct {
	Debug conga.Command[noopRunner] `hidden:"yes"`
}

type hiddenTypoBuiltinCLI struct {
	noopRunner

	Help conga.HelpCommand `hidden:"yes"`
}

type hiddenTypoConfigFileCLI struct {
	noopRunner

	Config conga.ConfigFile `help:"Config file" hidden:"yes"`
}

type negatableTypoCLI struct {
	noopRunner

	Color conga.OptionalFlag[bool] `help:"Color" negatable:"ture"`
}

type bootstrapTypoFlagCLI struct {
	noopRunner

	Dir conga.OptionalFlag[string] `bootstrap:"yes" help:"Directory"`
}

func (*bootstrapTypoFlagCLI) Bootstrap(context.Context, *conga.Bootstrap) error { return nil }

type bootstrapTypoArgCLI struct {
	noopRunner

	Dir conga.OptionalArg[string] `bootstrap:"yes" help:"Directory"`
}

func (*bootstrapTypoArgCLI) Bootstrap(context.Context, *conga.Bootstrap) error { return nil }

func TestSchema_StrictBoolTagRejectsTypos(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		run      func() error
		expected string
	}{
		{
			name:     "hidden on a flag",
			run:      func() error { return conga.New[hiddenTypoFlagCLI]().Args(nil).Run() },
			expected: `struct hiddenTypoFlagCLI field Debug: tag "hidden" expects "true" or "false", got "yes"`,
		},
		{
			name:     "hidden on a subcommand",
			run:      func() error { return conga.New[hiddenTypoSubcommandCLI]().Args(nil).Run() },
			expected: `struct hiddenTypoSubcommandCLI field Debug: tag "hidden" expects "true" or "false", got "yes"`,
		},
		{
			name:     "hidden on a built-in command",
			run:      func() error { return conga.New[hiddenTypoBuiltinCLI]().Args(nil).Run() },
			expected: `struct hiddenTypoBuiltinCLI field Help: tag "hidden" expects "true" or "false", got "yes"`,
		},
		{
			name:     "hidden on a ConfigFile",
			run:      func() error { return conga.New[hiddenTypoConfigFileCLI]().Args(nil).Run() },
			expected: `struct hiddenTypoConfigFileCLI field Config: tag "hidden" expects "true" or "false", got "yes"`,
		},
		{
			name:     "negatable on a bool flag",
			run:      func() error { return conga.New[negatableTypoCLI]().Args(nil).Run() },
			expected: `struct negatableTypoCLI field Color: tag "negatable" expects "true" or "false", got "ture"`,
		},
		{
			name:     "bootstrap on a flag",
			run:      func() error { return conga.New[bootstrapTypoFlagCLI]().Args(nil).Run() },
			expected: `struct bootstrapTypoFlagCLI field Dir: tag "bootstrap" expects "true" or "false", got "yes"`,
		},
		{
			name:     "bootstrap on a positional argument",
			run:      func() error { return conga.New[bootstrapTypoArgCLI]().Args(nil).Run() },
			expected: `struct bootstrapTypoArgCLI field Dir: tag "bootstrap" expects "true" or "false", got "yes"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

type hiddenValuesCLI struct {
	noopRunner

	Bare     conga.OptionalFlag[bool] `help:"Bare"     hidden:""`
	True     conga.OptionalFlag[bool] `help:"True"     hidden:"true"`
	False    conga.OptionalFlag[bool] `help:"False"    hidden:"false"`
	Untagged conga.OptionalFlag[bool] `help:"Untagged"`
}

type negatableValuesCLI struct {
	noopRunner

	Bare  conga.OptionalFlag[bool] `help:"Bare"  negatable:""`
	True  conga.OptionalFlag[bool] `help:"True"  negatable:"true"`
	False conga.OptionalFlag[bool] `help:"False" negatable:"false"`
}

type bootstrapFalseCLI struct {
	noopRunner

	Dir conga.OptionalFlag[string] `bootstrap:"false" help:"Directory"`
}

func TestSchema_StrictBoolTagValues(t *testing.T) {
	t.Parallel()

	t.Run("hidden accepts bare, true, and false", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[hiddenValuesCLI]().
			Name("tool").
			Stdout(&buffer).
			Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--bare")
		assert.NotContains(t, buffer.String(), "--true")
		assert.Contains(t, buffer.String(), "--false")
		assert.Contains(t, buffer.String(), "--untagged")
	})

	t.Run("negatable accepts bare and true", func(t *testing.T) {
		t.Parallel()

		cli := new(negatableValuesCLI)
		app := conga.New(cli).
			Args([]string{"--no-bare", "--no-true"})

		err := app.Run()

		assert.NoError(t, err)
		assert.False(t, cli.Bare.Get())
		assert.True(t, cli.Bare.IsSet())
		assert.False(t, cli.True.Get())
		assert.True(t, cli.True.IsSet())
	})

	t.Run("negatable false adds no --no- form", func(t *testing.T) {
		t.Parallel()

		app := conga.New[negatableValuesCLI]().
			Args([]string{"--no-false"})

		err := app.Run()

		assertUsageError(t, err)
		assert.Equal(t, "unknown flag: --no-false", err.Error())
	})

	t.Run("bootstrap false needs no Bootstrapper", func(t *testing.T) {
		t.Parallel()

		cli := new(bootstrapFalseCLI)
		app := conga.New(cli).
			Args([]string{"--dir", "x"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "x", cli.Dir.Get())
	})
}

type hiddenOnArgCLI struct {
	noopRunner

	Target conga.OptionalArg[string] `help:"Target" hidden:"yes"`
}

type negatableOnConfigFileCLI struct {
	noopRunner

	Config conga.ConfigFile `help:"Config file" negatable:"yes"`
}

type bootstrapOnSubcommandCLI struct {
	Sub conga.Command[noopRunner] `bootstrap:"yes"`
}

func (*bootstrapOnSubcommandCLI) Bootstrap(context.Context, *conga.Bootstrap) error { return nil }

type bootstrapTypoDashArgsCLI struct {
	noopRunner

	Rest conga.DashArgs `bootstrap:"yes"`
}

func (*bootstrapTypoDashArgsCLI) Bootstrap(context.Context, *conga.Bootstrap) error { return nil }

type bootstrapTypoRawArgsCLI struct {
	noopRunner

	Rest conga.RawArgs `bootstrap:"yes"`
}

func (*bootstrapTypoRawArgsCLI) Bootstrap(context.Context, *conga.Bootstrap) error { return nil }

func TestSchema_StrictBoolTagPlacementAndCollectors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		run      func() error
		expected string
	}{
		{
			name:     "hidden typo on a positional argument names the placement",
			run:      func() error { return conga.New[hiddenOnArgCLI]().Args(nil).Run() },
			expected: `struct hiddenOnArgCLI field Target: tag "hidden" is invalid on positional argument (note: tag "hidden" is only valid on flags, subcommands, built-in commands, and config file flags)`,
		},
		{
			name:     "negatable typo on a ConfigFile names the placement",
			run:      func() error { return conga.New[negatableOnConfigFileCLI]().Args(nil).Run() },
			expected: `struct negatableOnConfigFileCLI field Config: tag "negatable" is invalid on config file flag (note: tag "negatable" is only valid on flags)`,
		},
		{
			name:     "bootstrap typo on a subcommand names the placement",
			run:      func() error { return conga.New[bootstrapOnSubcommandCLI]().Args(nil).Run() },
			expected: `struct bootstrapOnSubcommandCLI field Sub: tag "bootstrap" is invalid on subcommand (note: tag "bootstrap" is only valid on flags, positional arguments, and trailing-argument collectors)`,
		},
		{
			name:     "bootstrap typo on DashArgs",
			run:      func() error { return conga.New[bootstrapTypoDashArgsCLI]().Args(nil).Run() },
			expected: `struct bootstrapTypoDashArgsCLI field Rest: tag "bootstrap" expects "true" or "false", got "yes"`,
		},
		{
			name:     "bootstrap typo on RawArgs",
			run:      func() error { return conga.New[bootstrapTypoRawArgsCLI]().Args(nil).Run() },
			expected: `struct bootstrapTypoRawArgsCLI field Rest: tag "bootstrap" expects "true" or "false", got "yes"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

type allowedHiddenOnArgCLI struct {
	noopRunner

	Target conga.OptionalArg[string] `help:"Target" hidden:"yes"`
}

func TestSchema_AllowedTagIsNotCheckedAsStrictBool(t *testing.T) {
	t.Parallel()

	cli := new(allowedHiddenOnArgCLI)
	app := conga.New(cli).
		AllowTags("hidden").
		Args([]string{"x"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "x", cli.Target.Get())
}

type hiddenFalseCommandsCLI struct {
	noopRunner

	Config conga.ConfigFile          `help:"Config file" hidden:"false"`
	Sub    conga.Command[noopRunner] `hidden:"false"     summary:"Sub command"`
	Help   conga.HelpCommand         `hidden:"false"`
}

func TestSchema_HiddenFalseKeepsCommandsShown(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[hiddenFalseCommandsCLI]().
		Name("tool").
		Stdout(&buffer).
		Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "--config-file")
	assert.Contains(t, buffer.String(), "  sub ")
	assert.Contains(t, buffer.String(), "  help ")
}

// bootstrapSplitCLI records what its bootstrap pass saw: Early is a bootstrap
// field, Late turns bootstrap off and is bound only in the normal pass.
type bootstrapSplitCLI struct {
	noopRunner

	Early conga.OptionalFlag[string] `bootstrap:"true"  help:"Early"`
	Late  conga.OptionalFlag[string] `bootstrap:"false" help:"Late"`

	earlySeen string
	lateSeen  string
}

func (c *bootstrapSplitCLI) Bootstrap(context.Context, *conga.Bootstrap) error {
	c.earlySeen, c.lateSeen = c.Early.Get(), c.Late.Get()

	return nil
}

func TestSchema_BootstrapFalseFieldIsNotBoundForBootstrap(t *testing.T) {
	t.Parallel()

	cli := new(bootstrapSplitCLI)
	app := conga.New(cli).
		Args([]string{"--early", "a", "--late", "b"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "a", cli.earlySeen)
	assert.Equal(t, "", cli.lateSeen)
	assert.Equal(t, "b", cli.Late.Get())
}
