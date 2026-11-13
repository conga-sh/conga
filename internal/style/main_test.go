package style_test

import (
	"os"
	"testing"
)

// lsColorsFixture exercises LS_COLORS precedence ("*.go", "*.TAR", "*.JSON",
// "*.log", "*.cmd"), SGR translation (attributes, colors, a zero-padded code,
// a 256-color entry, and a reset-only entry), key case-insensitivity, the
// directory, symlink, orphan, and executable indicators, and malformed entry
// skipping ("novalue", the empty entry, and "=orphan").
const lsColorsFixture = "*.go=01;36:*.TAR=01;31:novalue::=orphan:*.JSON=044:*.log=38;5;208:*.cmd=00:di=01;33:ln=35:or=41:ex=04;32"

func TestMain(m *testing.M) {
	os.Setenv("LS_COLORS", lsColorsFixture)
	os.Exit(m.Run())
}
