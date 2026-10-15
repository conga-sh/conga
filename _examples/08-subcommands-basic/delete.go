package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type DeleteCmd struct {
	Name  conga.RequiredArg[string] `help:"Service name to delete"`
	Force conga.OptionalFlag[bool]  `help:"Force removal"          short:"f"`
}

func (d *DeleteCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), d)

	return nil
}
