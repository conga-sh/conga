//go:build shelltest

package shell_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// An LS_COLORS entry styles a matching file or directory candidate in the
// shells that render candidate colors (zsh through list-colors, PowerShell
// through the list item text). The SGR codes are translated into the style
// vocabulary, so "01;35" renders as "1;35".
func TestLsColorsSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupShellCompletion(t)
	workspace.WriteFile(t, "app.yaml", []byte("name: app"))
	workspace.Mkdir(t, "configs")

	t.Run("zsh list-colors carry the LS_COLORS style", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "shell-completion", "shell-completion deploy --manifest ", "LS_COLORS=*.yaml=01;35")

		assert.True(t, result.HasItem("app.yaml"))
		assert.Contains(t, result.ListColors, "=(#b)(app.yaml)=0=1;35")
	})

	t.Run("powershell list item carries the LS_COLORS style", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "shell-completion", "shell-completion deploy --manifest ", "LS_COLORS=*.yaml=01;35")

		assert.True(t, result.HasItem("app.yaml"))

		var listItemText string

		for _, item := range result.Items {
			if strings.TrimSpace(item.CompletionText) == "app.yaml" {
				listItemText = item.ListItemText
			}
		}

		assert.Equal(t, "\x1b[1;35mapp.yaml\x1b[0m", listItemText)
	})
	t.Run("zsh list-colors carry the LS_COLORS directory style", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "shell-completion", "shell-completion deploy --manifest ", "LS_COLORS=di=01;33")

		assert.True(t, result.HasItem("configs/"))
		assert.Contains(t, result.ListColors, "=(#b)(configs/)=0=1;33")
	})

	t.Run("powershell list item carries the LS_COLORS directory style", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "shell-completion", "shell-completion deploy --manifest ", "LS_COLORS=di=01;33")

		assert.True(t, result.HasItem("configs/"))

		var listItemText string

		for _, item := range result.Items {
			if strings.TrimSpace(item.CompletionText) == "configs/" {
				listItemText = item.ListItemText
			}
		}

		assert.Equal(t, "\x1b[1;33mconfigs/\x1b[0m", listItemText)
	})
}
