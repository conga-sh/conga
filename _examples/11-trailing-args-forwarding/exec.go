package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// ExecCmd uses RawArgs (Docker run / Sudo pattern).
type ExecCmd struct {
	Container conga.RequiredArg[string] `help:"Target container" name:"<container>"`
	Command   conga.RawArgs
}

func (e *ExecCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	fmt.Fprintf(session.Stdout(), "RawArgs container: %v\n", e.Command.Get())

	return nil
}
