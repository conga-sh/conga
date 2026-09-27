// Package appname derives the invoking binary's display name from os.Args[0].
package appname

import (
	"os"
	"path/filepath"
	"strings"
)

// Binary returns the base name of the invoking binary with any Windows ".exe"
// extension stripped. It reports false when no usable name can be derived:
// an empty os.Args slice, an empty argv[0], or a Go test runner binary (".test" suffix).
func Binary() (string, bool) {
	if len(os.Args) == 0 || os.Args[0] == "" {
		return "", false
	}

	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "" || strings.HasSuffix(name, ".test") {
		return "", false
	}

	return name, true
}
