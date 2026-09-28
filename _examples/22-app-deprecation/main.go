package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates whole-command deprecation: app.Deprecated emits a warning
// banner on every invocation of the binary, including its subcommands.
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy the application"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)
	fmt.Fprintln(session.Stdout(), "root command executed")

	return nil
}

// DeployCmd handles the deploy subcommand.
type DeployCmd struct {
	Force conga.OptionalFlag[bool] `help:"Force the deployment" short:"f"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())
	fmt.Fprintln(session.Stdout(), "deployment started")

	return nil
}

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("app-deprecated").
		Deprecated("use the new unified tool")

	app.FatalIfError(app.Run())
}
