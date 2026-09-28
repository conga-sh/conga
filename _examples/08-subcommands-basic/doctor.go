package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// DoctorCmd is a hidden housekeeping subcommand.
type DoctorCmd struct {
	Fix conga.OptionalFlag[bool] `help:"Apply repairs instead of only reporting them" short:"f"`
}

func (d *DoctorCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	return nil
}
