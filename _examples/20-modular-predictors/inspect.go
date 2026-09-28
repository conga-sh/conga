package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/predict/system"
)

// InspectCmd inspects process metadata by PID or process name.
type InspectCmd struct {
	Process conga.OptionalFlag[system.Process] `help:"Process executable name" short:"p"`
	PID     conga.OptionalArg[system.PID]      `help:"Process ID to inspect"`
}

func (i *InspectCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	return nil
}
