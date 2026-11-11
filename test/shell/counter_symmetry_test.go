//go:build shelltest

package shell_test

import (
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// counterHelp is the help text of the flag-primitives --verbosity counter,
// which completion shows as a usage line for flags that take a value.
const counterHelp = "Increase verbosity level (-v, -vv, -vvv)"

// TestCounterCompletionSymmetry pins counter completion in every primary
// shell: a used counter is still offered, because counters repeat, and an
// attached value position offers nothing and leaves the buffer unchanged,
// because a counter takes no value.
func TestCounterCompletionSymmetry(t *testing.T) {
	t.Parallel()

	workspace := setupFlagPrimitives(t)

	t.Run("bash/used_counter_still_offered", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "flag-primitives", "flag-primitives -v --verb")

		// bash adds no trailing space after any completed flag name.
		assert.True(t, result.HasItem("--verbosity"))
		assert.Equal(t, "flag-primitives -v --verbosity", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("bash/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunBashCompletion(t, "flag-primitives", "flag-primitives --verbosity=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "flag-primitives --verbosity=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("zsh/used_counter_still_offered", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "flag-primitives", "flag-primitives -v --verb")

		assert.True(t, result.HasItem("--verbosity"))
		assert.Equal(t, "flag-primitives -v --verbosity ", result.BufferTab1)
	})

	t.Run("zsh/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunZshCompletion(t, "flag-primitives", "flag-primitives --verbosity=")

		assert.Equal(t, 0, len(result.RawItems))
		assert.False(t, slices.Contains(result.Messages, counterHelp), "a counter has no value to describe")
		assert.Equal(t, "flag-primitives --verbosity=", result.BufferTab1)
	})

	t.Run("fish/used_counter_still_offered", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "flag-primitives", "flag-primitives -v --verb")

		// fish adds no trailing space after any completed flag name.
		assert.True(t, result.HasItem("--verbosity"))
		assert.Equal(t, "flag-primitives -v --verbosity", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("fish/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunFishCompletion(t, "flag-primitives", "flag-primitives --verbosity=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "flag-primitives --verbosity=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell/used_counter_still_offered", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "flag-primitives", "flag-primitives -v --verb")

		assert.True(t, result.HasItem("--verbosity"))
		assert.Equal(t, "flag-primitives -v --verbosity ", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})

	t.Run("powershell/attached_value_offers_nothing", func(t *testing.T) {
		t.Parallel()

		result := workspace.RunPowerShellCompletion(t, "flag-primitives", "flag-primitives --verbosity=")

		assert.Equal(t, 0, len(result.Items))
		assert.Equal(t, "flag-primitives --verbosity=", result.BufferTab1)
		assert.Equal(t, "", result.StdoutLeak)
	})
}
