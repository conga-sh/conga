package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// TestCmd uses DashArgs (Go test / Cargo test pattern).
type TestCmd struct {
	Package    conga.RequiredArg[string] `help:"Package to test"                name:"<pkg>"`
	RunnerArgs conga.DashArgs            `help:"Arguments forwarded after '--'"`
}

func (t *TestCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	// DashArgs tokens are parsed into the container, so the session-level raw
	// tail (session.Args()) stays empty for this command.
	fmt.Fprintf(session.Stdout(), "DashArgs container: %v | session.Args(): %v\n", t.RunnerArgs.Get(), session.Args())

	return nil
}
