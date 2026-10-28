package help_test

import (
	"os"
	"testing"
)

// TestMain clears COLUMNS so golden screens render at the engine default width
// regardless of the invoking shell: an exported COLUMNS would otherwise wrap
// every fixture at the developer's terminal width.
func TestMain(m *testing.M) {
	os.Unsetenv("COLUMNS")

	os.Exit(m.Run())
}
