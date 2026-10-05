package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI is a flat CLI that explicitly opts into the completion subcommand
// using conga.CompletionCommand (Cobra / gh style). When present, the --completion
// flag is suppressed from the help display in favor of the completion subcommand.
type CLI struct {
	DryRun     conga.OptionalFlag[bool]  `help:"Simulate execution without modifying files" short:"d"`
	Completion conga.CompletionCommand   `summary:"Generate shell completion script"`
	Task       conga.RequiredArg[string] `help:"Task name to execute"`
}

// Run executes the task runner command.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)
	fmt.Fprintf(session.Stdout(), "Running task %s (dry-run=%t)\n", c.Task.Get(), c.DryRun.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("runner").
		Description("Task runner (flat CLI with opt-in completion subcommand)").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
