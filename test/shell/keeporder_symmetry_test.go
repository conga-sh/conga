//go:build shelltest

package shell_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/test/shell"
)

// TestKeepOrderSymmetry pins candidate order in every shell for a KeepOrder
// predictor (logs --severity, declared debug, info, warn, error) and for a
// plain enum (logs --env, declared staging, production, qa).
//
// bash and zsh harnesses capture the candidates before the shell sorts them
// for display, so their order is the engine's either way; what differs is the
// instruction to the shell (compopt -o nosort, an unsorted -V group). The
// PTY tests check the order on screen. fish and PowerShell run the real
// shell, so their order is what the shell shows. The elvish and nushell
// harnesses read the completer's output, before any shell-side sorting.
func TestKeepOrderSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	severityOrder := []string{"debug", "info", "warn", "error"}

	t.Run("bash keeporder requests nosort", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, severityOrder, result.Items)
		assert.True(t, slices.Contains(result.Compopts, "-o nosort"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash plain enum leaves sorting to bash", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"staging", "production", "qa"}, result.Items)
		assert.False(t, slices.Contains(result.Compopts, "-o nosort"))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("zsh keeporder adds unsorted group", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, severityOrder, candidateValues(result.Groups["values"]))
		assert.Equal(t, []string{"describe:values", "compadd:values"}, result.Unsorted)
	})

	t.Run("zsh plain enum adds sorted group", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"staging", "production", "qa"}, candidateValues(result.Groups["values"]))
		assert.Zero(t, result.Unsorted)
	})

	t.Run("fish keeporder ignored", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, []string{"debug", "error", "info", "warn"}, candidateValues(result.Items))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("fish plain enum sorted", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"production", "qa", "staging"}, candidateValues(result.Items))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell keeporder kept", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, severityOrder, powerShellValues(result.Items))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell plain enum kept", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"staging", "production", "qa"}, powerShellValues(result.Items))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("elvish keeporder completer output order", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, severityOrder, candidateValues(result.Candidates))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("elvish plain enum completer output order", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunElvishCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"staging", "production", "qa"}, candidateValues(result.Candidates))
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("nushell keeporder completer output order", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunNushellCompletion(t, "shell-completion", "shell-completion logs --severity ")

		assert.Equal(t, severityOrder, candidateValues(result.Candidates))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})

	t.Run("nushell plain enum completer output order", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunNushellCompletion(t, "shell-completion", "shell-completion logs --env ")

		assert.Equal(t, []string{"staging", "production", "qa"}, candidateValues(result.Candidates))
		assert.Equal(t, "", result.Stderr, "completion must not write to stderr")
	})
}

func candidateValues(candidates []shell.Candidate) []string {
	values := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		values = append(values, strings.TrimSpace(candidate.Value))
	}

	return values
}

func powerShellValues(items []shell.PowerShellItem) []string {
	values := make([]string, 0, len(items))

	for _, item := range items {
		values = append(values, strings.TrimSpace(item.CompletionText))
	}

	return values
}
