//go:build shelltest

package shell_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/bash"
	"github.com/conga-sh/conga/internal/completion/elvish"
	"github.com/conga-sh/conga/internal/completion/fish"
	"github.com/conga-sh/conga/internal/completion/nushell"
	"github.com/conga-sh/conga/internal/completion/powershell"
	"github.com/conga-sh/conga/internal/completion/zsh"
)

// snippetParseNames is the shared hostile command-name corpus for the
// per-shell snippet parse checks: every generated registration script must
// parse in its host shell, which the raw-name identifiers and unquoted
// targets previously violated (function names with spaces, bare `;`
// payloads).
var snippetParseNames = []string{"myapp", "my app", "app.exe", "./app", "app-1", "app;payload", `app"quote`, "café"} //nolint:gochecknoglobals // Shared hostile-name corpus for per-shell parse checks.

func lookupShellBin(t *testing.T, envVar, bin, skipMsg string) string {
	t.Helper()

	if envPath := os.Getenv(envVar); envPath != "" {
		return envPath
	}

	path, err := exec.LookPath(bin)
	if err != nil {
		t.Skip(skipMsg)
	}

	return path
}

func writeSnippetFile(t *testing.T, dir, fileName, content string) string {
	t.Helper()

	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write snippet file: %v", err)
	}

	return path
}

func TestBash_SnippetParses(t *testing.T) {
	t.Parallel()

	bashPath := lookupShellBin(t, "CONGA_BASH_BIN", "bash", "bash binary not found on PATH; skipping snippet parse check")

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.bash", bash.Snippet(name, nil, ""))

			out, err := exec.CommandContext(t.Context(), bashPath, "--noprofile", "--norc", "-n", snippetPath).CombinedOutput()

			assert.NoError(t, err, "bash -n must parse the generated snippet:\n%s", out)
		})
	}

	t.Run("aliases form", func(t *testing.T) {
		t.Parallel()

		snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.bash", bash.Snippet("my app", []string{"ma"}, ""))

		out, err := exec.CommandContext(t.Context(), bashPath, "--noprofile", "--norc", "-n", snippetPath).CombinedOutput()

		assert.NoError(t, err, "bash -n must parse the generated snippet:\n%s", out)
	})
}

func TestZsh_SnippetParses(t *testing.T) {
	t.Parallel()

	zshPath := lookupShellBin(t, "CONGA_ZSH_BIN", "zsh", "zsh binary not found on PATH; skipping snippet parse check")

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.zsh", zsh.Snippet(name, nil, "TEST_"))

			out, err := exec.CommandContext(t.Context(), zshPath, "-f", "-d", "-n", snippetPath).CombinedOutput()

			assert.NoError(t, err, "zsh -n must parse the generated snippet:\n%s", out)
		})
	}

	t.Run("aliases form", func(t *testing.T) {
		t.Parallel()

		snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.zsh", zsh.Snippet("my app", []string{"ma"}, "TEST_"))

		out, err := exec.CommandContext(t.Context(), zshPath, "-f", "-d", "-n", snippetPath).CombinedOutput()

		assert.NoError(t, err, "zsh -n must parse the generated snippet:\n%s", out)
	})
}

func TestFish_SnippetParses(t *testing.T) {
	t.Parallel()

	fishPath := lookupShellBin(t, "CONGA_FISH_BIN", "fish", "fish binary not found on PATH; skipping snippet parse check")

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.fish", fish.Snippet(name, nil, ""))

			out, err := exec.CommandContext(t.Context(), fishPath, "--no-execute", snippetPath).CombinedOutput()

			assert.NoError(t, err, "fish --no-execute must parse the generated snippet:\n%s", out)
		})
	}
}

func TestNushell_SnippetParses(t *testing.T) {
	t.Parallel()

	nuPath := lookupShellBin(t, "CONGA_NU_BIN", "nu", "nu binary not found on PATH; skipping snippet parse check")

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := exec.CommandContext(t.Context(), nuPath, "-c", nushell.Snippet(name, nil, "")).CombinedOutput()

			assert.NoError(t, err, "nu -c must parse the generated snippet:\n%s", out)
		})
	}
}

func TestPowerShell_SnippetParses(t *testing.T) {
	t.Parallel()

	pwshPath := lookupShellBin(t, "CONGA_PWSH_BIN", "pwsh", "pwsh binary not found on PATH; skipping snippet parse check")

	parseScript := func(snippetPath string) string {
		return fmt.Sprintf(`$tokens = $null; $errors = $null; [System.Management.Automation.Language.Parser]::ParseFile('%s', [ref]$tokens, [ref]$errors) | Out-Null; if ($errors.Count -gt 0) { $errors | ForEach-Object { Write-Error $_.Message }; exit 1 }`, snippetPath)
	}

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.ps1", powershell.Snippet(name, nil, ""))

			out, err := exec.CommandContext(t.Context(), pwshPath, "-NoProfile", "-NonInteractive", "-Command", parseScript(snippetPath)).CombinedOutput()

			assert.NoError(t, err, "PowerShell parser must accept the generated snippet:\n%s", out)
		})
	}

	t.Run("aliases form", func(t *testing.T) {
		t.Parallel()

		snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.ps1", powershell.Snippet("my app", []string{"ma"}, ""))

		out, err := exec.CommandContext(t.Context(), pwshPath, "-NoProfile", "-NonInteractive", "-Command", parseScript(snippetPath)).CombinedOutput()

		assert.NoError(t, err, "PowerShell parser must accept the generated aliases snippet:\n%s", out)
	})
}

func TestElvish_SnippetParses(t *testing.T) {
	t.Parallel()

	elvishPath := lookupShellBin(t, "CONGA_ELVISH_BIN", "elvish", "elvish binary not found on PATH; skipping snippet parse check")

	// The real edit: module is unavailable in non-interactive sessions, so the
	// probe mirrors the completion harness: mock the edit: namespace, eval the
	// snippet, and assert the arg-completer registration landed (counting the
	// keys also validates the quoted registration of hostile names).
	registrationScript := func(snippetPath string) string {
		return fmt.Sprintf(`
var edit: = (ns [
    &completion:=(ns [
        &arg-completer=[&]
        &complex-candidate~={|val &display="" &code-suffix=""| }
    ])
])
eval (slurp < %q)
print (to-string (keys $edit:completion:arg-completer | count)) "\n"
`, snippetPath)
	}

	for _, name := range snippetParseNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.elv", elvish.Snippet(name, nil, ""))

			out, err := exec.CommandContext(t.Context(), elvishPath, "-c", registrationScript(snippetPath)).CombinedOutput()

			assert.NoError(t, err, "elvish must accept the generated snippet:\n%s", out)
			assert.Equal(t, "1", strings.TrimSpace(string(out)), "snippet must register exactly one arg-completer")
		})
	}

	t.Run("aliases form", func(t *testing.T) {
		t.Parallel()

		snippetPath := writeSnippetFile(t, t.TempDir(), "snippet.elv", elvish.Snippet("my app", []string{"ma"}, ""))

		out, err := exec.CommandContext(t.Context(), elvishPath, "-c", registrationScript(snippetPath)).CombinedOutput()

		assert.NoError(t, err, "elvish must accept the generated aliases snippet:\n%s", out)
		assert.Equal(t, "2", strings.TrimSpace(string(out)), "aliases snippet must register one arg-completer per name")
	})
}
