// Package termwidth reports the terminal width attached to a stream so help
// output can wrap to it. It stays free of dynamic package-level initializers,
// keeping the package linkable and inert for binaries that never render help.
package termwidth

import (
	"os"
	"strconv"
)

const (
	// Default is the wrap width used when no terminal width is discoverable:
	// piped output, redirected files, buffers, and terminals that do not
	// report a size.
	Default = 100

	// minColumns is the smallest terminal width honored; anything below counts
	// as unset so a stray tiny value cannot shred help output. It floors both
	// the COLUMNS override and the width probed from the terminal.
	minColumns = 20
)

// Resolve returns the width help output should wrap at: a COLUMNS environment
// value of at least minColumns wins, then the terminal width of file (nil or a
// non-terminal stream reports none), then Default.
func Resolve(file *os.File) int {
	if columns, ok := envColumns(); ok {
		return columns
	}

	if file != nil {
		if width := widthOf(file); width >= minColumns {
			return width
		}
	}

	return Default
}

// envColumns parses the COLUMNS environment override.
func envColumns() (int, bool) {
	columns, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || columns < minColumns {
		return 0, false
	}

	return columns, true
}
