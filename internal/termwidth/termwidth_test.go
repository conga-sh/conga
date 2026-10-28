package termwidth_test

import (
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/termwidth"
)

// TestResolve_EnvColumnsWins pins the COLUMNS override: an explicit value is
// honored even when no terminal is attached.
func TestResolve_EnvColumnsWins(t *testing.T) {
	t.Setenv("COLUMNS", "120")

	assert.Equal(t, 120, termwidth.Resolve(nil))
}

// TestResolve_EnvColumnsBelowMinimum verifies that a stray tiny value counts as
// unset instead of shredding help output.
func TestResolve_EnvColumnsBelowMinimum(t *testing.T) {
	t.Setenv("COLUMNS", "10")

	assert.Equal(t, termwidth.Default, termwidth.Resolve(nil))
}

// TestResolve_EnvColumnsInvalid verifies that an unparsable value falls through
// to the default width.
func TestResolve_EnvColumnsInvalid(t *testing.T) {
	t.Setenv("COLUMNS", "wide")

	assert.Equal(t, termwidth.Default, termwidth.Resolve(nil))
}

// TestResolve_PipeFallsBackToDefault verifies that a non-terminal stream
// reports no width, keeping piped output deterministic.
func TestResolve_PipeFallsBackToDefault(t *testing.T) {
	t.Setenv("COLUMNS", "")

	read, write, err := os.Pipe()
	assert.NoError(t, err)

	defer read.Close()
	defer write.Close()

	assert.Equal(t, termwidth.Default, termwidth.Resolve(read))
}
