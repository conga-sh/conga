package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type MigrateCmd struct {
	DryRun conga.OptionalFlag[bool] `help:"Simulate migration without applying" short:"d"`
}

func (m *MigrateCmd) PreRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 2. MigrateCmd.PreRun: Opening database connection pool")

	return nil
}

func (m *MigrateCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), m)
	fmt.Fprintf(session.Stdout(), "[HOOK] 3. MigrateCmd.Run: Executing migration (dry_run=%t)\n", m.DryRun.Get())

	return nil
}

func (m *MigrateCmd) PostRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 3.5. MigrateCmd.PostRun: Committing transaction log")

	return nil
}

func (m *MigrateCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] 4.5. MigrateCmd.Finally: Closing database connection pool")

	return nil
}
