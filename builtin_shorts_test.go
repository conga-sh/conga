package conga_test

import (
	"bytes"
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

		err := conga.New(&cli).Name("app").Args([]string{"-h", "example.com"}).RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "example.com", cli.Host.Get())
	})
}
