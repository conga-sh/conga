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

	workspace := setupCloudctl(t)

	optSetup := `
shopt -s extglob
shopt -s nocaseglob 2>/dev/null || true
`

	t.Run("subcommands", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "cloudctl", "cloudctl ", optSetup)
		assert.True(t, result.HasItem("deploy"))
		assert.True(t, result.HasItem("logs"))
		assert.True(t, result.HasItem("completion"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("flags", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "cloudctl", "cloudctl deploy --", optSetup)
		assert.True(t, result.HasItem("--env"))
		assert.True(t, result.HasItem("--cluster"))
		assert.True(t, result.HasItem("--dry-run"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("dynamic_context_predictors", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletionWithSetup(t, "cloudctl", "cloudctl deploy --env production --cluster prod-", optSetup)
		assert.True(t, result.HasItem("prod-us-east-1"))
		assert.True(t, result.HasItem("prod-eu-west-1"))
		assert.False(t, result.HasItem("staging-cluster-alpha"))
		assert.Equal(t, "", result.StdoutLeak)
	})
}

// TestBash_AliasInvocation tests completion when the command is invoked through a bash alias.
func TestBash_AliasInvocation(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	aliasTarget := filepath.Join(workspace.BinDir, "c")
	assert.NoError(t, os.Symlink(filepath.Join(workspace.BinDir, "cloudctl"), aliasTarget))

	aliasSetup := `
alias c=cloudctl
complete -F _cloudctl_completion c
`
	result := workspace.RunBashCompletionWithSetup(t, "cloudctl", "c deploy --env ", aliasSetup)

	assert.True(t, result.HasItem("staging"))
	assert.True(t, result.HasItem("production"))
	assert.True(t, result.HasItem("qa"))
	assert.Equal(t, "", result.StdoutLeak)
}

// TestBash_NestedSubcommands tests completion for the built-in completion subcommand.
func TestBash_NestedSubcommands(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("completion shell candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl completion ")

		assert.True(t, result.HasItem("bash"))
		assert.True(t, result.HasItem("zsh"))
		assert.True(t, result.HasItem("fish"))
		assert.True(t, result.HasItem("powershell"))
		assert.True(t, result.HasItem("elvish"))
		assert.True(t, result.HasItem("nushell"))
	})

	t.Run("completion prefix matching", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl completion z")

		assert.True(t, result.HasItem("zsh"))
		assert.False(t, result.HasItem("bash"))
	})
}

// TestBash_MenuAndNoSpace tests bash-specific nospace/menu behaviour for values.
func TestBash_MenuAndNoSpace(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	t.Run("directory_nospace", func(t *testing.T) {
		t.Parallel()

		workspace.Mkdir(t, "build")
		workspace.Mkdir(t, "dist")

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl deploy --out-dir ")

		assert.True(t, result.HasItem("build/"))
		assert.True(t, result.HasItem("dist/"))
	})

	t.Run("multiparts_nospace", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "cloudctl", "cloudctl logs --labels team=")

		assert.True(t, result.HasItem("core") || result.HasItem(`"core"`))
	})
}
