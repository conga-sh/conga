//nolint:testpackage // white-box: exercises the unexported shell classifier and path helpers
package completion

import (
	"runtime"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestShellDescriptions(t *testing.T) {
	t.Parallel()

	infos := ShellDescriptions()

	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name
		assert.True(t, info.Description != "", "shell %q must carry a description", info.Name)
	}

	assert.Equal(t, SupportedShells(), names)
}

func TestDetectFromChain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		chain []string
		want  Shell
	}{
		{name: "direct bash", chain: []string{"bash"}, want: ShellBash},
		{name: "direct elvish", chain: []string{"elvish"}, want: ShellElvish},
		{name: "via xargs", chain: []string{"xargs", "zsh"}, want: ShellZsh},
		{name: "via wrapper then shell", chain: []string{"go", "zsh"}, want: ShellZsh},
		{name: "login shell marker", chain: []string{"-zsh"}, want: ShellZsh},
		{name: "brew bash path", chain: []string{"/opt/homebrew/bin/bash"}, want: ShellBash},
		{name: "pwsh alias", chain: []string{"pwsh"}, want: ShellPowerShell},
		{name: "unsupported chain", chain: []string{"go", "tmux"}, want: ""},
		{name: "empty chain", chain: nil, want: ""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, detectFromChain(test.chain))
		})
	}
}

func TestInferBashNeedsUpgrade(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		execPath  string
		wantOld   bool
		wantKnown bool
	}{
		{name: "bundled macos bash", execPath: "/bin/bash", wantOld: true, wantKnown: true},
		{name: "usr bin bash", execPath: "/usr/bin/bash", wantOld: true, wantKnown: true},
		{name: "login shell marker", execPath: "-bash", wantOld: true, wantKnown: true},
		{name: "brew arm bash", execPath: "/opt/homebrew/bin/bash", wantOld: false, wantKnown: true},
		{name: "brew cellar bash", execPath: "/opt/homebrew/Cellar/bash/5.2.0/bin/bash", wantOld: false, wantKnown: true},
		{name: "intel brew cellar bash", execPath: "/usr/local/Cellar/bash/5.2.0/bin/bash", wantOld: false, wantKnown: true},
		{name: "bare name unknown", execPath: "bash", wantOld: false, wantKnown: false},
		{name: "empty unknown", execPath: "", wantOld: false, wantKnown: false},
		{name: "non bash known", execPath: "/bin/fish", wantOld: false, wantKnown: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			old, known := InferBashNeedsUpgrade(test.execPath)

			assert.Equal(t, test.wantOld, old)
			assert.Equal(t, test.wantKnown, known)
		})
	}
}

func TestShouldWarnBash(t *testing.T) {
	t.Parallel()

	current := CurrentShell{Shell: ShellBash, ExecPath: "/bin/bash"}

	if runtime.GOOS == "darwin" {
		assert.True(t, ShouldWarnBash(current))
	} else {
		assert.False(t, ShouldWarnBash(current))
	}
}

func TestEnableInstructions_AllShells(t *testing.T) {
	t.Parallel()

	for _, info := range ShellDescriptions() {
		t.Run(info.Name, func(t *testing.T) {
			t.Parallel()

			out := EnableInstructions(Shell(info.Name), "myapp", EnableOptions{})

			assert.Contains(t, out, "To load completions in your current ")
			assert.Contains(t, out, "To load completions in every new session,")

			for _, line := range SessionCommands(Shell(info.Name), "myapp", false) {
				assert.Contains(t, out, line, "expected the session command %q in:\n%s", line, out)
			}
		})
	}
}

func TestEnableInstructions_NushellAutoloadFormat(t *testing.T) {
	t.Parallel()

	out := EnableInstructions(ShellNushell, "myapp", EnableOptions{})

	assert.Contains(t, out, `myapp completion nushell | save -f ($nu.default-config-dir | path join "autoload/myapp.nu")`)
	assert.Contains(t, out, `source ($nu.default-config-dir | path join "autoload/myapp.nu")`)
	assert.NotContains(t, out, ">>")
	assert.NotContains(t, out, "echo '")
}

func TestEnableInstructions_BashUpgradeNote(t *testing.T) {
	t.Parallel()

	t.Run("note rendered when bash needs upgrade", func(t *testing.T) {
		t.Parallel()

		out := EnableInstructions(ShellBash, "myapp", EnableOptions{BashNeedsUpgrade: true})

		assert.Contains(t, out, "Bash completion requires bash 4.4+")
	})

	t.Run("note omitted when bash is modern", func(t *testing.T) {
		t.Parallel()

		out := EnableInstructions(ShellBash, "myapp", EnableOptions{})

		assert.NotContains(t, out, "Bash completion requires bash 4.4+")
	})
}

func TestEnableInstructions_ZshHomebrewTarget(t *testing.T) {
	t.Parallel()

	t.Run("homebrew site-functions target", func(t *testing.T) {
		t.Parallel()

		out := EnableInstructions(ShellZsh, "myapp", EnableOptions{Homebrew: true})

		assert.Contains(t, out, "myapp completion zsh > \"$(brew --prefix)/share/zsh/site-functions/_myapp\"")
		assert.NotContains(t, out, "~/.zfunc")
	})

	t.Run("plain fpath target", func(t *testing.T) {
		t.Parallel()

		out := EnableInstructions(ShellZsh, "myapp", EnableOptions{})

		assert.Contains(t, out, "mkdir -p ~/.zfunc")
		assert.Contains(t, out, "myapp completion zsh > ~/.zfunc/_myapp")
		assert.Contains(t, out, "echo 'fpath=(~/.zfunc $fpath)' >> ~/.zshrc")
		assert.NotContains(t, out, "brew --prefix")
	})
}

func TestEnableInstructions_ZshPersistLines(t *testing.T) {
	t.Parallel()

	out := EnableInstructions(ShellZsh, "myapp", EnableOptions{})

	// The conditional compinit note may mention autoload once, but the quick
	// persistence step must recommend plain sourcing instead.
	assert.Contains(t, out, "autoload -U compinit; compinit")
	assert.Contains(t, out, "echo 'source <(myapp completion zsh)' >> ~/.zshrc")
}

func TestNowSection(t *testing.T) {
	t.Parallel()

	out := NowSection(CurrentShell{Shell: ShellZsh, ExecPath: "/bin/zsh"}, "myapp", false, true)

	assert.Contains(t, out, "Enable completions for your current zsh shell now:")
	assert.Contains(t, out, "\tsource <(myapp completion zsh)")
	assert.Contains(t, out, "For persistent setups, run 'myapp completion zsh --help'.")
}

func TestNowSection_HelpDisabled(t *testing.T) {
	t.Parallel()

	out := NowSection(CurrentShell{Shell: ShellZsh, ExecPath: "/bin/zsh"}, "myapp", false, false)

	assert.Contains(t, out, "Enable completions for your current zsh shell now:")
	assert.Contains(t, out, "\tsource <(myapp completion zsh)")
	assert.NotContains(t, out, "For persistent setups")
	assert.NotContains(t, out, "--help")
}

func TestSessionCommands(t *testing.T) {
	t.Parallel()

	for _, info := range ShellDescriptions() {
		t.Run(info.Name, func(t *testing.T) {
			t.Parallel()

			lines := SessionCommands(Shell(info.Name), "myapp", false)

			assert.True(t, len(lines) >= 1, "shell %q must expose a session command", info.Name)

			// The first line carries the invocation; follow-up lines (e.g.
			// nushell's temp-file source) are continuation commands.
			assert.True(t, strings.Contains(lines[0], "myapp completion "+info.Name), "expected invocation in %q", lines[0])
		})
	}
}

func TestDetectHomebrewPath(t *testing.T) {
	t.Parallel()

	assert.True(t, isHomebrewPath("/opt/homebrew/bin/myapp"))
	assert.True(t, isHomebrewPath("/opt/homebrew/Cellar/myapp/1.0/bin/myapp"))
	assert.True(t, isHomebrewPath("/usr/local/Cellar/myapp/1.0/bin/myapp"))
	assert.False(t, isHomebrewPath("/usr/local/bin/myapp"))
	assert.False(t, isHomebrewPath("/usr/bin/myapp"))
}

func detectFromChain(execPaths []string) Shell {
	for _, execPath := range execPaths {
		if shell := shellFromExecPath(execPath); shell != "" {
			return shell
		}
	}

	return ""
}
