package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type AbortCmd struct{}

func (a *AbortCmd) PreRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 2. AbortCmd.PreRun: Allocating temporary resources")

	return nil
}

func (a *AbortCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 3. AbortCmd.Run: Encountered fatal simulation error!")

	return errSimulatedFatal
}

func (a *AbortCmd) PostRun(ctx context.Context, session *conga.Session) error {
	// Should NOT execute on error
	fmt.Fprintln(session.Stdout(), "[HOOK] Should NOT be printed")

	return nil
}

func (a *AbortCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	fmt.Fprintf(session.Stdout(), "[HOOK] 4. AbortCmd.Finally: Rolling back temporary resources (error: %v)\n", err)

	return nil
}
