package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

// RunCmd executes a file and forwards every remaining token.
type RunCmd struct {
	DryRun conga.OptionalFlag[bool] `help:"Print what would run" short:"n"`
	File   conga.RequiredArg[conga.File]
	Args   conga.RawArgs
}

func (c *RunCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "run file=%s dry-run=%t args=%v\n", c.File.Get(), c.DryRun.Get(), c.Args.Get())

	return nil
}
