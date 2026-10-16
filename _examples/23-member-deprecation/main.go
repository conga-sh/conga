package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates flag and subcommand deprecation: the deprecated:"..." tag
// emits provenance-aware warnings at runtime, marks help columns, and hides
// deprecated members from shell completion.
type CLI struct {
	Old    conga.OptionalFlag[string] `deprecated:"use --modern instead"`
	Modern conga.OptionalFlag[string]

	Legacy conga.Command[LegacyCmd] `deprecated:"use deploy instead" summary:"Legacy command"`
	Deploy conga.Command[DeployCmd] `summary:"Deploy command"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)
	fmt.Fprintf(session.Stdout(), "old=%q modern=%q\n", c.Old.Get(), c.Modern.Get())

	return nil
}

// LegacyCmd is a deprecated subcommand.
type LegacyCmd struct{}

func (l *LegacyCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), l)
	fmt.Fprintln(session.Stdout(), "legacy command executed")

	return nil
}

// DeployCmd is the replacement subcommand.
type DeployCmd struct{}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), d)
	fmt.Fprintln(session.Stdout(), "deploy command executed")

	return nil
}

func main() {
	app := conga.New[CLI]().Name("member-deprecated").EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
