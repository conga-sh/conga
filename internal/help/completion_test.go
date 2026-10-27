//nolint:testpackage // white-box: injects a deterministic shell detector into the unexported footer renderer
package help

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/spec"
)

func TestRenderCompletionFooter_Generic(t *testing.T) {
	t.Parallel()

	t.Run("empty command chain renders nothing", func(t *testing.T) {
		t.Parallel()

		formatted := RenderCompletionFooter(nil, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Equal(t, "", formatted)
	})

	t.Run("lists supported shells", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "myapp"}
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, leaf}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Contains(t, formatted, "Supported shells:")
		assert.True(t, len(completion.ShellDescriptions()) > 0, "shell registry must not be empty")

		for _, info := range completion.ShellDescriptions() {
			assert.Contains(t, formatted, info.Name)
			assert.Contains(t, formatted, info.Description)
		}
	})

	t.Run("static fallback pointer when shell undetected", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "myapp"}
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, leaf}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Contains(t, formatted, "To enable completions for your shell, run:")
		assert.Contains(t, formatted, "myapp completion <shell> --help")
	})

	t.Run("current shell now section when detected", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "myapp"}
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, leaf}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{Shell: completion.ShellZsh, ExecPath: "/bin/zsh"}, true
		})

		assert.Contains(t, formatted, "Enable completions for your current zsh shell now:")
		assert.Contains(t, formatted, "\tsource <(myapp completion zsh)")
		assert.Contains(t, formatted, "For persistent setups, run 'myapp completion zsh --help'.")
		assert.NotContains(t, formatted, "To enable completions for your shell, run:")
	})

	t.Run("suppresses help pointer when help flag disabled and shell undetected", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "myapp"}
		disabled := &spec.Command{Name: "completion", IsCompletionCmd: true, DisableHelp: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, disabled}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Contains(t, formatted, "Supported shells:")
		assert.NotContains(t, formatted, "To enable completions for your shell, run:")
		assert.NotContains(t, formatted, "--help")
	})

	t.Run("suppresses help pointer when help flag disabled and shell detected", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "myapp"}
		disabled := &spec.Command{Name: "completion", IsCompletionCmd: true, DisableHelp: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, disabled}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{Shell: completion.ShellZsh, ExecPath: "/bin/zsh"}, true
		})

		assert.Contains(t, formatted, "Enable completions for your current zsh shell now:")
		assert.Contains(t, formatted, "\tsource <(myapp completion zsh)")
		assert.NotContains(t, formatted, "For persistent setups")
		assert.NotContains(t, formatted, "--help")
	})
}

func TestEnableInstructionsSection_PerShell(t *testing.T) {
	t.Parallel()

	root := &spec.Command{Name: "myapp"}
	leaf := &spec.Command{Name: "completion", IsCompletionCmd: true, Shell: "zsh"}

	formatted := RenderCompletionInstructions(leaf.Shell, leaf.IsCompletionFlag, invokedName([]*spec.Command{root, leaf}))

	assert.Contains(t, formatted, "To load completions in your current shell session, run:")
	assert.Contains(t, strings.ReplaceAll(formatted, "\n", " "), "source <(myapp completion zsh)")
	assert.Contains(t, formatted, "echo 'source <(myapp completion zsh)' >> ~/.zshrc")
	assert.Contains(t, formatted, "mkdir -p ~/.zfunc")

	// The per-shell body renders enable instructions, not the shell list.
	assert.NotContains(t, formatted, "Supported shells:")
}

func TestFormat_CompletionDescriptionOverride(t *testing.T) {
	t.Parallel()

	root := &spec.Command{Name: "myapp"}
	leaf := &spec.Command{Name: "completion", Summary: "Generate shell completion scripts", IsCompletionCmd: true, Shell: "zsh"}

	formatted := Format([]*spec.Command{root, leaf})

	assert.Contains(t, formatted, "Generate the shell completion script for the zsh shell.")
	assert.NotContains(t, formatted, "Generate shell completion scripts\n")
}

func TestFormat_CompletionPerShellPinsUsageAndOmitsArgs(t *testing.T) {
	t.Parallel()

	root := &spec.Command{Name: "myapp"}
	leaf := &spec.Command{
		Name:            "completion",
		Summary:         "Generate shell completion scripts",
		IsCompletionCmd: true,
		Shell:           "zsh",
		Args:            []*spec.Arg{{Name: "<shell>", Help: "Target shell name"}},
	}

	formatted := Format([]*spec.Command{root, leaf})

	// The already-supplied shell is pinned in the usage synopsis and the
	// Arguments section is dropped as noise.
	assert.Contains(t, formatted, "Usage:\n  myapp completion zsh")
	assert.NotContains(t, formatted, "myapp completion <shell>")
	assert.NotContains(t, formatted, "Arguments:")
}

//nolint:paralleltest // Mutates global os.Args for invocation name testing.
func TestHelpArgv0Resolution(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"/usr/local/bin/custom-bin"}

	root := &spec.Command{Name: "default-app"}

	t.Run("generic completion footer uses argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, leaf}, false, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Contains(t, formatted, "custom-bin completion <shell> --help")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("flag-style generic completion footer uses argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true, IsCompletionFlag: true}

		formatted := RenderCompletionFooter([]*spec.Command{root, leaf}, true, func() (completion.CurrentShell, bool) {
			return completion.CurrentShell{}, false
		})

		assert.Contains(t, formatted, "custom-bin --completion <shell> --help")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("per-shell instructions use argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true, Shell: "zsh"}

		formatted := RenderCompletionInstructions(leaf.Shell, leaf.IsCompletionFlag, invokedName([]*spec.Command{root, leaf}))

		assert.Contains(t, formatted, "source <(custom-bin completion zsh)")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("flag-style per-shell instructions use argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true, IsCompletionFlag: true, Shell: "zsh"}

		formatted := RenderCompletionInstructions(leaf.Shell, leaf.IsCompletionFlag, invokedName([]*spec.Command{root, leaf}))

		assert.Contains(t, formatted, "source <(custom-bin --completion zsh)")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("format per-shell usage pins argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{
			Name:            "completion",
			Summary:         "Generate shell completion scripts",
			IsCompletionCmd: true,
			Shell:           "zsh",
			Args:            []*spec.Arg{{Name: "<shell>", Help: "Target shell name"}},
		}

		formatted := Format([]*spec.Command{root, leaf})

		assert.Contains(t, formatted, "Usage:\n  custom-bin completion zsh")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("format flag-style per-shell usage pins argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{
			Name:             "completion",
			Summary:          "Generate shell completion scripts",
			IsCompletionCmd:  true,
			IsCompletionFlag: true,
			Shell:            "zsh",
			Args:             []*spec.Arg{{Name: "<shell>", Help: "Target shell name"}},
		}

		formatted := Format([]*spec.Command{root, leaf})

		assert.Contains(t, formatted, "Usage:\n  custom-bin --completion zsh")
		assert.NotContains(t, formatted, "default-app")
	})

	t.Run("root command usage synopsis uses the configured command name", func(t *testing.T) {
		usage := Usage([]*spec.Command{root})

		assert.Contains(t, usage, "Usage:\n  default-app")
		assert.NotContains(t, usage, "custom-bin")
	})

	t.Run("deprecation banner uses the configured command name", func(t *testing.T) {
		depRoot := &spec.Command{Name: "default-app", Deprecated: "use new-cli instead"}

		formatted := Format([]*spec.Command{depRoot})

		assert.Contains(t, formatted, "WARNING: default-app is deprecated: use new-cli instead")
		assert.NotContains(t, formatted, "custom-bin")
	})

	t.Run("completion screen hint keeps the argv0 binary name", func(t *testing.T) {
		leaf := &spec.Command{Name: "completion", IsCompletionCmd: true}

		hint := Hint([]*spec.Command{root, leaf})

		assert.Contains(t, hint, "Run 'custom-bin completion --help'")
		assert.NotContains(t, hint, "default-app")
	})

	t.Run("non-completion screen hint uses the configured command name", func(t *testing.T) {
		leaf := &spec.Command{Name: "deploy"}

		hint := Hint([]*spec.Command{root, leaf})

		assert.Contains(t, hint, "Run 'default-app deploy --help'")
		assert.NotContains(t, hint, "custom-bin")
	})
}
