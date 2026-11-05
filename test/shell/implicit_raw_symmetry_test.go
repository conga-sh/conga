//go:build shelltest

package shell_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// The implicit raw-fallback tests pin that a flag declared only by an implicit
// leaf selects that leaf's RawArgs predictor exactly like the explicit
// "rawtool run" form: a dash word offers the forwarded candidates next to the
// flags, and an unknown flag switches completion into raw mode. Bash and zsh
// assert the real line-editor buffers; fish and powershell buffers are
// simulated by the harness, so those tests pin parity with the explicit form.

func TestBash_ImplicitRawFallback(t *testing.T) {
	t.Parallel()

	workspace := setupRawtool(t)

	t.Run("flag_selected_leaf_offers_forwarded_dash_candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "rawtool", "rawtool -n -")

		assert.Equal(t, []string{"--help", "--script-watch"}, result.Items)
		assert.Equal(t, "rawtool -n --", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("unknown_flag_enters_raw_mode", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "rawtool", "rawtool -n --unknown-flag -")

		assert.Equal(t, []string{"--script-watch"}, result.Items)
		assert.Equal(t, "rawtool -n --unknown-flag --script-watch", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestZsh_ImplicitRawFallback(t *testing.T) {
	t.Parallel()

	workspace := setupRawtool(t)

	t.Run("flag_selected_leaf_offers_forwarded_dash_candidates", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "rawtool", "rawtool -n -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.True(t, result.HasItem("--help"))
		assert.Equal(t, "rawtool -n --", result.BufferTab1)
	})

	t.Run("unknown_flag_enters_raw_mode", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "rawtool", "rawtool -n --unknown-flag -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.False(t, result.HasItem("--help"))
		assert.Equal(t, "rawtool -n --unknown-flag --script-watch ", result.BufferTab1)
	})
}

func TestFish_ImplicitRawFallback(t *testing.T) {
	t.Parallel()

	workspace := setupRawtool(t)

	t.Run("flag_selected_leaf_offers_forwarded_dash_candidates", func(t *testing.T) {
		t.Parallel()

		explicit := workspace.RunFishCompletion(t, "rawtool", "rawtool run -n -")
		result := workspace.RunFishCompletion(t, "rawtool", "rawtool -n -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.True(t, result.HasItem("--help"))
		assert.Equal(t, implicitFormBuffer(explicit.BufferTab1), result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("unknown_flag_enters_raw_mode", func(t *testing.T) {
		t.Parallel()

		explicit := workspace.RunFishCompletion(t, "rawtool", "rawtool run -n --unknown-flag -")
		result := workspace.RunFishCompletion(t, "rawtool", "rawtool -n --unknown-flag -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.False(t, result.HasItem("--help"))
		assert.Equal(t, implicitFormBuffer(explicit.BufferTab1), result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}

func TestPowerShell_ImplicitRawFallback(t *testing.T) {
	t.Parallel()

	workspace := setupRawtool(t)

	t.Run("flag_selected_leaf_offers_forwarded_dash_candidates", func(t *testing.T) {
		t.Parallel()

		explicit := workspace.RunPowerShellCompletion(t, "rawtool", "rawtool run -n -")
		result := workspace.RunPowerShellCompletion(t, "rawtool", "rawtool -n -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.True(t, result.HasItem("--help"))
		assert.Equal(t, implicitFormBuffer(explicit.BufferTab1), result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("unknown_flag_enters_raw_mode", func(t *testing.T) {
		t.Parallel()

		explicit := workspace.RunPowerShellCompletion(t, "rawtool", "rawtool run -n --unknown-flag -")
		result := workspace.RunPowerShellCompletion(t, "rawtool", "rawtool -n --unknown-flag -")

		assert.True(t, result.HasItem("--script-watch"))
		assert.False(t, result.HasItem("--help"))
		assert.Equal(t, implicitFormBuffer(explicit.BufferTab1), result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}

// implicitFormBuffer rewrites a buffer completed for the explicit
// "rawtool run" form into the buffer the implicit form must produce.
func implicitFormBuffer(explicitBuffer string) string {
	return strings.Replace(explicitBuffer, "rawtool run ", "rawtool ", 1)
}
