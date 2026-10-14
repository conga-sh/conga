package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type CreateCmd struct {
	Name     conga.RequiredArg[string]  `help:"Service name"`
	Image    conga.OptionalFlag[string] `default:"nginx:latest" help:"Container image to deploy" short:"i"`
	Replicas conga.OptionalFlag[int]    `default:"1"            help:"Number of pod replicas"    short:"r"`
}

func (c *CreateCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), c)

	return nil
}
