// Package system provides standard library system plugins for the CONGA CLI
// builder. Apply [User] and [Group] with App.Use to register *user.User and
// *user.Group lookup through os/user, and [Signal] to register syscall.Signal
// parsing from names such as "SIGTERM", "TERM", or "hup" and numeric strings.
package system

import (
	"fmt"
	"os/user"
	"syscall"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/signals"
	"github.com/conga-sh/conga/predict/system"
)

// Group registers *user.Group with os/user.LookupGroup and system group completion.
func Group[T any](app *conga.App[T]) {
	app.RegisterType(user.LookupGroup, conga.WithPredictor(system.Groups()))
}

// Signal registers syscall.Signal with POSIX signal parsing and signal completion.
func Signal[T any](app *conga.App[T]) {
	app.RegisterType(parseSignal, conga.WithPredictor(system.Signals()))
}

// User registers *user.User with os/user.Lookup and system user completion.
func User[T any](app *conga.App[T]) {
	app.RegisterType(user.Lookup, conga.WithPredictor(system.Users()))
}

// parseSignal parses a signal by name (e.g. "SIGTERM", "TERM", "hup") or numeric string ("15").
func parseSignal(raw string) (syscall.Signal, error) {
	sig, ok := signals.Lookup(raw)
	if !ok {
		return 0, fmt.Errorf("%w %q", signals.ErrInvalid, raw)
	}

	return sig, nil
}
