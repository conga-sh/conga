package main

import (
	"context"
	"fmt"

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

	// RawArgs tokens are also unparsed at the session level: session.Args()
	// returns the same raw tail without requiring the typed container.
	fmt.Fprintf(session.Stdout(), "RawArgs container: %v | session.Args(): %v\n", e.Command.Get(), session.Args())

	return nil
}
