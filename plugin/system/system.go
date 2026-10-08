// Package system integrates POSIX signals and system user/group types into CONGA applications.
//
// Mount [Signal], [User], or [Group] using App.Use to register system types
// and automatic shell autocompletion:
//
//	type CLI struct {
//		Sig  conga.OptionalFlag[syscall.Signal] `short:"s" default:"SIGTERM" help:"Termination signal"`
//		User conga.OptionalFlag[*user.User]     `short:"u" help:"System user"`
//	}
//
//	app := conga.New[CLI]().Use(system.Signal, system.User)
//
// Signals accept POSIX names (e.g. "SIGHUP", "SIGINT", "TERM") or numeric values (e.g. "15").
// Users and groups resolve via os/user lookups. When shell completion is enabled,
// pressing Tab suggests matching signals, usernames, or group names.
//
// See https://docs.conga.sh/reference/plugins/ for the plugin reference.
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
