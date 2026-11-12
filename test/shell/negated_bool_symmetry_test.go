//go:build shelltest

package shell_test

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// dryRunHelp is the help text of the shell-completion deploy --dry-run flag,
// which completion shows as a usage line for flags that take a value.
const dryRunHelp = "Simulate deployment"

// TestNegatedBoolCompletionSymmetry pins negated bool completion in every
// primary shell: the attached value position of a negated bool offers nothing
// and leaves the buffer unchanged, because a negated bool takes no value.
func TestNegatedBoolCompletionSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)

	t.Run("bash/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "shell-completion", "shell-completion deploy --no-dry-run=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "shell-completion deploy --no-dry-run=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("zsh/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "shell-completion", "shell-completion deploy --no-dry-run=")

		assert.Equal(t, 0, len(result.RawItems))
		assert.False(t, slices.Contains(result.Messages, dryRunHelp), "a negated bool has no value to describe")
		assert.Equal(t, "shell-completion deploy --no-dry-run=", result.BufferTab1)
	})

	t.Run("fish/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "shell-completion", "shell-completion deploy --no-dry-run=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "shell-completion deploy --no-dry-run=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "shell-completion", "shell-completion deploy --no-dry-run=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "shell-completion deploy --no-dry-run=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}
