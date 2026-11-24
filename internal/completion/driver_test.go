package completion_test

import (
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/errlines"
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
			assert.Equal(t, fmt.Sprintf(`unknown shell %q`, input), err.Error())
			assert.Equal(t, "supported shells: bash, elvish, fish, nushell, powershell, zsh", errlines.Note(err))
		}
	})

	t.Run("error message lists the supported shells", func(t *testing.T) {
		t.Parallel()

		_, err := completion.ParseShell("pwsh")

		assert.Error(t, err)
		assert.Equal(t, `unknown shell "pwsh"`, err.Error())
		assert.Equal(t, "supported shells: bash, elvish, fish, nushell, powershell, zsh", errlines.Note(err))
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
		assert.Equal(t, `unknown shell "unknown-shell"`, err.Error())
		assert.Equal(t, "supported shells: bash, elvish, fish, nushell, powershell, zsh", errlines.Note(err))
	})
}

func TestSupportedShells(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"bash", "elvish", "fish", "nushell", "powershell", "zsh"}, completion.SupportedShells())
}
