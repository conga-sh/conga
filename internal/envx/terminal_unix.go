//go:build !windows

package envx

import (
	"io"
	"os"

	"github.com/conga-sh/conga/internal/termwidth"
)

// isTerminal reports whether writer is a terminal, by the TIOCGWINSZ ioctl.
// /dev/null and other character devices that are not terminals do not count.
func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	return termwidth.IsTerminal(file)
}

// enableEscapeCodes does nothing: terminals other than the Windows console
// interpret escape codes already.
func enableEscapeCodes(_ io.Writer) {}
