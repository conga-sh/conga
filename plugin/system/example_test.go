package system_test

import (
	"context"
	"fmt"
	"syscall"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/plugin/system"
)

// CLI is an application whose flag accepts a POSIX signal.
type CLI struct {
	Signal conga.OptionalFlag[syscall.Signal] `help:"Termination signal"`
}

func (c *CLI) Run(_ context.Context, session *conga.Session) error {
	_, err := fmt.Fprintln(session.Stdout(), int(c.Signal.Get()))

	return err
}

// Signal mounts syscall.Signal support: the flag accepts a POSIX signal name or
// number and completion offers the platform's signal table.
func ExampleSignal() {
	app := conga.New[CLI]().
		Use(system.Signal).
		Args([]string{"--signal", "SIGTERM"})

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	// Output: 15
}
