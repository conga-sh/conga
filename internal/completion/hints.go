package completion

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/slicex"
)

const (
	bashUpgradeNote = "Bash completion requires bash 4.4+. macOS's bundled /bin/bash is still 3.2.57,\nso install a newer bash via Homebrew (or use zsh, the default since Catalina).\n"

	// restartNote closes every shell's instructions.
	restartNote = "Note: restart your shell for the changes to take effect.\n"
)

// ShellInfo pairs a supported shell identifier with a short human description.
type ShellInfo struct {
	Name        string
	Description string
}

// CurrentShell reports the detected caller shell and the executable path used
// to identify it, so callers can tailor hints to the actual running shell
// (e.g. macOS's bundled bash versus a Homebrew bash).
type CurrentShell struct {
	Shell    Shell
	ExecPath string
}

// EnableOptions tailors the enable-instructions block to the caller's setup.
type EnableOptions struct {
	// Homebrew is true when the user's shell runs brew shellenv, which puts
	// $HOMEBREW_PREFIX/share/zsh/site-functions on zsh's fpath; the zsh
	// faster-startup step then writes there instead of ~/.zfunc.
	Homebrew bool

	// BashNeedsUpgrade is true when the caller's shell is macOS's bundled bash
	// (3.2.57), which lacks the builtins the generated bash script requires.
	BashNeedsUpgrade bool

	// UseFlag is true when instructions should be formatted using the
	// --completion flag syntax rather than the completion subcommand.
	UseFlag bool
}

// ShellDescriptions returns the canonically supported shells with a short
// description for each, used by the completion predictor and help screens.
func ShellDescriptions() []ShellInfo {
	return slicex.Map(shellDrivers, func(driver shellDriver) ShellInfo {
		return ShellInfo{Name: driver.name, Description: driver.description}
	})
}

// DetectCurrentShell identifies the caller's shell by walking the parent
// process ancestry and matching executable names against the supported shells.
// It does not rely on $SHELL (which reflects the login shell, not the shell
// running the command) and honours no environment overrides.
func DetectCurrentShell() (CurrentShell, bool) {
	pid := os.Getppid()

	for pid > 1 {
		ppid, execPath, err := parentProcess(pid)
		if err == nil {
			// Match before the break check: Windows walks ancestors internally
			// and returns the matched shell executable with ppid 0, and an
			// unmatched empty exec path falls through to the break conditions unchanged.
			if shell := shellFromExecPath(execPath); shell != "" {
				return CurrentShell{Shell: shell, ExecPath: execPath}, true
			}
		}

		if err != nil || ppid <= 0 || ppid == pid {
			break
		}

		pid = ppid
	}

	return CurrentShell{}, false
}

// DetectHomebrew reports whether the user's shell environment comes from
// brew shellenv, on macOS or Linux, by the HOMEBREW_PREFIX variable it
// exports. brew shellenv also adds Homebrew's zsh site-functions directory to
// fpath, so a completion file written there loads without editing ~/.zshrc.
func DetectHomebrew() bool {
	return os.Getenv("HOMEBREW_PREFIX") != ""
}

// InferBashNeedsUpgrade inspects the executable path of a detected bash shell
// to infer whether it predates the features the generated bash script requires
// (bash 4.4+). The bundled macOS bash lives at /bin/bash and is still 3.2.57;
// a Homebrew-installed bash is modern. known is false when the version cannot
// be determined from the path alone, so callers warn only on positive evidence.
func InferBashNeedsUpgrade(execPath string) (bool, bool) {
	if execPath == "" {
		return false, false
	}

	base := strings.TrimPrefix(filepath.Base(execPath), "-")
	if base != "bash" {
		return false, true
	}

	switch filepath.Clean(execPath) {
	case "/bin/bash", "/usr/bin/bash":
		return true, true
	}

	// A login shell's argv[0] carries a leading dash ("-bash"); on macOS this
	// is always the bundled shell, since the terminal launches /bin/bash.
	if strings.HasPrefix(execPath, "-") {
		return true, true
	}

	if isHomebrewPath(execPath) {
		return false, true
	}

	return false, false
}

// ShouldWarnBash reports whether the enable hints should call out an outdated
// bash. This only ever applies on macOS, where the bundled bash is 3.2.57.
func ShouldWarnBash(current CurrentShell) bool {
	if runtime.GOOS != "darwin" || current.Shell != ShellBash {
		return false
	}

	old, known := InferBashNeedsUpgrade(current.ExecPath)

	return old && known
}

// SessionCommands returns the one-time commands that enable completions,
// formatted for either subcommand syntax or flag syntax.
func SessionCommands(shell Shell, cmdName string, useFlag bool) []string {
	switch shell {
	case ShellBash:
		return []string{"source <(" + completionInvocation(cmdName, ShellBash, useFlag) + ")"}
	case ShellElvish:
		return []string{"eval (" + completionInvocation(cmdName, ShellElvish, useFlag) + " | slurp)"}
	case ShellFish:
		return []string{completionInvocation(cmdName, ShellFish, useFlag) + " | source"}
	case ShellNushell:
		autoloadFile := "($nu.default-config-dir | path join \"autoload/" + cmdName + ".nu\")"

		return []string{
			"mkdir ($nu.default-config-dir | path join \"autoload\"); " + completionInvocation(cmdName, ShellNushell, useFlag) + " | save -f " + autoloadFile,
			"source " + autoloadFile,
		}
	case ShellPowerShell:
		return []string{completionInvocation(cmdName, ShellPowerShell, useFlag) + " | Out-String | Invoke-Expression"}
	case ShellZsh:
		return []string{"source <(" + completionInvocation(cmdName, ShellZsh, useFlag) + ")"}
	default:
		return nil
	}
}

// NowSection renders the "enable for your current shell now" block: the session
// commands for the detected shell, an upgrade warning when applicable, and,
// when helpAvailable is true, a pointer to the per-shell help for persistence
// options. The useFlag parameter formats the output using either subcommand
// syntax or flag syntax. When the completion command's help flag is disabled,
// the pointer is omitted because the referenced invocation is a usage error.
func NowSection(current CurrentShell, cmdName string, useFlag bool, helpAvailable bool) string {
	var builder strings.Builder

	builder.WriteString("\nEnable completions for your current ")
	builder.WriteString(string(current.Shell))
	builder.WriteString(" shell now:\n\n")

	if ShouldWarnBash(current) {
		builder.WriteString(bashUpgradeNote)
		builder.WriteString("\n")
	}

	for _, line := range SessionCommands(current.Shell, cmdName, useFlag) {
		builder.WriteByte('\t')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	if !helpAvailable {
		return builder.String()
	}

	builder.WriteString("\nFor persistent setups, run '")
	builder.WriteString(cmdName)

	if useFlag {
		builder.WriteString(" --completion ")
	} else {
		builder.WriteString(" completion ")
	}

	builder.WriteString(string(current.Shell))
	builder.WriteString(" --help'.\n")

	return builder.String()
}

// EnableInstructions renders the full enable block for one shell: session and
// persistent commands, plus platform-specific notes, in prose with
// tab-indented commands to copy-paste.
func EnableInstructions(shell Shell, cmdName string, opts EnableOptions) string {
	var builder strings.Builder

	switch shell {
	case ShellBash:
		writeBashInstructions(&builder, cmdName, opts)
	case ShellElvish:
		writeElvishInstructions(&builder, cmdName, opts)
	case ShellFish:
		writeFishInstructions(&builder, cmdName, opts)
	case ShellNushell:
		writeNushellInstructions(&builder, cmdName, opts)
	case ShellPowerShell:
		writePowerShellInstructions(&builder, cmdName, opts)
	case ShellZsh:
		writeZshInstructions(&builder, cmdName, opts)
	default:
		// Unknown shells render no instructions.
	}

	return builder.String()
}

func completionInvocation(cmdName string, shell Shell, useFlag bool) string {
	if useFlag {
		return cmdName + " --completion " + string(shell)
	}

	return cmdName + " completion " + string(shell)
}

// writeStep renders one instruction step: a prose lead-in, a blank line, and
// the tab-indented commands to run.
func writeStep(builder *strings.Builder, intro string, lines []string) {
	builder.WriteString(intro)
	builder.WriteString("\n\n")

	for _, line := range lines {
		builder.WriteByte('\t')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	builder.WriteByte('\n')
}

func writeBashInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	if opts.BashNeedsUpgrade {
		builder.WriteString(bashUpgradeNote)
		builder.WriteString("\n")
	}

	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellBash, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, append the following to ~/.bashrc:", []string{
		"echo 'source <(" + completionInvocation(cmdName, ShellBash, opts.UseFlag) + ")' >> ~/.bashrc",
	})
	writeStep(builder, "For faster startup with the bash-completion package installed, write the completion file once instead. bash-completion loads it the first time you press Tab:", []string{
		"mkdir -p ~/.local/share/bash-completion/completions",
		completionInvocation(cmdName, ShellBash, opts.UseFlag) + " > ~/.local/share/bash-completion/completions/" + cmdName,
	})

	builder.WriteString(restartNote)
}

func writeElvishInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellElvish, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, add the session command to ~/.config/elvish/rc.elv. Run once:", []string{
		"mkdir -p ~/.config/elvish",
		"echo 'eval (" + completionInvocation(cmdName, ShellElvish, opts.UseFlag) + " | slurp)' >> ~/.config/elvish/rc.elv",
	})

	builder.WriteString(restartNote)
}

func writeFishInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellFish, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, run once:", []string{
		"mkdir -p ~/.config/fish/completions",
		completionInvocation(cmdName, ShellFish, opts.UseFlag) + " > ~/.config/fish/completions/" + cmdName + ".fish",
	})

	builder.WriteString(restartNote)
}

func writeNushellInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	autoloadPath := "($nu.default-config-dir | path join \"autoload/" + cmdName + ".nu\")"

	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellNushell, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, save the completion script once. Nushell 0.102 or later loads it at startup:", []string{
		"mkdir ($nu.default-config-dir | path join \"autoload\")",
		completionInvocation(cmdName, ShellNushell, opts.UseFlag) + " | save -f " + autoloadPath,
	})

	builder.WriteString(restartNote)
}

func writePowerShellInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current PowerShell session, run:", SessionCommands(ShellPowerShell, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, add the session command to your PowerShell profile ($PROFILE), creating the profile if needed:", []string{
		"if (!(Test-Path $PROFILE)) { New-Item -ItemType File -Path $PROFILE -Force | Out-Null }",
		"Add-Content -Path $PROFILE -Value '" + completionInvocation(cmdName, ShellPowerShell, opts.UseFlag) + " | Out-String | Invoke-Expression'",
	})
	writeStep(builder, "If PowerShell reports that running scripts is disabled when it loads the profile, allow local scripts for your account:", []string{
		"Set-ExecutionPolicy -Scope CurrentUser RemoteSigned",
	})

	builder.WriteString(restartNote)
}

func writeZshInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "zsh's completion system (compinit) must be active for completions to load. If tab completion doesn't work, enable it once:", []string{
		"echo 'autoload -U compinit; compinit' >> ~/.zshrc",
	})

	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellZsh, cmdName, opts.UseFlag))

	writeStep(builder, "To load completions in every new session, append the following to ~/.zshrc:", []string{
		"echo 'source <(" + completionInvocation(cmdName, ShellZsh, opts.UseFlag) + ")' >> ~/.zshrc",
	})

	if opts.Homebrew {
		writeStep(builder, "For faster startup, write the completion file once instead, into the directory brew shellenv adds to fpath:", []string{
			completionInvocation(cmdName, ShellZsh, opts.UseFlag) + " > \"$HOMEBREW_PREFIX/share/zsh/site-functions/_" + cmdName + "\"",
		})
	} else {
		writeStep(builder, "For faster startup, write the completion file once instead:", []string{
			"mkdir -p ~/.zfunc",
			completionInvocation(cmdName, ShellZsh, opts.UseFlag) + " > ~/.zfunc/_" + cmdName,
		})
		writeStep(builder, "Make sure ~/.zshrc adds ~/.zfunc to fpath before compinit runs, since compinit reads fpath only when it starts:", []string{
			"fpath=(~/.zfunc $fpath)",
		})
	}

	builder.WriteString(restartNote)
}

func shellFromExecPath(execPath string) Shell {
	trimmed := strings.TrimSpace(execPath)
	if index := strings.LastIndexAny(trimmed, `/\`); index >= 0 {
		trimmed = trimmed[index+1:]
	}

	base := strings.TrimPrefix(trimmed, "-")
	base = strings.TrimSuffix(base, ".exe")

	for _, driver := range shellDrivers {
		if driver.name == base || slices.Contains(driver.execNames, base) {
			return driver.shell
		}
	}

	return ""
}

func isHomebrewPath(path string) bool {
	clean := filepath.Clean(path)

	return strings.Contains(clean, "/homebrew/") || strings.Contains(clean, "/Cellar/")
}
