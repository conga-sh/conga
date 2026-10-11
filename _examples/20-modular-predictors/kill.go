package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/system"
)

// KillCmd sends a termination or custom signal to a target process or PID.
type KillCmd struct {
	Signal  conga.OptionalFlag[system.Signal]  `default:"SIGTERM"              help:"Signal to send (name or number)" short:"s"`
	Process conga.OptionalFlag[system.Process] `help:"Target process name"     short:"p"`
	Target  conga.OptionalArg[system.PID]      `help:"Target process ID (PID)"`
}

func (k *KillCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	targetPid := k.Target.Get()
	targetSignal := k.Signal.Get()
	stdout := session.Stdout()

	if targetPid.Int() > 0 {
		fmt.Fprintf(stdout, "Signaling PID %d with %s (signal %d)\n", targetPid.Int(), targetSignal, targetSignal.Signal())
	} else if k.Process.Get() != "" {
		fmt.Fprintf(stdout, "Signaling process %q with %s (signal %d)\n", k.Process.Get(), targetSignal, targetSignal.Signal())
	} else {
		fmt.Fprintf(stdout, "No target specified (signal %s)\n", targetSignal)
	}

	return nil
}
