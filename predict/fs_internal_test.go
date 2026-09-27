package predict

import (
	"runtime"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestSplitPathTarget(t *testing.T) {
	t.Parallel()

	dir, file := splitPathTarget("foo/bar")
	assert.Equal(t, "foo/", dir)
	assert.Equal(t, "bar", file)

	if runtime.GOOS == "windows" {
		dirWin, fileWin := splitPathTarget(`foo\bar`)
		assert.Equal(t, `foo\`, dirWin)
		assert.Equal(t, "bar", fileWin)
	} else {
		dirUnix, fileUnix := splitPathTarget(`foo\bar`)
		assert.Equal(t, "", dirUnix, "backslash must not be treated as path separator on Unix")
		assert.Equal(t, `foo\bar`, fileUnix)
	}
}
