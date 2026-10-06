package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// TestCmd uses DashArgs (Go test / Cargo test pattern).
type TestCmd struct {
	Package    conga.RequiredArg[string] `help:"Package to test" name:"<pkg>"`
	RunnerArgs conga.DashArgs
}

func (t *TestCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	fmt.Fprintf(session.Stdout(), "DashArgs container: %v\n", t.RunnerArgs.Get())

	return nil
}
