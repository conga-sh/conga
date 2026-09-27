package completion

import (
	"fmt"
	"strings"

	"github.com/conga-sh/conga/internal/completion/bash"
	"github.com/conga-sh/conga/internal/completion/elvish"
	"github.com/conga-sh/conga/internal/completion/export"
	"github.com/conga-sh/conga/internal/completion/fish"
	"github.com/conga-sh/conga/internal/completion/nushell"
	"github.com/conga-sh/conga/internal/completion/powershell"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/completion/zsh"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

//nolint:gochecknoglobals // internal single source of truth for shell driver definitions
var shellDrivers = []shellDriver{
	{shell: ShellBash, name: "bash", description: "Bourne Again SHell", format: bash.Format, snippet: bash.Snippet, patch: bash.Patch},
	{shell: ShellElvish, name: "elvish", description: "Elvish shell", format: elvish.Format, snippet: elvish.Snippet},
	{shell: ShellFish, name: "fish", description: "Friendly interactive shell", format: fish.Format, snippet: fish.Snippet},
	{shell: ShellNushell, name: "nushell", description: "Nushell shell", execNames: []string{"nu", "nushell"}, format: nushell.Format, snippet: nushell.Snippet, patch: patchNushell},
	{shell: ShellPowerShell, name: "powershell", description: "PowerShell / pwsh", execNames: []string{"pwsh"}, format: powershell.Format, snippet: powershell.Snippet},
	{shell: ShellZsh, name: "zsh", description: "Z shell", format: zsh.Format, snippet: zsh.Snippet},
}

// Format renders a completion result in the framing of targetShell. The
// protocol shells render natively; any other identifier — unknown, removed,
// or the empty auto-detect form — falls back to the export framing.
func Format(targetShell string, toComplete string, result predict.Result) string {
	candidates, directives := wire.FromResult(result)

	driver := findDriver(normalizeShell(targetShell))
	if driver == nil {
		return export.Format(toComplete, directives, candidates)
	}

	return driver.format(toComplete, directives, candidates)
}

// Patch applies shell-specific completion argument patching: bash's redirect
// rejection and wordbreak trimming, and nushell's raw-span dequoting. Every
// other shell passes the arguments through unchanged.
func Patch(targetShell string, args []string) ([]string, error) {
	driver := findDriver(normalizeShell(targetShell))
	if driver == nil || driver.patch == nil {
		return args, nil
	}

	return driver.patch(args)
}

// Snippet renders the registration snippet for shellName ("" auto-detects the
// caller's shell). Only the protocol shells have registration snippets; any
// other identifier — including the export framing, which is a machine-
// readable entry-point target without a script — is rejected.
func Snippet(opts DriverOptions, shellName string) (string, error) {
	driver := findDriver(normalizeShell(shellName))
	if driver == nil {
		return "", fmt.Errorf("%w %q, supported shells: %s", sentinel.ErrUnknownShell, shellName, strings.Join(SupportedShells(), ", "))
	}

	return driver.snippet(opts.CommandName, opts.Aliases), nil
}

// SupportedShells returns the shells of the wire protocol
// (docs/adr/003-native-completion-driver.md). The export framing is an
// internal entry-point target and is not listed.
func SupportedShells() []string {
	return slicex.Map(shellDrivers, func(driver shellDriver) string {
		return driver.name
	})
}

// shellDriver is one entry of the shell driver registry: the shell's
// identifier metadata plus its framing, snippet, and patch hooks. It is the
// single source of truth for shell definitions — dispatch routing
// (Format/Patch/Snippet), SupportedShells, ParseShell, ShellDescriptions, and
// exec-path detection all read from the same table.
type shellDriver struct {
	shell       Shell
	name        string
	description string
	execNames   []string
	format      func(toComplete string, directives wire.Directives, candidates []wire.Candidate) string
	snippet     func(name string, aliases []string) string
	patch       func(args []string) ([]string, error)
}

// patchNushell adapts nushell.Patch (error-free signature) to the registry's
// patch hook signature.
func patchNushell(args []string) ([]string, error) {
	return nushell.Patch(args), nil
}

// findDriver returns the registry entry whose canonical name matches
// normalizedName, or nil when the identifier is unknown.
func findDriver(normalizedName string) *shellDriver {
	for i := range shellDrivers {
		if shellDrivers[i].name == normalizedName {
			return &shellDrivers[i]
		}
	}

	return nil
}

// normalizeShell normalizes a shell identifier for routing decisions.
func normalizeShell(targetShell string) string {
	return strings.ToLower(strings.TrimSpace(targetShell))
}
