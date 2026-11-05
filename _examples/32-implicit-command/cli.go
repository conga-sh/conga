package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

// CLI demonstrates an implicit command: implicit-command runs a file directly, exactly
// like the explicit "implicit-command run" form.
type CLI struct {
	Exec    conga.Command[RunCmd]     `implicit:"true"                name:"run" summary:"Execute a file"`
	Install conga.Command[InstallCmd] `summary:"Install dependencies"`
}

// Run prints help when implicit-command is invoked without an operand or subcommand.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprint(session.Stdout(), session.Help())

	return nil
}
