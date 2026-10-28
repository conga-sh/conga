package conga_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// hasCandidate reports whether any completion candidate carries the value.
func hasCandidate(result predict.Result, value string) bool {
	for _, item := range result.Items {
		if item.Value == value {
			return true
		}
	}

	return false
}

// TestBuiltinVisibility_RootHelpScreen pairs each app-wide built-in visibility
// toggle with an absence assertion on the root help screen, so a leaked row
// (the #60 bug class) fails the suite instead of passing a presence-only check.
func TestBuiltinVisibility_RootHelpScreen(t *testing.T) {
	t.Parallel()

	t.Run("renders help and version flags when enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
		assert.Contains(t, buffer.String(), "--version")
	})

	t.Run("omits version flag when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().DisableVersionFlag().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--version")
		// The version subcommand is user-declared and remains listed; only the
		// synthesized flag is suppressed.
		assert.Contains(t, buffer.String(), "version  Show version information")
	})

	t.Run("omits help flag when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		// With the flag disabled the help screen is reached through the help
		// subcommand, the same surface a user would have.
		app := newGoldenApp().DisableHelpFlag().Stdout(&buffer).Args([]string{"help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--help")
		assert.Contains(t, buffer.String(), "help     Show help for a command")
	})

	t.Run("omits why flag when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("omits completion flag when completion is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--completion")
	})

	t.Run("omits hidden flags", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		// goldenCLI declares a hidden:"true" flag named --hidden.
		app := newGoldenApp().Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
		assert.NotContains(t, buffer.String(), "--hidden")
	})
}

// TestBuiltinVisibility_SubcommandHelpScreen mirrors the root sweep on a
// subcommand screen, where --version is root-only and the remaining builtins
// obey the same app-wide toggles.
func TestBuiltinVisibility_SubcommandHelpScreen(t *testing.T) {
	t.Parallel()

	t.Run("renders help flag when enabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"deploy", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
	})

	t.Run("omits help flag when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().DisableHelpFlag().Stdout(&buffer).Args([]string{"help", "deploy"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--help")
	})

	t.Run("omits version flag when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().DisableVersionFlag().Stdout(&buffer).Args([]string{"deploy", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--version")
	})

	t.Run("omits why flag when why is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"deploy", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--why")
	})

	t.Run("omits completion flag when completion is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"deploy", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--completion")
	})
}

// TestBuiltinVisibility_UsageErrorScreen pins that usage diagnostics stop
// advertising the --help hint and the --version row when those flags are
// disabled, since the referenced invocations no longer parse.
func TestBuiltinVisibility_UsageErrorScreen(t *testing.T) {
	t.Parallel()

	t.Run("summary hint references help by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			UsageOnError(conga.UsageHint).
			Args([]string{"clustr"}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Contains(t, buffer.String(), "for more information")
		assert.Contains(t, buffer.String(), "--help")
	})

	t.Run("hint omits help when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			DisableHelpFlag().
			UsageOnError(conga.UsageHint).
			Args([]string{"clustr"}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.NotContains(t, buffer.String(), "--help")
	})

	t.Run("full usage omits version when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			DisableVersionFlag().
			UsageOnError(conga.UsageFull).
			Args([]string{"clustr"}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Contains(t, buffer.String(), "--help")
		assert.NotContains(t, buffer.String(), "--version")
	})
}

// TestBuiltinVisibility_VersionCommand pairs the opt-in version command help
// screen (reachable via `version --help` or `help version`) with its disabled
// configurations.
func TestBuiltinVisibility_VersionCommand(t *testing.T) {
	t.Parallel()

	t.Run("renders json and help flags by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().Stdout(&buffer).Args([]string{"version", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--json")
		assert.Contains(t, buffer.String(), "--help")
	})

	t.Run("omits help flag when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().DisableHelpFlag().Stdout(&buffer).Args([]string{"help", "version"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--help")
	})

	t.Run("version --help is a usage error when help is disabled", func(t *testing.T) {
		t.Parallel()

		app := newGoldenApp().DisableHelpFlag().Args([]string{"version", "--help"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
	})

	t.Run("omits version flag when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().DisableVersionFlag().Stdout(&buffer).Args([]string{"version", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--version")
	})
}

// TestBuiltinVisibility_SynthesizedCompletionCommand pairs the auto-mounted
// completion subcommand help screen with its disabled configurations.
func TestBuiltinVisibility_SynthesizedCompletionCommand(t *testing.T) {
	t.Parallel()

	t.Run("renders help flag and footer pointer by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().EnableCompletion().Stdout(&buffer).Args([]string{"completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
		// The footer help pointer is dynamic: a detected shell yields the
		// "For persistent setups" hint, while an undetected shell (CI runners,
		// containers, task test:all) falls back to the generic pointer. Either
		// proves the pointer renders when help is available.
		assert.True(t, strings.Contains(buffer.String(), "For persistent setups") ||
			strings.Contains(buffer.String(), "To enable completions for your shell, run:"))
	})

	t.Run("renders the configured help short on the shared help row", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().
			EnableCompletion().
			HelpShortFlag('h').
			VersionShortFlag('V').
			Stdout(&buffer).
			Args([]string{"completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "-h, --help")
		assert.NotContains(t, buffer.String(), "-V, --version")
	})

	t.Run("configured help short parses on the completion screen", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().
			EnableCompletion().
			HelpShortFlag('h').
			Stdout(&buffer).
			Args([]string{"completion", "-h"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Supported shells")
	})

	t.Run("omits help flag and footer pointer when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().EnableCompletion().DisableHelpFlag().Stdout(&buffer).Args([]string{"help", "completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Supported shells")
		assert.NotContains(t, buffer.String(), "--help")
		assert.NotContains(t, buffer.String(), "To enable completions for your shell, run:")
		assert.NotContains(t, buffer.String(), "For persistent setups")
	})

	t.Run("completion --help is a usage error when help is disabled", func(t *testing.T) {
		t.Parallel()

		app := newGoldenApp().EnableCompletion().DisableHelpFlag().Args([]string{"completion", "--help"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
	})

	t.Run("omits version flag when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := newGoldenApp().EnableCompletion().DisableVersionFlag().Stdout(&buffer).Args([]string{"completion", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--version")
	})
}

// TestBuiltinVisibility_EphemeralCompletionScreens sweeps the flat-CLI
// completion help screens built by the binder, whose footer hint previously
// advertised a disabled --help invocation.
func TestBuiltinVisibility_EphemeralCompletionScreens(t *testing.T) {
	t.Parallel()

	t.Run("bare completion renders the footer help pointer by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Supported shells")
		assert.Contains(t, buffer.String(), "--help")
	})

	t.Run("bare completion omits the footer help pointer when help is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			DisableHelpFlag().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Supported shells")
		assert.NotContains(t, buffer.String(), "--help")
	})

	t.Run("per-shell completion renders the help flag by default", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion", "zsh", "--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--help")
	})

	t.Run("per-shell completion is a usage error when help is disabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			DisableHelpFlag().
			Name("myapp").
			Args([]string{"--completion", "zsh", "--help"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
	})

	t.Run("bare completion omits version when version is disabled", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			DisableVersionFlag().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), "--version")
	})

	t.Run("bare completion advertises only text why format without JSON", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[whyFlatCLI]().
			EnableCompletion().
			EnableWhy().
			Name("myapp").
			Stdout(&buffer).
			Args([]string{"--completion"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--why")
		assert.NotContains(t, buffer.String(), "--why[=FORMAT]")
		assert.NotContains(t, buffer.String(), "[enum: text")
	})
}

// TestBuiltinVisibility_CompletionCandidates pins each built-in flag candidate
// to a single scenario: a default or enabled configuration yields the
// candidate, while a suppressed configuration yields none.
func TestBuiltinVisibility_CompletionCandidates(t *testing.T) {
	t.Parallel()

	t.Run("help candidate present by default", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]()

		result, _ := app.CompleteForShell("fish", []string{"--he"})

		assert.True(t, hasCandidate(result, "--help"))
	})

	t.Run("help candidate absent when disabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]().DisableHelpFlag()

		result, _ := app.CompleteForShell("fish", []string{"--he"})

		assert.False(t, hasCandidate(result, "--help"))
	})

	t.Run("version candidate present by default", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]()

		result, _ := app.CompleteForShell("fish", []string{"--ver"})

		assert.True(t, hasCandidate(result, "--version"))
	})

	t.Run("version candidate absent when disabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]().DisableVersionFlag()

		result, _ := app.CompleteForShell("fish", []string{"--ver"})

		assert.False(t, hasCandidate(result, "--version"))
	})

	t.Run("why candidate present when enabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]().EnableWhy()

		result, _ := app.CompleteForShell("fish", []string{"--wh"})

		assert.True(t, hasCandidate(result, "--why"))
	})

	t.Run("why candidate absent by default", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]()

		result, _ := app.CompleteForShell("fish", []string{"--wh"})

		assert.False(t, hasCandidate(result, "--why"))
	})

	t.Run("completion candidate present when enabled", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]().EnableCompletion()

		result, _ := app.CompleteForShell("fish", []string{"--co"})

		assert.True(t, hasCandidate(result, "--completion"))
	})

	t.Run("completion candidate absent by default", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SimpleAppCLI]()

		result, _ := app.CompleteForShell("fish", []string{"--co"})

		assert.False(t, hasCandidate(result, "--completion"))
	})

	t.Run("hidden flag yields no candidate", func(t *testing.T) {
		t.Parallel()

		app := conga.New[whyCLI]()

		result, _ := app.CompleteForShell("fish", []string{"--sec"})

		assert.False(t, hasCandidate(result, "--secret"))
	})
}
