package predict_test

import (
	"os"
	"testing"
)

// lsColorsFixture pins LS_COLORS for the package: the style package reads it
// once per process, so the developer's own value must not leak into style
// assertions. The "ex" entry must outrank the reset-only "*.exe" entry for
// executable files, as in ls, and "or" styles links whose target is missing.
// di and ln stay unset so the bold blue and cyan defaults hold.
const lsColorsFixture = "*.exe=00:ex=04;32:or=41"

func TestMain(m *testing.M) {
	os.Setenv("LS_COLORS", lsColorsFixture)
	os.Exit(m.Run())
}
