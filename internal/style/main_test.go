package style_test

import (
	"os"
	"testing"
)

// lsColorsFixture exercises LS_COLORS precedence ("*.go", "*.TAR", "*.JSON"),
// key case-insensitivity, and malformed entry skipping ("novalue", the empty
// entry, and "=orphan").
const lsColorsFixture = "*.go=01;36:*.TAR=01;31:novalue::=orphan:*.JSON=044"

func TestMain(m *testing.M) {
	os.Setenv("LS_COLORS", lsColorsFixture)
	os.Exit(m.Run())
}
