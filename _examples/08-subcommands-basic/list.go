package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

type ListCmd struct {
	Namespace conga.OptionalFlag[string] `default:"default"                 help:"Target namespace" short:"n"`
	All       conga.OptionalFlag[bool]   `help:"List across all namespaces" short:"A"`
}

func (l *ListCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI](), l)

	return nil
}
