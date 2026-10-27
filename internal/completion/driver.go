// Package completion provides shell script generation adapters for CONGA command line tools.
package completion

import (
	"fmt"
	"strings"

	"github.com/conga-sh/conga/internal/sentinel"
)

const (
	// ShellBash represents the GNU Bash shell.
	ShellBash Shell = "bash"

	// ShellElvish represents the Elvish shell.
	ShellElvish Shell = "elvish"

	// ShellFish represents the Friendly Interactive Shell.
	ShellFish Shell = "fish"

	// ShellNushell represents the Nushell shell.
	ShellNushell Shell = "nushell"

	// ShellPowerShell represents Microsoft PowerShell (5.1 & Core 7+).
	ShellPowerShell Shell = "powershell"

	// ShellZsh represents the Z Shell.
	ShellZsh Shell = "zsh"
)

// Shell represents a supported target shell environment for completion scripts.
type Shell string

// DriverOptions configures template rendering parameters for shell script generation.
type DriverOptions struct {
	CommandName string
	Aliases     []string
	// EnvPrefix is the application's normalized completion environment prefix
	// (for example TASK_), used by drivers whose generated script forwards
	// description and alias suppression to the binary.
	EnvPrefix string
}

// Generate renders the completion driver script for the specified shell and options.
// All shells render their registration snippet natively
// (docs/adr/003-native-completion-driver.md).
func Generate(shell Shell, opts DriverOptions) (string, error) {
	snippet, err := Snippet(opts, string(shell))
	if err != nil {
		return "", fmt.Errorf("generate completion script for %s: %w", shell, err)
	}

	return snippet, nil
}

// ParseShell parses a shell name string into a validated Shell enum.
// Only the canonically supported shells are accepted; experimental and alias
// names are rejected. Matching is case-insensitive and trims surrounding whitespace.
func ParseShell(name string) (Shell, error) {
	if driver := findDriver(strings.ToLower(strings.TrimSpace(name))); driver != nil {
		return driver.shell, nil
	}

	return "", fmt.Errorf("%w %q, supported shells: %s", sentinel.ErrUnknownShell, name, strings.Join(SupportedShells(), ", "))
}
