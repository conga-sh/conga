package scanx_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga/internal/scanx"
)

//go:embed testdata/input.txt
var fixtureInput []byte

func TestScanLines(t *testing.T) {
	t.Parallel()

	var lines []string

	err := scanx.ScanLines(bytes.NewReader(fixtureInput), func(line string) bool {
		lines = append(lines, line)

		return true
	})

	assert.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)

	t.Run("early exit", func(t *testing.T) {
		t.Parallel()

		var limited []string

		err := scanx.ScanLines(strings.NewReader("a\nb\nc"), func(line string) bool {
			limited = append(limited, line)

			return len(limited) < 2
		})

		assert.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, limited)
	})
}
