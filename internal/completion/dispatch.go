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
	{shell: ShellBash, name: "bash", description: "Bourne Again SHell", newSession: bashSession, snippet: bash.Snippet},
	{shell: ShellElvish, name: "elvish", description: "Elvish shell", session: statelessSession{format: elvish.Format}, snippet: elvish.Snippet},
	{shell: ShellFish, name: "fish", description: "Friendly interactive shell", session: statelessSession{format: fish.Format}, snippet: fish.Snippet},
	{shell: ShellNushell, name: "nushell", description: "Nushell shell", execNames: []string{"nu", "nushell"}, session: statelessSession{format: nushell.Format, patch: patchNushell}, snippet: nushell.Snippet},
	{shell: ShellPowerShell, name: "powershell", description: "PowerShell / pwsh", execNames: []string{"pwsh"}, session: statelessSession{format: powershell.Format}, snippet: powershell.Snippet},
	{shell: ShellZsh, name: "zsh", description: "Z shell", session: statelessSession{format: zsh.Format}, snippet: zsh.Snippet},
}

// EnvLookup returns the value of an environment variable and whether it was set.
type EnvLookup = bash.EnvLookup

// Session is one completion invocation's driver: Patch derives the shell's
// presentation inputs and Format consumes them, so concurrent completions never
// share state. Unknown identifiers — and the empty auto-detect form — render in
// the export framing.
type Session struct {
	driver *shellDriver
	state  driverSession
}

// NewSession returns the invocation-scoped driver for targetShell.
func NewSession(targetShell string) *Session {
	return NewSessionWithEnv(targetShell, nil)
}

// NewSessionWithEnv returns the invocation-scoped driver for targetShell using
// lookup to resolve environment variables.
func NewSessionWithEnv(targetShell string, lookup EnvLookup) *Session {
	driver := findDriver(normalizeShell(targetShell))
	if driver == nil {
		return &Session{}
	}

	return &Session{driver: driver, state: driver.open(lookup)}
}

// Patch applies shell-specific completion argument patching: bash's redirect
// rejection and wordbreak trimming, and nushell's raw-span dequoting. Every
// other shell passes the arguments through unchanged.
func (s *Session) Patch(args []string) ([]string, error) {
	if s.driver == nil {
		return args, nil
	}

	return s.state.Patch(args)
}

// Format renders a completion result in the framing of the session's shell. The
// protocol shells render natively; an unknown, removed, or empty identifier
// falls back to the export framing.
func (s *Session) Format(toComplete string, result predict.Result) string {
	candidates, directives := wire.FromResult(result)

	if s.driver == nil {
		return export.Format(toComplete, directives, candidates)
	}

	return s.state.Format(toComplete, directives, candidates)
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

// driverSession is the per-invocation state contract of one shell driver: Patch
// derives the shell's presentation inputs, Format consumes them. Drivers without
// cross-pass state share one statelessSession value.
type driverSession interface {
	Patch(args []string) ([]string, error)
	Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string
}

// shellDriver is one entry of the shell driver registry: the shell's
// identifier metadata plus its framing, snippet, and per-invocation session
// hooks. It is the single source of truth for shell definitions — dispatch
// routing (NewSession, Snippet), SupportedShells, ParseShell,
// ShellDescriptions, and exec-path detection all read from the same table.
type shellDriver struct {
	shell       Shell
	name        string
	description string
	execNames   []string
	// session is the shared stateless session (every shell but bash); exactly
	// one of session/newSession is set.
	session driverSession
	// newSession mints the per-invocation session of a stateful driver.
	newSession func(lookup EnvLookup) driverSession
	snippet    func(name string, aliases []string) string
}

// open returns the driver session of one completion invocation.
func (d *shellDriver) open(lookup EnvLookup) driverSession {
	if d.newSession != nil {
		return d.newSession(lookup)
	}

	return d.session
}

// statelessSession adapts a driver whose Patch and Format hooks share no state
// to the driverSession contract.
type statelessSession struct {
	format func(toComplete string, directives wire.Directives, candidates []wire.Candidate) string
	patch  func(args []string) ([]string, error)
}

// Patch applies the driver's argument patch, or passes the arguments through
// when the driver declares no patch hook.
func (s statelessSession) Patch(args []string) ([]string, error) {
	if s.patch == nil {
		return args, nil
	}

	return s.patch(args)
}

// Format renders the driver's framing.
func (s statelessSession) Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	return s.format(toComplete, directives, candidates)
}

// bashSession adapts the bash driver's per-invocation session to the registry
// contract.
func bashSession(lookup EnvLookup) driverSession {
	return bash.NewSessionWithEnv(lookup)
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
