//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

// The PowerShell snippet passes an empty argument as an empty single-quoted
// string under Standard argument passing, and as a single-quoted pair of
// double quotes under Legacy (and Windows PowerShell 5.1), which drops a bare
// empty string.
func TestPowerShell_ArgumentPassingModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"Standard", "Legacy"} {
		setup := "$PSNativeCommandArgumentPassing = '" + mode + "'"

		t.Run(mode+" bare space completes the next argument", func(t *testing.T) {
			t.Parallel()

			workspace := setupShellCompletion(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "shell-completion", "shell-completion logs ", setup)

			assert.Equal(t, "", result.Stderr)
			assert.True(t, result.HasItem("auth"), "expected the service positional")
			assert.True(t, result.HasItem("frontend"), "expected the service positional")
			assert.False(t, result.HasItem("logs"), "the empty trailing word must reach the engine, not re-complete logs")
		})

		t.Run(mode+" open quote completes the next argument", func(t *testing.T) {
			t.Parallel()

			workspace := setupShellCompletion(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "shell-completion", "shell-completion logs '", setup)

			assert.Equal(t, "", result.Stderr)
			assert.True(t, result.HasItem("auth"), "expected the service positional")
			assert.True(t, result.HasItem("frontend"), "expected the service positional")
		})

		t.Run(mode+" empty element keeps its position", func(t *testing.T) {
			t.Parallel()

			workspace := setupShellCompletion(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "shell-completion", "shell-completion deploy --env '' --", setup)

			assert.Equal(t, "", result.Stderr)
			assert.True(t, result.HasItem("--region"), "expected deploy flags after the empty --env value")
			assert.False(t, result.HasItem("--env"), "the empty element must count as --env's value")
		})
	}
}
