//go:build shelltest

package shell_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/test/shell"
)

// The PowerShell snippet resolves each word's quoting with PowerShell's own
// parser, so a word typed or inserted with doubled quotes (ASCII or Unicode)
// reaches the engine unquoted, under both argument-passing modes.
func TestPowerShell_QuotedWordsResolve(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"Standard", "Legacy"} {
		setup := "$PSNativeCommandArgumentPassing = '" + mode + "'"

		t.Run(mode+" closed doubled ASCII quote completes again", func(t *testing.T) {
			t.Parallel()

			workspace := setupWordtool(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "wordtool", "wordtool --env 'it''s'", setup)

			assert.Equal(t, "", result.Stderr)
			assert.Equal(t, []string{"'it''s' "}, powerShellCompletionTexts(result.Items))
			assert.Equal(t, "wordtool --env 'it''s' ", result.BufferTab1)
		})

		t.Run(mode+" open doubled Unicode quote completes", func(t *testing.T) {
			t.Parallel()

			workspace := setupWordtool(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "wordtool", "wordtool --env 'it’’s", setup)

			assert.Equal(t, "", result.Stderr)
			assert.Equal(t, []string{"'it’’s' "}, powerShellCompletionTexts(result.Items))
			assert.Equal(t, "wordtool --env 'it’’s' ", result.BufferTab1)
		})

		t.Run(mode+" open Unicode quote completes", func(t *testing.T) {
			t.Parallel()

			workspace := setupWordtool(t)

			result := workspace.RunPowerShellCompletionWithSetup(t, "wordtool", "wordtool --env ‘it", setup)

			assert.Equal(t, "", result.Stderr)
			assert.Equal(t, []string{"'it''s' ", "'it’’s' "}, powerShellCompletionTexts(result.Items))
		})
	}
}

// powerShellCompletionTexts returns the CompletionText of each item, in order.
func powerShellCompletionTexts(items []shell.PowerShellItem) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.CompletionText)
	}

	return texts
}
