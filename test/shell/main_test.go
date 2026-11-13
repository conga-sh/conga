//go:build shelltest

package shell_test

import (
	"os"
	"testing"

	"github.com/conga-sh/conga/test/shell"
)

func TestMain(m *testing.M) {
	os.Exit(shell.RunTests(m))
}
