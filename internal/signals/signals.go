// Package signals provides the platform signal table shared by the signal
// completion predictors and the syscall.Signal type parser.
package signals

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

// ErrInvalid indicates an unrecognized signal name or number.
var ErrInvalid = errors.New("unknown or invalid signal")

// Entry describes a single known POSIX / system signal.
type Entry struct {
	Name        string
	ShortName   string
	Signal      syscall.Signal
	Description string
	Style       string
}

// Known returns the signal table for the current platform.
func Known() []Entry {
	return known()
}

// Lookup parses a signal by name (e.g. "SIGTERM", "TERM", "hup") or numeric
// string ("15"), reporting whether the input references a known signal.
func Lookup(raw string) (syscall.Signal, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	if num, err := strconv.Atoi(raw); err == nil && num > 0 {
		return syscall.Signal(num), true
	}

	upper := strings.ToUpper(raw)

	for _, sig := range known() {
		if strings.EqualFold(sig.Name, upper) || strings.EqualFold(sig.ShortName, upper) {
			return sig.Signal, true
		}
	}

	return 0, false
}
