package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

// CLI is the root CLI implementing PreRunner, PostRunner, and Finalizer.
type CLI struct {
	Environment conga.OptionalFlag[string] `default:"staging"                                             help:"Environment" short:"e"`
	Migrate     conga.Command[MigrateCmd]  `summary:"Run database migrations"`
	Abort       conga.Command[AbortCmd]    `summary:"Simulate a failed execution to test Finally cleanup"`
	Deploy      conga.Command[DeployCmd]   `summary:"Simulate a PreRun failure to test early teardown"`
}

func (c *CLI) PreRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 1. Root.PreRun: Initializing global logger & telemetry")

	return nil
}

func (c *CLI) PostRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 4. Root.PostRun: Flushing telemetry & metrics")

	return nil
}

func (c *CLI) Finally(ctx context.Context, session *conga.Session, err error) error {
	selectedCmd := "none"
	if c.Migrate.IsSelected() {
		selectedCmd = "migrate"
	} else if c.Abort.IsSelected() {
		selectedCmd = "abort"
	} else if c.Deploy.IsSelected() {
		selectedCmd = "deploy"
	}

	if err != nil {
		fmt.Fprintf(session.Stdout(), "[HOOK] 5. Root.Finally: Global teardown executed for %q (handling error: %v)\n", selectedCmd, err)
	} else {
		fmt.Fprintf(session.Stdout(), "[HOOK] 5. Root.Finally: Global teardown completed successfully for %q\n", selectedCmd)
	}

	return nil
}
