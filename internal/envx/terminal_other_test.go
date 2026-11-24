//go:build !linux

package envx_test

import (
	"os"
	"testing"
)

// openTerminal skips: only Linux has the pseudo-terminal helper.
func openTerminal(t *testing.T) *os.File {
	t.Helper()

	t.Skip("pseudo-terminal helper is Linux only")

	return nil
}
