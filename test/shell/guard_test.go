//go:build shelltest

package shell_test

import (
	"os/exec"
	"testing"
)

// TestShellSuiteRequiresAtLeastOneShell fails loudly when none of the
// integration shells are installed, instead of letting every driver silently
// skip and report a vacuously green run.
func TestShellSuiteRequiresAtLeastOneShell(t *testing.T) {
	t.Parallel()

	shells := []string{"bash", "zsh", "fish", "pwsh", "powershell", "nu", "nushell", "elvish"}

	for _, interpreter := range shells {
		if _, err := exec.LookPath(interpreter); err == nil {
			return
		}
	}

	t.Fatalf("no supported shell found on PATH (tried %v); every completion driver would silently skip — install at least one of: bash, zsh, fish, pwsh/powershell, nu/nushell, elvish", shells)
}
