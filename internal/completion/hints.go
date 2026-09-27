package completion

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/slicex"
)

const bashUpgradeNote = "Bash completion requires bash 4.4+. macOS's bundled /bin/bash is still 3.2.57,\nso install a newer bash via Homebrew (or use zsh, the default since Catalina).\n"

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
	// Homebrew is true when the command was installed via Homebrew on macOS,
	// tailoring the zsh lazy-load target to Homebrew's zsh site-functions dir.
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

// DetectHomebrew reports whether the current binary was installed via Homebrew
// on macOS, detected by resolving the executable path and looking for a
// Homebrew layout. Returns false on all other platforms.
func DetectHomebrew() bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	exe, err := os.Executable()
	if err != nil {
		return false
	}

	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	return isHomebrewPath(exe)
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
// commands for the detected shell, an upgrade warning when applicable, and a
// pointer to the per-shell help for persistence options. The useFlag parameter
// formats the output using either subcommand syntax or flag syntax.
func NowSection(current CurrentShell, cmdName string, useFlag bool) string {
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

	builder.WriteString("You will need to start a new shell for the setup to take effect.\n")
}

func writeElvishInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellElvish, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, append the following to ~/.config/elvish/rc.elv:", []string{
		"echo 'eval (" + completionInvocation(cmdName, ShellElvish, opts.UseFlag) + " | slurp)' >> ~/.config/elvish/rc.elv",
	})

	builder.WriteString("You will need to start a new shell for the setup to take effect.\n")
}

func writeFishInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellFish, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, run once:", []string{
		"mkdir -p ~/.config/fish/completions",
		completionInvocation(cmdName, ShellFish, opts.UseFlag) + " > ~/.config/fish/completions/" + cmdName + ".fish",
	})

	builder.WriteString("You will need to start a new shell for the setup to take effect.\n")
}

func writeNushellInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	autoloadPath := "($nu.default-config-dir | path join \"autoload/" + cmdName + ".nu\")"

	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellNushell, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, save the completion script once:", []string{
		"mkdir ($nu.default-config-dir | path join \"autoload\")",
		completionInvocation(cmdName, ShellNushell, opts.UseFlag) + " | save -f " + autoloadPath,
	})

	builder.WriteString("You will need to start a new shell for the setup to take effect.\n")
}

func writePowerShellInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "To load completions in your current PowerShell session, run:", SessionCommands(ShellPowerShell, cmdName, opts.UseFlag))
	writeStep(builder, "To load completions in every new session, add the following to your PowerShell profile ($PROFILE):", []string{
		completionInvocation(cmdName, ShellPowerShell, opts.UseFlag) + " | Out-String | Invoke-Expression",
	})

	builder.WriteString("You will need to start a new PowerShell session for the setup to take effect.\n")
}

func writeZshInstructions(builder *strings.Builder, cmdName string, opts EnableOptions) {
	writeStep(builder, "zsh's completion system (compinit) must be active for completions to load. If tab completion doesn't work, enable it once:", []string{
		"echo 'autoload -U compinit; compinit' >> ~/.zshrc",
	})

	writeStep(builder, "To load completions in your current shell session, run:", SessionCommands(ShellZsh, cmdName, opts.UseFlag))

	if opts.Homebrew {
		writeStep(builder, "To load completions in every new session, write the completion file once:", []string{
			completionInvocation(cmdName, ShellZsh, opts.UseFlag) + " > \"$(brew --prefix)/share/zsh/site-functions/_" + cmdName + "\"",
		})
	} else {
		writeStep(builder, "To load completions in every new session, append the following to ~/.zshrc:", []string{
			"echo 'source <(" + completionInvocation(cmdName, ShellZsh, opts.UseFlag) + ")' >> ~/.zshrc",
		})

		writeStep(builder, "For faster startup, write the completion file once instead:", []string{
			"mkdir -p ~/.zfunc",
			completionInvocation(cmdName, ShellZsh, opts.UseFlag) + " > ~/.zfunc/_" + cmdName,
			"echo 'fpath=(~/.zfunc $fpath)' >> ~/.zshrc",
		})
	}

	builder.WriteString("You will need to start a new shell for the setup to take effect.\n")
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
