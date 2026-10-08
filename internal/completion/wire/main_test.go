package wire_test

import (
	"os"
	"testing"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// TestMain neutralizes the ambient colour environment so the package's tests
// observe the documented default (colour enabled unless CONGA_COLOR, NO_COLOR,
// or CLICOLOR disables it). Shells and CI harnesses commonly export NO_COLOR=1;
// without this, every colour-on assertion fails for reasons unrelated to the
// code under test.
func TestMain(m *testing.M) {
	os.Unsetenv(wire.EnvColor)
	os.Unsetenv(wire.EnvNoColor)
	os.Unsetenv(wire.EnvCLIColor)

	os.Exit(m.Run())
}
