package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// ExecCmd uses RawArgs (Docker run / Sudo pattern).
type ExecCmd struct {
	Container conga.RequiredArg[string] `help:"Target container"                     name:"<container>"`
	Command   conga.RawArgs             `help:"Raw command and arguments to forward"`
}

func (e *ExecCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	return nil
}
