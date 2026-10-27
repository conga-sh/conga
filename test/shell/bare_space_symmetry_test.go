//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/test/shell"
)

// TestBareSpaceFlagSymmetry pins the shell-uniform bare-space flag behaviour:
// every shell offers the positional arguments at a bare space and withholds the
// flags, and the dash prefix restores them.
func TestBareSpaceFlagSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupCloudctl(t)

	for _, shellName := range []string{"bash", "zsh", "fish", "powershell", "elvish", "nushell"} {
		t.Run(shellName+"/bare_space_withholds_flags", func(t *testing.T) {
			t.Parallel()

			bare := runShellCompletion(t, workspace, shellName, "cloudctl logs ")

			assert.True(t, bare("auth"), "expected the positional argument at a bare space")
			assert.True(t, bare("billing"), "expected the positional argument at a bare space")
			assert.False(t, bare("--env"), "flags are withheld at a bare space")
			assert.False(t, bare("--follow"), "flags are withheld at a bare space")
		})

		t.Run(shellName+"/dash_prefix_restores_flags", func(t *testing.T) {
			t.Parallel()

			dash := runShellCompletion(t, workspace, shellName, "cloudctl logs --")

			assert.True(t, dash("--env"), "the dash prefix offers flags")
			assert.True(t, dash("--follow"), "the dash prefix offers flags")
		})
	}
}

// runShellCompletion runs one completion through the named shell driver and
// returns a predicate over the candidate values.
func runShellCompletion(t *testing.T, workspace *shell.Workspace, shellName, cmdLine string) func(string) bool {
	t.Helper()

	switch shellName {
	case "bash":
		return workspace.RunBashCompletion(t, "cloudctl", cmdLine).HasItem
	case "zsh":
		return workspace.RunZshCompletion(t, "cloudctl", cmdLine).HasItem
	case "fish":
		return workspace.RunFishCompletion(t, "cloudctl", cmdLine).HasItem
	case "powershell":
		return workspace.RunPowerShellCompletion(t, "cloudctl", cmdLine).HasItem
	case "elvish":
		return workspace.RunElvishCompletion(t, "cloudctl", cmdLine).HasItem
	case "nushell":
		return workspace.RunNushellCompletion(t, "cloudctl", cmdLine).HasItem
	default:
		assertx.Unreachable(t, "unknown shell "+shellName)

		return nil
	}
}
