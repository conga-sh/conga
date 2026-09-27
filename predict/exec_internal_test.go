package predict

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestParsePathExts(t *testing.T) {
	t.Parallel()

	t.Run("empty returns default extensions", func(t *testing.T) {
		t.Parallel()

		extensions := parsePathExtensions("")
		assert.Equal(t, []string{".com", ".exe", ".bat", ".cmd"}, extensions)
	})

	t.Run("custom semicolon list with spaces and uppercase", func(t *testing.T) {
		t.Parallel()

		extensions := parsePathExtensions(".EXE;.BAT; .CMD ;.ps1;")
		assert.Equal(t, []string{".exe", ".bat", ".cmd", ".ps1"}, extensions)
	})
}
