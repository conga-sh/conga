package completion_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion"
)

func TestParseShell(t *testing.T) {
	t.Parallel()

	t.Run("canonical shell names", func(t *testing.T) {
		t.Parallel()

		cases := map[string]completion.Shell{
			"bash":       completion.ShellBash,
			"elvish":     completion.ShellElvish,
			"fish":       completion.ShellFish,
			"nushell":    completion.ShellNushell,
			"powershell": completion.ShellPowerShell,
			"zsh":        completion.ShellZsh,
		}

		for input, expected := range cases {
			shell, err := completion.ParseShell(input)

			assert.NoError(t, err)
			assert.Equal(t, expected, shell)
		}
	})

	t.Run("case insensitivity", func(t *testing.T) {
		t.Parallel()

		cases := map[string]completion.Shell{
			"BASH":       completion.ShellBash,
			"ELVISH":     completion.ShellElvish,
			"NUSHELL":    completion.ShellNushell,
			"ZSH":        completion.ShellZsh,
			"FISH":       completion.ShellFish,
			"POWERSHELL": completion.ShellPowerShell,
		}

		for input, expected := range cases {
			shell, err := completion.ParseShell(input)

			assert.NoError(t, err)
			assert.Equal(t, expected, shell)
		}
	})

	t.Run("rejects removed shells and aliases", func(t *testing.T) {
		t.Parallel()

		for _, input := range []string{"pwsh", "nu", "osh", "bash-ble", "bash_ble", "cmd-clink", "cmd_clink", "export", "ion", "tcsh", "oil", "xonsh"} {
			shell, err := completion.ParseShell(input)

			assert.Error(t, err)
			assert.Equal(t, completion.Shell(""), shell)
			assert.True(t, strings.Contains(err.Error(), "unknown shell"))
		}
	})

	t.Run("error message lists the supported shells", func(t *testing.T) {
		t.Parallel()

		_, err := completion.ParseShell("pwsh")

		assert.Error(t, err)
		assert.Equal(t, `unknown shell "pwsh", supported shells: bash, elvish, fish, nushell, powershell, zsh`, err.Error())
	})

	t.Run("trims leading and trailing whitespace", func(t *testing.T) {
		t.Parallel()

		shell, err := completion.ParseShell("  bash  ")

		assert.NoError(t, err)
		assert.Equal(t, completion.ShellBash, shell)
	})

	t.Run("unknown shell name", func(t *testing.T) {
		t.Parallel()

		shell, err := completion.ParseShell("unknown-shell")

		assert.Error(t, err)
		assert.Equal(t, completion.Shell(""), shell)
		assert.True(t, strings.Contains(err.Error(), "unknown shell"))
	})
}

func TestSupportedShells(t *testing.T) {
	t.Parallel()

	shells := completion.SupportedShells()

	assert.Equal(t, []string{"bash", "elvish", "fish", "nushell", "powershell", "zsh"}, shells)
}

func TestGenerate_AllShells(t *testing.T) {
	t.Parallel()

	for _, shell := range completion.SupportedShells() {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			opts := completion.DriverOptions{
				CommandName: "myapp",
				Aliases:     []string{"ma", "app"},
			}

			out, err := completion.Generate(completion.Shell(shell), opts)

			assert.NoError(t, err)
			assert.True(t, out != "")
			assert.True(t, strings.Contains(out, "myapp"))
		})
	}
}
