package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type DeployCmd struct{}

func (d *DeployCmd) PreRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 2. DeployCmd.PreRun: Pulling container image")

	return errRegistryUnreachable
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	// Should NOT execute when PreRun fails
	fmt.Fprintln(session.Stdout(), "[HOOK] DeployCmd.Run: Should NOT be printed")

	return nil
}

func (d *DeployCmd) PostRun(ctx context.Context, session *conga.Session) error {
	// Should NOT execute when PreRun fails
	fmt.Fprintln(session.Stdout(), "[HOOK] DeployCmd.PostRun: Should NOT be printed")

	return nil
}

func (d *DeployCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	// Should NOT execute: a command whose PreRun failed is never considered initialized
	fmt.Fprintln(session.Stdout(), "[HOOK] DeployCmd.Finally: Should NOT be printed")

	return nil
}
