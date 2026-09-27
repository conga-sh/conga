// Package scanx provides line-scanning primitives for whitespace-trimmed,
// comment-aware readers.
package scanx

import (
	"bufio"
	"io"
	"strings"
)

// ScanLines reads r line-by-line and calls processLine on each line after trimming whitespace.
// Blank lines and lines beginning with '#' are skipped automatically.
// Scanning terminates early if processLine returns false.
func ScanLines(r io.Reader, processLine func(line string) bool) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !processLine(line) {
			break
		}
	}

	return scanner.Err()
}
