//go:build shelltest

package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var ptyScriptExtensions = map[string]string{ //nolint:gochecknoglobals // Static shell-to-extension mapping shared across PTY tests.
	"bash":       ".sh",
	"zsh":        ".zsh",
	"fish":       ".fish",
	"powershell": ".ps1",
	"nushell":    ".nu",
	"elvish":     ".elv",
}

var ptyShellSpecs = map[string]ptyShellSpec{ //nolint:gochecknoglobals // Static launch and bootstrap registry for PTY-supported shells.
	"bash": {
		lookups:     []string{"bash"},
		envOverride: "CONGA_BASH_BIN",
		launch:      []string{"--noprofile", "--norc"},
		bootstrap:   ptyBashBootstrap,
	},
	"zsh": {
		lookups:     []string{"zsh"},
		envOverride: "CONGA_ZSH_BIN",
		launch:      []string{"-f"},
		bootstrap:   ptyZshBootstrap,
	},
	"fish": {
		lookups:     []string{"fish"},
		envOverride: "CONGA_FISH_BIN",
		launch:      []string{"--no-config"},
		bootstrap:   ptyFishBootstrap,
	},
	"powershell": {
		lookups:     []string{"pwsh", "powershell"},
		envOverride: "CONGA_PWSH_BIN",
		launch:      []string{"-NoProfile"},
		extraEnv:    []string{"POWERSHELL_UPDATECHECK=Off"},
		bootstrap:   ptyPowerShellBootstrap,
	},
	"nushell": {
		lookups:     []string{"nu", "nushell"},
		envOverride: "CONGA_NU_BIN",
		launch:      []string{"--no-config-file"},
		bootstrap:   ptyNushellBootstrap,
	},
	"elvish": {
		lookups:     []string{"elvish"},
		envOverride: "CONGA_ELVISH_BIN",
		writeRC:     ptyWriteElvishRC,
	},
}

// ptyShellSpec describes how to launch one interactive shell inside a PTY and
// how to register the CONGA completion driver plus a deterministic prompt in it.
type ptyShellSpec struct {
	lookups     []string
	envOverride string
	launch      []string
	// extraEnv is appended to the pane environment for this shell only.
	extraEnv []string
	// bootstrap is typed at the first prompt to register completion and the
	// sentinel prompt. Shells with no runtime-evaluable bootstrap (elvish)
	// use writeRC instead and keep bootstrap nil.
	bootstrap func(t *testing.T, shellPath, scriptPath string) string
	writeRC   func(t *testing.T, workspace *Workspace, scriptPath string)
}

func ptyBashBootstrap(t *testing.T, shellPath, scriptPath string) string {
	t.Helper()

	return fmt.Sprintf("source %s; PS1=%q", scriptPath, ptyPrompt)
}

func ptyZshBootstrap(t *testing.T, shellPath, scriptPath string) string {
	t.Helper()

	dumpPath := getZshDumpFile(t, shellPath)

	return fmt.Sprintf("autoload -Uz compinit; compinit -C -d %s; source %s; PROMPT=%q", dumpPath, scriptPath, ptyPrompt)
}

func ptyFishBootstrap(t *testing.T, shellPath, scriptPath string) string {
	t.Helper()

	return fmt.Sprintf("source %s; function fish_prompt; printf %q; end", scriptPath, ptyPrompt)
}

func ptyPowerShellBootstrap(t *testing.T, shellPath, scriptPath string) string {
	t.Helper()

	return fmt.Sprintf(". %s; function prompt { %q }", scriptPath, ptyPrompt)
}

func ptyNushellBootstrap(t *testing.T, shellPath, scriptPath string) string {
	t.Helper()

	return fmt.Sprintf(
		"source %s; $env.PROMPT_COMMAND = {|| %q }; $env.PROMPT_INDICATOR = ''; $env.PROMPT_COMMAND_RIGHT = {|| '' }",
		scriptPath, ptyPrompt,
	)
}

// ptyWriteElvishRC registers the completer and prompt via rc.elv: elvish has
// no runtime source builtin, so the generated script content is baked into the
// rc file before the shell starts.
func ptyWriteElvishRC(t *testing.T, workspace *Workspace, scriptPath string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Clean(scriptPath))
	if err != nil {
		t.Fatalf("failed to read elvish completion script: %v", err)
	}

	rcDir := filepath.Join(workspace.HomeDir, ".config", "elvish")
	if err := os.MkdirAll(rcDir, 0o700); err != nil {
		t.Fatalf("failed to create elvish config dir: %v", err)
	}

	rc := string(content) +
		"\nset edit:prompt = { put '" + ptyPrompt + "' }\n" +
		"set edit:rprompt = { }\n"

	if err := os.WriteFile(filepath.Join(rcDir, "rc.elv"), []byte(rc), 0o600); err != nil {
		t.Fatalf("failed to write elvish rc.elv: %v", err)
	}
}
