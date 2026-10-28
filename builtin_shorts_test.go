package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// TestBuiltinShort_HelpScreens pins the rendered short runes of the injected
// --help/--version flags once app.HelpShortFlag and app.VersionShortFlag assign them:
// the help short appears on every screen, the version short on the root screen
// alone, and both stay absent when the builders are unset.
func TestBuiltinShort_HelpScreens(t *testing.T) {
	t.Parallel()

	t.Run("root screen renders both shorts", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().HelpShortFlag('h').VersionShortFlag('V').Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "-h, --help")
		assert.Contains(t, buffer.String(), "-V, --version")
	})

	t.Run("subcommand screen renders the help short only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().HelpShortFlag('h').VersionShortFlag('V').Stdout(&buffer).Args([]string{"deploy", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "-h, --help")
		assert.NotContains(t, buffer.String(), "-V, --version")
	})

	t.Run("screens stay long-only by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
		assert.Contains(t, buffer.String(), "--version")
		assert.NotContains(t, buffer.String(), "-h, --help")
		assert.NotContains(t, buffer.String(), "-V, --version")
	})
}

// TestBuiltinShort_Parsing verifies the configured runes route to their
// built-ins: -h renders the active command's help screen and -V prints the
// version summary, while an unconfigured rune stays an unknown flag.
func TestBuiltinShort_Parsing(t *testing.T) {
	t.Parallel()

	t.Run("short help flag renders the root screen", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().HelpShortFlag('h').Stdout(&buffer).Args([]string{"-h"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Usage:\n  golden <command>")
		assert.Contains(t, buffer.String(), "-h, --help")
	})

	t.Run("short help flag renders the subcommand screen", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().HelpShortFlag('h').Stdout(&buffer).Args([]string{"deploy", "-h"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Usage:\n  golden deploy --reason=STRING <target> [flags...]")
	})

	t.Run("short version flag prints the version summary", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().VersionShortFlag('V').Stdout(&buffer).Args([]string{"-V"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "golden has version 1.2.3 built with ")
	})

	t.Run("short version flag is root-only", func(t *testing.T) {
		t.Parallel()

		app := newGoldenApp().VersionShortFlag('V').Stdout(io.Discard).Args([]string{"deploy", "-V"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("unconfigured short stays an unknown flag", func(t *testing.T) {
		t.Parallel()

		app := newGoldenApp().Stdout(io.Discard).Args([]string{"-h"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("user flag keeps its rune when no built-in short is assigned", func(t *testing.T) {
		t.Parallel()

		type UserShortCLI struct {
			noopRunner

			Host conga.OptionalFlag[string] `help:"Host" short:"h"`
		}

		var cli UserShortCLI

		app := conga.New(&cli).Name("app").Args([]string{"-h", "example.com"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "example.com", cli.Host.Get())
	})
}

// shadowedHelpCLI reuses the built-in help name, which only
// app.DisableHelpFlag frees; its runner renders the active screen so tests can
// assert what row the shadow flag actually gets.
type shadowedHelpCLI struct {
	Help conga.OptionalFlag[bool] `help:"Custom help" name:"help"`
}

func (c *shadowedHelpCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

// shadowedHelpAliasCLI reaches the help classification through an alias rather
// than the flag name.
type shadowedHelpAliasCLI struct {
	Assist conga.OptionalFlag[bool] `aliases:"help" help:"Custom assist"`
}

func (c *shadowedHelpAliasCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

type shadowedVersionCLI struct {
	Version conga.OptionalFlag[bool] `help:"Custom version" name:"version"`
}

func (c *shadowedVersionCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

type featureShadowGates struct {
	Alpha bool
}

type featureHelpShadowCLI struct {
	Features conga.Features[featureShadowGates] `flag:"help"`
}

func (c *featureHelpShadowCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

type featureVersionShadowCLI struct {
	Features conga.Features[featureShadowGates] `flag:"version"`
}

func (c *featureVersionShadowCLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}

// TestBuiltinShort_UserShadows pins the rule that a configured built-in rune
// never leaks onto a user-declared flag reusing the freed help/version name or
// alias: the binder registers the rune on the injected built-in alone, so a
// shadow row advertising it would promise a short the parser rejects.
func TestBuiltinShort_UserShadows(t *testing.T) {
	t.Parallel()

	t.Run("help shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[shadowedHelpCLI]().
			DisableHelpFlag().
			HelpShortFlag('h').
			Stdout(&buffer).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Custom help")
		assert.NotContains(t, buffer.String(), "-h, --help")
	})

	t.Run("help alias shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[shadowedHelpAliasCLI]().
			DisableHelpFlag().
			HelpShortFlag('h').
			Stdout(&buffer).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Custom assist")
		assert.NotContains(t, buffer.String(), "-h, --assist")
	})

	t.Run("version shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[shadowedVersionCLI]().
			DisableVersionFlag().
			VersionShortFlag('V').
			Stdout(&buffer).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Custom version")
		assert.NotContains(t, buffer.String(), "-V, --version")
	})

	t.Run("help shadow rune stays unknown at parse time", func(t *testing.T) {
		t.Parallel()

		app := conga.New[shadowedHelpCLI]().
			DisableHelpFlag().
			HelpShortFlag('h').
			Stdout(io.Discard).
			Args([]string{"-h"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: -h", err.Error())
	})
}

// TestBuiltinShort_EngineOwnedShadows pins that an engine-owned Features map
// flag reusing a freed built-in name renders long-only: the binder registers
// the configured rune on the injected built-in alone, so the synthesized flag
// must not advertise a short the parser rejects.
func TestBuiltinShort_EngineOwnedShadows(t *testing.T) {
	t.Parallel()

	t.Run("features help shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[featureHelpShadowCLI]().
			EnableFeatures().
			DisableHelpFlag().
			HelpShortFlag('h').
			Stdout(&buffer).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help=KEY=VALUE")
		assert.NotContains(t, buffer.String(), "-h, --help")
	})

	t.Run("features help shadow rune stays unknown at parse time", func(t *testing.T) {
		t.Parallel()

		app := conga.New[featureHelpShadowCLI]().
			EnableFeatures().
			DisableHelpFlag().
			HelpShortFlag('h').
			Stdout(io.Discard).
			Args([]string{"-h"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: -h", err.Error())
	})

	t.Run("features version shadow renders long-only", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[featureVersionShadowCLI]().
			EnableFeatures().
			DisableVersionFlag().
			VersionShortFlag('V').
			Stdout(&buffer).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--version=KEY=VALUE")
		assert.NotContains(t, buffer.String(), "-V, --version")
	})
}
