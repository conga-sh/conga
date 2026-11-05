//go:build shelltest

package shell_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// TestBash_FrameworkCompatibility tests completion behaviour under common bash configurations.
func TestBash_FrameworkCompatibility(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	optSetup := `
shopt -s extglob
shopt -s nocaseglob 2>/dev/null || true
`

	t.Run("subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "shell-completion", "shell-completion ", optSetup)
		assert.True(t, result.HasItem("deploy"))
		assert.True(t, result.HasItem("logs"))
		assert.True(t, result.HasItem("completion"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "shell-completion", "shell-completion deploy --", optSetup)
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("dynamic_context_predictors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "shell-completion", "shell-completion deploy --env production --cluster prod-", optSetup)
		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

// TestBash_AliasInvocation tests completion when the command is invoked through a bash alias.
func TestBash_AliasInvocation(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	aliasTarget := filepath.Join(workspace.BinDir, "c")
	assert.NoError(t, os.Symlink(filepath.Join(workspace.BinDir, "shell-completion"), aliasTarget))

	aliasSetup := `
alias c=shell-completion
complete -F _shell_completion_completion c
`
	result := workspace.RunBashCompletionWithSetup(t, "shell-completion", "c deploy --env ", aliasSetup)

	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
	assert.Equal(t, "", result.StdoutLeak)
}

// TestBash_NestedSubcommands tests completion for the built-in completion subcommand.
func TestBash_NestedSubcommands(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	t.Run("completion shell candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion completion ")

		assert.True(t, result.HasItem("bash"))
		assert.True(t, result.HasItem("zsh"))
		assert.True(t, result.HasItem("fish"))
		assert.True(t, result.HasItem("powershell"))
		assert.True(t, result.HasItem("elvish"))
		assert.True(t, result.HasItem("nushell"))
	})

	t.Run("completion prefix matching", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion completion z")

		assert.True(t, result.HasItem("zsh"))
		assert.False(t, result.HasItem("bash"))
	})
}

// TestBash_MenuAndNoSpace tests bash-specific nospace/menu behaviour for values.
func TestBash_MenuAndNoSpace(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	t.Run("directory_nospace", func(t *testing.T) {
		t.Parallel()

		workspace.Mkdir(t, "build")
		workspace.Mkdir(t, "dist")

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion deploy --out-dir ")

		assert.True(t, result.HasItem("build/"))
		assert.True(t, result.HasItem("dist/"))
	})

	t.Run("multiparts_nospace", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion logs --labels team=")

		assert.True(t, result.HasItem("core") || result.HasItem(`"core"`))
	})
}
